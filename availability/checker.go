package availability

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	dbusService                = "org.freedesktop.DBus"
	dbusPath                   = "/org/freedesktop/DBus"
	dbusInterface              = "org.freedesktop.DBus"
	dbusObjectManagerInterface = "org.freedesktop.DBus.ObjectManager"

	systemdService             = "org.freedesktop.systemd1"
	systemdPath                = "/org/freedesktop/systemd1"
	systemdInterface           = "org.freedesktop.systemd1.Manager"
	systemdUnitInterface       = "org.freedesktop.systemd1.Unit"
	systemdPropertiesInterface = "org.freedesktop.DBus.Properties"

	jollaLedService = "toh-leds.service"

	symbiosisService   = "org.sailfishos.tohd1"
	symbiosisRootPath  = "/org/sailfishos/tohd1"
	symbiosisObject    = "/org/sailfishos/tohd1/toh"
	symbiosisInterface = "org.sailfishos.tohd1.Toh"
)

type Checker interface {
	Available(ctx context.Context) (bool, error)
	Watch(ctx context.Context, changes chan<- bool) error
}

type checker struct {
	mutex             sync.Mutex
	sessionConnection *dbus.Conn
	systemConnection  *dbus.Conn
}

func NewChecker(sessionConnection, systemConnection *dbus.Conn) Checker {
	return &checker{
		sessionConnection: sessionConnection,
		systemConnection:  systemConnection,
	}
}

func (receiver *checker) ledServiceObject(ctx context.Context) (dbus.ObjectPath, error) {
	var unitPath dbus.ObjectPath
	err := receiver.sessionConnection.Object(
		systemdService,
		systemdPath,
	).CallWithContext(
		ctx,
		systemdInterface+".LoadUnit",
		0,
		jollaLedService,
	).Store(&unitPath)
	if err != nil {
		return "", fmt.Errorf("load %s unit: %w", jollaLedService, err)
	}

	return unitPath, nil
}

// Available reports whether the original LED service has yielded ownership,
// is not running, and Symbiosis has detected and published a TOH. All three
// conditions are required before this daemon may control the LEDs.
func (receiver *checker) Available(ctx context.Context) (bool, error) {
	var unitFileState string
	err := receiver.sessionConnection.Object(
		systemdService,
		systemdPath,
	).CallWithContext(
		ctx,
		systemdInterface+".GetUnitFileState",
		0,
		jollaLedService,
	).Store(&unitFileState)
	if err != nil {
		return false, fmt.Errorf("get %s unit file state: %w", jollaLedService, err)
	}
	if unitFileState != "masked" && unitFileState != "masked-runtime" {
		return false, nil
	}

	unitPath, err := receiver.ledServiceObject(ctx)
	if err != nil {
		return false, err
	}

	stateVariant, err := receiver.sessionConnection.Object(
		systemdService,
		unitPath,
	).GetProperty(systemdUnitInterface + ".ActiveState")
	if err != nil {
		return false, fmt.Errorf("get %s active state: %w", jollaLedService, err)
	}
	state, ok := stateVariant.Value().(string)
	if !ok {
		return false, fmt.Errorf("read %s active state: unexpected type %T", jollaLedService, stateVariant.Value())
	}
	if state != "inactive" && state != "failed" {
		return false, nil
	}

	objects := map[dbus.ObjectPath]map[string]map[string]dbus.Variant{}

	err = receiver.systemConnection.Object(
		symbiosisService,
		symbiosisRootPath,
	).CallWithContext(
		ctx,
		dbusObjectManagerInterface+".GetManagedObjects",
		0,
	).Store(&objects)
	if err != nil {
		return false, fmt.Errorf("get managed Symbiosis objects: %w", err)
	}

	interfaces, exists := objects[symbiosisObject]
	if !exists {
		return false, nil
	}

	_, exists = interfaces[symbiosisInterface]
	return exists, nil
}

