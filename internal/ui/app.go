package ui

import (
	"fmt"
	"image"
	"os/exec"
	"sync"
	"time"

	"momirbox/internal/config"
	"momirbox/internal/hardware"
	"momirbox/internal/ui/widget"
)

// App is the thin host that owns the Dispatcher, runs the input/render loop,
// and polls the UPS so the home view can read its state.
type App struct {
	display hardware.Display
	input   hardware.Input
	ups     *hardware.UPS
	Printer hardware.Printer

	dispatcher *Dispatcher
	frame      *image.RGBA
	canvas     *widget.Canvas

	quitChan chan struct{}

	batteryPct float64
	isCharging bool
	batteryMu  sync.Mutex
}

func NewApp(d hardware.Display, i hardware.Input, u *hardware.UPS, p hardware.Printer) *App {
	frame := image.NewRGBA(image.Rect(0, 0, config.ScreenWidth, config.ScreenHeight))
	return &App{
		display:    d,
		input:      i,
		ups:        u,
		Printer:    p,
		dispatcher: NewDispatcher(),
		frame:      frame,
		canvas:     widget.NewCanvas(frame),
		quitChan:   make(chan struct{}),
	}
}

func (app *App) Run() {
	app.startBatteryPoller()

	// Splash -> Home. The splash auto-completes after its duration and
	// Replaces itself with the home scene.
	app.dispatcher.Push(widget.NewSplashView("momir_splash.png", 3*time.Second, func() {
		app.dispatcher.Replace(buildHomeScene(app), widget.Instant{})
	}), widget.Instant{})

	last := time.Now()
	lastInputAt := time.Now()
	for {
		select {
		case <-app.quitChan:
			fmt.Println("Shutting down UI loop...")
			return
		default:
		}

		now := time.Now()
		dt := now.Sub(last)
		last = now

		timeout := time.Duration(config.CurrentPrefs.ScreenTimeoutSec) * time.Second
		asleep := timeout > 0 && now.Sub(lastInputAt) > timeout

		if action := app.input.Poll(); action != hardware.InputNone {
			// The first input after sleep wakes the screen but is not
			// delivered, so the user can't accidentally activate anything.
			if !asleep {
				app.dispatcher.HandleInput(action)
			}
			lastInputAt = time.Now()
			asleep = false
		}

		if asleep {
			app.canvas.Clear(widget.ColorBlack)
		} else {
			app.dispatcher.Render(app.canvas, dt)
		}
		app.display.DrawFrame(app.frame)

		time.Sleep(config.FrameDelay)
	}
}

func (app *App) Quit() {
	select {
	case <-app.quitChan:
	default:
		close(app.quitChan)
	}
}

// PowerOff halts the UI loop and shuts down the Pi.
func (app *App) PowerOff() {
	app.Quit()
	_ = exec.Command("sudo", "shutdown", "-h", "now").Run()
}

func (app *App) startBatteryPoller() {
	if app.ups == nil {
		return
	}

	poll := func() {
		pct, _ := app.ups.GetBatteryPercentage()
		charging, _ := app.ups.IsCharging()
		app.batteryMu.Lock()
		app.batteryPct = pct
		app.isCharging = charging
		app.batteryMu.Unlock()
	}
	poll()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				poll()
			case <-app.quitChan:
				return
			}
		}
	}()
}
