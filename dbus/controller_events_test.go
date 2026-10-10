package dbus

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestIdleActiveCallClearsStaleIncomingCalls(t *testing.T) {
	receiver := &controller{
		incomingCalls: map[dbus.ObjectPath]struct{}{
			dbus.ObjectPath(activeVoiceCallPath): {},
			"/calls/stale":                       {},
		},
	}

	receiver.handleCallStatus(&dbus.Signal{
		Path: activeVoiceCallPath,
		Body: []any{int32(voiceCallStatusIdle)},
	})

	if len(receiver.incomingCalls) != 0 {
		t.Fatalf("incoming calls after idle active call = %v, want none", receiver.incomingCalls)
	}
}

func TestNonActiveIdleCallDoesNotClearOtherIncomingCalls(t *testing.T) {
	activePath := dbus.ObjectPath(activeVoiceCallPath)
	otherPath := dbus.ObjectPath("/calls/other")
	receiver := &controller{
		incomingCalls: map[dbus.ObjectPath]struct{}{
			activePath: {},
			otherPath:  {},
		},
	}

	receiver.handleCallStatus(&dbus.Signal{
		Path: otherPath,
		Body: []any{int32(voiceCallStatusIdle)},
	})

	if _, ok := receiver.incomingCalls[activePath]; !ok {
		t.Fatal("idle non-active call cleared the active incoming call")
	}
}
