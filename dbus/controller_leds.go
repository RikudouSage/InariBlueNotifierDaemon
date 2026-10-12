package dbus

import (
	"InariBlueNotifier/config"
	"log"

	"github.com/godbus/dbus/v5"
)

func (receiver *controller) updateLED() error {
	eventEnabled := func(event config.EventType) bool {
		// Missing event settings use the same defaults as a newly created config.
		if receiver.config == nil || receiver.config.Events == nil {
			return event != config.EventNotification
		}

		enabled, configured := receiver.config.Events[event]
		if !configured {
			return event != config.EventNotification
		}

		return enabled
	}

	switch {
	case eventEnabled(config.EventFavoriteCall) && hasFavoriteCall(receiver.incomingCalls):
		pattern := receiver.config.FavoriteCallPattern

		log.Printf(
			"LED update: requested_pattern=%s reason=incoming favourite call incoming_calls=%d",
			pattern.Name,
			len(receiver.incomingCalls),
		)

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		log.Printf("LED update applied: pattern=%s", pattern.Name)
		return nil

	case eventEnabled(config.EventCall) && len(receiver.incomingCalls) > 0:
		pattern := receiver.config.CallPattern

		log.Printf(
			"LED update: requested_pattern=%s reason=incoming call incoming_calls=%d",
			pattern.Name,
			len(receiver.incomingCalls),
		)

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		log.Printf("LED update applied: pattern=%s", pattern.Name)
		return nil

	case eventEnabled(config.EventNotification) && len(receiver.activeNotifications) > 0:
		pattern := receiver.config.NotificationPattern

		log.Printf(
			"LED update: requested_pattern=%s reason=active notifications notifications=%d",
			pattern.Name,
			len(receiver.activeNotifications),
		)

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		log.Printf("LED update applied: pattern=%s", pattern.Name)
		return nil

	default:
		log.Printf("LED update: requested_stop reason=no incoming calls or active notifications")
		if err := receiver.ledController.Stop(); err != nil {
			return err
		}

		log.Printf("LED update applied: stopped")
		return nil
	}
}

func hasFavoriteCall(calls map[dbus.ObjectPath]bool) bool {
	for _, favorite := range calls {
		if favorite {
			return true
		}
	}
	return false
}
