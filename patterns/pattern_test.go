package patterns

import (
	"strings"
	"testing"
	"time"
)

func TestPatternValidateRejectsInvalidChannels(t *testing.T) {
	for name, channels := range map[string][]uint8{
		"duplicate": {1, 1},
	} {
		t.Run(name, func(t *testing.T) {
			pattern := Pattern{Name: "Invalid channels", Channels: channels}
			if err := pattern.Validate(); err == nil || !strings.Contains(err.Error(), "channel") {
				t.Fatalf("Validate() error = %v, want channel validation error", err)
			}
		})
	}
}

func TestIndefiniteHoldValidation(t *testing.T) {
	tests := []struct {
		name    string
		pattern Pattern
		valid   bool
	}{
		{"steady", Steady, true},
		{"negative hold", Pattern{Name: "bad", Hold: -2}, false},
		{"negative rise", Pattern{Name: "bad", Rise: -1}, false},
		{"negative decay", Pattern{Name: "bad", Decay: -1}, false},
		{"negative off", Pattern{Name: "bad", Off: -1}, false},
		{"negative delay", Pattern{Name: "bad", Delay: -1}, false},
		{"steady with rise", Pattern{Name: "bad", Hold: HoldIndefinitely, Rise: time.Millisecond}, false},
		{"steady with decay", Pattern{Name: "bad", Hold: HoldIndefinitely, Decay: time.Millisecond}, false},
		{"steady with off", Pattern{Name: "bad", Hold: HoldIndefinitely, Off: time.Millisecond}, false},
		{"steady with delay", Pattern{Name: "bad", Hold: HoldIndefinitely, Delay: time.Millisecond}, false},
		{"steady with repeat", Pattern{Name: "bad", Hold: HoldIndefinitely, Repeat: 1}, false},
		{"ordinary animation", SlowBlink, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pattern.Validate()
			if (err == nil) != tt.valid {
				t.Fatalf("Validate() error = %v, valid = %t", err, tt.valid)
			}
		})
	}
}
