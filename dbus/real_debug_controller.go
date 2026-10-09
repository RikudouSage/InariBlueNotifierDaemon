//go:build debug_dbus

package dbus

import (
	"InariBlueNotifier/led"
	"fmt"

	"github.com/godbus/dbus/v5"
)

type debugController struct {
	ledController led.Controller
}

func NewDebugController(ledController led.Controller) DebugController {
	return &debugController{
		ledController: ledController,
	}
}

func (receiver *debugController) SetState(state uint8) *dbus.Error {
	if err := receiver.ledController.SetState(led.State(state)); err != nil {
		return dbus.MakeFailedError(err)
	}

	return nil
}

func (receiver *debugController) Release() *dbus.Error {
	if err := receiver.ledController.Release(); err != nil {
		return dbus.MakeFailedError(err)
	}

	return nil
}

func (receiver *debugController) Shutdown() *dbus.Error {
	if err := receiver.ledController.Shutdown(); err != nil {
		return dbus.MakeFailedError(err)
	}

	return nil
}

func (receiver *debugController) Register(connection *dbus.Conn) error {
	if err := connection.Export(
		receiver,
		debugObject,
		debugInterface,
	); err != nil {
		return fmt.Errorf("export debug D-Bus interface: %w", err)
	}

	reply, err := connection.RequestName(
		debugBusName,
		dbus.NameFlagDoNotQueue,
	)
	if err != nil {
		return fmt.Errorf("request debug D-Bus name: %w", err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("debug D-Bus name already owned")
	}

	return nil
}

func (receiver *debugController) Unregister(connection *dbus.Conn) error {
	if err := connection.Export(nil, debugObject, debugInterface); err != nil {
		return fmt.Errorf("unexport debug D-Bus interface: %w", err)
	}

	if _, err := connection.ReleaseName(debugBusName); err != nil {
		return fmt.Errorf("release debug D-Bus name: %w", err)
	}

	return nil
}
