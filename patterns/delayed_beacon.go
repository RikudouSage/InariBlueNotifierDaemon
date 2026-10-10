package patterns

import "time"

var DelayedBeacon = Pattern{
	Name:       "Delayed Beacon",
	Rise:       350 * time.Millisecond,
	Hold:       350 * time.Millisecond,
	Decay:      350 * time.Millisecond,
	Off:        2 * time.Second,
	Delay:      time.Second,
	Brightness: 192,
}
