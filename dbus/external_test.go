package dbus

import (
	"InariBlueNotifier/led"
	"InariBlueNotifier/patterns"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	invalidArgsErrorName = "org.freedesktop.DBus.Error.InvalidArgs"
	failedErrorName      = "org.freedesktop.DBus.Error.Failed"
)

type fakeExternalLEDController struct {
	played []patterns.Pattern
	err    error
}

func (receiver *fakeExternalLEDController) SetState(led.State) error { return receiver.err }

func (receiver *fakeExternalLEDController) PlayPattern(pattern patterns.Pattern) error {
	receiver.played = append(receiver.played, pattern)
	return receiver.err
}

func (receiver *fakeExternalLEDController) Stop() error           { return receiver.err }
func (receiver *fakeExternalLEDController) Release() error        { return nil }
func (receiver *fakeExternalLEDController) Shutdown() error       { return nil }
func (receiver *fakeExternalLEDController) Done() <-chan struct{} { return nil }

func TestExternalControllerExportsExpectedDBusMethods(t *testing.T) {
	dbusErrorType := reflect.TypeOf((*dbus.Error)(nil))
	controllerType := reflect.TypeOf(&ExternalController{})

	var exported []string
	for i := 0; i < controllerType.NumMethod(); i++ {
		methodType := controllerType.Method(i).Type
		if methodType.NumOut() > 0 && methodType.Out(methodType.NumOut()-1) == dbusErrorType {
			exported = append(exported, controllerType.Method(i).Name)
		}
	}

	want := []string{"PlayPattern", "PlayPatternJSON", "SetState", "Stop"}
	if !slices.Equal(exported, want) {
		t.Fatalf("D-Bus methods = %v, want %v", exported, want)
	}
}

func TestPlayPatternJSONTakesSingleStringArgument(t *testing.T) {
	method, ok := reflect.TypeOf(&ExternalController{}).MethodByName("PlayPatternJSON")
	if !ok {
		t.Fatal("PlayPatternJSON not found")
	}

	if method.Type.NumIn() != 2 || method.Type.In(1).Kind() != reflect.String {
		t.Fatalf("PlayPatternJSON signature = %v, want func(string) *dbus.Error", method.Type)
	}
	if method.Type.NumOut() != 1 {
		t.Fatalf("PlayPatternJSON returns %d values, want only *dbus.Error", method.Type.NumOut())
	}
}

func TestPlayPatternJSONDecodesAllFields(t *testing.T) {
	ledController := &fakeExternalLEDController{}
	receiver := NewExternalController(ledController)

	err := receiver.PlayPatternJSON(`{
		"name": "custom",
		"rise": 500000000,
		"hold": 200000000,
		"decay": 400000000,
		"off": 300000000,
		"delay": 1000000000,
		"repeat": 3,
		"brightness": 128,
		"channels": [1, 2]
	}`)
	if err != nil {
		t.Fatalf("play pattern JSON: %v", err)
	}

	want := patterns.Pattern{
		Name:       "custom",
		Rise:       500 * time.Millisecond,
		Hold:       200 * time.Millisecond,
		Decay:      400 * time.Millisecond,
		Off:        300 * time.Millisecond,
		Delay:      time.Second,
		Repeat:     3,
		Brightness: 128,
		Channels:   []uint8{1, 2},
	}
	if len(ledController.played) != 1 {
		t.Fatalf("played %d patterns, want 1", len(ledController.played))
	}
	if !reflect.DeepEqual(ledController.played[0], want) {
		t.Fatalf("played pattern = %+v, want %+v", ledController.played[0], want)
	}
}

