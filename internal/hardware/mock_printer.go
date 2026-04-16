package hardware

import (
	"fmt"
	"image"
)

// MockPrinter simulates a thermal printer for the desktop emulator.
type MockPrinter struct{}

func NewMockPrinter() *MockPrinter {
	return &MockPrinter{}
}

// PrintImage logs the receipt of image data to the console instead of a serial port.
func (p *MockPrinter) PrintImage(img image.Image) error {
	bounds := img.Bounds()
	// Replicates the successful processing logs from the original utility script
	fmt.Printf("[EMULATOR] 🖨️  Successfully 'printed' image. Size: %dx%d\n", bounds.Dx(), bounds.Dy())
	return nil
}

func (p *MockPrinter) Close() error {
	return nil
}
