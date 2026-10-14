package notifications

type matrixFilter struct{}

func (*matrixFilter) Name() string {
	return "matrix"
}

func (*matrixFilter) Matches(notification Notification) bool {
	return isNemoOwner(
		notification,
		"xmatic",
		"sailtrix",
	) || isFromAndroidPackage(
		notification,
		"io.element.android.x",
		"im.vector.app",
		"chat.fluffy.fluffychat",
		"chat.schildi.android",
		"de.spiritcroc.riotx",
		"io.element.enterprise",
		"de.connect2x.tammy",
		"chat.schildi.revenge",
	)
}
