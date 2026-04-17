package ui

import (
	"fmt"
	"sync"
	"time"

	"momirbox/internal/hardware"
)

type AppState int

const (
	StateSplash AppState = iota
	StateMenu
	StateGambling
	StateStatusOverlay
	StateConfirmCancel
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
	visualIndex  float64
	IsEditing    bool
	menuStack    []*Menu
	indexStack   []int
	currentMenu  *Menu
	currentIndex int

	// Cancellation state
	lastStatus StatusUpdate
	confirmYes bool
	cancelChan chan struct{}

	// Concurrency
	StatusChan chan StatusUpdate
	quitChan   chan struct{}

	// Render Locking
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
			app.lastStatus = status

			if status.IsDone {
				app.currentState = StateMenu
				app.currentMenu = BuildMenuTree()
				app.menuStack = nil
				app.indexStack = nil
				app.currentIndex = 0
			} else if app.currentState != StateConfirmCancel {
				app.currentState = StateStatusOverlay
				app.renderStatus(status)
			}

		default:
			action := app.input.Poll()
			if action != hardware.InputNone {
				app.handleInput(action)
			}

			// Render routing
			if app.currentState == StateMenu {
				app.renderMenu()
			} else if app.currentState == StateConfirmCancel {
				app.renderConfirmModal()
			}

			time.Sleep(16 * time.Millisecond)
		}
	}
}

// handleInput translates hardware actions into menu navigation or function calls.
func (app *App) handleInput(action hardware.InputAction) {
	switch app.currentState {
	case StateMenu:
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

	case StateStatusOverlay:
		if action == hardware.InputBack {
			app.currentState = StateConfirmCancel
			app.confirmYes = false
		}

	case StateConfirmCancel:
		switch action {
		case hardware.InputLeft, hardware.InputRight:
			app.confirmYes = !app.confirmYes
		case hardware.InputBack:
			app.currentState = StateStatusOverlay
			app.renderStatus(app.lastStatus)
		case hardware.InputSelect:
			if app.confirmYes {
				if app.cancelChan != nil {
					close(app.cancelChan)
					app.cancelChan = nil
				}
				app.currentState = StateMenu
			} else {
				app.currentState = StateStatusOverlay
				app.renderStatus(app.lastStatus)
			}
		}
	}
}

// PowerOff initiates a graceful shutdown of the application loop.
func (app *App) PowerOff() {
	close(app.quitChan)
}
