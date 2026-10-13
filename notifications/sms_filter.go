package notifications

import "strings"

type smsFilter struct{}

func (*smsFilter) Name() string {
	return "sms"
}

func (*smsFilter) Matches(notification Notification) bool {
	feedback, ok := notification.Hints["x-nemo-feedback"]
	if !ok {
		return false
	}

	value, ok := feedback.Value().(string)
	if !ok {
		return false
	}

	for token := range strings.SplitSeq(value, ",") {
		if strings.TrimSpace(token) == "sms" {
			return true
		}
	}

	return false
}
