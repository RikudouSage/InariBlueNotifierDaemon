package dbus

import (
	"context"
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
)

const (
	mceService   = "com.nokia.mce"
	mcePath      = "/com/nokia/mce/signal"
	mceInterface = "com.nokia.mce.signal"
	tklockSignal = mceInterface + ".tklock_mode_ind"
)

func (receiver *controller) subscribeUnlock() (chan *dbus.Signal, []dbus.MatchOption, error) {
	if receiver.systemConnection == nil {
		return nil, nil, fmt.Errorf("unlock watcher requires a system bus connection")
	}

	match := []dbus.MatchOption{
		dbus.WithMatchSender(mceService),
		dbus.WithMatchObjectPath(mcePath),
		dbus.WithMatchInterface(mceInterface),
		dbus.WithMatchMember("tklock_mode_ind"),
		dbus.WithMatchArg(0, "unlocked"),
	}

	signals := make(chan *dbus.Signal, 16)
	receiver.systemConnection.Signal(signals)

	if err := receiver.systemConnection.AddMatchSignal(match...); err != nil {
		receiver.systemConnection.RemoveSignal(signals)
		return nil, nil, fmt.Errorf("subscribe to touchscreen unlock: %w", err)
	}

	return signals, match, nil
}

func (receiver *controller) watchUnlock(ctx context.Context, signals chan *dbus.Signal, match []dbus.MatchOption, clearKnock func()) {
	defer receiver.systemConnection.RemoveSignal(signals)
	defer func() {
		if err := receiver.systemConnection.RemoveMatchSignal(match...); err != nil {
			log.Printf("failed to remove unlock signal match: %v", err)
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
			if signal != nil && signal.Name == tklockSignal && signal.Path == mcePath && len(signal.Body) == 1 && signal.Body[0] == "unlocked" {
				clearKnock()
			}
		}
	}
}
