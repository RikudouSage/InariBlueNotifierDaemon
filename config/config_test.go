package config

import (
	"InariBlueNotifier/patterns"
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

func TestNewLeavesSMSPatternUnset(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := New()
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	if cfg.SMSPattern != nil {
		t.Fatalf("SMS pattern = %#v, want nil so it inherits the notification pattern", cfg.SMSPattern)
	}
}

func TestSaveRemovesUnsetSMSPattern(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := New()
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	pattern := patterns.RapidBlink
	cfg.SMSPattern = &pattern
	if err := cfg.Save(); err != nil {
		t.Fatalf("save SMS pattern: %v", err)
	}

	cfg.SMSPattern = nil
	if err := cfg.Save(); err != nil {
		t.Fatalf("save without SMS pattern: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(configHome, "dev.rikudou", "inari-blue-notifier", "config_v3.ini"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if strings.Contains(string(contents), "sms =") {
		t.Fatalf("saved config retained an unset SMS pattern:\n%s", contents)
	}
}
