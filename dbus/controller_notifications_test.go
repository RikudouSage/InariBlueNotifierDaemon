package dbus

import (
	"InariBlueNotifier/notifications"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestNotificationLifecycleTracksUntilClosed(t *testing.T) {
	receiver := &controller{
		notificationMatcher:      notifications.DefaultMatcher(),
		activeNotifications:      make(map[uint32]trackedNotification),
		pendingNotificationCalls: make(map[uint32]trackedNotification),
	}

	// Locally constructed D-Bus messages use serial zero. It is sufficient here
	// to verify the serial/reply_serial correlation logic.
	if changed := receiver.handleNotificationMessage(&dbus.Message{
		Type: dbus.TypeMethodCall,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldInterface: dbus.MakeVariant(notificationsInterface),
			dbus.FieldMember:    dbus.MakeVariant(notificationNotify),
		},
		Body: validNotifyBody(nil),
	}); changed {
		t.Fatal("Notify call changed active notification state before its reply")
	}

	if changed := receiver.handleNotificationMessage(&dbus.Message{
		Type: dbus.TypeMethodReply,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldReplySerial: dbus.MakeVariant(uint32(0)),
		},
		Body: []any{uint32(42)},
	}); !changed {
		t.Fatal("Notify reply did not activate notification")
	}
	if _, ok := receiver.activeNotifications[42]; !ok {
		t.Fatal("notification ID was not tracked")
	}

	if changed := receiver.handleNotificationMessage(&dbus.Message{
		Type: dbus.TypeSignal,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldInterface: dbus.MakeVariant(notificationsInterface),
			dbus.FieldMember:    dbus.MakeVariant(notificationClosed),
		},
		Body: []any{uint32(42), uint32(2)},
	}); !changed {
		t.Fatal("NotificationClosed did not update active notification state")
	}
	if len(receiver.activeNotifications) != 0 {
		t.Fatalf("active notifications = %v, want none", receiver.activeNotifications)
	}
}

func TestNotificationCloseKeepsBlinkingForOtherNotification(t *testing.T) {
	receiver := &controller{
		notificationMatcher: notifications.DefaultMatcher(),
		activeNotifications: trackedNotifications(1, 2),
	}

	changed := receiver.handleNotificationMessage(&dbus.Message{
		Type: dbus.TypeSignal,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldInterface: dbus.MakeVariant(notificationsInterface),
			dbus.FieldMember:    dbus.MakeVariant(notificationClosed),
		},
		Body: []any{uint32(1), uint32(2)},
	})
	if !changed || len(receiver.activeNotifications) != 1 {
		t.Fatalf("close result changed=%t active=%v, want one active notification", changed, receiver.activeNotifications)
	}
}

func TestSMSNotificationTracksSMSFilter(t *testing.T) {
	receiver := &controller{
		notificationMatcher:      notifications.DefaultMatcher(),
		activeNotifications:      make(map[uint32]trackedNotification),
		pendingNotificationCalls: make(map[uint32]trackedNotification),
	}

	receiver.handleNotificationMessage(&dbus.Message{
		Type: dbus.TypeMethodCall,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldInterface: dbus.MakeVariant(notificationsInterface),
			dbus.FieldMember:    dbus.MakeVariant(notificationNotify),
		},
		Body: validNotifyBody(map[string]dbus.Variant{
			"x-nemo-feedback": dbus.MakeVariant("sms,sms_exists"),
		}),
	})
	receiver.handleNotificationMessage(&dbus.Message{
		Type: dbus.TypeMethodReply,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldReplySerial: dbus.MakeVariant(uint32(0)),
		},
		Body: []any{uint32(42)},
	})

	if !receiver.hasActiveNotificationFilter("sms") {
		t.Fatal("active SMS notification did not retain the SMS filter")
	}
}

func TestEmailNotificationTracksEmailFilter(t *testing.T) {
	receiver := &controller{
		notificationMatcher:      notifications.DefaultMatcher(),
		activeNotifications:      make(map[uint32]trackedNotification),
		pendingNotificationCalls: make(map[uint32]trackedNotification),
	}

	receiver.handleNotificationMessage(&dbus.Message{
		Type: dbus.TypeMethodCall,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldInterface: dbus.MakeVariant(notificationsInterface),
			dbus.FieldMember:    dbus.MakeVariant(notificationNotify),
		},
		Body: validNotifyBody(map[string]dbus.Variant{
			"x-nemo-feedback": dbus.MakeVariant("email_exists"),
		}),
	})
	receiver.handleNotificationMessage(&dbus.Message{
		Type: dbus.TypeMethodReply,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldReplySerial: dbus.MakeVariant(uint32(0)),
		},
		Body: []any{uint32(42)},
	})

	if !receiver.hasActiveNotificationFilter("email") {
		t.Fatal("active email notification did not retain the email filter")
	}
}

func TestNonRemovableNotificationIsIgnored(t *testing.T) {
	receiver := &controller{
		notificationMatcher:      notifications.DefaultMatcher(),
		activeNotifications:      make(map[uint32]trackedNotification),
		pendingNotificationCalls: make(map[uint32]trackedNotification),
	}

	if changed := receiver.handleNotificationMessage(&dbus.Message{
		Type: dbus.TypeMethodCall,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldInterface: dbus.MakeVariant(notificationsInterface),
			dbus.FieldMember:    dbus.MakeVariant(notificationNotify),
		},
		// Sailfish marks notifications that cannot be swiped away with this hint.
		Body: []any{"app", uint32(0), "", "summary", "body", []string{}, map[string]dbus.Variant{
			"x-nemo-user-removable": dbus.MakeVariant(false),
		}, int32(-1)},
	}); changed {
		t.Fatal("non-removable Notify call changed active notification state")
	}
	if len(receiver.pendingNotificationCalls) != 0 {
		t.Fatalf("pending notifications = %v, want none", receiver.pendingNotificationCalls)
	}

	if changed := receiver.handleNotificationMessage(&dbus.Message{
		Type: dbus.TypeMethodReply,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldReplySerial: dbus.MakeVariant(uint32(0)),
		},
		Body: []any{uint32(42)},
	}); changed {
		t.Fatal("non-removable notification reply activated notification")
	}
	if len(receiver.activeNotifications) != 0 {
		t.Fatalf("active notifications = %v, want none", receiver.activeNotifications)
	}
}

func validNotifyBody(hints map[string]dbus.Variant) []any {
	if hints == nil {
		hints = make(map[string]dbus.Variant)
	}
	return []any{"app", uint32(0), "", "summary", "body", []string{}, hints, int32(-1)}
}

func trackedNotifications(ids ...uint32) map[uint32]trackedNotification {
	result := make(map[uint32]trackedNotification, len(ids))
	for _, id := range ids {
		result[id] = trackedNotification{}
	}
	return result
}
