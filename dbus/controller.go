package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/led"
	"InariBlueNotifier/patterns"
	"context"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	voiceCallInterface     = "org.nemomobile.voicecall.VoiceCall"
	voiceCallStatusChanged = voiceCallInterface + ".statusChanged"
	activeVoiceCallPath    = "/calls/active"

	voiceCallStatusIdle     = 0
	voiceCallStatusIncoming = 5
)

type Controller interface {
	Start() error
	Stop() error
}

type controller struct {
	connection    *dbus.Conn
	ledController led.Controller

	config *config.Config
	mutex  sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}

	// incomingCalls maps each ringing call to whether its caller is a favourite.
	incomingCalls map[dbus.ObjectPath]bool
}

func NewController(connection *dbus.Conn, ledController led.Controller) Controller {
	return NewControllerWithConfig(connection, ledController, &config.Config{
		CallPattern:         patterns.DefaultCallPattern,
		FavoriteCallPattern: patterns.DefaultCallPattern,
	})
}

// NewControllerWithConfig creates a controller that uses the configured call
// patterns and selects the favourite pattern when the contacts helper is
// available over the session bus.
func NewControllerWithConfig(
	connection *dbus.Conn,
	ledController led.Controller,
	configuration *config.Config,
) Controller {
	if configuration == nil {
		configuration = &config.Config{
			CallPattern:         patterns.DefaultCallPattern,
			FavoriteCallPattern: patterns.DefaultCallPattern,
		}
	}

	return &controller{
		connection:    connection,
		ledController: ledController,
		config:        configuration,
		incomingCalls: make(map[dbus.ObjectPath]bool),
	}
}
