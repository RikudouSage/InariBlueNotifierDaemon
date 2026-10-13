package notifications

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestSMSMatchesFeedbackTokenAmongAdditionalTokens(t *testing.T) {
	notification := Notification{Hints: map[string]dbus.Variant{
		"x-nemo-feedback": dbus.MakeVariant("sms,sms_exists"),
	}}

	if !(&smsFilter{}).Matches(notification) {
		t.Fatal("SMS filter did not match sms feedback token")
	}
}

func TestSMSDoesNotMatchPartialFeedbackToken(t *testing.T) {
	notification := Notification{Hints: map[string]dbus.Variant{
		"x-nemo-feedback": dbus.MakeVariant("mms_exists"),
	}}

	if (&smsFilter{}).Matches(notification) {
		t.Fatal("SMS filter matched a non-SMS feedback token")
	}
}

func TestFromNotify(t *testing.T) {
	notification, ok := FromNotify([]any{
		"Messages", uint32(0), "icon-lock-sms", "Sender", "Text", []string{"default", ""}, map[string]dbus.Variant{}, int32(-1),
	})
	if !ok {
		t.Fatal("valid Notify body was rejected")
	}
	if notification.AppIcon != "icon-lock-sms" || notification.Summary != "Sender" {
		t.Fatalf("parsed notification = %#v", notification)
	}
}
