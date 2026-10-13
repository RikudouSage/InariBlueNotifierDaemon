package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/led"
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
	connection    *dbus.Conn
	ledController led.Controller

	now func() time.Time

	config     *config.Config
	mutex      sync.Mutex
	eventMutex sync.Mutex
	cancel     context.CancelFunc
	done       chan struct{}

	// incomingCalls maps each ringing call to whether its caller is a favourite.
	incomingCalls map[dbus.ObjectPath]bool

	// activeNotifications contains notification IDs returned by the notification
	// server. pendingNotificationCalls maps Notify request serials until the
	// server assigns their IDs.
	activeNotifications      map[uint32]struct{}
	pendingNotificationCalls map[uint32]struct{}
}

func NewController(connection *dbus.Conn, ledController led.Controller) Controller {
	return NewControllerWithConfig(connection, ledController, &config.Config{
		CallPattern:         patterns.DefaultCallPattern,
		FavoriteCallPattern: patterns.DefaultCallPattern,
		NotificationPattern: patterns.DefaultNotificationPattern,
	})
}

// NewControllerWithConfig creates a controller that uses the configured call
// patterns and selects the favourite pattern when the contacts helper is
// available over the session bus.
func NewControllerWithConfig(
	connection *dbus.Conn,
	ledController led.Controller,
	configuration *config.Config,
) Controller {
	if configuration == nil {
		configuration = &config.Config{
			CallPattern:         patterns.DefaultCallPattern,
			FavoriteCallPattern: patterns.DefaultCallPattern,
			NotificationPattern: patterns.DefaultNotificationPattern,
		}
	}

	return &controller{
		connection:               connection,
		ledController:            ledController,
		config:                   configuration,
		incomingCalls:            make(map[dbus.ObjectPath]bool),
		activeNotifications:      make(map[uint32]struct{}),
		pendingNotificationCalls: make(map[uint32]struct{}),
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
	receiver.activeNotifications = make(map[uint32]struct{})
	receiver.pendingNotificationCalls = make(map[uint32]struct{})

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
