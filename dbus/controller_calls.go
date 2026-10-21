package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/contacts"
	"context"
	"errors"
	"fmt"
	"log/slog"
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
			receiver.logger.Error("failed to remove voice call match", slog.String("error", err.Error()))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case signal, ok := <-signals:
			if !ok {
				receiver.logger.Error("voice call signal channel closed")
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
				receiver.logger.Error(
					"LED update failed after call signal",
					slog.String("name", signal.Name),
					slog.String("path", string(signal.Path)),
					slog.Int("incoming_calls", len(receiver.incomingCalls)),
					slog.String("error", err.Error()),
				)
			}
			receiver.eventMutex.Unlock()
		}
	}
}

func (receiver *controller) handleCallStatus(signal *dbus.Signal) {
	if len(signal.Body) == 0 {
		receiver.logger.Warn("ignored call status with no body", slog.String("path", string(signal.Path)))
		return
	}

	status, err := parseCallStatus(signal.Body)
	if err != nil {
		receiver.logger.Error(
			"failed to parse call status",
			slog.String("path", string(signal.Path)),
			slog.String("body", fmt.Sprintf("%#v", signal.Body)),
			slog.String("error", err.Error()),
		)
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

	receiver.logger.Info(
		"call status",
		slog.String("path", string(signal.Path)),
		slog.Int("status", status),
		slog.String("action", action),
		slog.Bool("was_incoming", wasIncoming),
		slog.Int("incoming_calls", len(receiver.incomingCalls)),
		slog.String("paths", receiver.incomingCallPaths()),
	)
}

func (receiver *controller) isFavoriteCall(path dbus.ObjectPath) bool {
	if receiver.connection == nil {
		return false
	}

	lineID, err := receiver.connection.Object("org.nemomobile.voicecall", path).GetProperty(voiceCallInterface + ".lineId")
	if err != nil {
		receiver.logger.Error("cannot get incoming caller number", slog.String("path", string(path)), slog.String("error", err.Error()))
		return false
	}

	number, ok := lineID.Value().(string)
	if !ok || number == "" {
		receiver.logger.Warn("incoming caller number is unavailable", slog.String("path", string(path)))
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	favorite, err := contacts.NewClient(receiver.connection).IsFavorite(ctx, number)
	if err != nil {
		receiver.logger.Error("favorite caller lookup failed", slog.String("path", string(path)), slog.String("error", err.Error()))
		return false
	}

	receiver.logger.Info("incoming caller favorite lookup", slog.String("path", string(path)), slog.Bool("favorite", favorite))
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
