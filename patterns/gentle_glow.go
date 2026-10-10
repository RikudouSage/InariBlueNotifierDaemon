package patterns

import "time"

var GentleGlow = Pattern{
	Name:       "Gentle Glow",
	Rise:       1500 * time.Millisecond,
	Hold:       500 * time.Millisecond,
	Decay:      1500 * time.Millisecond,
	Off:        2 * time.Second,
	Brightness: 64,
}
