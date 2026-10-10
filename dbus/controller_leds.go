package dbus

import (
	"InariBlueNotifier/led"
	"fmt"
	"log"
)

func (receiver *controller) updateLED() error {
	var state led.State
	reason := "no incoming calls or unread notifications"

	switch {
	case len(receiver.incomingCalls) > 0:
		state = led.Blinking
		reason = "incoming call"

	case receiver.hasUnread:
		state = led.Breathing
		reason = "unread notification"

	default:
		state = led.Off
	}

	log.Printf(
		"LED update: requested_state=%s reason=%s incoming_calls=%d has_unread=%t",
		stateName(state),
		reason,
		len(receiver.incomingCalls),
		receiver.hasUnread,
	)

	if err := receiver.ledController.SetState(state); err != nil {
		return err
	}

	log.Printf("LED update applied: state=%s", stateName(state))
	return nil
}

func stateName(state led.State) string {
	switch state {
	case led.Off:
		return "off"
	case led.Breathing:
		return "breathing"
	case led.Blinking:
		return "blinking"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}
