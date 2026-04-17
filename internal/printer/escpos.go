package printer

import (
	"bytes"
	"fmt"
	"image"

	"go.bug.st/serial"
	"golang.org/x/image/draw"
)

type ThermalPrinter struct {
	port serial.Port
}

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

	port.Write([]byte{0x1B, 0x40})
	return &ThermalPrinter{port: port}, nil
}

func (p *ThermalPrinter) Close() error {
	if p.port != nil {
		return p.port.Close()
	}
	return nil
}

func (p *ThermalPrinter) PrintImage(img image.Image) error {
	bounds := img.Bounds()
	targetWidth := 320
	aspectRatio := float64(bounds.Dy()) / float64(bounds.Dx())
	newHeight := int(float64(targetWidth) * aspectRatio)

	rasterHeight := newHeight / 2

	resizedImg := image.NewRGBA(image.Rect(0, 0, targetWidth, rasterHeight))
	draw.ApproxBiLinear.Scale(resizedImg, resizedImg.Bounds(), img, bounds, draw.Over, nil)

	rasterData := convertToESCPOSDithered(resizedImg)

	if _, err := p.port.Write(rasterData); err != nil {
		return err
	}

	p.port.Write([]byte{0x0A, 0x0A, 0x0A})
	p.port.Write([]byte{0x1D, 0x56, 0x41, 0x00})

	return nil
}

// convertToESCPOSDithered applies Floyd-Steinberg dithering optimized for Pi Zero
func convertToESCPOSDithered(img *image.RGBA) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	widthBytes := (width + 7) / 8

	// Pre-calculate grayscale values into a fast 1D array
	grayLevels := make([]int16, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y * img.Stride) + (x * 4)
			r := int32(img.Pix[offset])
			g := int32(img.Pix[offset+1])
			b := int32(img.Pix[offset+2])
			// Fast luminosity calculation
			grayLevels[y*width+x] = int16((r*299 + g*587 + b*114) / 1000)
		}
	}

	var buffer bytes.Buffer

	// GS v 0 command header
	buffer.Write([]byte{0x1D, 0x76, 0x30, 0x00})
	buffer.WriteByte(byte(widthBytes % 256))
	buffer.WriteByte(byte(widthBytes / 256))
	buffer.WriteByte(byte(height % 256))
	buffer.WriteByte(byte(height / 256))

	// Apply Floyd-Steinberg and pack bits
	for y := 0; y < height; y++ {
		for xByte := 0; xByte < widthBytes; xByte++ {
			var b byte = 0
			for bit := 0; bit < 8; bit++ {
				x := xByte*8 + bit
				if x < width {
					idx := y*width + x
					oldPixel := grayLevels[idx]

					var newPixel int16
					// 128 threshold
					if oldPixel < 128 {
						newPixel = 0
						b |= 1 << (7 - bit)
					} else {
						newPixel = 255
					}

					quantError := oldPixel - newPixel

					// Push the quantization error to neighboring pixels
					if x+1 < width {
						grayLevels[idx+1] += quantError * 7 / 16
					}
					if y+1 < height {
						if x-1 >= 0 {
							grayLevels[(y+1)*width+(x-1)] += quantError * 3 / 16
						}
						grayLevels[(y+1)*width+x] += quantError * 5 / 16
						if x+1 < width {
							grayLevels[(y+1)*width+(x+1)] += quantError * 1 / 16
						}
					}
				}
			}
			buffer.WriteByte(b)
		}
	}

	return buffer.Bytes()
}
