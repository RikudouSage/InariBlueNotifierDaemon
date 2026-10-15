package dbus

import (
	"context"
	"fmt"
	"log"

	"InariBlueNotifier/notifications"

	"github.com/godbus/dbus/v5"
)

type trackedNotification struct {
	filters       map[string]struct{}
	customFilters map[string]struct{}
	customMatched bool
}

const (
	notificationsInterface = "org.freedesktop.Notifications"
	notificationNotify     = "Notify"
	notificationClosed     = "NotificationClosed"
)

// startNotificationMonitor uses a separate connection because Eavesdrop sends
// every message to its channel and disables the connection's normal D-Bus
// dispatching. The daemon's primary session-bus connection remains available
// for calls, contact lookups, and voice-call signals.
func (receiver *controller) startNotificationMonitor() (chan *dbus.Message, *dbus.Conn, error) {
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, nil, fmt.Errorf("connect to session bus: %w", err)
	}

	rules := []string{
		"type='method_call',interface='org.freedesktop.Notifications',member='Notify',eavesdrop='true'",
		"type='method_return',sender='org.freedesktop.Notifications',eavesdrop='true'",
		"type='signal',interface='org.freedesktop.Notifications',member='NotificationClosed',eavesdrop='true'",
	}
	for _, rule := range rules {
		if call := connection.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule); call.Err != nil {
			_ = connection.Close()
			return nil, nil, fmt.Errorf("add monitor match %q: %w", rule, call.Err)
		}
	}

	messages := make(chan *dbus.Message, 64)
	connection.Eavesdrop(messages)
	log.Printf("subscribed to notification lifecycle events")

	return messages, connection, nil
}

func (receiver *controller) watchNotifications(
	ctx context.Context,
	messages <-chan *dbus.Message,
	connection *dbus.Conn,
) {
	defer connection.Eavesdrop(nil)
	defer connection.Close()

	for {
		select {
		case <-ctx.Done():
			return

		case message, ok := <-messages:
			if !ok {
				log.Printf("notification monitor channel closed")
				return
			}
			if message == nil {
				continue
			}

			receiver.eventMutex.Lock()
			changed := receiver.handleNotificationMessage(message)
			if changed {
				if err := receiver.updateLED(); err != nil {
					log.Printf("LED update failed after notification event: active_notifications=%d: %v", len(receiver.activeNotifications), err)
				}
			}

			receiver.eventMutex.Unlock()
		}
	}
}

// handleNotificationMessage tracks only notifications whose complete lifecycle
// was observed. A Notify call is paired with its method reply via the D-Bus
// serial and reply_serial headers; the reply contains the server-assigned ID
// later emitted by NotificationClosed.
func (receiver *controller) handleNotificationMessage(message *dbus.Message) (changed bool) {
	switch message.Type {
	case dbus.TypeMethodCall:
		if messageInterface(message) != notificationsInterface || messageMember(message) != notificationNotify {
			return false
		}
		notification, ok := notifications.FromNotify(message.Body)
		if !ok {
			log.Printf("ignored notification with invalid Notify body: serial=%d body=%#v", message.Serial(), message.Body)
			return false
		}
		if !notification.IsUserRemovable() {
			log.Printf("ignored non-removable notification: serial=%d", message.Serial())
			return false
		}

		customFilters := receiver.notificationMatcher.MatchCustom(notification)
		receiver.pendingNotificationCalls[message.Serial()] = trackedNotification{
			filters:       receiver.notificationMatcher.Match(notification),
			customFilters: customFilters,
			customMatched: len(customFilters) > 0,
		}
		log.Printf("notification notify: serial=%d pending=%d", message.Serial(), len(receiver.pendingNotificationCalls))

		return false

	case dbus.TypeMethodReply:
		replySerial, ok := messageReplySerial(message)
		if !ok {
			return false
		}

		pending, ok := receiver.pendingNotificationCalls[replySerial]
		if !ok {
			return false
		}
		delete(receiver.pendingNotificationCalls, replySerial)

		id, ok := notificationID(message.Body)
		if !ok {
			log.Printf("ignored notification reply with invalid ID: reply_serial=%d body=%#v", replySerial, message.Body)
			return false
		}

		_, alreadyActive := receiver.activeNotifications[id]
		receiver.activeNotifications[id] = pending
		log.Printf("notification active: id=%d reply_serial=%d active_notifications=%d", id, replySerial, len(receiver.activeNotifications))

		return !alreadyActive

	case dbus.TypeSignal:
		if messageInterface(message) != notificationsInterface || messageMember(message) != notificationClosed {
			return false
		}

		id, ok := notificationID(message.Body)
		if !ok {
			log.Printf("ignored notification close with invalid ID: body=%#v", message.Body)
			return false
		}

		if _, active := receiver.activeNotifications[id]; !active {
			return false
		}
		delete(receiver.activeNotifications, id)
		log.Printf("notification closed: id=%d active_notifications=%d", id, len(receiver.activeNotifications))

		return true
	}

	return false
}

func messageInterface(message *dbus.Message) string {
	value, ok := message.Headers[dbus.FieldInterface]
	if !ok {
		return ""
	}
	interfaceName, _ := value.Value().(string)

	return interfaceName
}

func messageMember(message *dbus.Message) string {
	value, ok := message.Headers[dbus.FieldMember]
	if !ok {
		return ""
	}
	member, _ := value.Value().(string)
	return member
}

func messageReplySerial(message *dbus.Message) (uint32, bool) {
	value, ok := message.Headers[dbus.FieldReplySerial]
	if !ok {
		return 0, false
	}
	serial, ok := value.Value().(uint32)
	return serial, ok
}

func notificationID(body []any) (uint32, bool) {
	if len(body) == 0 {
		return 0, false
	}
	id, ok := body[0].(uint32)
	return id, ok
}
