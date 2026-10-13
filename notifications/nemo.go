package notifications

import "slices"

const nemoOwnerHint = "x-nemo-owner"

func isNemoOwner(notification Notification, owners ...string) bool {
	owner, ok := notification.Hints[nemoOwnerHint]
	if !ok {
		return false
	}

	ownerValue, ok := owner.Value().(string)
	return ok && slices.Contains(owners, ownerValue)
}
