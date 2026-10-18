package config

import (
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
	if value := saved.Section(sectionKnock).Key(knockKeyTimeout).String(); value != "37000000000" {
		t.Fatalf("saved timeout = %q, want 37000000000", value)
	}
}

func TestLegacyKnockTimeoutIsMigrated(t *testing.T) {
	for _, tt := range []struct {
		name, input, saved string
		want               time.Duration
	}{
		{"seconds", "37s", "37000000000", 37 * time.Second},
		{"fractional duration", "1m0.000000001s", "60000000001", time.Minute + time.Nanosecond},
		{"zero duration", "0s", "0", 0},
		{"negative duration", "-1ms", "-1000000", -time.Millisecond},
		{"numeric nanoseconds", "37000000000", "37000000000", 37 * time.Second},
		{"invalid duration", "invalid", "60000000000", time.Minute},
		{"overflow", "9223372036854775808", "60000000000", time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cfg, err := New()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := ini.Load(cfg.cfgFilePath)
			if err != nil {
				t.Fatal(err)
			}
			raw.Section(sectionKnock).Key(knockKeyTimeout).SetValue(tt.input)
			if err := raw.SaveTo(cfg.cfgFilePath); err != nil {
				t.Fatal(err)
			}
			loaded, err := New()
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Knock.Timeout != tt.want {
				t.Fatalf("loaded timeout = %s, want %s", loaded.Knock.Timeout, tt.want)
			}
			saved, err := ini.Load(cfg.cfgFilePath)
			if err != nil {
				t.Fatal(err)
			}
			if value := saved.Section(sectionKnock).Key(knockKeyTimeout).String(); value != tt.saved {
				t.Fatalf("saved timeout = %q, want %q", value, tt.saved)
			}
		})
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
