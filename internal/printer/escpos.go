package printer

import (
	"fmt"

	"go.bug.st/serial"
)

// ThermalPrinter handles serial communication with an ESC/POS thermal printer.
type ThermalPrinter struct {
	port serial.Port
}

// NewThermalPrinter initializes a serial connection to the thermal printer.
func NewThermalPrinter(devicePath string) (*ThermalPrinter, error) {
	mode := &serial.Mode{
		BaudRate: 9600,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(devicePath, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %w", devicePath, err)
	}

	// ESC @ initializes the printer
	port.Write([]byte{0x1B, 0x40})
	return &ThermalPrinter{port: port}, nil
}

// Close terminates the serial connection.
func (p *ThermalPrinter) Close() error {
	if p.port != nil {
		return p.port.Close()
	}
	return nil
}

// PrintRaw sends pre-computed ESC/POS raster data directly to the printer.
func (p *ThermalPrinter) PrintRaw(data []byte) error {
	if _, err := p.port.Write(data); err != nil {
		return fmt.Errorf("failed to write raster data: %w", err)
	}

	// Advance paper and perform a partial cut
	p.port.Write([]byte{0x0A, 0x0A, 0x0A})
	p.port.Write([]byte{0x1D, 0x56, 0x41, 0x00})

	return nil
}
