package config

import (
	"InariBlueNotifier/patterns"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/ini.v1"
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

func TestNewMigratesConfigWithoutSMSSettings(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	if _, err := New(); err != nil {
		t.Fatalf("create initial config: %v", err)
	}
	configPath := filepath.Join(configHome, "dev.rikudou", "inari-blue-notifier", "config_v3.ini")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read initial config: %v", err)
	}
	// Simulate a config saved by a version from before the SMS event existed.
	if err := os.WriteFile(configPath, []byte(strings.Replace(string(contents), "sms = false\n", "", 1)), 0600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	cfg, err := New()
	if err != nil {
		t.Fatalf("load legacy config: %v", err)
	}
	if cfg.SMSPattern != nil {
		t.Fatalf("legacy SMS pattern = %#v, want nil", cfg.SMSPattern)
	}
	if cfg.Events[EventSMS] {
		t.Fatal("legacy config enabled the SMS event")
	}

	migrated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	migratedINI, err := ini.Load(migrated)
	if err != nil {
		t.Fatalf("parse migrated config: %v", err)
	}
	enabled, err := migratedINI.Section(sectionEvents).Key(patternKeySMS).Bool()
	if err != nil || enabled {
		t.Fatalf("migrated config does not add the disabled SMS event: enabled=%t err=%v", enabled, err)
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
