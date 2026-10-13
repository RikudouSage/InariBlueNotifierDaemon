package notifications

type Matcher struct {
	filters []Filter
}

func NewMatcher(filters ...Filter) *Matcher {
	return &Matcher{filters: filters}
}

func DefaultMatcher() *Matcher {
	return NewMatcher(
		&smsFilter{},
		&emailFilter{},
	)
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
