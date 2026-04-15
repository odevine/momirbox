package ui

import (
	"fmt"
	"time"

	"momirbox/internal/hardware"
)

type AppState int

const (
	StateSplash AppState = iota
	StateMenu
	StateGambling
	StateStatusOverlay
)

// StatusUpdate facilitates UI-neutral progress reporting from background tasks.
type StatusUpdate struct {
	Title    string
	Row1     string
	Row2     string
	Progress float64
	IsDone   bool
}

// App manages the global state, input handling, and navigation of the MomirBox.
type App struct {
	display hardware.Display
	input   hardware.Input
	Printer hardware.Printer

	// Navigation state
	currentState AppState
	menuStack    []*Menu
	indexStack   []int
	currentMenu  *Menu
	currentIndex int

	// Concurrency
	StatusChan chan StatusUpdate
	quitChan   chan struct{}
}

// NewApp creates and initializes a new App instance with the provided hardware interfaces.
func NewApp(d hardware.Display, i hardware.Input, p hardware.Printer) *App {
	return &App{
		display:      d,
		input:        i,
		Printer:      p,
		currentState: StateSplash,
		StatusChan:   make(chan StatusUpdate, 10),
		quitChan:     make(chan struct{}),
	}
}

// Run starts the main application loop for the UI.
// It renders the initial splash screen, builds the menu tree, and enters a loop that processes
// hardware inputs, handles status updates, and renders the appropriate UI state.
func (app *App) Run() {
	app.renderSplash()
	time.Sleep(2 * time.Second) // Matches original SPLASH_DURATION

	app.currentMenu = BuildMenuTree()
	app.currentState = StateMenu

	for {
		select {
		case <-app.quitChan:
			fmt.Println("Shutting down UI loop...")
			return

		case status := <-app.StatusChan:
			// Handle status updates from the downloader or sync engine
			if status.IsDone {
				app.currentState = StateMenu
			} else {
				app.currentState = StateStatusOverlay
				app.renderStatus(status)
			}

		default:
			// Process hardware inputs non-blockingly
			action := app.input.Poll()
			if action != hardware.InputNone {
				app.handleInput(action)
			}

			if app.currentState == StateMenu {
				app.renderMenu()
			}

			// Target ~60 FPS to keep the horizontal menu smooth
			time.Sleep(16 * time.Millisecond)
		}
	}
}

// handleInput translates hardware actions into menu navigation or function calls.
func (app *App) handleInput(action hardware.InputAction) {
	if app.currentState != StateMenu {
		return 
	}

	switch action {
	case hardware.InputRight:
		app.currentIndex = (app.currentIndex + 1) % len(app.currentMenu.Items)
	case hardware.InputLeft:
		app.currentIndex--
		if app.currentIndex < 0 {
			app.currentIndex = len(app.currentMenu.Items) - 1
		}
	case hardware.InputSelect:
		item := app.currentMenu.Items[app.currentIndex]
		if item.Submenu != nil {
			// Push current state to stack and descend into submenu
			app.menuStack = append(app.menuStack, app.currentMenu)
			app.indexStack = append(app.indexStack, app.currentIndex)
			app.currentMenu = item.Submenu
			app.currentIndex = 0
		} else if item.Action != nil {
			item.Action(app)
		}
	case hardware.InputBack:
		if len(app.menuStack) > 0 {
			// Return to previous menu level and restore selection index
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