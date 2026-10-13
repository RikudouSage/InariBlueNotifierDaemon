package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/godbus/dbus/v5"
)

const (
	notificationsInterface = "org.freedesktop.Notifications"
	notificationNotify     = "Notify"
	notificationClosed     = "NotificationClosed"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Fatalf("connect to session bus: %v", err)
	}
	defer connection.Close()

	if err := addNotificationMatches(connection); err != nil {
		log.Fatalf("subscribe to notification events: %v", err)
	}

	messages := make(chan *dbus.Message, 64)
	connection.Eavesdrop(messages)
	defer connection.Eavesdrop(nil)

	fmt.Fprintln(os.Stderr, "Listening for notification events; press Ctrl+C to stop.")
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok {
				log.Print("notification monitor channel closed")
				return
			}
			if message != nil {
				printMessage(message)
			}
		}
	}
}

func addNotificationMatches(connection *dbus.Conn) error {
	rules := []string{
		"type='method_call',interface='org.freedesktop.Notifications',member='Notify',eavesdrop='true'",
		"type='method_return',sender='org.freedesktop.Notifications',eavesdrop='true'",
		"type='signal',interface='org.freedesktop.Notifications',member='NotificationClosed',eavesdrop='true'",
	}

	for _, rule := range rules {
		if call := connection.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule); call.Err != nil {
			return fmt.Errorf("add monitor match %q: %w", rule, call.Err)
		}
	}
	return nil
}

func printMessage(message *dbus.Message) {
	fmt.Printf("\n--- %s ---\n%s\nraw: %#v\n", eventName(message), message, message)
}

func eventName(message *dbus.Message) string {
	switch message.Type {
	case dbus.TypeMethodCall:
		return notificationNotify
	case dbus.TypeMethodReply:
		return notificationNotify + " reply"
	case dbus.TypeSignal:
		return notificationClosed
	default:
		return message.Type.String()
	}
}
