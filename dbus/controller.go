package dbus

import (
	"InariBlueNotifier/led"
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
