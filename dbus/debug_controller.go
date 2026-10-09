package dbus

import "github.com/godbus/dbus/v5"

const (
	debugBusName   = "dev.rikudou.InariBlueNotifier.Debug"
	debugObject    = "/dev/rikudou/InariBlueNotifier/Debug"
	debugInterface = "dev.rikudou.InariBlueNotifier.Debug"
)

type DebugController interface {
	SetState(state uint8) *dbus.Error
	Release() *dbus.Error
	Shutdown() *dbus.Error
	Register(connection *dbus.Conn) error
	Unregister(connection *dbus.Conn) error
}
