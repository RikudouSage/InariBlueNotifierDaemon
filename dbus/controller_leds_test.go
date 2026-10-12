package dbus

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/led"
	"InariBlueNotifier/patterns"
	"testing"

	"github.com/godbus/dbus/v5"
)

type recordingLEDController struct {
	lastPattern patterns.Pattern
}

func (receiver *recordingLEDController) SetState(led.State) error { return nil }

func (receiver *recordingLEDController) PlayPattern(pattern patterns.Pattern) error {
	receiver.lastPattern = pattern
	return nil
}

func (receiver *recordingLEDController) Stop() error    { return nil }
func (receiver *recordingLEDController) Release() error { return nil }
func (receiver *recordingLEDController) Shutdown() error {
	return nil
}
func (receiver *recordingLEDController) Done() <-chan struct{} { return nil }

func TestUpdateLEDAppliesCallPattern(t *testing.T) {
	normal := patterns.SlowBlink
	favourite := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			CallPattern:         normal,
			FavoriteCallPattern: favourite,
		},
		incomingCalls: map[dbus.ObjectPath]bool{
			"/calls/one": false,
		},
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != normal.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, normal.Name)
	}
}

func TestUpdateLEDAppliesFavouriteCallPattern(t *testing.T) {
	normal := patterns.SlowBlink
	favourite := patterns.RapidBlink
	ledController := &recordingLEDController{}
	receiver := &controller{
		ledController: ledController,
		config: &config.Config{
			CallPattern:         normal,
			FavoriteCallPattern: favourite,
		},
		incomingCalls: map[dbus.ObjectPath]bool{
			"/calls/one": false,
			"/calls/two": true,
		},
	}

	if err := receiver.updateLED(); err != nil {
		t.Fatalf("update LED: %v", err)
	}
	if ledController.lastPattern.Name != favourite.Name {
		t.Fatalf("applied pattern = %q, want %q", ledController.lastPattern.Name, favourite.Name)
	}
}
