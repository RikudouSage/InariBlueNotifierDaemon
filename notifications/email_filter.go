package notifications

import "strings"

type emailFilter struct{}

func (*emailFilter) Name() string {
	return "email"
}

func (*emailFilter) Matches(notification Notification) bool {
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
