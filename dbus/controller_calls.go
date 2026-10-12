package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/contacts"
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

func (receiver *controller) watchCalls(
	ctx context.Context,
	signals chan *dbus.Signal,
	matchOptions []dbus.MatchOption,
) {
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
				log.Printf("voice call signal channel closed")
				return
			}

			if signal == nil {
				continue
			}

			receiver.eventMutex.Lock()
			switch signal.Name {
			case voiceCallStatusChanged:
				receiver.handleCallStatus(signal)

			default:
				receiver.eventMutex.Unlock()
				continue
			}

			if err := receiver.updateLED(); err != nil {
				log.Printf(
					"LED update failed after call signal: name=%s path=%s incoming_calls=%d: %v",
					signal.Name,
					signal.Path,
					len(receiver.incomingCalls),
					err,
				)
			}
			receiver.eventMutex.Unlock()
		}
	}
}

func (receiver *controller) handleCallStatus(signal *dbus.Signal) {
	if len(signal.Body) == 0 {
		log.Printf("ignored call status with no body: path=%s", signal.Path)
		return
	}

	status, err := parseCallStatus(signal.Body)
	if err != nil {
		log.Printf("failed to parse call status: path=%s body=%#v: %v", signal.Path, signal.Body, err)
		return
	}

	action := "cleared"
	wasIncoming := false
	switch status {
	case voiceCallStatusIncoming:
		favorite := false
		if receiver.config == nil || receiver.config.Events == nil || receiver.config.Events[config.EventFavoriteCall] {
			favorite = receiver.isFavoriteCall(signal.Path)
		}
		receiver.incomingCalls[signal.Path] = favorite
		action = "tracked"

	default:
		_, wasIncoming = receiver.incomingCalls[signal.Path]
		delete(receiver.incomingCalls, signal.Path)

		// /calls/active is the voicecall manager's canonical active-call
		// object. Once it becomes idle, no call can still be ringing. Clear
		// any paths left behind by a missed or differently-addressed signal.
		if signal.Path == activeVoiceCallPath && status == voiceCallStatusIdle {
			receiver.incomingCalls = make(map[dbus.ObjectPath]bool)
			action = "cleared-active-call"
		}
	}

	log.Printf(
		"call status: path=%s status=%d action=%s was_incoming=%t incoming_calls=%d paths=%s",
		signal.Path,
		status,
		action,
		wasIncoming,
		len(receiver.incomingCalls),
		receiver.incomingCallPaths(),
	)
}

func (receiver *controller) isFavoriteCall(path dbus.ObjectPath) bool {
	if receiver.connection == nil {
		return false
	}

	lineID, err := receiver.connection.Object("org.nemomobile.voicecall", path).GetProperty(voiceCallInterface + ".lineId")
	if err != nil {
		log.Printf("cannot get incoming caller number: path=%s: %v", path, err)
		return false
	}

	number, ok := lineID.Value().(string)
	if !ok || number == "" {
		log.Printf("incoming caller number is unavailable: path=%s", path)
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	favorite, err := contacts.NewClient(receiver.connection).IsFavorite(ctx, number)
	if err != nil {
		log.Printf("favorite caller lookup failed: path=%s: %v", path, err)
		return false
	}

	log.Printf("incoming caller favorite lookup: path=%s favorite=%t", path, favorite)
	return favorite
}

func (receiver *controller) incomingCallPaths() string {
	paths := make([]string, 0, len(receiver.incomingCalls))
	for path := range receiver.incomingCalls {
		paths = append(paths, string(path))
	}
	sort.Strings(paths)
	return strings.Join(paths, ",")
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
