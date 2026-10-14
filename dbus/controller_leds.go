package dbus

import (
	"InariBlueNotifier/config"
	"log"
	"time"

	"github.com/godbus/dbus/v5"
)

func (receiver *controller) updateLED() error {
	eventEnabled := func(event config.EventType) bool {
		// Missing event settings use the same defaults as a newly created config.
		if receiver.config == nil || receiver.config.Events == nil {
			return event == config.EventCall || event == config.EventFavoriteCall
		}

		enabled, configured := receiver.config.Events[event]
		if !configured {
			return event == config.EventCall || event == config.EventFavoriteCall
		}

		return enabled
	}

	timeInRange := func(now time.Time) bool {
		if receiver.config == nil {
			return true
		}

		start := receiver.config.Constraints.StartTime
		end := receiver.config.Constraints.EndTime

		success := true
		if !start.IsZero() && start.After(now) {
			success = false
		}
		if !end.IsZero() && end.Before(now) {
			success = false
		}

		return success
	}(receiver.now())

	switch {
	case timeInRange && eventEnabled(config.EventFavoriteCall) && hasFavoriteCall(receiver.incomingCalls):
		pattern := receiver.config.CallPattern
		if receiver.config.FavoriteCallPattern != nil {
			pattern = *receiver.config.FavoriteCallPattern
		}

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

	case timeInRange && eventEnabled(config.EventCall) && len(receiver.incomingCalls) > 0:
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

	case timeInRange && eventEnabled(config.EventSMS) && receiver.hasActiveNotificationFilter(string(config.EventSMS)):
		pattern := receiver.config.NotificationPattern
		if receiver.config.SMSPattern != nil {
			pattern = *receiver.config.SMSPattern
		}

		log.Printf(
			"LED update: requested_pattern=%s reason=active SMS notification notifications=%d",
			pattern.Name,
			len(receiver.activeNotifications),
		)

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		log.Printf("LED update applied: pattern=%s", pattern.Name)
		return nil

	case timeInRange && eventEnabled(config.EventEmail) && receiver.hasActiveNotificationFilter(string(config.EventEmail)):
		pattern := receiver.config.NotificationPattern
		if receiver.config.EmailPattern != nil {
			pattern = *receiver.config.EmailPattern
		}

		log.Printf(
			"LED update: requested_pattern=%s reason=active email notification notifications=%d",
			pattern.Name,
			len(receiver.activeNotifications),
		)

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		log.Printf("LED update applied: pattern=%s", pattern.Name)
		return nil

	case timeInRange && eventEnabled(config.EventSignal) && receiver.hasActiveNotificationFilter(string(config.EventSignal)):
		pattern := receiver.config.NotificationPattern
		if receiver.config.SignalPattern != nil {
			pattern = *receiver.config.SignalPattern
		}

		log.Printf(
			"LED update: requested_pattern=%s reason=active Signal notification notifications=%d",
			pattern.Name,
			len(receiver.activeNotifications),
		)

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		log.Printf("LED update applied: pattern=%s", pattern.Name)
		return nil

	case timeInRange && eventEnabled(config.EventMatrix) && receiver.hasActiveNotificationFilter(string(config.EventMatrix)):
		pattern := receiver.config.NotificationPattern
		if receiver.config.MatrixPattern != nil {
			pattern = *receiver.config.MatrixPattern
		}

		log.Printf(
			"LED update: requested_pattern=%s reason=active Matrix notification notifications=%d",
			pattern.Name,
			len(receiver.activeNotifications),
		)

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		log.Printf("LED update applied: pattern=%s", pattern.Name)
		return nil

	// TODO: Move this above built-in matchers once custom matchers support a configured pattern.
	case timeInRange && receiver.hasActiveCustomNotification():
		pattern := receiver.config.NotificationPattern

		log.Printf(
			"LED update: requested_pattern=%s reason=active custom-matched notification notifications=%d",
			pattern.Name,
			len(receiver.activeNotifications),
		)

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		log.Printf("LED update applied: pattern=%s", pattern.Name)
		return nil

	case timeInRange && eventEnabled(config.EventNotification) && len(receiver.activeNotifications) > 0:
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
		reason := "no incoming calls or active notifications"
		if !timeInRange {
			reason = "outside of allowed hours"
		}

		log.Printf("LED update: requested_stop reason=%s", reason)
		if err := receiver.ledController.Stop(); err != nil {
			return err
		}

		log.Printf("LED update applied: stopped")
		return nil
	}
}

func (receiver *controller) hasActiveNotificationFilter(filter string) bool {
	for _, notification := range receiver.activeNotifications {
		if _, matches := notification.filters[filter]; matches {
			return true
		}
	}

	return false
}

func (receiver *controller) hasActiveCustomNotification() bool {
	for _, notification := range receiver.activeNotifications {
		if notification.customMatched {
			return true
		}
	}

	return false
}

func hasFavoriteCall(calls map[dbus.ObjectPath]bool) bool {
	for _, favorite := range calls {
		if favorite {
			return true
		}
	}
	return false
}
