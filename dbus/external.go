package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/led"
	"InariBlueNotifier/patterns"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	externalDBusName      = "dev.rikudou.InariBlueNotifier"
	externalDBusObject    = "/dev/rikudou/InariBlueNotifier"
	externalDBusInterface = "dev.rikudou.InariBlueNotifier"

	KnockDisabled  = "disabled"
	KnockStarting  = "starting"
	KnockListening = "listening"
	KnockFailed    = "failed"
	KnockStopped   = "stopped"
)

type ExternalController struct {
	ledController led.Controller

	statusMutex sync.Mutex
	statusConn  *dbus.Conn
	knockStatus string
	knockActive bool
	knockError  string

	logger *config.Logger
}

func NewExternalController(ledController led.Controller, logger *config.Logger) *ExternalController {
	return &ExternalController{
		ledController: ledController,
		knockStatus:   KnockDisabled,
		logger:        logger,
	}
}

// GetKnockStatus returns listener state, whether a knock is active, and an
// error message (empty except when the listener has failed). The same three
// values are sent in the KnockStatusChanged D-Bus signal.
func (receiver *ExternalController) GetKnockStatus() (status string, active bool, err string, dbusErr *dbus.Error) {
	receiver.statusMutex.Lock()
	defer receiver.statusMutex.Unlock()

	return receiver.knockStatus, receiver.knockActive, receiver.knockError, nil
}

// UpdateKnockStatus publishes a listener state change to D-Bus clients.
func (receiver *ExternalController) UpdateKnockStatus(status, message string) {
	receiver.statusMutex.Lock()
	defer receiver.statusMutex.Unlock()

	if receiver.knockStatus == status && receiver.knockError == message {
		return
	}

	receiver.knockStatus, receiver.knockError = status, message
	receiver.emitKnockStatus()
}

// KnockListening records successful startup unless a worker already failed.
func (receiver *ExternalController) KnockListening() {
	receiver.statusMutex.Lock()
	defer receiver.statusMutex.Unlock()

	if receiver.knockStatus != KnockStarting {
		return
	}

	receiver.knockStatus, receiver.knockError = KnockListening, ""
	receiver.emitKnockStatus()
}

// UpdateKnockActive publishes whether the temporary knock event is active.
func (receiver *ExternalController) UpdateKnockActive(active bool) {
	receiver.statusMutex.Lock()
	defer receiver.statusMutex.Unlock()

	if receiver.knockActive == active {
		return
	}

	receiver.knockActive = active
	receiver.emitKnockStatus()
}

// emitKnockStatus must be called with statusMutex held to preserve ordering.
func (receiver *ExternalController) emitKnockStatus() {
	if receiver.statusConn != nil {
		if err := receiver.statusConn.Emit(externalDBusObject, externalDBusInterface+".KnockStatusChanged", receiver.knockStatus, receiver.knockActive, receiver.knockError); err != nil {
			receiver.logger.Error("failed to emit knock status", slog.String("error", err.Error()))
		}
	}
}

func (receiver *ExternalController) PlayPattern(pattern patterns.Pattern) *dbus.Error {
	if err := receiver.ledController.PlayPattern(pattern); err != nil {
		return dbus.MakeFailedError(err)
	}

	return nil
}

func (receiver *ExternalController) PlayPatternJSON(pattern string) *dbus.Error {
	var patternStruct patterns.Pattern

	decoder := json.NewDecoder(strings.NewReader(pattern))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&patternStruct); err != nil {
		return makeInvalidArgsError(err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return makeInvalidArgsError(errors.New("unexpected data after pattern JSON"))
	}

	return receiver.PlayPattern(patternStruct)
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

	receiver.statusMutex.Lock()
	receiver.statusConn = connection
	receiver.statusMutex.Unlock()

	return nil
}

func (receiver *ExternalController) Unregister(connection *dbus.Conn) error {
	receiver.statusMutex.Lock()
	receiver.statusConn = nil
	receiver.statusMutex.Unlock()

	if err := connection.Export(nil, externalDBusObject, externalDBusInterface); err != nil {
		return fmt.Errorf("unexport external D-Bus interface: %w", err)
	}

	if _, err := connection.ReleaseName(externalDBusName); err != nil {
		return fmt.Errorf("release external D-Bus name: %w", err)
	}

	return nil
}

func makeInvalidArgsError(err error) *dbus.Error {
	return dbus.NewError(dbus.ErrMsgInvalidArg.Name, []any{err.Error()})
}
