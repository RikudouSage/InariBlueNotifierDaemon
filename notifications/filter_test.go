package notifications

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestIsNemoOwner(t *testing.T) {
	notification := Notification{Hints: map[string]dbus.Variant{
		nemoOwnerHint: dbus.MakeVariant("example-owner"),
	}}

	if !isNemoOwner(notification, "example-owner") {
		t.Fatal("Nemo owner did not match")
	}
	if isNemoOwner(notification, "another-owner") {
		t.Fatal("Nemo owner matched a different owner")
	}
}

func TestIsNemoOwnerRejectsMissingOrNonStringHint(t *testing.T) {
	for _, hints := range []map[string]dbus.Variant{
		{},
		{nemoOwnerHint: dbus.MakeVariant(true)},
	} {
		if isNemoOwner(Notification{Hints: hints}, "example-owner") {
			t.Fatalf("Nemo owner matched hints %#v", hints)
		}
	}
}

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

func TestEmailMatchesFeedbackTokens(t *testing.T) {
	for _, feedback := range []string{"email", "email_exists"} {
		t.Run(feedback, func(t *testing.T) {
			notification := Notification{Hints: map[string]dbus.Variant{
				"x-nemo-feedback": dbus.MakeVariant(feedback),
			}}

			if !(&emailFilter{}).Matches(notification) {
				t.Fatalf("email filter did not match %q feedback token", feedback)
			}
		})
	}
}

func TestEmailDoesNotMatchPartialFeedbackToken(t *testing.T) {
	notification := Notification{Hints: map[string]dbus.Variant{
		"x-nemo-feedback": dbus.MakeVariant("voicemail_exists"),
	}}

	if (&emailFilter{}).Matches(notification) {
		t.Fatal("email filter matched a non-email feedback token")
	}
}

func TestDefaultMatcherIncludesEmailFilter(t *testing.T) {
	matches := DefaultMatcher().Match(Notification{Hints: map[string]dbus.Variant{
		"x-nemo-feedback": dbus.MakeVariant("email_exists"),
	}})

	if _, ok := matches["email"]; !ok {
		t.Fatal("default matcher did not classify an email notification")
	}
}

func TestSignalMatchesSignalAndMollyAndroidNotifications(t *testing.T) {
	for _, packageName := range []string{"org.thoughtcrime.securesms", "im.molly.app"} {
		t.Run(packageName, func(t *testing.T) {
			notification := Notification{Hints: map[string]dbus.Variant{
				nemoOwnerHint:         dbus.MakeVariant(apkdBridgeOwner),
				nemoOriginPackageHint: dbus.MakeVariant(packageName),
			}}

			if !(&signalFilter{}).Matches(notification) {
				t.Fatalf("Signal filter did not match Android package %q", packageName)
			}
		})
	}
}

func TestSignalRequiresAPKDBridgeAndKnownPackage(t *testing.T) {
	for _, hints := range []map[string]dbus.Variant{
		{
			nemoOriginPackageHint: dbus.MakeVariant("org.thoughtcrime.securesms"),
		},
		{
			nemoOwnerHint:         dbus.MakeVariant("another-owner"),
			nemoOriginPackageHint: dbus.MakeVariant("org.thoughtcrime.securesms"),
		},
		{
			nemoOwnerHint:         dbus.MakeVariant(apkdBridgeOwner),
			nemoOriginPackageHint: dbus.MakeVariant("io.element.android.x"),
		},
	} {
		if (&signalFilter{}).Matches(Notification{Hints: hints}) {
			t.Fatalf("Signal filter matched hints %#v", hints)
		}
	}
}

func TestDefaultMatcherIncludesSignalFilter(t *testing.T) {
	matches := DefaultMatcher().Match(Notification{Hints: map[string]dbus.Variant{
		nemoOwnerHint:         dbus.MakeVariant(apkdBridgeOwner),
		nemoOriginPackageHint: dbus.MakeVariant("im.molly.app"),
	}})

	if _, ok := matches["signal"]; !ok {
		t.Fatal("default matcher did not classify a Molly notification as Signal")
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
