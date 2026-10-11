package dbus

import (
	"InariBlueNotifier/led"
	"InariBlueNotifier/patterns"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	externalDBusName      = "dev.rikudou.InariBlueNotifier"
	externalDBusObject    = "/dev/rikudou/InariBlueNotifier"
	externalDBusInterface = "dev.rikudou.InariBlueNotifier"
)

type ExternalController struct {
	ledController led.Controller
}

func NewExternalController(ledController led.Controller) *ExternalController {
	return &ExternalController{
		ledController: ledController,
	}
}

func (receiver *ExternalController) SetState(state uint8) *dbus.Error {
	if err := receiver.ledController.SetState(led.State(state)); err != nil {
		return dbus.MakeFailedError(err)
	}

	return nil
}

func (receiver *ExternalController) PlayPattern(pattern patterns.Pattern) *dbus.Error {
	if err := receiver.ledController.PlayPattern(pattern); err != nil {
		return dbus.MakeFailedError(err)
	}

	return nil
}

func (receiver *ExternalController) Stop() *dbus.Error {
	if err := receiver.ledController.Stop(); err != nil {
		return dbus.MakeFailedError(err)
	}

	return nil
}

// todo decide whether I want these or not
//func (receiver *ExternalController) Release() *dbus.Error {
//	if err := receiver.ledController.Release(); err != nil {
//		return dbus.MakeFailedError(err)
//	}
//
//	return nil
//}

//func (receiver *ExternalController) Shutdown() *dbus.Error {
//	if err := receiver.ledController.Shutdown(); err != nil {
//		return dbus.MakeFailedError(err)
//	}
//
//	return nil
//}

func (receiver *ExternalController) Register(connection *dbus.Conn) error {
	if err := connection.Export(
		receiver,
		externalDBusObject,
		externalDBusInterface,
	); err != nil {
		return fmt.Errorf("export external D-Bus interface: %w", err)
	}

	reply, err := connection.RequestName(
		externalDBusName,
		dbus.NameFlagDoNotQueue,
	)
	if err != nil {
		return fmt.Errorf("request external D-Bus name: %w", err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("external D-Bus name already owned")
	}

	return nil
}

func (receiver *ExternalController) Unregister(connection *dbus.Conn) error {
	if err := connection.Export(nil, externalDBusObject, externalDBusInterface); err != nil {
		return fmt.Errorf("unexport external D-Bus interface: %w", err)
	}

	if _, err := connection.ReleaseName(externalDBusName); err != nil {
		return fmt.Errorf("release external D-Bus name: %w", err)
	}

	return nil
}
