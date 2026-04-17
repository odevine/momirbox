package ui

import (
	"fmt"

	"momirbox/internal/config"
	"momirbox/internal/momir"
)

// ActionFunc represents a callback triggered by selecting a menu item.
type ActionFunc func(app *App)

// AdjustFunc represents a callback for modifying a numerical or boolean setting.
type AdjustFunc func(app *App, delta int)

// GetValueFunc retrieves the display string for a configurable setting.
type GetValueFunc func() string

// MenuItem represents a single entry in a menu.
type MenuItem struct {
	Label    string
	Icon     string
	Submenu  *Menu
	Action   ActionFunc
	Adjust   AdjustFunc
	GetValue GetValueFunc
}

// Menu acts as a container for a list of MenuItems.
type Menu struct {
	Title      string
	Items      []MenuItem
	IsVertical bool
}

// BuildMenuTree constructs the application's top-level navigation hierarchy.
func BuildMenuTree() *Menu {
	momirSubmenu := &Menu{
		Title: "Select CMC",
		Items: make([]MenuItem, 0),
	}

	for i := 0; i < 17; i++ {
		cmc := i
		if momir.HasValidImages(cmc) {
			momirSubmenu.Items = append(momirSubmenu.Items, MenuItem{
				Icon: fmt.Sprintf("cmc_%d.png", cmc),
				Action: func(app *App) {
					go app.PlayGamblingSequence(cmc)
				},
			})
		}
	}

	if len(momirSubmenu.Items) == 0 {
		momirSubmenu.Items = append(momirSubmenu.Items, MenuItem{
			Label:  "No Images Synced",
			Action: func(app *App) {},
		})
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
					if newSpeed < 0.05 {
						newSpeed = 0.05
					}
					if newSpeed > 1.00 {
						newSpeed = 1.00
					}
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

	rootItems := []MenuItem{
		{
			Label:   "Momir Basic",
			Icon:    "momir.png",
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
			Label:   "Tokens",
			Icon:    "token.png",
			Submenu: tokensSubmenu,
		})
	}

	rootItems = append(rootItems, MenuItem{
		Label:   "Settings",
		Icon:    "settings.png",
		Submenu: settingsSubmenu,
	})

	rootItems = append(rootItems, MenuItem{
		Label: "Power Off",
		Icon:  "power.png",
		Action: func(app *App) {
			app.PowerOff()
		},
	})

	return &Menu{
		Title: "Main",
		Items: rootItems,
	}
}
