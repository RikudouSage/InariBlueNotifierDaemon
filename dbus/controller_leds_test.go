package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/patterns"
	"testing"
	"time"

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
			FavoriteCallPattern: &favourite,
		},
		incomingCalls: map[dbus.ObjectPath]bool{
			"/calls/one": false,
		},
		now: time.Now,
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
			FavoriteCallPattern: &favourite,
		},
		incomingCalls: map[dbus.ObjectPath]bool{
			"/calls/one": false,
			"/calls/two": true,
		},
		now: time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != favourite.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, favourite.Name)
	}
}

func TestUpdateLEDUsesCallPatternForFavouriteCallWhenFavoriteCallPatternIsUnset(t *testing.T) {
	call := patterns.SlowBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			CallPattern: call,
		},
		incomingCalls: map[dbus.ObjectPath]bool{
			"/calls/one": true,
		},
		now: time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != call.Name {
		t.Fatalf("applied pattern = %q, want call pattern %q", ledController.lastPattern.Name, call.Name)
	}
}

func TestUpdateLEDAppliesNotificationPatternWhileNotificationIsActive(t *testing.T) {
	notification := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			NotificationPattern: notification,
			Events: map[config.EventType]bool{
				config.EventNotification: true,
			},
		},
		activeNotifications: trackedNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != notification.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, notification.Name)
	}
}

func TestUpdateLEDAppliesNotificationPatternForCustomMatch(t *testing.T) {
	notification := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			NotificationPattern: notification,
			Events: map[config.EventType]bool{
				config.EventNotification: false,
			},
		},
		activeNotifications: map[uint32]trackedNotification{
			1: {customMatched: true},
		},
		now: time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != notification.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, notification.Name)
	}
}

func TestUpdateLEDAppliesSMSPatternWhenGenericNotificationsAreDisabled(t *testing.T) {
	sms := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			SMSPattern: &sms,
			Events: map[config.EventType]bool{
				config.EventNotification: false,
				config.EventSMS:          true,
			},
		},
		activeNotifications: trackedSMSNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != sms.Name {
		t.Fatalf("applied pattern = %q, want SMS pattern %q", ledController.lastPattern.Name, sms.Name)
	}
}

func TestUpdateLEDUsesNotificationPatternForSMSWhenSMSPatternIsUnset(t *testing.T) {
	notification := patterns.SlowBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			NotificationPattern: notification,
			Events: map[config.EventType]bool{
				config.EventNotification: false,
				config.EventSMS:          true,
			},
		},
		activeNotifications: trackedSMSNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != notification.Name {
		t.Fatalf("applied pattern = %q, want notification pattern %q", ledController.lastPattern.Name, notification.Name)
	}
}

func TestUpdateLEDAppliesEmailPatternWhenGenericNotificationsAreDisabled(t *testing.T) {
	email := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			EmailPattern: &email,
			Events: map[config.EventType]bool{
				config.EventNotification: false,
				config.EventEmail:        true,
			},
		},
		activeNotifications: trackedEmailNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != email.Name {
		t.Fatalf("applied pattern = %q, want email pattern %q", ledController.lastPattern.Name, email.Name)
	}
}

func TestUpdateLEDUsesNotificationPatternForEmailWhenEmailPatternIsUnset(t *testing.T) {
	notification := patterns.SlowBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			NotificationPattern: notification,
			Events: map[config.EventType]bool{
				config.EventNotification: false,
				config.EventEmail:        true,
			},
		},
		activeNotifications: trackedEmailNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != notification.Name {
		t.Fatalf("applied pattern = %q, want notification pattern %q", ledController.lastPattern.Name, notification.Name)
	}
}

func TestUpdateLEDAppliesSignalPatternWhenGenericNotificationsAreDisabled(t *testing.T) {
	signal := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			SignalPattern: &signal,
			Events: map[config.EventType]bool{
				config.EventNotification: false,
				config.EventSignal:       true,
			},
		},
		activeNotifications: trackedSignalNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != signal.Name {
		t.Fatalf("applied pattern = %q, want Signal pattern %q", ledController.lastPattern.Name, signal.Name)
	}
}

