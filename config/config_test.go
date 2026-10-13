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

func TestNewLeavesEmailPatternUnsetAndEventDisabled(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := New()
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	if cfg.EmailPattern != nil {
		t.Fatalf("email pattern = %#v, want nil so it inherits the notification pattern", cfg.EmailPattern)
	}
	if cfg.Events[EventEmail] {
		t.Fatal("email event is enabled by default")
	}

	contents, err := os.ReadFile(filepath.Join(configHome, "dev.rikudou", "inari-blue-notifier", "config_v3.ini"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	saved, err := ini.Load(contents)
	if err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	enabled, err := saved.Section(sectionEvents).Key(patternKeyEmail).Bool()
	if err != nil || enabled {
		t.Fatalf("saved config does not disable email event: enabled=%t err=%v", enabled, err)
	}
}

func TestNewLeavesSignalPatternUnsetAndEventDisabled(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := New()
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	if cfg.SignalPattern != nil {
		t.Fatalf("Signal pattern = %#v, want nil so it inherits the notification pattern", cfg.SignalPattern)
	}
	if cfg.Events[EventSignal] {
		t.Fatal("Signal event is enabled by default")
	}

	contents, err := os.ReadFile(filepath.Join(configHome, "dev.rikudou", "inari-blue-notifier", "config_v3.ini"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	saved, err := ini.Load(contents)
	if err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	enabled, err := saved.Section(sectionEvents).Key(patternKeySignal).Bool()
	if err != nil || enabled {
		t.Fatalf("saved config does not disable Signal event: enabled=%t err=%v", enabled, err)
	}
}

func TestNewLeavesFavoriteCallPatternUnset(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := New()
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	if cfg.FavoriteCallPattern != nil {
		t.Fatalf("favorite call pattern = %#v, want nil so it inherits the call pattern", cfg.FavoriteCallPattern)
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
	saved, err := ini.Load(contents)
	if err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if saved.Section(sectionActivePatterns).HasKey(patternKeySMS) {
		t.Fatalf("saved config retained an unset SMS pattern:\n%s", contents)
	}
}

func TestSaveAndLoadEmailSettings(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := New()
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	pattern := patterns.RapidBlink
	cfg.EmailPattern = &pattern
	cfg.Events[EventEmail] = true
	if err := cfg.Save(); err != nil {
		t.Fatalf("save email settings: %v", err)
	}

	loaded, err := New()
	if err != nil {
		t.Fatalf("load email settings: %v", err)
	}
	if loaded.EmailPattern == nil || loaded.EmailPattern.Name != pattern.Name {
		t.Fatalf("loaded email pattern = %#v, want %q", loaded.EmailPattern, pattern.Name)
	}
	if !loaded.Events[EventEmail] {
		t.Fatal("loaded email event is disabled")
	}
}

func TestSaveRemovesUnsetEmailPattern(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := New()
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	pattern := patterns.RapidBlink
	cfg.EmailPattern = &pattern
	if err := cfg.Save(); err != nil {
		t.Fatalf("save email pattern: %v", err)
	}

	cfg.EmailPattern = nil
	if err := cfg.Save(); err != nil {
		t.Fatalf("save without email pattern: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(configHome, "dev.rikudou", "inari-blue-notifier", "config_v3.ini"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	saved, err := ini.Load(contents)
	if err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if saved.Section(sectionActivePatterns).HasKey(patternKeyEmail) {
		t.Fatalf("saved config retained an unset email pattern:\n%s", contents)
	}
}

func TestSaveAndLoadSignalSettings(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := New()
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	pattern := patterns.RapidBlink
	cfg.SignalPattern = &pattern
	cfg.Events[EventSignal] = true
	if err := cfg.Save(); err != nil {
		t.Fatalf("save Signal settings: %v", err)
	}

	loaded, err := New()
	if err != nil {
		t.Fatalf("load Signal settings: %v", err)
	}
	if loaded.SignalPattern == nil || loaded.SignalPattern.Name != pattern.Name {
		t.Fatalf("loaded Signal pattern = %#v, want %q", loaded.SignalPattern, pattern.Name)
	}
	if !loaded.Events[EventSignal] {
		t.Fatal("loaded Signal event is disabled")
	}
}

func TestSaveRemovesUnsetFavoriteCallPattern(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg, err := New()
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	pattern := patterns.RapidBlink
	cfg.FavoriteCallPattern = &pattern
	if err := cfg.Save(); err != nil {
		t.Fatalf("save favorite call pattern: %v", err)
	}

	cfg.FavoriteCallPattern = nil
	if err := cfg.Save(); err != nil {
		t.Fatalf("save without favorite call pattern: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(configHome, "dev.rikudou", "inari-blue-notifier", "config_v3.ini"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	saved, err := ini.Load(contents)
	if err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if saved.Section(sectionActivePatterns).HasKey(patternKeyFavoriteCall) {
		t.Fatalf("saved config retained an unset favorite call pattern:\n%s", contents)
	}
}
