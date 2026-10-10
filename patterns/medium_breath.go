package patterns

import "time"

var MediumLengthBreath = Pattern{
	Name:       "Medium Length Breath",
	Rise:       1125 * time.Millisecond,
	Hold:       1125 * time.Millisecond,
	Decay:      1125 * time.Millisecond,
	Off:        1125 * time.Millisecond,
	Repeat:     0,
	Brightness: 255,
}
