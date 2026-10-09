package dbus

import (
	"InariBlueNotifier/led"
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	voiceCallInterface     = "org.nemomobile.voicecall.VoiceCall"
	voiceCallStatusChanged = voiceCallInterface + ".statusChanged"

	voiceCallStatusIncoming = 5
)

type Controller interface {
	Start() error
	Stop() error
}

type controller struct {
	connection    *dbus.Conn
	ledController led.Controller

	mutex  sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}

	incomingCalls map[dbus.ObjectPath]struct{}
	hasUnread     bool
}

func NewController(connection *dbus.Conn, ledController led.Controller) Controller {
	return &controller{
		connection:    connection,
		ledController: ledController,
		incomingCalls: make(map[dbus.ObjectPath]struct{}),
	}
}

func (receiver *controller) Start() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if err := receiver.ledController.SetState(led.Off); err != nil {
		return fmt.Errorf("initialize LED state: %w", err)
	}

	if receiver.cancel != nil {
		return errors.New("controller already started")
	}

	signals := make(chan *dbus.Signal, 16)

	matchOptions := []dbus.MatchOption{
		dbus.WithMatchInterface(voiceCallInterface),
		dbus.WithMatchMember("statusChanged"),
	}

	receiver.connection.Signal(signals)

	if err := receiver.connection.AddMatchSignal(matchOptions...); err != nil {
		receiver.connection.RemoveSignal(signals)
		return fmt.Errorf("subscribe to voice call signals: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	receiver.cancel = cancel
	receiver.done = make(chan struct{})
	receiver.incomingCalls = make(map[dbus.ObjectPath]struct{})
	receiver.hasUnread = false

	go receiver.watch(ctx, signals, matchOptions, receiver.done)

	return nil
}

func (receiver *controller) Stop() error {
	receiver.mutex.Lock()

	if receiver.cancel == nil {
		receiver.mutex.Unlock()
		return nil
	}

	cancel := receiver.cancel
	done := receiver.done

	cancel()
	<-done

	receiver.cancel = nil
	receiver.done = nil

	receiver.mutex.Unlock()

	return nil
}

func (receiver *controller) updateLED() error {
	switch {
	case len(receiver.incomingCalls) > 0:
		return receiver.ledController.SetState(led.Blinking)

	case receiver.hasUnread:
		return receiver.ledController.SetState(led.Breathing)

	default:
		return receiver.ledController.SetState(led.Off)
	}
}

func (receiver *controller) watch(
	ctx context.Context,
	signals chan *dbus.Signal,
	matchOptions []dbus.MatchOption,
	done chan struct{},
) {
	defer close(done)
	defer receiver.connection.RemoveSignal(signals)

	defer func() {
		if err := receiver.connection.RemoveMatchSignal(matchOptions...); err != nil {
			log.Printf("failed to remove voice call match: %v", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case signal, ok := <-signals:
			if !ok {
				return
			}

			if signal == nil {
				continue
			}

			switch signal.Name {
			case voiceCallStatusChanged:
				receiver.handleCallStatus(signal)

			default:
				continue
			}

			if err := receiver.updateLED(); err != nil {
				log.Printf("failed to update LED: %v", err)
			}
		}
	}
}

func (receiver *controller) handleCallStatus(signal *dbus.Signal) {
	if len(signal.Body) == 0 {
		return
	}

	status, err := parseCallStatus(signal.Body)
	if err != nil {
		log.Printf("failed to parse call status: %v", err)
		return
	}

	switch status {
	case voiceCallStatusIncoming:
		receiver.incomingCalls[signal.Path] = struct{}{}

	default:
		delete(receiver.incomingCalls, signal.Path)
	}
}

func parseCallStatus(body []any) (int, error) {
	if len(body) == 0 {
		return 0, errors.New("missing call status")
	}

	switch value := body[0].(type) {
	case int32:
		return int(value), nil
	case uint32:
		return int(value), nil
	case string:
		return strconv.Atoi(value)
	default:
		return 0, fmt.Errorf("unexpected call status type: %T", value)
	}
}