func TestPlayPatternJSONLeavesOmittedFieldsZero(t *testing.T) {
	ledController := &fakeExternalLEDController{}
	receiver := NewExternalController(ledController)

	if err := receiver.PlayPatternJSON(`{"name":"minimal"}`); err != nil {
		t.Fatalf("play pattern JSON: %v", err)
	}

	want := patterns.Pattern{Name: "minimal"}
	if len(ledController.played) != 1 || !reflect.DeepEqual(ledController.played[0], want) {
		t.Fatalf("played patterns = %+v, want [%+v]", ledController.played, want)
	}
}

func TestPlayPatternJSONAcceptsEmptyChannels(t *testing.T) {
	ledController := &fakeExternalLEDController{}
	receiver := NewExternalController(ledController)

	if err := receiver.PlayPatternJSON(`{"name":"all","channels":[]}`); err != nil {
		t.Fatalf("play pattern JSON: %v", err)
	}

	if len(ledController.played) != 1 || len(ledController.played[0].Channels) != 0 {
		t.Fatalf("played patterns = %+v, want one pattern without channels", ledController.played)
	}
}

func TestPlayPatternJSONAcceptsBase64Channels(t *testing.T) {
	ledController := &fakeExternalLEDController{}
	receiver := NewExternalController(ledController)

	if err := receiver.PlayPatternJSON(`{"name":"encoded","channels":"AQI="}`); err != nil {
		t.Fatalf("play pattern JSON: %v", err)
	}

	if len(ledController.played) != 1 || !slices.Equal(ledController.played[0].Channels, []uint8{1, 2}) {
		t.Fatalf("played patterns = %+v, want one pattern with channels [1 2]", ledController.played)
	}
}

func TestPlayPatternJSONPassesNullThroughAsEmptyPattern(t *testing.T) {
	ledController := &fakeExternalLEDController{}
	receiver := NewExternalController(ledController)

	if err := receiver.PlayPatternJSON(`null`); err != nil {
		t.Fatalf("play pattern JSON: %v", err)
	}

	if len(ledController.played) != 1 || !reflect.DeepEqual(ledController.played[0], patterns.Pattern{}) {
		t.Fatalf("played patterns = %+v, want one empty pattern", ledController.played)
	}
}

func TestPlayPatternJSONAcceptsSurroundingWhitespace(t *testing.T) {
	ledController := &fakeExternalLEDController{}
	receiver := NewExternalController(ledController)

	if err := receiver.PlayPatternJSON("\n\t  {\"name\":\"padded\"}  \n"); err != nil {
		t.Fatalf("play pattern JSON: %v", err)
	}

	if len(ledController.played) != 1 || ledController.played[0].Name != "padded" {
		t.Fatalf("played patterns = %+v, want one pattern named padded", ledController.played)
	}
}

func TestPlayPatternJSONAcceptsBoundaryByteValues(t *testing.T) {
	ledController := &fakeExternalLEDController{}
	receiver := NewExternalController(ledController)

	if err := receiver.PlayPatternJSON(`{"name":"edges","repeat":255,"brightness":0,"channels":[0,255]}`); err != nil {
		t.Fatalf("play pattern JSON: %v", err)
	}

	played := ledController.played[0]
	if played.Repeat != 255 || played.Brightness != 0 || !slices.Equal(played.Channels, []uint8{0, 255}) {
		t.Fatalf("played pattern = %+v, want repeat 255, brightness 0, channels [0 255]", played)
	}
}

func TestPlayPatternJSONRoundTripsBuiltInPatterns(t *testing.T) {
	builtIn := []patterns.Pattern{
		patterns.SlowBlink,
		patterns.RapidBlink,
		patterns.QuickFlash,
		patterns.GentleGlow,
		patterns.DelayedBeacon,
		patterns.MediumLengthBreath,
	}

	for _, pattern := range builtIn {
		t.Run(pattern.Name, func(t *testing.T) {
			encoded, err := json.Marshal(pattern)
			if err != nil {
				t.Fatalf("marshal pattern: %v", err)
			}

			ledController := &fakeExternalLEDController{}
			receiver := NewExternalController(ledController)

			if err := receiver.PlayPatternJSON(string(encoded)); err != nil {
				t.Fatalf("play pattern JSON %s: %v", encoded, err)
			}
			if len(ledController.played) != 1 || !reflect.DeepEqual(ledController.played[0], pattern) {
				t.Fatalf("played patterns = %+v, want [%+v]", ledController.played, pattern)
			}
		})
	}
}

