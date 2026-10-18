package main

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/patterns"
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type testLEDController struct{}

func (*testLEDController) PlayPattern(patterns.Pattern) error { return nil }
func (*testLEDController) Stop() error                        { return nil }
func (*testLEDController) Release() error                     { return nil }
func (*testLEDController) Shutdown() error                    { return nil }
func (*testLEDController) Done() <-chan struct{}              { return nil }

func testSessionBus(t *testing.T) *dbus.Conn {
	t.Helper()
	daemonPath, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon is not available")
	}
	daemon := exec.Command(daemonPath, "--session", "--nofork", "--print-address=1")
	stdout, err := daemon.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := daemon.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = daemon.Process.Kill()
		_ = daemon.Wait()
	})
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatalf("read D-Bus address: %v", scanner.Err())
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", scanner.Text())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func TestStartKeepsOtherServicesRunningWhenKnockCannotBind(t *testing.T) {
	bus := testSessionBus(t)
	client, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	signals := make(chan *dbus.Signal, 5)
	client.Signal(signals)
	if err := client.AddMatchSignal(dbus.WithMatchInterface("dev.rikudou.InariBlueNotifier"), dbus.WithMatchMember("KnockStatusChanged")); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = blocked.Close() })
	cfg.Knock.Enabled = true
	cfg.Knock.Protocols = []string{"tcp"}
	cfg.Knock.Addresses = []string{"127.0.0.1"}
	cfg.Knock.Port = uint16(blocked.Addr().(*net.TCPAddr).Port)

	cleanup, err := start(&testLEDController{}, bus, bus, cfg)
	if err != nil {
		t.Fatalf("start services with occupied knock port: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	var owned bool
	err = bus.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "dev.rikudou.InariBlueNotifier").Store(&owned)
	if err != nil || !owned {
		t.Fatalf("external controller not registered: owned=%t err=%v", owned, err)
	}
	status, active, reason := queryKnockStatus(t, bus)
	if status != "failed" || active || !strings.Contains(reason, "listen for TCP knocks") {
		t.Fatalf("knock status = %q active=%t reason=%q, want failed bind", status, active, reason)
	}
	signal := receiveKnockSignal(t, signals)
	if len(signal.Body) != 3 || signal.Body[0] != status || signal.Body[1] != active || signal.Body[2] != reason {
		t.Fatalf("failure signal = %#v, want current status", signal)
	}
}

func queryKnockStatus(t *testing.T, bus *dbus.Conn) (string, bool, string) {
	t.Helper()
	object := bus.Object("dev.rikudou.InariBlueNotifier", "/dev/rikudou/InariBlueNotifier")
	var status, reason string
	var active bool
	if err := object.Call("dev.rikudou.InariBlueNotifier.GetKnockStatus", 0).Store(&status, &active, &reason); err != nil {
		t.Fatalf("query knock status: %v", err)
	}
	return status, active, reason
}

func receiveKnockSignal(t *testing.T, signals <-chan *dbus.Signal) *dbus.Signal {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case signal := <-signals:
			if signal.Name == "dev.rikudou.InariBlueNotifier.KnockStatusChanged" {
				return signal
			}
		case <-timer.C:
			t.Fatal("timed out waiting for a knock status signal")
		}
	}
}

func waitKnockSignal(t *testing.T, signals <-chan *dbus.Signal, status string, active bool) {
	t.Helper()
	signal := receiveKnockSignal(t, signals)
	if len(signal.Body) != 3 || signal.Body[0] != status || signal.Body[1] != active || signal.Body[2] != "" {
		t.Fatalf("knock signal = %#v, want %s active=%t with no error", signal, status, active)
	}
}

