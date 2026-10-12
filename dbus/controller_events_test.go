package dbus

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestIsFavouriteCallSkipsLookupWithoutSessionBus(t *testing.T) {
	receiver := &controller{}

	if receiver.isFavoriteCall("/calls/one") {
		t.Fatal("missing session bus was treated as available")
	}
}

func TestIdleActiveCallClearsStaleIncomingCalls(t *testing.T) {
	receiver := &controller{
		incomingCalls: map[dbus.ObjectPath]bool{
			dbus.ObjectPath(activeVoiceCallPath): false,
			"/calls/stale":                       false,
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
		incomingCalls: map[dbus.ObjectPath]bool{
			activePath: false,
			otherPath:  false,
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
