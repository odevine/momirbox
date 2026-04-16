package printer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	"go.bug.st/serial"
	"golang.org/x/image/draw"
)

// ThermalPrinter provides high-level control over an ESC/POS compatible thermal printer.
type ThermalPrinter struct {
	port serial.Port
}

// NewThermalPrinter establishes a serial connection and initializes the hardware.
func NewThermalPrinter(devicePath string) (*ThermalPrinter, error) {
	mode := &serial.Mode{
		BaudRate: 9600, // Common default for 58mm POS printers
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(devicePath, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %w", devicePath, err)
	}

	// Reset printer to default state
	port.Write([]byte{0x1B, 0x40})

	return &ThermalPrinter{port: port}, nil
}

func (p *ThermalPrinter) Close() error {
	if p.port != nil {
		return p.port.Close()
	}
	return nil
}

// PrintImage scales the source image to the printer width and spools it to the serial port.
func (p *ThermalPrinter) PrintImage(img image.Image) error {
	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	printerWidth := 384 // Native dot width for standard 58mm paper
	aspectRatio := float64(originalHeight) / float64(originalWidth)
	newHeight := int(float64(printerWidth) * aspectRatio)

	// Resize using Catmull-Rom for sharper detail on small thermal prints
	resizedImg := image.NewRGBA(image.Rect(0, 0, printerWidth, newHeight))
	draw.CatmullRom.Scale(resizedImg, resizedImg.Bounds(), img, bounds, draw.Over, nil)

	rasterData := convertToESCPOS(resizedImg)

	_, err := p.port.Write(rasterData)
	if err != nil {
		return fmt.Errorf("printer write failed: %w", err)
	}

	// Advance paper and trigger the cutter if available
	p.port.Write([]byte{0x0A, 0x0A, 0x0A})       // Feed 3 lines
	p.port.Write([]byte{0x1D, 0x56, 0x41, 0x00}) // Partial cut command

	return nil
}

// convertToESCPOS transforms an image into a GS v 0 raster bit-image command.
func convertToESCPOS(img *image.RGBA) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate bytes per line; 8 pixels are packed into 1 byte
	widthBytes := (width + 7) / 8

	var buffer bytes.Buffer

	// GS v 0 command header: mode, width (xL xH), height (yL yH)
	buffer.Write([]byte{0x1D, 0x76, 0x30, 0x00})
	buffer.WriteByte(byte(widthBytes % 256))
	buffer.WriteByte(byte(widthBytes / 256))
	buffer.WriteByte(byte(height % 256))
	buffer.WriteByte(byte(height / 256))

	// Threshold pixels into 1-bit data (1 = black/heat, 0 = white)
	for y := 0; y < height; y++ {
		for xByte := 0; xByte < widthBytes; xByte++ {
			var b byte = 0
			for bit := 0; bit < 8; bit++ {
				x := xByte*8 + bit
				if x < width {
					c := img.At(x, y)
					grayColor := color.GrayModel.Convert(c).(color.Gray)

					// Simple 50% threshold for monochrome conversion
					if grayColor.Y < 128 {
						b |= 1 << (7 - bit)
					}
				}
			}
			buffer.WriteByte(b)
		}
	}

	return buffer.Bytes()
}
