package patterns

import (
	"strings"
	"testing"
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
