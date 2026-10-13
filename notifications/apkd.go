package notifications

import "slices"

const (
	nemoOwnerHint         = "x-nemo-owner"
	nemoOriginPackageHint = "x-nemo-origin-package"
	apkdBridgeOwner       = "apkd-bridge"
)

func isFromAndroidPackage(notification Notification, packages ...string) bool {
	owner, ok := notification.Hints[nemoOwnerHint]
	if !ok {
		return false
	}
	ownerValue, ok := owner.Value().(string)
	if !ok || ownerValue != apkdBridgeOwner {
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
