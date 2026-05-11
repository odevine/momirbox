package hardware

import (
	"image"
)

// InputAction represents a normalized user interaction from any hardware source.
type InputAction int

const (
	InputNone InputAction = iota
	InputUp
	InputDown
	InputLeft
	InputRight
	InputSelect
	InputBack
)

// Display defines the contract for pushing pixels to a screen, whether physical or emulated.
type Display interface {
	DrawFrame(img image.Image) error
	Close() error
}

// Input defines how the application polls for user interactions.
type Input interface {
	Poll() InputAction
	Close() error
}

// Printer defines the contract for sending raw binary data to a thermal printer.
type Printer interface {
	PrintRaw(data []byte) error
	Close() error
}