func TestPlayPatternJSONRejectsInvalidInput(t *testing.T) {
	cases := map[string]string{
		"empty string":            ``,
		"whitespace only":         "  \n\t ",
		"not JSON":                `not json`,
		"truncated object":        `{"name":"a"`,
		"trailing comma":          `{"name":"a",}`,
		"array":                   `[{"name":"a"}]`,
		"string":                  `"a"`,
		"number":                  `42`,
		"boolean":                 `true`,
		"unknown field":           `{"name":"a","colour":"red"}`,
		"misspelled field":        `{"name":"a","brigthness":10}`,
		"name as number":          `{"name":1}`,
		"duration as string":      `{"name":"a","rise":"500ms"}`,
		"fractional duration":     `{"name":"a","rise":0.5}`,
		"overflowing duration":    `{"name":"a","rise":9223372036854775808}`,
		"negative repeat":         `{"name":"a","repeat":-1}`,
		"repeat above byte":       `{"name":"a","repeat":256}`,
		"fractional brightness":   `{"name":"a","brightness":1.5}`,
		"brightness above byte":   `{"name":"a","brightness":256}`,
		"channel above byte":      `{"name":"a","channels":[256]}`,
		"negative channel":        `{"name":"a","channels":[-1]}`,
		"channel as string":       `{"name":"a","channels":[1,"2"]}`,
		"channels as object":      `{"name":"a","channels":{}}`,
		"trailing garbage":        `{"name":"a"} garbage`,
		"second object":           `{"name":"a"}{"name":"b"}`,
		"trailing null":           `{"name":"a"} null`,
		"trailing closing brace":  `{"name":"a"}}`,
		"trailing closing square": `{"name":"a"}]`,
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			ledController := &fakeExternalLEDController{}
			receiver := NewExternalController(ledController)

			err := receiver.PlayPatternJSON(payload)
			if err == nil {
				t.Fatalf("PlayPatternJSON(%q) error = nil, want error", payload)
			}
			if err.Name != invalidArgsErrorName {
				t.Fatalf("PlayPatternJSON(%q) error name = %q, want %q", payload, err.Name, invalidArgsErrorName)
			}
			if len(ledController.played) != 0 {
				t.Fatalf("PlayPatternJSON(%q) played %+v, want nothing", payload, ledController.played)
			}
		})
	}
}

func TestPlayPatternJSONReportsDecodeErrorDetails(t *testing.T) {
	receiver := NewExternalController(&fakeExternalLEDController{})

	err := receiver.PlayPatternJSON(`{"name":"a","brigthness":10}`)
	if err == nil {
		t.Fatal("error = nil, want unknown field error")
	}
	if !strings.Contains(err.Error(), "brigthness") {
		t.Fatalf("error = %q, want it to name the unknown field", err.Error())
	}
}

func TestPlayPatternJSONPropagatesControllerError(t *testing.T) {
	ledController := &fakeExternalLEDController{err: errors.New("device unplugged")}
	receiver := NewExternalController(ledController)

	err := receiver.PlayPatternJSON(`{"name":"a"}`)
	if err == nil {
		t.Fatal("error = nil, want controller error")
	}
	if err.Name != failedErrorName {
		t.Fatalf("error name = %q, want %q", err.Name, failedErrorName)
	}
	if !strings.Contains(err.Error(), "device unplugged") {
		t.Fatalf("error = %q, want it to contain the controller error", err.Error())
	}
	if len(ledController.played) != 1 {
		t.Fatalf("played %d patterns, want 1", len(ledController.played))
	}
}
