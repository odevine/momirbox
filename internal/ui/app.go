package ui

import (
	"fmt"
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
}

// NewApp creates and initializes a new App instance with the provided hardware interfaces.
func NewApp(d hardware.Display, i hardware.Input, p hardware.Printer) *App {
	return &App{
		display:      d,
		input:        i,
		Printer:      p,
		currentState: StateSplash,
		IsEditing:    false,
		quitChan:     make(chan struct{}),
	}
}

// Run starts the main application loop for the UI.
func (app *App) Run() {
	app.renderSplash()
	time.Sleep(2 * time.Second)

	app.currentMenu = BuildMenuTree()
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
		if item.Adjust != nil {
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

// PowerOff initiates a graceful shutdown of the application loop.
func (app *App) PowerOff() {
	close(app.quitChan)
}
