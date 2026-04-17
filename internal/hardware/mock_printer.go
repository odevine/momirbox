package hardware

import (
	"fmt"
)

// MockPrinter simulates a thermal printer for the desktop emulator.
type MockPrinter struct{}

// NewMockPrinter initializes a dummy printer instance.
func NewMockPrinter() *MockPrinter {
	return &MockPrinter{}
}

// PrintRaw logs the receipt of binary data to the console instead of a serial port.
func (p *MockPrinter) PrintRaw(data []byte) error {
	fmt.Printf("[EMULATOR] 🖨️  Successfully 'printed' raw data. Size: %d bytes\n", len(data))
	return nil
}

// Close gracefully shuts down the mock interface.
func (p *MockPrinter) Close() error {
	return nil
}
