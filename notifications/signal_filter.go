package notifications

type signalFilter struct{}

func (*signalFilter) Name() string {
	return "signal"
}

func (*signalFilter) Matches(notification Notification) bool {
	return isFromAndroidPackage(
		notification,
		"org.thoughtcrime.securesms",
		"im.molly.app",
	)
}
