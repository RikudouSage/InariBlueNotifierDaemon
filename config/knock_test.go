package config

import (
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/ini.v1"
)

func TestKnockTimeoutRoundTrips(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := New()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Knock.Timeout = 37 * time.Second
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Knock.Timeout != cfg.Knock.Timeout {
		t.Fatalf("loaded timeout = %s, want %s", loaded.Knock.Timeout, cfg.Knock.Timeout)
	}
	saved, err := ini.Load(cfg.cfgFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if value := saved.Section(sectionKnock).Key(knockKeyTimeout).String(); value != "37s" {
		t.Fatalf("saved timeout = %q, want 37s", value)
	}
}

func TestLegacyKnockTimeoutIsMigrated(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	cfg, err := New()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configHome, "dev.rikudou", "inari-blue-notifier", "config_v3.ini")
	raw, err := ini.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	raw.Section(sectionKnock).Key(knockKeyTimeout).SetValue("37000000000")
	if err := raw.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Knock.Timeout != 37*time.Second {
		t.Fatalf("legacy timeout = %s, want 37s", loaded.Knock.Timeout)
	}
	saved, err := ini.Load(cfg.cfgFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if value := saved.Section(sectionKnock).Key(knockKeyTimeout).String(); value != "37s" {
		t.Fatalf("migrated timeout = %q, want 37s", value)
	}
}

func TestInvalidKnockBindAddressesArePreserved(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := New()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Knock.Addresses = []string{"invalid-address"}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Knock.Addresses) != 1 || loaded.Knock.Addresses[0] != "invalid-address" {
		t.Fatalf("invalid addresses = %v, want preserved input", loaded.Knock.Addresses)
	}
}
