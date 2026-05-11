package ui

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"momirbox/internal/hardware"
)

// AppState represents the primary view modes for the UI.
type AppState int

const (
	StateSplash AppState = iota
	StateMenu
	StateGambling
)

// App manages the global state, input handling, and navigation of the MomirBox.
type App struct {
	display hardware.Display
	input   hardware.Input
	ups     *hardware.UPS
	Printer hardware.Printer

	currentState AppState
	visualIndex  float64
	IsEditing    bool
	menuStack    []*Menu
	indexStack   []int
	currentMenu  *Menu
	currentIndex int

	quitChan chan struct{}
	renderMu sync.Mutex

	batteryPct float64
	isCharging bool
	batteryMu  sync.Mutex
}

// NewApp creates and initializes a new App instance with the provided hardware interfaces.
func NewApp(d hardware.Display, i hardware.Input, u *hardware.UPS, p hardware.Printer) *App {
	return &App{
		display:      d,
		input:        i,
		ups:          u,
		Printer:      p,
		currentState: StateSplash,
		IsEditing:    false,
		quitChan:     make(chan struct{}),
	}
}

// Run starts the main application loop for the UI.
func (app *App) Run() {
	app.renderSplash()
	time.Sleep(3 * time.Second)

	// Start the background battery poller for the toolbar
	if app.ups != nil {
		// Do an initial read so it isn't 0% for the first 5 seconds
		pct, _ := app.ups.GetBatteryPercentage()
		charging, _ := app.ups.IsCharging()

		app.batteryMu.Lock()
		app.batteryPct = pct
		app.isCharging = charging
		app.batteryMu.Unlock()

		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					newPct, _ := app.ups.GetBatteryPercentage()
					newCharging, _ := app.ups.IsCharging()

					app.batteryMu.Lock()
					app.batteryPct = newPct
					app.isCharging = newCharging
					app.batteryMu.Unlock()
				case <-app.quitChan:
					return
				}
			}
		}()
	}

	app.currentMenu = BuildMenuTree(app.ups)
	app.currentState = StateMenu

	for {
		select {
		case <-app.quitChan:
			fmt.Println("Shutting down UI loop...")
			return

		default:
			action := app.input.Poll()
			if action != hardware.InputNone {
				app.handleInput(action)
			}

			if app.currentState == StateMenu {
				app.renderMenu()
			}

			time.Sleep(16 * time.Millisecond)
		}
	}
}

// handleInput translates hardware actions into menu navigation or function calls.
func (app *App) handleInput(action hardware.InputAction) {
	if app.currentState != StateMenu {
		return
	}

	item := app.currentMenu.Items[app.currentIndex]
	switch action {
	case hardware.InputRight:
		if app.IsEditing && item.Adjust != nil {
			item.Adjust(app, 1)
		} else {
			app.currentIndex = (app.currentIndex + 1) % len(app.currentMenu.Items)
		}
	case hardware.InputLeft:
		if app.IsEditing && item.Adjust != nil {
			item.Adjust(app, -1)
		} else {
			app.currentIndex--
			if app.currentIndex < 0 {
				app.currentIndex = len(app.currentMenu.Items) - 1
			}
		}
	case hardware.InputSelect:
		if item.Adjust != nil && !item.IsReadOnly {
			app.IsEditing = !app.IsEditing
		} else if item.Submenu != nil {
			app.menuStack = append(app.menuStack, app.currentMenu)
			app.indexStack = append(app.indexStack, app.currentIndex)
			app.currentMenu = item.Submenu
			app.currentIndex = 0
			app.IsEditing = false
		} else if item.Action != nil {
			item.Action(app)
		}
	case hardware.InputBack:
		if app.IsEditing {
			app.IsEditing = false
		} else if len(app.menuStack) > 0 {
			lastIdx := len(app.menuStack) - 1
			app.currentMenu = app.menuStack[lastIdx]
			app.currentIndex = app.indexStack[lastIdx]
			app.menuStack = app.menuStack[:lastIdx]
			app.indexStack = app.indexStack[:lastIdx]
		}
	}
}

func (app *App) Quit() {
	select {
	case <-app.quitChan: // Already closed
	default:
		close(app.quitChan)
	}
}

// PowerOff physically shuts down the Raspberry Pi.
func (app *App) PowerOff() {
	app.Quit() // Stop the UI loop first so the screen clears

	// Execute the Linux shutdown command
	_ = exec.Command("sudo", "shutdown", "-h", "now").Run()
}
