package dbus

import "log"

func (receiver *controller) updateLED() error {
	switch {
	case len(receiver.incomingCalls) > 0:
		pattern := receiver.config.CallPattern
		reason := "incoming call"

		for _, favorite := range receiver.incomingCalls {
			if favorite {
				pattern = receiver.config.FavoriteCallPattern
				reason = "incoming favourite call"
				break
			}
		}

		log.Printf(
			"LED update: requested_pattern=%s reason=%s incoming_calls=%d",
			pattern.Name,
			reason,
			len(receiver.incomingCalls),
		)

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		log.Printf("LED update applied: pattern=%s", pattern.Name)
		return nil

	default:
		log.Printf("LED update: requested_stop reason=no incoming calls")
		if err := receiver.ledController.Stop(); err != nil {
			return err
		}

		log.Printf("LED update applied: stopped")
		return nil
	}
}
