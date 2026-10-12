// Package led coordinates LED states and patterns independently of hardware.
package led

import (
	"InariBlueNotifier/devices"
	"InariBlueNotifier/patterns"
	"errors"
	"sync"
)

var ErrClosed = errors.New("LED controller is closed")

type Controller interface {
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

	closed bool
}

// New creates a controller for the given LED device.
func New(device devices.Device) (Controller, error) {
	if device == nil {
		return nil, errors.New("LED device is nil")
	}

	return &controller{device: device}, nil
}

func (receiver *controller) PlayPattern(pattern patterns.Pattern) error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.closed {
		return ErrClosed
	}

	return receiver.device.ApplyPattern(pattern)
}

func (receiver *controller) Stop() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.closed {
		return ErrClosed
	}

	return receiver.device.Disable()
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
