package hardware

import (
	"fmt"
	"time"

	"momirbox/internal/config"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

const (
	buttonDebounce  = 200 * time.Millisecond
	encoderDebounce = 30 * time.Millisecond
)

type button struct {
	pin      gpio.PinIn
	action   InputAction
	lastTime time.Time
}

type PiInput struct {
	inputQueue chan InputAction
	quitChan   chan struct{}

	encA    gpio.PinIn
	encB    gpio.PinIn
	lastEnc time.Time

	buttons []*button
}

func NewPiInput() (*PiInput, error) {
	pi := &PiInput{
		inputQueue: make(chan InputAction, 10),
		quitChan:   make(chan struct{}),
	}

	initButton := func(pinNum int, action InputAction) error {
		pin := gpioreg.ByName(fmt.Sprintf("GPIO%d", pinNum))
		if pin == nil {
			return fmt.Errorf("failed to locate GPIO%d", pinNum)
		}
		pin.In(gpio.PullUp, gpio.FallingEdge)
		pi.buttons = append(pi.buttons, &button{pin: pin, action: action})
		return nil
	}

	if err := initButton(config.PinEncoderCen, InputSelect); err != nil { return nil, err }
	if err := initButton(config.PinBackBtn, InputBack); err != nil { return nil, err }
	if err := initButton(config.PinEncoderUp, InputUp); err != nil { return nil, err }
	if err := initButton(config.PinEncoderRgt, InputRight); err != nil { return nil, err }
	if err := initButton(config.PinEncoderDwn, InputDown); err != nil { return nil, err }
	if err := initButton(config.PinEncoderLft, InputLeft); err != nil { return nil, err }


	pi.encA = gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinEncoderA))
	pi.encB = gpioreg.ByName(fmt.Sprintf("GPIO%d", config.PinEncoderB))

	if pi.encA == nil || pi.encB == nil {
		return nil, fmt.Errorf("failed to locate encoder GPIO pins")
	}

	pi.encA.In(gpio.PullUp, gpio.BothEdges)
	pi.encB.In(gpio.PullUp, gpio.NoEdge)

	go pi.watchHardware()

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

func (p *PiInput) watchHardware() {
	for {
		select {
		case <-p.quitChan:
			return
		default:
			if p.encA.WaitForEdge(10 * time.Millisecond) {
				if time.Since(p.lastEnc) > encoderDebounce {
					if p.encA.Read() == p.encB.Read() {
						p.inputQueue <- InputRight
					} else {
						p.inputQueue <- InputLeft
					}
					p.lastEnc = time.Now()
				}
			}

			now := time.Now()
			for _, b := range p.buttons {
				if b.pin.Read() == gpio.Low && now.Sub(b.lastTime) > buttonDebounce {
					p.inputQueue <- b.action
					b.lastTime = now
				}
			}
		}
	}
}