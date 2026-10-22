package dbus

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"
)

const (
	privacySwitchInterface = "org.sailfishos.privacyswitch"
	privacySwitchSignal    = privacySwitchInterface + ".privacyModeActiveChanged"
)

func (receiver *controller) subscribePrivacySwitch() (chan *dbus.Signal, []dbus.MatchOption, error) {
	if receiver.connection == nil {
		return nil, nil, fmt.Errorf("privacy-switch watcher requires a session bus connection")
	}

	match := []dbus.MatchOption{
		dbus.WithMatchInterface(privacySwitchInterface),
		dbus.WithMatchMember("privacyModeActiveChanged"),
	}
	signals := make(chan *dbus.Signal, 16)
	receiver.connection.Signal(signals)

	if err := receiver.connection.AddMatchSignal(match...); err != nil {
		receiver.connection.RemoveSignal(signals)

		return nil, nil, fmt.Errorf("subscribe to privacy-switch changes: %w", err)
	}

	receiver.logger.Info("subscribed to privacy-switch changes")

	return signals, match, nil
}

func (receiver *controller) watchPrivacySwitch(ctx context.Context, signals chan *dbus.Signal, match []dbus.MatchOption) {
	defer receiver.connection.RemoveSignal(signals)
	defer func() {
		if err := receiver.connection.RemoveMatchSignal(match...); err != nil {
			receiver.logger.Error("failed to remove privacy-switch signal match", slog.String("error", err.Error()))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case signal, ok := <-signals:
			if !ok {
				return
			}
			if signal == nil || signal.Name != privacySwitchSignal || len(signal.Body) != 1 {
				continue
			}

			active, ok := signal.Body[0].(bool)
			if !ok {
				receiver.logger.Warn("ignored privacy-switch signal with invalid body", slog.Any("body", signal.Body))
				continue
			}

			receiver.eventMutex.Lock()
			if receiver.privacyModeActive != active {
				receiver.privacyModeActive = active
				if err := receiver.updateLED(); err != nil {
					receiver.logger.Error("LED update failed after privacy-switch change", slog.Bool("active", active), slog.String("error", err.Error()))
				}
			}

			receiver.eventMutex.Unlock()
		}
	}
}
