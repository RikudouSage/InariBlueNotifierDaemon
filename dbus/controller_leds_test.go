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
	plays       int
}

func (receiver *recordingLEDController) PlayPattern(pattern patterns.Pattern) error {
	receiver.lastPattern = pattern
	receiver.plays++
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

func TestKnockEventCoordinatesCallsAndNotifications(t *testing.T) {
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:        config.NewNilLogger(),
		ledController: ledController,
		config: &config.Config{
			CallPattern:         patterns.SlowBlink,
			NotificationPattern: patterns.QuickFlash,
			OffPattern:          patterns.Off,
			Knock:               config.Knock{Pattern: patterns.RapidBlink},
			Events:              map[config.EventType]bool{config.EventCall: true, config.EventNotification: true},
		},
		incomingCalls:       make(map[dbus.ObjectPath]bool),
		activeNotifications: trackedNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.SetKnockActive(true); err != nil {
		t.Fatal(err)
	}
	if got := ledController.lastPattern.Name; got != patterns.RapidBlink.Name {
		t.Fatalf("active knock selected %q, want %q", got, patterns.RapidBlink.Name)
	}
	if err := receiver.SetKnockActive(true); err != nil {
		t.Fatal(err)
	}
	if ledController.plays != 1 {
		t.Fatalf("repeated active event played %d patterns, want 1", ledController.plays)
	}

	receiver.incomingCalls["/calls/one"] = false
	if err := receiver.updateLED(); err != nil {
		t.Fatal(err)
	}
	if got := ledController.lastPattern.Name; got != patterns.RapidBlink.Name {
		t.Fatalf("incoming call selected %q, want active knock %q", got, patterns.RapidBlink.Name)
	}
	delete(receiver.incomingCalls, "/calls/one")
	if err := receiver.updateLED(); err != nil {
		t.Fatal(err)
	}
	if got := ledController.lastPattern.Name; got != patterns.RapidBlink.Name {
		t.Fatalf("call ending selected %q, want active knock %q", got, patterns.RapidBlink.Name)
	}
	if err := receiver.SetKnockActive(false); err != nil {
		t.Fatal(err)
	}
	if got := ledController.lastPattern.Name; got != patterns.QuickFlash.Name {
		t.Fatalf("knock ending selected %q, want active notification %q", got, patterns.QuickFlash.Name)
	}
}

func TestKnockEndingDoesNotStopActiveCall(t *testing.T) {
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:        config.NewNilLogger(),
		ledController: ledController,
		config: &config.Config{
			CallPattern: patterns.SlowBlink,
			Knock:       config.Knock{Pattern: patterns.RapidBlink},
		},
		incomingCalls: map[dbus.ObjectPath]bool{"/calls/one": true},
		now:           time.Now,
	}
	if err := receiver.SetKnockActive(true); err != nil {
		t.Fatal(err)
	}
	if err := receiver.SetKnockActive(false); err != nil {
		t.Fatal(err)
	}
	if got := ledController.lastPattern.Name; got != patterns.SlowBlink.Name || ledController.stopped {
		t.Fatalf("knock ending selected %q (stopped=%t), want call %q", got, ledController.stopped, patterns.SlowBlink.Name)
	}
}

func TestKnockEndingRestoresOffPattern(t *testing.T) {
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:        config.NewNilLogger(),
		ledController: ledController,
		config:        &config.Config{Knock: config.Knock{Pattern: patterns.RapidBlink}, OffPattern: patterns.Off},
		now:           time.Now,
	}
	if err := receiver.SetKnockActive(true); err != nil {
		t.Fatal(err)
	}
	if err := receiver.SetKnockActive(false); err != nil {
		t.Fatal(err)
	}
	if got := ledController.lastPattern.Name; got != patterns.Off.Name {
		t.Fatalf("knock ending selected %q, want %q", got, patterns.Off.Name)
	}
}

func TestUpdateLEDAppliesCallPattern(t *testing.T) {
	normal := patterns.SlowBlink
	favourite := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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

func TestUpdateLEDAppliesConfiguredCustomPattern(t *testing.T) {
	custom := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:        config.NewNilLogger(),
		ledController: ledController,
		config: &config.Config{
			NotificationPattern: patterns.SlowBlink,
			CustomActivePatterns: map[string]patterns.Pattern{
				"example-app": custom,
			},
			Events: map[config.EventType]bool{
				config.EventNotification: false,
			},
		},
		activeNotifications: map[uint32]trackedNotification{
			1: {customFilters: map[string]struct{}{"example-app": {}}},
		},
		now: time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != custom.Name {
		t.Fatalf("applied pattern = %q, want configured custom pattern %q", ledController.lastPattern.Name, custom.Name)
	}
}

func TestUpdateLEDUsesCustomPatternBeforeBuiltInNotificationPattern(t *testing.T) {
	custom := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:        config.NewNilLogger(),
		ledController: ledController,
		config: &config.Config{
			NotificationPattern: patterns.SlowBlink,
			SMSPattern:          &patterns.GentleGlow,
			CustomActivePatterns: map[string]patterns.Pattern{
				"example-app": custom,
			},
			Events: map[config.EventType]bool{
				config.EventSMS: true,
			},
		},
		activeNotifications: map[uint32]trackedNotification{
			1: {
				filters:       map[string]struct{}{string(config.EventSMS): {}},
				customFilters: map[string]struct{}{"example-app": {}},
			},
		},
		now: time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != custom.Name {
		t.Fatalf("applied pattern = %q, want custom pattern %q", ledController.lastPattern.Name, custom.Name)
	}
}

func TestUpdateLEDAppliesSMSPatternWhenGenericNotificationsAreDisabled(t *testing.T) {
	sms := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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

func TestUpdateLEDAppliesOffPatternWhenNotificationEventIsMissing(t *testing.T) {
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:              config.NewNilLogger(),
		ledController:       ledController,
		config:              &config.Config{NotificationPattern: patterns.RapidBlink, OffPattern: patterns.Off},
		activeNotifications: trackedNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != patterns.Off.Name {
		t.Fatalf("applied pattern = %q, want off pattern %q", ledController.lastPattern.Name, patterns.Off.Name)
	}
}

func TestUpdateLEDKeepsCallPatternOverNotificationPattern(t *testing.T) {
	call := patterns.SlowBlink
	notification := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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
		logger:        config.NewNilLogger(),
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

func TestUpdateLEDAppliesOffPatternWhenAllActiveEventsAreDisabled(t *testing.T) {
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:        config.NewNilLogger(),
		ledController: ledController,
		config: &config.Config{
			OffPattern: patterns.Off,
			Events: map[config.EventType]bool{
				config.EventCall:         false,
				config.EventFavoriteCall: false,
				config.EventNotification: false,
			},
		},
		incomingCalls:       map[dbus.ObjectPath]bool{"/calls/one": false},
		activeNotifications: trackedNotifications(1),
		now:                 time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != patterns.Off.Name {
		t.Fatalf("applied pattern = %q, want off pattern %q", ledController.lastPattern.Name, patterns.Off.Name)
	}
}

func TestUpdateLEDAppliesOffPatternWhenThereAreNoEvents(t *testing.T) {
	ledController := &recordingLEDController{}
	receiver := &controller{
		logger:        config.NewNilLogger(),
		ledController: ledController,
		config:        &config.Config{OffPattern: patterns.Off},
		now:           time.Now,
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != patterns.Off.Name {
		t.Fatalf("applied pattern = %q, want off pattern %q", ledController.lastPattern.Name, patterns.Off.Name)
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
