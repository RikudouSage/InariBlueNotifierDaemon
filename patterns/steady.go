package patterns

// Steady keeps the selected LEDs on until another pattern is applied.
var Steady = Pattern{
	Name:       "Steady light",
	Hold:       HoldIndefinitely,
	Brightness: 255,
}
