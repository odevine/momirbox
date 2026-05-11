package hardware

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/ssd1306"
	"periph.io/x/host/v3"

	"momirbox/internal/config"
)

// spiPortWrapper wraps an spi.Port to manually toggle a Chip Select (CS) pin.
type spiPortWrapper struct {
	spi.Port
	cs gpio.PinOut
}

func (w *spiPortWrapper) Connect(maxHz physic.Frequency, mode spi.Mode, bits int) (spi.Conn, error) {
	conn, err := w.Port.Connect(maxHz, mode, bits)
	if err != nil {
		return nil, err
	}
	return &spiConnWrapper{Conn: conn, cs: w.cs}, nil
}

// spiConnWrapper wraps an spi.Conn to assert the CS pin before and after transmissions.
type spiConnWrapper struct {
	spi.Conn
	cs gpio.PinOut
}

func (c *spiConnWrapper) Tx(w, r []byte) error {
	c.cs.Out(gpio.Low)        // Active low: select the display
	defer c.cs.Out(gpio.High) // Deselect after transmission
	return c.Conn.Tx(w, r)
}

func (c *spiConnWrapper) TxPackets(p []spi.Packet) error {
	c.cs.Out(gpio.Low)
	defer c.cs.Out(gpio.High)
	return c.Conn.TxPackets(p)
}

// PiDisplay handles the physical SSD1306 OLED via SPI.
type PiDisplay struct {
	dev *ssd1306.Dev
}

// NewPiDisplay initializes the periph.io host and sets up the SPI communication.
func NewPiDisplay() (*PiDisplay, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph.io: %w", err)
	}

	// Opens the first available SPI bus
	port, err := spireg.Open("")
	if err != nil {
		return nil, fmt.Errorf("failed to open SPI bus: %w", err)
	}

	// Because of our hardware constraints, we know CS is physically on GPIO 7.
	csPin := gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinDisplayCS))
	if csPin == nil {
		return nil, fmt.Errorf("failed to find CS pin (GPIO%d)", config.PinDisplayCS)
	}
	// Initialize CS to HIGH (deselected) so it's ready for output
	csPin.Out(gpio.High)

	// Wrap the SPI port to handle CS manually
	wrappedPort := &spiPortWrapper{
		Port: port,
		cs:   csPin,
	}

	dcPin := gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinDisplayDC))
	rstPin := gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinDisplayRST))

	if dcPin == nil || rstPin == nil {
		return nil, fmt.Errorf("failed to find display control pins (DC: %d, RST: %d)", config.PinDisplayDC, config.PinDisplayRST)
	}

	// Manually reset the display via the RST pin (GPIO 8)
	rstPin.Out(gpio.Low)
	time.Sleep(10 * time.Millisecond)
	rstPin.Out(gpio.High)
	time.Sleep(10 * time.Millisecond)

	// Pass the manually-wrapped port to the driver instead of the raw port
	dev, err := ssd1306.NewSPI(wrappedPort, dcPin, &ssd1306.DefaultOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ssd1306 over SPI: %w", err)
	}

	return &PiDisplay{dev: dev}, nil
}

// DrawFrame pushes the 128x64 image buffer to the physical hardware.
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
