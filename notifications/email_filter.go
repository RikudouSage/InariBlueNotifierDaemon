package notifications

import "strings"

type emailFilter struct{}

func (*emailFilter) Name() string {
	return "email"
}

func (*emailFilter) Matches(notification Notification) bool {
	if isFromAndroidPackage(
		notification,
		"net.thunderbird.android",
		"net.thunderbird.android.beta",
		"com.fsck.k9",
		"eu.faircode.email",
		"ch.protonmail.android",
		"de.tutao.tutanota",
	) {
		return true
	}

	feedback, ok := notification.Hints["x-nemo-feedback"]
	if !ok {
		return false
	}

	value, ok := feedback.Value().(string)
	if !ok {
		return false
	}

	for token := range strings.SplitSeq(value, ",") {
		switch strings.TrimSpace(token) {
		case "email", "email_exists":
			return true
		}
	}

	return false
}
