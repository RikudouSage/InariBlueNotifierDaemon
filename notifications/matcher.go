package notifications

import "InariBlueNotifier/config"

type Matcher struct {
	filters []Filter
}

func NewMatcher(filters ...Filter) *Matcher {
	return &Matcher{filters: filters}
}

func DefaultMatcher(config *config.Config) *Matcher {
	filters := make([]Filter, 0, len(config.NotificationFilters)+4)
	for _, filter := range config.NotificationFilters {
		if !filter.Enabled {
			continue
		}
		filters = append(filters, newCustomFilter(filter))
	}
	filters = append(
		filters,
		&smsFilter{},
		&emailFilter{},
		&signalFilter{},
		&matrixFilter{},
	)

	return NewMatcher(filters...)
}

func (receiver *Matcher) Match(notification Notification) map[string]struct{} {
	matched := make(map[string]struct{})
	for _, filter := range receiver.filters {
		if filter.Matches(notification) {
			matched[filter.Name()] = struct{}{}
		}
	}

	return matched
}
