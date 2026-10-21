package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/patterns"
	"log/slog"
	"sort"
	"time"

	"github.com/godbus/dbus/v5"
)

func (receiver *controller) updateLED() error {
	eventEnabled := func(event config.EventType) bool {
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

	customPattern, hasCustomPattern := receiver.activeCustomPattern()
	hasCustomNotification := receiver.hasActiveCustomNotification()

	switch {
	case receiver.knockActive:
		pattern := receiver.config.Knock.Pattern
		receiver.logger.Info("LED update", slog.String("requested_pattern", pattern.Name), slog.String("reason", "active knock"))
		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}
		return nil

	case timeInRange && eventEnabled(config.EventFavoriteCall) && hasFavoriteCall(receiver.incomingCalls):
		pattern := receiver.config.CallPattern
		if receiver.config.FavoriteCallPattern != nil {
			pattern = *receiver.config.FavoriteCallPattern
		}

		receiver.logger.Info("LED update", slog.String("requested_pattern", pattern.Name), slog.String("reason", "incoming favourite call"), slog.Int("incoming_calls", len(receiver.incomingCalls)))

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		receiver.logger.Info("LED update applied", slog.String("pattern", pattern.Name))
		return nil

	case timeInRange && eventEnabled(config.EventCall) && len(receiver.incomingCalls) > 0:
		pattern := receiver.config.CallPattern

		receiver.logger.Info("LED update", slog.String("requested_pattern", pattern.Name), slog.String("reason", "incoming call"), slog.Int("incoming_calls", len(receiver.incomingCalls)))

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		receiver.logger.Info("LED update applied", slog.String("pattern", pattern.Name))
		return nil

	case timeInRange && hasCustomPattern:
		receiver.logger.Info("LED update", slog.String("requested_pattern", customPattern.Name), slog.String("reason", "active custom notification filter"), slog.Int("notifications", len(receiver.activeNotifications)))

		if err := receiver.ledController.PlayPattern(customPattern); err != nil {
			return err
		}

		receiver.logger.Info("LED update applied", slog.String("pattern", customPattern.Name))
		return nil

	case timeInRange && eventEnabled(config.EventSMS) && receiver.hasActiveNotificationFilter(string(config.EventSMS)):
		pattern := receiver.config.NotificationPattern
		if receiver.config.SMSPattern != nil {
			pattern = *receiver.config.SMSPattern
		}

		receiver.logger.Info("LED update", slog.String("requested_pattern", pattern.Name), slog.String("reason", "active SMS notification"), slog.Int("notifications", len(receiver.activeNotifications)))

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		receiver.logger.Info("LED update applied", slog.String("pattern", pattern.Name))
		return nil

	case timeInRange && eventEnabled(config.EventEmail) && receiver.hasActiveNotificationFilter(string(config.EventEmail)):
		pattern := receiver.config.NotificationPattern
		if receiver.config.EmailPattern != nil {
			pattern = *receiver.config.EmailPattern
		}

		receiver.logger.Info("LED update", slog.String("requested_pattern", pattern.Name), slog.String("reason", "active email notification"), slog.Int("notifications", len(receiver.activeNotifications)))

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		receiver.logger.Info("LED update applied", slog.String("pattern", pattern.Name))
		return nil

	case timeInRange && eventEnabled(config.EventSignal) && receiver.hasActiveNotificationFilter(string(config.EventSignal)):
		pattern := receiver.config.NotificationPattern
		if receiver.config.SignalPattern != nil {
			pattern = *receiver.config.SignalPattern
		}

		receiver.logger.Info("LED update", slog.String("requested_pattern", pattern.Name), slog.String("reason", "active Signal notification"), slog.Int("notifications", len(receiver.activeNotifications)))

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		receiver.logger.Info("LED update applied", slog.String("pattern", pattern.Name))
		return nil

	case timeInRange && eventEnabled(config.EventMatrix) && receiver.hasActiveNotificationFilter(string(config.EventMatrix)):
		pattern := receiver.config.NotificationPattern
		if receiver.config.MatrixPattern != nil {
			pattern = *receiver.config.MatrixPattern
		}

		receiver.logger.Info("LED update", slog.String("requested_pattern", pattern.Name), slog.String("reason", "active Matrix notification"), slog.Int("notifications", len(receiver.activeNotifications)))

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		receiver.logger.Info("LED update applied", slog.String("pattern", pattern.Name))
		return nil

	case timeInRange && hasCustomNotification,
		timeInRange && eventEnabled(config.EventNotification) && len(receiver.activeNotifications) > 0:

		pattern := receiver.config.NotificationPattern
		reason := "active notifications"
		if hasCustomNotification {
			reason = "active custom-matched notification"
		}

		receiver.logger.Info("LED update", slog.String("requested_pattern", pattern.Name), slog.String("reason", reason), slog.Int("notifications", len(receiver.activeNotifications)))

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		receiver.logger.Info("LED update applied", slog.String("pattern", pattern.Name))
		return nil

	case timeInRange:
		pattern := receiver.config.OffPattern

		receiver.logger.Info("LED update", slog.String("requested_pattern", pattern.Name), slog.String("reason", "no incoming calls or active notifications"), slog.Int("notifications", len(receiver.activeNotifications)))

		if err := receiver.ledController.PlayPattern(pattern); err != nil {
			return err
		}

		return nil
	default:
		receiver.logger.Info("LED update", slog.String("requested_stop", "outside of allowed hours"))
		if err := receiver.ledController.Stop(); err != nil {
			return err
		}

		receiver.logger.Info("LED update applied", slog.String("state", "stopped"))
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

// activeCustomPattern returns the pattern configured for an active custom
// filter. When more than one custom filter is active, names are sorted so the
// selected pattern is stable instead of depending on Go map iteration order.
func (receiver *controller) activeCustomPattern() (pattern patterns.Pattern, ok bool) {
	if receiver.config == nil || len(receiver.config.CustomActivePatterns) == 0 {
		return patterns.Pattern{}, false
	}

	filterNames := make([]string, 0)
	for _, notification := range receiver.activeNotifications {
		for filterName := range notification.customFilters {
			if _, configured := receiver.config.CustomActivePatterns[filterName]; configured {
				filterNames = append(filterNames, filterName)
			}
		}
	}

	if len(filterNames) == 0 {
		return patterns.Pattern{}, false
	}

	sort.Strings(filterNames)
	return receiver.config.CustomActivePatterns[filterNames[0]], true
}

func hasFavoriteCall(calls map[dbus.ObjectPath]bool) bool {
	for _, favorite := range calls {
		if favorite {
			return true
		}
	}
	return false
}
