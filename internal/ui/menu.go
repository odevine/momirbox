package ui

import (
	"fmt"

	"momirbox/internal/config"
	"momirbox/internal/mtgdb"
)

type ActionFunc func(app *App)
type AdjustFunc func(app *App, delta int)
type GetValueFunc func() string

// MenuItem represents a single entry in a menu, which can either trigger an action or open a submenu.
type MenuItem struct {
	Label    string
	Icon     string
	Submenu  *Menu
	Action   ActionFunc
	Adjust   AdjustFunc
	GetValue GetValueFunc
}

type Menu struct {
	Title string
	Items []MenuItem
	IsVertical bool
}

// BuildMenuTree constructs the application's navigation hierarchy based on current preferences.
// It is called recursively to rebuild the UI when settings like 'Enable Tokens' are toggled.
func BuildMenuTree() *Menu {
	// --- Submenus ---

	// Generate CMC selection for Momir Basic (0 through 16)
	momirSubmenu := &Menu{
		Title: "Select CMC",
		Items: make([]MenuItem, 17),
	}
	for i := 0; i < 17; i++ {
		cmc := i
		momirSubmenu.Items[i] = MenuItem{
			Icon: fmt.Sprintf("cmc_%d.png", cmc),
			Action: func(app *App) {
				go app.PlayGamblingSequence(cmc)
			},
		}
	}

	settingsSubmenu := &Menu{
		Title:      "Settings",
		IsVertical: true,
		Items: []MenuItem{
			{
				Label: "Anim Speed",
				GetValue: func() string {
					return fmt.Sprintf("%.2f", config.CurrentPrefs.AnimSpeed)
				},
				Adjust: func(app *App, delta int) {
					newSpeed := config.CurrentPrefs.AnimSpeed + (float64(delta) * 0.05)
					if newSpeed < 0.05 { newSpeed = 0.05 }
					if newSpeed > 1.00 { newSpeed = 1.00 }
					config.CurrentPrefs.AnimSpeed = newSpeed
				},
			},
			{
				Label: "Tokens Feature",
				GetValue: func() string {
					if config.CurrentPrefs.EnableTokens {
						return "On"
					}
					return "Off"
				},
				Adjust: func(app *App, delta int) {
					config.ToggleTokens()
				},
			},
			{
				Label: "Include Un-sets",
				GetValue: func() string {
					if config.CurrentPrefs.IncludeUnsets {
						return "On"
					}
					return "Off"
				},
				Adjust: func(app *App, delta int) {
					config.ToggleUnsets()
				},
			},
		},
	}

	syncSubmenu := &Menu{
		Title: "Sync",
		Items: []MenuItem{
			{
				Label: "Update DB",
				Icon: "update_db.png",
				Action: func(app *App) {
					app.StatusChan <- StatusUpdate{Title: "DB Update", Row1: "Starting up...", Progress: 0.0}
					go mtgdb.UpdateDatabase(func(row1, row2 string, progress float64, isDone bool) {
						app.StatusChan <- StatusUpdate{
							Title: "DB Update", Row1: row1, Row2: row2, Progress: progress, IsDone: isDone,
						}
					})
				},
			},
			{
				Label: "Sync Creatures",
				Icon: "download.png",
				Action: func(app *App) {
					app.StatusChan <- StatusUpdate{Title: "Sync Creatures", Row1: "Starting up...", Progress: 0.0}
					go mtgdb.SyncCreatures(func(row1, row2 string, progress float64, isDone bool) {
						app.StatusChan <- StatusUpdate{
							Title: "Sync Creatures", Row1: row1, Row2: row2, Progress: progress, IsDone: isDone,
						}
					})
				},
			},
		},
	}

	// Conditionally add Token synchronization if enabled in preferences
	if config.CurrentPrefs.EnableTokens {
		syncSubmenu.Items = append(syncSubmenu.Items, MenuItem{
			Label: "Sync Tokens",
			Icon: "download.png",
			Action: func(app *App) {
				app.StatusChan <- StatusUpdate{Title: "Sync Tokens", Row1: "Starting up...", Progress: 0.0}
				go mtgdb.SyncTokens(func(row1, row2 string, progress float64, isDone bool) {
					app.StatusChan <- StatusUpdate{
						Title: "Sync Tokens", Row1: row1, Row2: row2, Progress: progress, IsDone: isDone,
					}
				})
			},
		})
	}

	// --- Root Menu Construction ---

	rootItems := []MenuItem{
		{
			Label: "Momir Basic",
			Icon: "momir.png",
			Submenu: momirSubmenu,
		},
	}

	if config.CurrentPrefs.EnableTokens {
		tokensSubmenu := &Menu{
			Title: "Tokens",
			Items: []MenuItem{
				{Label: "Placeholder", Action: func(app *App) { fmt.Println("Tokens!") }},
			},
		}
		rootItems = append(rootItems, MenuItem{
			Label: "Tokens",
			Icon: "token.png",
			Submenu: tokensSubmenu,
		})
	}

	rootItems = append(rootItems, MenuItem{
		Label: "Sync",
		Icon: "sync.png",
		Submenu: syncSubmenu,
	})

	rootItems = append(rootItems, MenuItem{
		Label: "Settings",
		Icon: "settings.png",
		Submenu: settingsSubmenu,
	})

	rootItems = append(rootItems, MenuItem{
		Label: "Power Off",
		Icon: "power.png",
		Action: func(app *App) { app.PowerOff() },
	})

	return &Menu{
		Title: "Main",
		Items: rootItems,
	}
}