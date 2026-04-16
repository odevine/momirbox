package hardware

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/ssd1306"
	"periph.io/x/host/v3"

	"momirbox/internal/config"
)

// PiDisplay handles the physical SSD1306 OLED via SPI.
type PiDisplay struct {
	dev *ssd1306.Dev
}

// NewPiDisplay initializes the periph.io host and sets up the SPI communication.
func NewPiDisplay() (*PiDisplay, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph.io: %w", err)
	}

	// Opens the first available SPI bus, typically /dev/spidev0.0 on the Pi
	port, err := spireg.Open("")
	if err != nil {
		return nil, fmt.Errorf("failed to open SPI bus: %w", err)
	}

	dcPin := gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinDisplayDC))
	rstPin := gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinDisplayRST))

	if dcPin == nil || rstPin == nil {
		return nil, fmt.Errorf("failed to find display control pins (DC: %d, RST: %d)", config.PinDisplayDC, config.PinDisplayRST)
	}

	rstPin.Out(gpio.Low)
	time.Sleep(10 * time.Millisecond)
	rstPin.Out(gpio.High)
	time.Sleep(10 * time.Millisecond)

	dev, err := ssd1306.NewSPI(port, dcPin, &ssd1306.DefaultOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ssd1306 over SPI: %w", err)
	}

	return &PiDisplay{dev: dev}, nil
}

// Draw pushes the 128x64 image buffer to the physical hardware.
func (p *PiDisplay) DrawFrame(img image.Image) error {
  return p.dev.Draw(p.dev.Bounds(), img, image.Point{})
}

func (p *PiDisplay) Close() error {
	if p.dev != nil {
		// Create a blank, completely black 128x64 image
		blackImg := image.NewRGBA(image.Rect(0, 0, 128, 64))
		draw.Draw(blackImg, blackImg.Bounds(), &image.Uniform{color.Black}, image.Point{}, draw.Src)
		
		// Push the black frame to clear the screen
		p.DrawFrame(blackImg)
		
		// Halt the device hardware
		return p.dev.Halt()
	}
	return nil
}