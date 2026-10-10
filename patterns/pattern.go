package patterns

import (
	"errors"
	"fmt"
	"time"
)

var DefaultCallPattern = SlowBlink
var DefaultNotificationPattern = MediumLengthBreath

const maxTimingDuration = 7031250 * time.Microsecond

type Pattern struct {
	Name string `json:"name"`

	Rise       time.Duration `json:"rise"`
	Hold       time.Duration `json:"hold"`
	Decay      time.Duration `json:"decay"`
	Off        time.Duration `json:"off"`
	Delay      time.Duration `json:"delay"`
	Repeat     uint8         `json:"repeat"`
	Brightness uint8         `json:"brightness"`
}

func (receiver *Pattern) Validate() error {
	if receiver.Name == "" {
		return errors.New("empty name")
	}

	if receiver.Repeat > 15 {
		return errors.New("repeat must be between 0 and 15")
	}

	durations := map[string]time.Duration{
		"rise":  receiver.Rise,
		"hold":  receiver.Hold,
		"decay": receiver.Decay,
		"off":   receiver.Off,
		"delay": receiver.Delay,
	}

	for name, duration := range durations {
		if duration < 0 || duration > maxTimingDuration {
			return fmt.Errorf(
				"%s must be between 0 and %s",
				name,
				maxTimingDuration,
			)
		}
	}

	return nil
}
