package knock

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/patterns"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

type recordingEvents struct {
	events chan bool
}

func (receiver *recordingEvents) SetKnockActive(active bool) error {
	receiver.events <- active
	return nil
}

func expectEvent(t *testing.T, events <-chan bool, want bool, timeout time.Duration) {
	t.Helper()
	select {
	case got := <-events:
		if got != want {
			t.Fatalf("knock event = %t, want %t", got, want)
		}
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for knock event %t", want)
	}
}

func connectTCP(t *testing.T, address net.Addr) {
	t.Helper()
	connection, err := net.DialTimeout("tcp", address.String(), time.Second)
	if err != nil {
		t.Fatalf("send TCP knock: %v", err)
	}
	_ = connection.Close()
}

func sendUDP(t *testing.T, address net.Addr) {
	t.Helper()
	connection, err := net.Dial("udp", address.String())
	if err != nil {
		t.Fatalf("connect UDP: %v", err)
	}
	defer connection.Close()
	if _, err := connection.Write([]byte("knock")); err != nil {
		t.Fatalf("send UDP knock: %v", err)
	}
}

func TestKnocksActivateOnceAndExtendTimeout(t *testing.T) {
	recorder := &recordingEvents{events: make(chan bool, 10)}
	cfg := &config.Config{Knock: config.Knock{
		Enabled: true, Pattern: patterns.RapidBlink,
		Addresses: []string{"127.0.0.1"}, Protocols: []string{"tcp", "udp"},
		Timeout: 600 * time.Millisecond,
	}}
	receiver := New(cfg, recorder, nil, config.NewNilLogger()).(*knock)
	if err := receiver.StartListening(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = receiver.StopListening() })

	connectTCP(t, receiver.listeners[0].Addr())
	expectEvent(t, recorder.events, true, time.Second)
	time.Sleep(350 * time.Millisecond)
	sendUDP(t, receiver.packets[0].LocalAddr())
	select {
	case event := <-recorder.events:
		t.Fatalf("unexpected event %t while knock should remain active", event)
	case <-time.After(350 * time.Millisecond):
	}
	expectEvent(t, recorder.events, false, time.Second)
}

func TestStopClearsActiveKnockAndAllowsRestart(t *testing.T) {
	recorder := &recordingEvents{events: make(chan bool, 10)}
	cfg := &config.Config{Knock: config.Knock{
		Enabled: true, Pattern: patterns.RapidBlink,
		Addresses: []string{"127.0.0.1"}, Protocols: []string{"tcp"},
	}}
	receiver := New(cfg, recorder, nil, config.NewNilLogger()).(*knock)
	if err := receiver.StartListening(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = receiver.StopListening() })
	connectTCP(t, receiver.listeners[0].Addr())
	expectEvent(t, recorder.events, true, time.Second)
	if err := receiver.StopListening(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	expectEvent(t, recorder.events, false, time.Second)
	if err := receiver.StartListening(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	connectTCP(t, receiver.listeners[0].Addr())
	expectEvent(t, recorder.events, true, time.Second)
}

func TestClearActiveKeepsListenerAndCancelsOldTimeout(t *testing.T) {
	recorder := &recordingEvents{events: make(chan bool, 10)}
	cfg := &config.Config{Knock: config.Knock{
		Enabled: true, Pattern: patterns.RapidBlink,
		Addresses: []string{"127.0.0.1"}, Protocols: []string{"tcp"},
		Timeout: 400 * time.Millisecond,
	}}
	receiver := New(cfg, recorder, nil, config.NewNilLogger()).(*knock)
	if err := receiver.StartListening(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = receiver.StopListening() })
	connectTCP(t, receiver.listeners[0].Addr())
	expectEvent(t, recorder.events, true, time.Second)
	time.Sleep(250 * time.Millisecond)
	receiver.ClearActive()
	expectEvent(t, recorder.events, false, time.Second)
	connectTCP(t, receiver.listeners[0].Addr())
	expectEvent(t, recorder.events, true, time.Second)
	select {
	case event := <-recorder.events:
		t.Fatalf("old timeout cleared the new knock: %t", event)
	case <-time.After(250 * time.Millisecond):
	}
	expectEvent(t, recorder.events, false, time.Second)
}

func TestInvalidBindAddressDoesNotOpenAnyListeners(t *testing.T) {
	recorder := &recordingEvents{events: make(chan bool, 10)}
	cfg := &config.Config{Knock: config.Knock{
		Enabled: true, Pattern: patterns.RapidBlink,
		Addresses: []string{"127.0.0.1", "invalid-address"}, Protocols: []string{"tcp"},
	}}
	receiver := New(cfg, recorder, nil, config.NewNilLogger()).(*knock)
	if err := receiver.StartListening(); err == nil || !strings.Contains(err.Error(), "invalid-address") {
		t.Fatalf("start error = %v, want invalid bind address", err)
	}
	if len(receiver.listeners) != 0 || len(receiver.packets) != 0 {
		t.Fatal("listeners opened for invalid configuration")
	}
}

type failingListener struct{}

func (*failingListener) Accept() (net.Conn, error) { return nil, errors.New("accept failed") }
func (*failingListener) Close() error              { return nil }
func (*failingListener) Addr() net.Addr            { return &net.TCPAddr{} }

func TestListenerFailureIsReported(t *testing.T) {
	failures := make(chan error, 1)
	receiver := New(nil, nil, func(err error) { failures <- err }, config.NewNilLogger()).(*knock)
	receiver.listenTCP(make(chan struct{}), make(chan struct{}), &failingListener{})
	select {
	case err := <-failures:
		if !strings.Contains(err.Error(), "accept failed") {
			t.Fatalf("failure = %v, want accept error", err)
		}
	default:
		t.Fatal("listener failure was not reported")
	}
}
