package notifications

type matrixFilter struct{}

func (*matrixFilter) Name() string {
	return "matrix"
}

func (*matrixFilter) Matches(notification Notification) bool {
	return isNemoOwner(notification, "xmatic") || isFromAndroidPackage(notification, "io.element.android.x", "im.vector.app")
}
