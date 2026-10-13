package notifications

import "github.com/godbus/dbus/v5"

type Notification struct {
	AppName string
	AppIcon string
	Summary string
	Body    string
	Actions []string
	Hints   map[string]dbus.Variant
}

// FromNotify decodes the standard Notify body. It returns false for malformed
// messages, which must not be treated as a filter match.
func FromNotify(body []any) (Notification, bool) {
	const (
		appNameIndex = iota
		replacesIDIndex
		appIconIndex
		summaryIndex
		bodyIndex
		actionsIndex
		hintsIndex
	)

	if len(body) <= hintsIndex {
		return Notification{}, false
	}

	appName, appNameOK := body[appNameIndex].(string)
	appIcon, appIconOK := body[appIconIndex].(string)
	summary, summaryOK := body[summaryIndex].(string)
	notificationBody, bodyOK := body[bodyIndex].(string)
	actions, actionsOK := body[actionsIndex].([]string)
	hints, hintsOK := body[hintsIndex].(map[string]dbus.Variant)
	if !appNameOK || !appIconOK || !summaryOK || !bodyOK || !actionsOK || !hintsOK {
		return Notification{}, false
	}

	return Notification{
		AppName: appName,
		AppIcon: appIcon,
		Summary: summary,
		Body:    notificationBody,
		Actions: actions,
		Hints:   hints,
	}, true
}

func (receiver Notification) IsUserRemovable() bool {
	userRemovable, ok := receiver.Hints["x-nemo-user-removable"]
	if !ok {
		return true
	}

	removable, ok := userRemovable.Value().(bool)
	return !ok || removable
}
