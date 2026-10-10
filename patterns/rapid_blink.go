package patterns

import "time"

// RapidBlink uses the shortest non-zero interval supported by the LED
// controller: 31.25 ms on followed by 31.25 ms off.
var RapidBlink = Pattern{
	Name:       "Rapid Blink",
	Hold:       31250 * time.Microsecond,
	Off:        31250 * time.Microsecond,
	Brightness: 255,
}