func TestKnockStatusDBusSignals(t *testing.T) {
	bus := testSessionBus(t)
	client, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	signals := make(chan *dbus.Signal, 10)
	client.Signal(signals)
	if err := client.AddMatchSignal(
		dbus.WithMatchObjectPath("/dev/rikudou/InariBlueNotifier"),
		dbus.WithMatchInterface("dev.rikudou.InariBlueNotifier"),
		dbus.WithMatchMember("KnockStatusChanged"),
	); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.Addr().String()
	cfg.Knock.Port = uint16(probe.Addr().(*net.TCPAddr).Port)
	_ = probe.Close()
	cfg.Knock.Enabled = true
	cfg.Knock.Protocols = []string{"tcp"}
	cfg.Knock.Addresses = []string{"127.0.0.1"}
	cfg.Knock.Timeout = 500 * time.Millisecond

	cleanup, err := start(&testLEDController{}, bus, bus, cfg)
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			_ = cleanup()
		}
	})
	waitKnockSignal(t, signals, "listening", false)
	status, active, reason := queryKnockStatus(t, client)
	if status != "listening" || active || reason != "" {
		t.Fatalf("initial status = %q active=%t reason=%q", status, active, reason)
	}
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	waitKnockSignal(t, signals, "listening", true)
	status, active, reason = queryKnockStatus(t, client)
	if status != "listening" || !active || reason != "" {
		t.Fatalf("active status = %q active=%t reason=%q", status, active, reason)
	}
	waitKnockSignal(t, signals, "listening", false)
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	stopped = true
	waitKnockSignal(t, signals, "stopped", false)
}

func TestUnlockClearsActiveKnockWhenConfigured(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("stop-on-unlock-%t", enabled), func(t *testing.T) {
			bus := testSessionBus(t)
			mce, err := dbus.ConnectSessionBus()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = mce.Close() })
			if reply, err := mce.RequestName("com.nokia.mce", dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
				t.Fatalf("claim MCE service: reply=%v err=%v", reply, err)
			}
			client, err := dbus.ConnectSessionBus()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			signals := make(chan *dbus.Signal, 10)
			client.Signal(signals)
			if err := client.AddMatchSignal(dbus.WithMatchInterface("dev.rikudou.InariBlueNotifier"), dbus.WithMatchMember("KnockStatusChanged")); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.New()
			if err != nil {
				t.Fatal(err)
			}
			probe, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			address := probe.Addr().String()
			cfg.Knock.Port = uint16(probe.Addr().(*net.TCPAddr).Port)
			_ = probe.Close()
			cfg.Knock.Enabled = true
			cfg.Knock.StopOnUnlock = enabled
			cfg.Knock.Protocols = []string{"tcp"}
			cfg.Knock.Addresses = []string{"127.0.0.1"}
			cfg.Knock.Timeout = 0
			cleanup, err := start(&testLEDController{}, bus, bus, cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cleanup() })
			waitKnockSignal(t, signals, "listening", false)
			connection, err := net.DialTimeout("tcp", address, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			_ = connection.Close()
			waitKnockSignal(t, signals, "listening", true)
			if err := mce.Emit("/com/nokia/mce/signal", "com.nokia.mce.signal.display_status_ind", "on"); err != nil {
				t.Fatal(err)
			}
			if err := mce.Emit("/com/nokia/mce/signal", "com.nokia.mce.signal.tklock_mode_ind", "locked"); err != nil {
				t.Fatal(err)
			}
			select {
			case signal := <-signals:
				t.Fatalf("display or lock signal changed knock status: %#v", signal)
			case <-time.After(100 * time.Millisecond):
			}
			if err := mce.Emit("/com/nokia/mce/signal", "com.nokia.mce.signal.tklock_mode_ind", "unlocked"); err != nil {
				t.Fatal(err)
			}
			if enabled {
				waitKnockSignal(t, signals, "listening", false)
				connection, err := net.DialTimeout("tcp", address, time.Second)
				if err != nil {
					t.Fatal(err)
				}
				_ = connection.Close()
				waitKnockSignal(t, signals, "listening", true)
			} else {
				select {
				case signal := <-signals:
					t.Fatalf("unlock changed knock status with option disabled: %#v", signal)
				case <-time.After(100 * time.Millisecond):
				}
			}
		})
	}
}
