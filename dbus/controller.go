package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/led"
	"InariBlueNotifier/notifications"
	"InariBlueNotifier/patterns"
	"context"
	"errors"
	"fmt"
	"log"
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
}

type controller struct {
	connection          *dbus.Conn
	ledController       led.Controller
	notificationMatcher *notifications.Matcher

	now func() time.Time

	config     *config.Config
	mutex      sync.Mutex
	eventMutex sync.Mutex
	cancel     context.CancelFunc
	done       chan struct{}

	// incomingCalls maps each ringing call to whether its caller is a favourite.
	incomingCalls map[dbus.ObjectPath]bool

	activeNotifications      map[uint32]trackedNotification
	pendingNotificationCalls map[uint32]trackedNotification
}

// NewControllerWithConfig creates a controller that uses the configured call
// patterns and selects the favourite pattern when the contacts helper is
// available over the session bus.
func NewControllerWithConfig(
	connection *dbus.Conn,
	ledController led.Controller,
	configuration *config.Config,
	matcher *notifications.Matcher,
) Controller {
	if configuration == nil {
		configuration = &config.Config{
			CallPattern:         patterns.DefaultCallPattern,
			NotificationPattern: patterns.DefaultNotificationPattern,
		}
	}

	return &controller{
		connection:               connection,
		ledController:            ledController,
		notificationMatcher:      matcher,
		config:                   configuration,
		incomingCalls:            make(map[dbus.ObjectPath]bool),
		activeNotifications:      make(map[uint32]trackedNotification),
		pendingNotificationCalls: make(map[uint32]trackedNotification),
		now:                      time.Now,
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
	log.Printf("subscribed to voice call status changes")

	notificationMessages, notificationConnection, err := receiver.startNotificationMonitor()
	if err != nil {
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
	log.Printf("stopped voice call watcher")

	receiver.cancel = nil
	receiver.done = nil

	return nil
}
