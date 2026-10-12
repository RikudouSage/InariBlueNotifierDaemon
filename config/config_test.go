package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDefaultsNotificationEventToDisabled(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := New()
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	if cfg.Events[EventNotification] {
		t.Fatal("notification event is enabled by default")
	}

	contents, err := os.ReadFile(filepath.Join(configHome, "dev.rikudou", "inari-blue-notifier", "config_v3.ini"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(contents), "notification = false") {
		t.Fatalf("saved config does not disable notification event:\n%s", contents)
	}
}
