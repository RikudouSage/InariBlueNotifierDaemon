package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/led"
	"InariBlueNotifier/notifications"
	"InariBlueNotifier/patterns"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	voiceCallInterface     = "org.nemomobile.voicecall.VoiceCall"
	voiceCallStatusChanged = voiceCallInterface + ".statusChanged"
	activeVoiceCallPath    = "/calls/active"

	voiceCallStatusIdle     = 0
	voiceCallStatusIncoming = 5
)

type Controller interface {
	Start() error
	Stop() error
	SetKnockActive(active bool) error
	SetKnockClearHandler(handler func())
}

type controller struct {
	connection          *dbus.Conn
	systemConnection    *dbus.Conn
	ledController       led.Controller
	notificationMatcher *notifications.Matcher
	clearKnock          func()

	now func() time.Time

	config     *config.Config
	mutex      sync.Mutex
	eventMutex sync.Mutex
	cancel     context.CancelFunc
	done       chan struct{}

	// incomingCalls maps each ringing call to whether its caller is a favourite.
	incomingCalls     map[dbus.ObjectPath]bool
	knockActive       bool
	privacyModeActive bool

	activeNotifications      map[uint32]trackedNotification
	pendingNotificationCalls map[uint32]trackedNotification

	logger *config.Logger
}

// SetKnockClearHandler sets the action used to clear the current knock.
func (receiver *controller) SetKnockClearHandler(handler func()) {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	receiver.clearKnock = handler
}

// SetKnockActive updates the knock event and selects the appropriate LED state.
func (receiver *controller) SetKnockActive(active bool) error {
	receiver.eventMutex.Lock()
	defer receiver.eventMutex.Unlock()

	if receiver.knockActive == active {
		return nil
	}
	receiver.knockActive = active

	return receiver.updateLED()
}

// NewControllerWithConfig creates a controller that uses the configured call
// patterns and selects the favourite pattern when the contacts helper is
// available over the session bus.
func NewControllerWithConfig(
	connection, systemConnection *dbus.Conn,
	ledController led.Controller,
	configuration *config.Config,
	matcher *notifications.Matcher,
	logger *config.Logger,
) Controller {
	if configuration == nil {
		configuration = &config.Config{
			CallPattern:         patterns.DefaultCallPattern,
			NotificationPattern: patterns.DefaultNotificationPattern,
			OffPattern:          patterns.Off,
			Events:              make(map[config.EventType]bool),
		}
	}

	return &controller{
		connection:               connection,
		systemConnection:         systemConnection,
		ledController:            ledController,
		notificationMatcher:      matcher,
		config:                   configuration,
		incomingCalls:            make(map[dbus.ObjectPath]bool),
		activeNotifications:      make(map[uint32]trackedNotification),
		pendingNotificationCalls: make(map[uint32]trackedNotification),
		now:                      time.Now,
		logger:                   logger,
	}
}

func (receiver *controller) Start() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if err := receiver.ledController.Stop(); err != nil {
		return fmt.Errorf("initialize LED state: %w", err)
	}

	if receiver.cancel != nil {
		return errors.New("controller already started")
	}

	signals := make(chan *dbus.Signal, 16)

	matchOptions := []dbus.MatchOption{
		dbus.WithMatchInterface(voiceCallInterface),
		dbus.WithMatchMember("statusChanged"),
	}

	receiver.connection.Signal(signals)

	if err := receiver.connection.AddMatchSignal(matchOptions...); err != nil {
		receiver.connection.RemoveSignal(signals)
		return fmt.Errorf("subscribe to voice call signals: %w", err)
	}
	receiver.logger.Info("subscribed to voice call status changes")

	var unlockSignals chan *dbus.Signal
	var unlockMatch []dbus.MatchOption
	if receiver.config.Knock.Enabled && receiver.config.Knock.StopOnUnlock {
		if receiver.clearKnock == nil {
			_ = receiver.connection.RemoveMatchSignal(matchOptions...)
			receiver.connection.RemoveSignal(signals)
			return errors.New("unlock watcher requires a knock clear handler")
		}

		var err error
		unlockSignals, unlockMatch, err = receiver.subscribeUnlock()
		if err != nil {
			_ = receiver.connection.RemoveMatchSignal(matchOptions...)
			receiver.connection.RemoveSignal(signals)
			return err
		}
	}

	var privacySwitchSignals chan *dbus.Signal
	var privacySwitchMatch []dbus.MatchOption
	if receiver.config.Constraints.DisableOnPrivacySwitch {
		var privacySwitchErr error
		privacySwitchSignals, privacySwitchMatch, privacySwitchErr = receiver.subscribePrivacySwitch()
		if privacySwitchErr != nil {
			if unlockSignals != nil {
				receiver.systemConnection.RemoveSignal(unlockSignals)
				_ = receiver.systemConnection.RemoveMatchSignal(unlockMatch...)
			}

			_ = receiver.connection.RemoveMatchSignal(matchOptions...)
			receiver.connection.RemoveSignal(signals)

			return privacySwitchErr
		}
	}

	notificationMessages, notificationConnection, err := receiver.startNotificationMonitor()
	if err != nil {
		if privacySwitchSignals != nil {
			receiver.connection.RemoveSignal(privacySwitchSignals)
			_ = receiver.connection.RemoveMatchSignal(privacySwitchMatch...)
		}

		if unlockSignals != nil {
			receiver.systemConnection.RemoveSignal(unlockSignals)
			_ = receiver.systemConnection.RemoveMatchSignal(unlockMatch...)
		}

		_ = receiver.connection.RemoveMatchSignal(matchOptions...)
		receiver.connection.RemoveSignal(signals)
		return fmt.Errorf("start notification monitor: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	receiver.cancel = cancel
	receiver.done = make(chan struct{})
	receiver.incomingCalls = make(map[dbus.ObjectPath]bool)
	receiver.activeNotifications = make(map[uint32]trackedNotification)
	receiver.pendingNotificationCalls = make(map[uint32]trackedNotification)

	var watchers sync.WaitGroup
	watchers.Go(func() {
		receiver.watchCalls(ctx, signals, matchOptions)
	})
	watchers.Go(func() {
		receiver.watchNotifications(ctx, notificationMessages, notificationConnection)
	})

	if unlockSignals != nil {
		clearKnock := receiver.clearKnock
		watchers.Go(func() {
			receiver.watchUnlock(ctx, unlockSignals, unlockMatch, clearKnock)
		})
	}

	if privacySwitchSignals != nil {
		watchers.Go(func() {
			receiver.watchPrivacySwitch(ctx, privacySwitchSignals, privacySwitchMatch)
		})
	}

	go func() {
		watchers.Wait()
		close(receiver.done)
	}()

	return nil
}

func (receiver *controller) Stop() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.cancel == nil {
		return nil
	}

	cancel := receiver.cancel
	done := receiver.done

	cancel()
	<-done
	receiver.logger.Info("stopped voice call watcher")

	receiver.cancel = nil
	receiver.done = nil

	return nil
}
