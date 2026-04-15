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
	// Draw renders a 128x64 image to the output device.
	DrawFrame(img image.Image) error
	Close() error
}

// Input defines how the application polls for user interactions.
type Input interface {
	// Poll returns the most recent action or InputNone if the queue is empty.
	Poll() InputAction
	Close() error
}

// Printer defines the contract for sending visual data to a thermal printer.
type Printer interface {
	// PrintImage processes and sends an image to the printer via serial communication.
	PrintImage(img image.Image) error
	Close() error
}