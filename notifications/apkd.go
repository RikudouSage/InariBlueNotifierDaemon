package notifications

import "slices"

const (
	nemoOriginPackageHint = "x-nemo-origin-package"
	apkdBridgeOwner       = "apkd-bridge"
)

func isFromAndroidPackage(notification Notification, packages ...string) bool {
	if !isNemoOwner(notification, apkdBridgeOwner) {
		return false
	}

	originPackage, ok := notification.Hints[nemoOriginPackageHint]
	if !ok {
		return false
	}
	packageValue, ok := originPackage.Value().(string)
	if !ok {
		return false
	}

	return slices.Contains(packages, packageValue)
}
