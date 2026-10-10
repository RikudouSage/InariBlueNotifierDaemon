package dbus

import "InariBlueNotifier/led"

func (receiver *controller) updateLED() error {
	switch {
	case len(receiver.incomingCalls) > 0:
		return receiver.ledController.SetState(led.Blinking)

	case receiver.hasUnread:
		return receiver.ledController.SetState(led.Breathing)

	default:
		return receiver.ledController.SetState(led.Off)
	}
}
