package notifications

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/dto"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestCustomFilterName(t *testing.T) {
	filter := newCustomFilter(dto.CustomAppFilter{FilterName: "custom-app"})

	if got := filter.Name(); got != "custom-app" {
		t.Errorf("Name() = %q, want %q", got, "custom-app")
	}
}

func TestCustomFilterMatchesConfiguredAndroidPackagesAndNemoOwners(t *testing.T) {
	filter := newCustomFilter(dto.CustomAppFilter{
		AndroidPackageNames: []string{"org.example.app"},
		NemoOwners:          []string{"native-example"},
	})

	tests := map[string]struct {
		hints map[string]dbus.Variant
		want  bool
	}{
		"configured Android package": {
			hints: map[string]dbus.Variant{
				nemoOwnerHint:         dbus.MakeVariant(apkdBridgeOwner),
				nemoOriginPackageHint: dbus.MakeVariant("org.example.app"),
			},
			want: true,
		},
		"configured Nemo owner": {
			hints: map[string]dbus.Variant{
				nemoOwnerHint: dbus.MakeVariant("native-example"),
			},
			want: true,
		},
		"Android package without APKD bridge": {
			hints: map[string]dbus.Variant{
				nemoOriginPackageHint: dbus.MakeVariant("org.example.app"),
			},
			want: false,
		},
		"unconfigured Android package and owner": {
			hints: map[string]dbus.Variant{
				nemoOwnerHint:         dbus.MakeVariant(apkdBridgeOwner),
				nemoOriginPackageHint: dbus.MakeVariant("org.example.other"),
			},
			want: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := filter.Matches(Notification{Hints: test.hints}); got != test.want {
				t.Errorf("Matches() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestMatcherReportsEnabledCustomMatch(t *testing.T) {
	matcher := DefaultMatcher(&config.Config{NotificationFilters: map[string]dto.CustomAppFilter{
		"example": {
			FilterName: "example",
			NemoOwners: []string{"native-example"},
			Enabled:    true,
		},
		"disabled": {
			FilterName: "disabled",
			NemoOwners: []string{"native-example"},
		},
	}})

	if !matcher.MatchesCustom(Notification{Hints: map[string]dbus.Variant{
		nemoOwnerHint: dbus.MakeVariant("native-example"),
	}}) {
		t.Fatal("enabled custom filter did not match")
	}
}

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
	matches := DefaultMatcher(&config.Config{}).Match(Notification{Hints: map[string]dbus.Variant{
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
	matches := DefaultMatcher(&config.Config{}).Match(Notification{Hints: map[string]dbus.Variant{
		nemoOwnerHint:         dbus.MakeVariant(apkdBridgeOwner),
		nemoOriginPackageHint: dbus.MakeVariant("im.molly.app"),
	}})

	if _, ok := matches["signal"]; !ok {
		t.Fatal("default matcher did not classify a Molly notification as Signal")
	}
}

func TestMatrixMatchesXmaticAndElementNotifications(t *testing.T) {
	for name, hints := range map[string]map[string]dbus.Variant{
		"native Xmatic": {
			nemoOwnerHint: dbus.MakeVariant("xmatic"),
		},
		"Element X": {
			nemoOwnerHint:         dbus.MakeVariant(apkdBridgeOwner),
			nemoOriginPackageHint: dbus.MakeVariant("io.element.android.x"),
		},
		"legacy Element": {
			nemoOwnerHint:         dbus.MakeVariant(apkdBridgeOwner),
			nemoOriginPackageHint: dbus.MakeVariant("im.vector.app"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			if !(&matrixFilter{}).Matches(Notification{Hints: hints}) {
				t.Fatalf("Matrix filter did not match hints %#v", hints)
			}
		})
	}
}

func TestMatrixRequiresKnownNativeOwnerOrAPKDPackage(t *testing.T) {
	for _, hints := range []map[string]dbus.Variant{
		{nemoOwnerHint: dbus.MakeVariant("another-owner")},
		{nemoOriginPackageHint: dbus.MakeVariant("io.element.android.x")},
		{
			nemoOwnerHint:         dbus.MakeVariant(apkdBridgeOwner),
			nemoOriginPackageHint: dbus.MakeVariant("org.thoughtcrime.securesms"),
		},
	} {
		if (&matrixFilter{}).Matches(Notification{Hints: hints}) {
			t.Fatalf("Matrix filter matched hints %#v", hints)
		}
	}
}

func TestDefaultMatcherIncludesMatrixFilter(t *testing.T) {
	matches := DefaultMatcher(&config.Config{}).Match(Notification{Hints: map[string]dbus.Variant{
		nemoOwnerHint: dbus.MakeVariant("xmatic"),
	}})

	if _, ok := matches["matrix"]; !ok {
		t.Fatal("default matcher did not classify an Xmatic notification as Matrix")
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
