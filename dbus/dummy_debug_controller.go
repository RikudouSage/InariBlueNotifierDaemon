//go:build !debug_dbus

package dbus

import (
	"InariBlueNotifier/led"

	"github.com/godbus/dbus/v5"
)

func NewDebugController(_ led.Controller) DebugController {
	return &dummyDebugController{}
}

type dummyDebugController struct{}

func (receiver *dummyDebugController) SetState(_ uint8) *dbus.Error {
	return nil
}

func (receiver *dummyDebugController) Release() *dbus.Error {
	return nil
}

func (receiver *dummyDebugController) Shutdown() *dbus.Error {
	return nil
}

func (receiver *dummyDebugController) Register(_ *dbus.Conn) error {
	return nil
}

func (receiver *dummyDebugController) Unregister(connection *dbus.Conn) error {
	return nil
}
