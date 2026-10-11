// Package devices contains implementations of physical LED devices.
package devices

import "InariBlueNotifier/patterns"

const (
	// D-Bus services and interfaces used by TOH-based devices.
	dbusService            = "org.freedesktop.DBus"
	dbusInterface          = "org.freedesktop.DBus"
	objectManagerInterface = "org.freedesktop.DBus.ObjectManager"

	tohRootPath  = "/org/sailfishos/tohd1"
	tohService   = "org.sailfishos.tohd1"
	tohObject    = "/org/sailfishos/tohd1/toh"
	tohInterface = "org.sailfishos.tohd1.Toh"

	tohI2CSlaveIOCTL = 0x0703
)

// Device provides the primitives needed by an LED controller.
// Implementations own device-specific communication and lifecycle.
type Device interface {
	ApplyPattern(pattern patterns.Pattern) error
	Disable() error

	Release() error
	Shutdown() error
	Done() <-chan struct{}
}
