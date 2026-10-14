package notifications

import "InariBlueNotifier/dto"

type customFilter struct {
	filter dto.CustomAppFilter
}

func newCustomFilter(filter dto.CustomAppFilter) *customFilter {
	return &customFilter{
		filter: filter,
	}
}

func (receiver *customFilter) Name() string {
	return receiver.filter.FilterName
}

func (receiver *customFilter) Matches(notification Notification) bool {
	return isFromAndroidPackage(notification, receiver.filter.AndroidPackageNames...) ||
		isNemoOwner(notification, receiver.filter.NemoOwners...)
}
