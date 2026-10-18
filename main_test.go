package main

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/patterns"
	"bufio"
	"net"
	"os/exec"
	"testing"

	"github.com/godbus/dbus/v5"
)

type testLEDController struct{}

func (*testLEDController) PlayPattern(patterns.Pattern) error { return nil }
func (*testLEDController) Stop() error                        { return nil }
func (*testLEDController) Release() error                     { return nil }
func (*testLEDController) Shutdown() error                    { return nil }
func (*testLEDController) Done() <-chan struct{}              { return nil }

func TestStartKeepsOtherServicesRunningWhenKnockCannotBind(t *testing.T) {
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

	cleanup, err := start(&testLEDController{}, bus, cfg)
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
}
