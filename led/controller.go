// Package led coordinates LED states and patterns independently of hardware.
package led

import (
	"InariBlueNotifier/devices"
	"InariBlueNotifier/patterns"
	"errors"
	"fmt"
	"log"
	"sync"
)

var ErrClosed = errors.New("LED controller is closed")

type State uint8

const (
	Off State = iota
	Breathing
	Blinking
)

type Controller interface {
	SetState(State) error
	PlayPattern(patterns.Pattern) error
	Stop() error

	Release() error
	Shutdown() error
	Done() <-chan struct{}
}

type controller struct {
	mutex sync.Mutex

	// Hardware abstraction.
	device devices.Device

	// State-to-pattern mapping.
	notificationPattern patterns.Pattern
	callPattern         patterns.Pattern

	// Applied controller state.
	currentState State
	stateApplied bool
	closed       bool
}

// New creates a controller using the default patterns.
func New(device devices.Device) (Controller, error) {
	return NewWithPatterns(device, patterns.MediumLengthBreath, patterns.SlowBlink)
}

func NewWithPatterns(device devices.Device, notificationPattern, callPattern patterns.Pattern) (Controller, error) {
	if device == nil {
		return nil, errors.New("LED device is nil")
	}
	if err := notificationPattern.Validate(); err != nil {
		return nil, fmt.Errorf("notification pattern: %w", err)
	}
	if err := callPattern.Validate(); err != nil {
		return nil, fmt.Errorf("call pattern: %w", err)
	}

	return &controller{
		device:              device,
		notificationPattern: notificationPattern,
		callPattern:         callPattern,
	}, nil
}

func (receiver *controller) SetState(state State) error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.closed {
		log.Printf("LED state request rejected: state=%d controller is closed", state)
		return ErrClosed
	}

	if receiver.stateApplied && receiver.currentState == state {
		log.Printf("LED state unchanged: state=%d already applied", state)
		return nil
	}

	log.Printf("LED state transition: from=%d applied=%t to=%d", receiver.currentState, receiver.stateApplied, state)

	var err error
	switch state {
	case Off:
		err = receiver.device.Disable()
	case Breathing:
		err = receiver.device.ApplyPattern(receiver.notificationPattern)
	case Blinking:
		err = receiver.device.ApplyPattern(receiver.callPattern)
	default:
		return fmt.Errorf("unknown LED state: %d", state)
	}

	if err != nil {
		receiver.stateApplied = false
		log.Printf("LED state transition failed: to=%d: %v", state, err)

		return err
	}

	receiver.currentState, receiver.stateApplied = state, true
	log.Printf("LED state transition complete: state=%d", state)

	return nil
}

func (receiver *controller) PlayPattern(pattern patterns.Pattern) error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.closed {
		return ErrClosed
	}
	if err := receiver.device.ApplyPattern(pattern); err != nil {
		receiver.stateApplied = false
		return err
	}
	receiver.stateApplied = false

	return nil
}

func (receiver *controller) Stop() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.closed {
		return ErrClosed
	}
	if err := receiver.device.Disable(); err != nil {
		receiver.stateApplied = false
		return err
	}
	receiver.currentState, receiver.stateApplied = Off, true

	return nil
}

func (receiver *controller) Release() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.closed {
		return nil
	}
	receiver.closed = true

	return receiver.device.Release()
}

func (receiver *controller) Shutdown() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.closed {
		return nil
	}
	receiver.closed = true

	return receiver.device.Shutdown()
}

func (receiver *controller) Done() <-chan struct{} {
	return receiver.device.Done()
}
