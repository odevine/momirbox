package hardware

import (
	"fmt"
	"time"

	"momirbox/internal/config"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

// PiInput handles physical GPIO interactions on the Raspberry Pi.
type PiInput struct {
	inputQueue chan InputAction
	quitChan   chan struct{}
}

// NewPiInput initializes GPIO pins and starts the background monitoring loop.
func NewPiInput() (*PiInput, error) {
	pi := &PiInput{
		inputQueue: make(chan InputAction, 10),
		quitChan:   make(chan struct{}),
	}

	// Reference pins by their BCM numbering from the global config
	pinEncA := gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinEncoderA))
	pinEncB := gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinEncoderB))
	pinSelect := gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinEncoderBtn))
	pinBack := gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinBackBtn))

	if pinEncA == nil || pinEncB == nil || pinSelect == nil || pinBack == nil {
		return nil, fmt.Errorf("failed to locate one or more GPIO pins")
	}

	// Initialize pins with internal pull-up resistors to prevent floating states
	pinEncA.In(gpio.PullUp, gpio.BothEdges)
	pinEncB.In(gpio.PullUp, gpio.NoEdge)
	pinSelect.In(gpio.PullUp, gpio.FallingEdge)
	pinBack.In(gpio.PullUp, gpio.FallingEdge)

	go pi.watchHardware(pinEncA, pinEncB, pinSelect, pinBack)

	return pi, nil
}

func (p *PiInput) Poll() InputAction {
	select {
	case action := <-p.inputQueue:
		return action
	default:
		return InputNone
	}
}

func (p *PiInput) Close() error {
	close(p.quitChan)
	return nil
}

// watchHardware runs in a goroutine to process electrical signals into application events.
func (p *PiInput) watchHardware(pinA, pinB, pinSel, pinBack gpio.PinIn) {
	var lastSel, lastBack time.Time

	for {
		select {
		case <-p.quitChan:
			return
		default:
			// Handle Rotary Encoder rotation via quadrature decoding
			if pinA.WaitForEdge(10 * time.Millisecond) {
				aState := pinA.Read()
				bState := pinB.Read()
				
				if aState == bState {
					p.inputQueue <- InputRight
				} else {
					p.inputQueue <- InputLeft
				}
				// Basic mechanical debounce to prevent erratic scrolling
				time.Sleep(50 * time.Millisecond) 
			}

			// Process button presses with a 200ms software debounce
			if pinSel.Read() == gpio.Low && time.Since(lastSel) > 200*time.Millisecond {
				p.inputQueue <- InputSelect
				lastSel = time.Now()
			}

			if pinBack.Read() == gpio.Low && time.Since(lastBack) > 200*time.Millisecond {
				p.inputQueue <- InputBack
				lastBack = time.Now()
			}
		}
	}
}