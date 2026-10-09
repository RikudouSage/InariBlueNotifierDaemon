package availability

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	systemdService   = "org.freedesktop.systemd1"
	systemdPath      = "/org/freedesktop/systemd1"
	systemdInterface = "org.freedesktop.systemd1.Manager"

	ledService = "toh-leds.service"
)

type Checker interface {
	Available(ctx context.Context) (bool, error)
	Watch(ctx context.Context, changes chan<- bool) error
}

type checker struct {
	mutex      sync.Mutex
	connection *dbus.Conn
}

func NewChecker(connection *dbus.Conn) Checker {
	return &checker{
		connection: connection,
	}
}

func (receiver *checker) Available(ctx context.Context) (bool, error) {
	var state string

	err := receiver.connection.Object(
		systemdService,
		systemdPath,
	).CallWithContext(
		ctx,
		systemdInterface+".GetUnitFileState",
		0,
		ledService,
	).Store(&state)

	if err != nil {
		return false, fmt.Errorf(
			"get %s unit file state: %w",
			ledService,
			err,
		)
	}

	return state == "masked" || state == "masked-runtime", nil
}

func (receiver *checker) Watch(ctx context.Context, changes chan<- bool) error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if changes == nil {
		return errors.New("onChange cannot be nil")
	}

	signals := make(chan *dbus.Signal, 16)
	receiver.connection.Signal(signals)
	defer receiver.connection.RemoveSignal(signals)

	err := receiver.connection.AddMatchSignal(
		dbus.WithMatchObjectPath(
			systemdPath,
		),
		dbus.WithMatchInterface(systemdInterface),
		dbus.WithMatchMember("UnitFilesChanged"),
	)

	if err != nil {
		return fmt.Errorf(
			"subscribe to systemd unit changes: %w",
			err,
		)
	}

	defer receiver.connection.RemoveMatchSignal(
		dbus.WithMatchObjectPath(
			systemdPath,
		),
		dbus.WithMatchInterface(systemdInterface),
		dbus.WithMatchMember("UnitFilesChanged"),
	)

	var previous bool
	initialized := false

	check := func() error {
		available, err := receiver.Available(ctx)
		if err != nil {
			return err
		}

		if !initialized || available != previous {
			previous = available
			initialized = true

			select {
			case changes <- available:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	}

	if err := check(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case signal, ok := <-signals:
			if !ok {
				return errors.New("systemd signal channel closed")
			}

			if signal == nil || signal.Name != systemdInterface+".UnitFilesChanged" || signal.Path != systemdPath {
				continue
			}

			if err := check(); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}

				return err
			}
		}
	}
}
