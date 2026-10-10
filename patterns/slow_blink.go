package patterns

import "time"

var SlowBlink = Pattern{
	Name:       "Slow blink",
	Hold:       280 * time.Millisecond,
	Off:        280 * time.Millisecond,
	Brightness: 255,
}