func TestUpdateLEDUsesNotificationPatternForSignalWhenSignalPatternIsUnset(t *testing.T) {
	notification := patterns.SlowBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			NotificationPattern: notification,
			Events: map[config.EventType]bool{
				config.EventNotification: false,
				config.EventSignal:       true,
			},
		},
		activeNotifications: trackedSignalNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != notification.Name {
		t.Fatalf("applied pattern = %q, want notification pattern %q", ledController.lastPattern.Name, notification.Name)
	}
}

func TestUpdateLEDAppliesMatrixPatternWhenGenericNotificationsAreDisabled(t *testing.T) {
	matrix := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			MatrixPattern: &matrix,
			Events: map[config.EventType]bool{
				config.EventNotification: false,
				config.EventMatrix:       true,
			},
		},
		activeNotifications: trackedMatrixNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != matrix.Name {
		t.Fatalf("applied pattern = %q, want Matrix pattern %q", ledController.lastPattern.Name, matrix.Name)
	}
}

func TestUpdateLEDUsesGenericNotificationsWhenSMSIsDisabled(t *testing.T) {
	notification := patterns.SlowBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			NotificationPattern: notification,
			SMSPattern:          &patterns.RapidBlink,
			Events: map[config.EventType]bool{
				config.EventNotification: true,
				config.EventSMS:          false,
			},
		},
		activeNotifications: trackedSMSNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != notification.Name {
		t.Fatalf("applied pattern = %q, want generic notification pattern %q", ledController.lastPattern.Name, notification.Name)
	}
}

func TestUpdateLEDPrioritizesSMSOverGenericNotifications(t *testing.T) {
	sms := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			NotificationPattern: patterns.SlowBlink,
			SMSPattern:          &sms,
			Events: map[config.EventType]bool{
				config.EventNotification: true,
				config.EventSMS:          true,
			},
		},
		activeNotifications: trackedSMSNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != sms.Name {
		t.Fatalf("applied pattern = %q, want SMS pattern %q", ledController.lastPattern.Name, sms.Name)
	}
}

func TestUpdateLEDSkipsNotificationWhenEventIsMissing(t *testing.T) {
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController:       ledController,
		config:              &config.Config{NotificationPattern: patterns.RapidBlink},
		activeNotifications: trackedNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if !ledController.stopped {
		t.Fatal("LED controller was not stopped")
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
		activeNotifications: trackedNotifications(1),
		now:                 time.Now,
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
		activeNotifications: trackedNotifications(1),
		now:                 time.Now,
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
			FavoriteCallPattern: &favourite,
			Events: map[config.EventType]bool{
				config.EventCall:         true,
				config.EventFavoriteCall: false,
			},
		},
		incomingCalls: map[dbus.ObjectPath]bool{"/calls/one": true},
		now:           time.Now,
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
		activeNotifications: trackedNotifications(1),
		now:                 time.Now,
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
	receiver := &controller{ledController: ledController, now: time.Now}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if !ledController.stopped {
		t.Fatal("LED controller was not stopped")
	}
}

func trackedSMSNotifications(ids ...uint32) map[uint32]trackedNotification {
	result := trackedNotifications(ids...)
	for id := range result {
		result[id] = trackedNotification{filters: map[string]struct{}{
			string(config.EventSMS): {},
		}}
	}
	return result
}

func trackedEmailNotifications(ids ...uint32) map[uint32]trackedNotification {
	result := trackedNotifications(ids...)
	for id := range result {
		result[id] = trackedNotification{filters: map[string]struct{}{
			string(config.EventEmail): {},
		}}
	}
	return result
}

func trackedSignalNotifications(ids ...uint32) map[uint32]trackedNotification {
	result := trackedNotifications(ids...)
	for id := range result {
		result[id] = trackedNotification{filters: map[string]struct{}{
			string(config.EventSignal): {},
		}}
	}
	return result
}

func trackedMatrixNotifications(ids ...uint32) map[uint32]trackedNotification {
	result := trackedNotifications(ids...)
	for id := range result {
		result[id] = trackedNotification{filters: map[string]struct{}{
			string(config.EventMatrix): {},
		}}
	}
	return result
}
