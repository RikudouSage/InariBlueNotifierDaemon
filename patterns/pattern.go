package patterns

import (
	"errors"
	"fmt"
	"time"
)

var DefaultCallPattern = SlowBlink
var DefaultNotificationPattern = MediumLengthBreath

const maxTimingDuration = 7031250 * time.Microsecond

// HoldIndefinitely requests a steady light until another pattern is applied.
const HoldIndefinitely time.Duration = -1

type Pattern struct {
	Name string `json:"name"`

	Rise       time.Duration `json:"rise"`
	Hold       time.Duration `json:"hold"`
	Decay      time.Duration `json:"decay"`
	Off        time.Duration `json:"off"`
	Delay      time.Duration `json:"delay"`
	Repeat     uint8         `json:"repeat"`
	Brightness uint8         `json:"brightness"`

	Channels []uint8 `json:"channels,omitempty"`
}

func (receiver *Pattern) Validate() error {
	if receiver.Name == "" {
		return errors.New("empty name")
	}

	if receiver.Repeat > 15 {
		return errors.New("repeat must be between 0 and 15")
	}
	if receiver.Hold == HoldIndefinitely && (receiver.Rise != 0 || receiver.Decay != 0 || receiver.Off != 0 || receiver.Delay != 0 || receiver.Repeat != 0) {
		return errors.New("indefinite hold requires zero rise, decay, off, delay, and repeat")
	}

	seenChannels := make(map[uint8]struct{}, len(receiver.Channels))
	for _, channel := range receiver.Channels {
		if _, seen := seenChannels[channel]; seen {
			return fmt.Errorf("channel %d is configured more than once", channel)
		}
		seenChannels[channel] = struct{}{}
	}

	durations := map[string]time.Duration{
		"rise":  receiver.Rise,
		"hold":  receiver.Hold,
		"decay": receiver.Decay,
		"off":   receiver.Off,
		"delay": receiver.Delay,
	}

	for name, duration := range durations {
		if name == "hold" && duration == HoldIndefinitely {
			continue
		}
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
