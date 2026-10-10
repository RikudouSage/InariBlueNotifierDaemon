package patterns

import "time"

var QuickFlash = Pattern{
	Name:       "Quick Flash",
	Hold:       125 * time.Millisecond,
	Off:        125 * time.Millisecond,
	Brightness: 255,
}