func (receiver *checker) Watch(ctx context.Context, changes chan<- bool) error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if changes == nil {
		return errors.New("changes cannot be nil")
	}
	unitPath, err := receiver.ledServiceObject(ctx)
	if err != nil {
		return err
	}

	sessionSignals := make(chan *dbus.Signal, 16)
	systemSignals := make(chan *dbus.Signal, 16)
	receiver.sessionConnection.Signal(sessionSignals)
	receiver.systemConnection.Signal(systemSignals)
	defer receiver.sessionConnection.RemoveSignal(sessionSignals)
	defer receiver.systemConnection.RemoveSignal(systemSignals)

	ownerMatch := []dbus.MatchOption{
		dbus.WithMatchSender(dbusService),
		dbus.WithMatchObjectPath(dbusPath),
		dbus.WithMatchInterface(dbusInterface),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, symbiosisService),
	}
	addedMatch := []dbus.MatchOption{
		dbus.WithMatchObjectPath(symbiosisRootPath),
		dbus.WithMatchInterface(dbusObjectManagerInterface),
		dbus.WithMatchMember("InterfacesAdded"),
	}
	removedMatch := []dbus.MatchOption{
		dbus.WithMatchObjectPath(symbiosisRootPath),
		dbus.WithMatchInterface(dbusObjectManagerInterface),
		dbus.WithMatchMember("InterfacesRemoved"),
	}
	unitStateMatch := []dbus.MatchOption{
		dbus.WithMatchObjectPath(unitPath),
		dbus.WithMatchInterface(systemdPropertiesInterface),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(0, systemdUnitInterface),
	}
	unitFilesMatch := []dbus.MatchOption{
		dbus.WithMatchObjectPath(systemdPath),
		dbus.WithMatchInterface(systemdInterface),
		dbus.WithMatchMember("UnitFilesChanged"),
	}

	if err := receiver.systemConnection.AddMatchSignal(ownerMatch...); err != nil {
		return fmt.Errorf("subscribe to Symbiosis owner changes: %w", err)
	}
	defer receiver.systemConnection.RemoveMatchSignal(ownerMatch...)

	if err := receiver.systemConnection.AddMatchSignal(addedMatch...); err != nil {
		return fmt.Errorf("subscribe to TOH additions: %w", err)
	}
	defer receiver.systemConnection.RemoveMatchSignal(addedMatch...)

	if err := receiver.systemConnection.AddMatchSignal(removedMatch...); err != nil {
		return fmt.Errorf("subscribe to TOH removals: %w", err)
	}
	defer receiver.systemConnection.RemoveMatchSignal(removedMatch...)

	if err := receiver.sessionConnection.AddMatchSignal(unitStateMatch...); err != nil {
		return fmt.Errorf("subscribe to LED service changes: %w", err)
	}
	defer receiver.sessionConnection.RemoveMatchSignal(unitStateMatch...)

	if err := receiver.sessionConnection.AddMatchSignal(unitFilesMatch...); err != nil {
		return fmt.Errorf("subscribe to LED service ownership changes: %w", err)
	}
	defer receiver.sessionConnection.RemoveMatchSignal(unitFilesMatch...)

	var previous bool
	initialized := false
	emit := func(available bool) error {
		if initialized && available == previous {
			return nil
		}

		previous = available
		initialized = true

		select {
		case changes <- available:
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	}
	refresh := func() error {
		available, err := receiver.Available(ctx)
		if err != nil {
			// Symbiosis may be restarting. Treat it as absent and wait for its
			// NameOwnerChanged signal instead of terminating the notifier.
			available = false
		}

		return emit(available)
	}

	if err := refresh(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case signal, ok := <-sessionSignals:
			if !ok {
				return errors.New("session-bus signal channel closed")
			}
			if signal != nil && (signal.Name == systemdPropertiesInterface+".PropertiesChanged" || signal.Name == systemdInterface+".UnitFilesChanged") {
				if err := refresh(); err != nil {
					return err
				}
			}

		case signal, ok := <-systemSignals:
			if !ok {
				return errors.New("Symbiosis signal channel closed")
			}
			if signal == nil {
				continue
			}

			switch signal.Name {
			case dbusInterface + ".NameOwnerChanged":
				if len(signal.Body) != 3 {
					continue
				}
				name, _ := signal.Body[0].(string)
				newOwner, _ := signal.Body[2].(string)
				if name != symbiosisService {
					continue
				}
				if newOwner == "" {
					if err := emit(false); err != nil {
						return err
					}
					continue
				}
				if err := refresh(); err != nil {
					return err
				}

			case dbusObjectManagerInterface + ".InterfacesAdded", dbusObjectManagerInterface + ".InterfacesRemoved":
				if len(signal.Body) == 0 {
					continue
				}
				path, _ := signal.Body[0].(dbus.ObjectPath)
				if path != symbiosisObject {
					continue
				}
				if err := refresh(); err != nil {
					return err
				}
			}
		}
	}
}
