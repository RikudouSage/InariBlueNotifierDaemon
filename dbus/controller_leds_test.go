package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/patterns"
	"testing"

	"github.com/godbus/dbus/v5"
)

type recordingLEDController struct {
	lastPattern patterns.Pattern
	stopped     bool
}

func (receiver *recordingLEDController) PlayPattern(pattern patterns.Pattern) error {
	receiver.lastPattern = pattern
	return nil
}

func (receiver *recordingLEDController) Stop() error {
	receiver.stopped = true
	return nil
}
func (receiver *recordingLEDController) Release() error { return nil }
func (receiver *recordingLEDController) Shutdown() error {
	return nil
}
func (receiver *recordingLEDController) Done() <-chan struct{} { return nil }

func TestUpdateLEDAppliesCallPattern(t *testing.T) {
	normal := patterns.SlowBlink
	favourite := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			CallPattern:         normal,
			FavoriteCallPattern: favourite,
		},
		incomingCalls: map[dbus.ObjectPath]bool{
			"/calls/one": false,
		},
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != normal.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, normal.Name)
	}
}

func TestUpdateLEDAppliesFavouriteCallPattern(t *testing.T) {
	normal := patterns.SlowBlink
	favourite := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			CallPattern:         normal,
			FavoriteCallPattern: favourite,
		},
		incomingCalls: map[dbus.ObjectPath]bool{
			"/calls/one": false,
			"/calls/two": true,
		},
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != favourite.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, favourite.Name)
	}
}

func TestUpdateLEDAppliesNotificationPatternWhileNotificationIsActive(t *testing.T) {
	notification := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config:        &config.Config{NotificationPattern: notification},
		activeNotifications: map[uint32]struct{}{
			1: {},
		},
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != notification.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, notification.Name)
	}
}

func TestUpdateLEDKeepsCallPatternOverNotificationPattern(t *testing.T) {
	call := patterns.SlowBlink
	notification := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			CallPattern:         call,
			NotificationPattern: notification,
		},
		incomingCalls:       map[dbus.ObjectPath]bool{"/calls/one": false},
		activeNotifications: map[uint32]struct{}{1: {}},
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != call.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, call.Name)
	}
}

func TestUpdateLEDSkipsDisabledCallForEnabledNotification(t *testing.T) {
	call := patterns.SlowBlink
	notification := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			CallPattern:         call,
			NotificationPattern: notification,
			Events: map[config.EventType]bool{
				config.EventCall:         false,
				config.EventNotification: true,
			},
		},
		incomingCalls:       map[dbus.ObjectPath]bool{"/calls/one": false},
		activeNotifications: map[uint32]struct{}{1: {}},
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != notification.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, notification.Name)
	}
}

func TestUpdateLEDSkipsDisabledFavouritePattern(t *testing.T) {
	call := patterns.SlowBlink
	favourite := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			CallPattern:         call,
			FavoriteCallPattern: favourite,
			Events: map[config.EventType]bool{
				config.EventCall:         true,
				config.EventFavoriteCall: false,
			},
		},
		incomingCalls: map[dbus.ObjectPath]bool{"/calls/one": true},
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != call.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, call.Name)
	}
}

func TestUpdateLEDStopsWhenAllActiveEventsAreDisabled(t *testing.T) {
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{Events: map[config.EventType]bool{
			config.EventCall:         false,
			config.EventFavoriteCall: false,
			config.EventNotification: false,
		}},
		incomingCalls:       map[dbus.ObjectPath]bool{"/calls/one": false},
		activeNotifications: map[uint32]struct{}{1: {}},
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if !ledController.stopped {
		t.Fatal("LED controller was not stopped")
	}
}

func TestUpdateLEDStopsWhenThereAreNoEvents(t *testing.T) {
	ledController := &recordingLEDController{}
	receiver := &controller{ledController: ledController}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if !ledController.stopped {
		t.Fatal("LED controller was not stopped")
	}
}
