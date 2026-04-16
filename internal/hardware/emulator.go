//go:build !pi

package hardware

import (
	"image"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Emulator provides a desktop window for development and testing.
// It implements Display, Input, and the Ebitengine Game interface.
type Emulator struct {
	currentFrame *ebiten.Image
	mu           sync.Mutex
	inputQueue   chan InputAction
}

// NewEmulator creates and initializes a new OLED emulator instance for development/testing.
// It sets up Ebiten window dimensions and creates the input queue.
func NewEmulator() *Emulator {
	// Scaled up for visibility on high-resolution displays
	ebiten.SetWindowSize(128*4, 64*4)
	ebiten.SetWindowTitle("MomirBox OLED Emulator")

	return &Emulator{
		inputQueue: make(chan InputAction, 10),
	}
}

func (e *Emulator) DrawFrame(img image.Image) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.currentFrame = ebiten.NewImageFromImage(img)
	return nil
}

func (e *Emulator) Close() error {
	return nil
}

func (e *Emulator) Poll() InputAction {
	select {
	case action := <-e.inputQueue:
		return action
	default:
		return InputNone
	}
}

// Update polls the keyboard and translates keys into application actions.
func (e *Emulator) Update() error {
	var action InputAction = InputNone

	// Keyboard mapping mirrors the physical button layout
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		action = InputRight
	} else if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		action = InputLeft
	} else if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		action = InputUp
	} else if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		action = InputDown
	} else if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		action = InputSelect
	} else if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		action = InputBack
	}

	if action != InputNone {
		select {
		case e.inputQueue <- action:
		default:
			// Buffer full, dropping input
		}
	}
	return nil
}

func (e *Emulator) Draw(screen *ebiten.Image) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.currentFrame != nil {
		screen.DrawImage(e.currentFrame, &ebiten.DrawImageOptions{})
	}
}

func (e *Emulator) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 128, 64
}
