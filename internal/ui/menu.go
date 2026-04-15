package ui

import (
	"fmt"

	"momirbox/internal/config"
	"momirbox/internal/mtgdb"
)

type ActionFunc func(app *App)

// MenuItem represents a single entry in a menu, which can either trigger an action or open a submenu.
type MenuItem struct {
	Label   string
	Icon    string
	Submenu *Menu
	Action  ActionFunc
}

type Menu struct {
	Title string
	Items []MenuItem
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

	// Preference toggles that trigger a menu rebuild upon change
	prefsSubmenu := &Menu{
		Title: "Prefs",
		Items: []MenuItem{
			{
				Label: fmt.Sprintf("Tokens: %v", config.CurrentPrefs.EnableTokens),
				Action: func(app *App) {
					config.ToggleTokens()
					app.currentMenu = BuildMenuTree()
				},
			},
			{
				Label: fmt.Sprintf("Un-sets: %v", config.CurrentPrefs.IncludeUnsets),
				Action: func(app *App) {
					config.ToggleUnsets()
					app.currentMenu = BuildMenuTree()
				},
			},
		},
	}

	settingsSubmenu := &Menu{
		Title: "Settings",
		Items: []MenuItem{
			{
				Label: "Update DB",
				Icon: "update_db_icon.png",
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
				Icon: "download_icon.png",
				Action: func(app *App) {
					app.StatusChan <- StatusUpdate{Title: "Sync Creatures", Row1: "Starting up...", Progress: 0.0}
					go mtgdb.SyncCreatures(func(row1, row2 string, progress float64, isDone bool) {
						app.StatusChan <- StatusUpdate{
							Title: "Sync Creatures", Row1: row1, Row2: row2, Progress: progress, IsDone: isDone,
						}
					})
				},
			},
			{Label: "Prefs", Submenu: prefsSubmenu},
		},
	}

	// Conditionally add Token synchronization if enabled in preferences
	if config.CurrentPrefs.EnableTokens {
		settingsSubmenu.Items = append(settingsSubmenu.Items, MenuItem{
			Label: "Sync Tokens",
			Icon: "download_icon.png",
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
			Icon: "momir_icon.png",
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
			Icon: "token_icon.png",
			Submenu: tokensSubmenu,
		})
	}

	rootItems = append(rootItems, MenuItem{
		Label: "Settings",
		Icon: "settings_icon.png",
		Submenu: settingsSubmenu,
	})
	rootItems = append(rootItems, MenuItem{
		Label: "Power Off",
		Icon: "power_icon.png",
		Action: func(app *App) { app.PowerOff() },
	})

	return &Menu{
		Title: "Main",
		Items: rootItems,
	}
}