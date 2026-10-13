package notifications

type Filter interface {
	Name() string
	Matches(notification Notification) bool
}
