package ui

import (
	"fmt"
	"sync"
	"time"

	"momirbox/internal/config"
	"momirbox/internal/hardware"
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
	Label      string
	Icon       string
	Submenu    *Menu
	Action     ActionFunc
	Adjust     AdjustFunc
	GetValue   GetValueFunc
	IsReadOnly bool
}

// Menu acts as a container for a list of MenuItems.
type Menu struct {
	Title      string
	Items      []MenuItem
	IsVertical bool
}

// BuildMenuTree constructs the application's top-level navigation hierarchy.
func BuildMenuTree(ups *hardware.UPS) *Menu {
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

	var batterySubmenu *Menu
	if ups != nil {
		var (
			isChg bool
			pct   float64
			v     float64
			mA    float64
			w     float64
			mu    sync.Mutex
		)

		go func() {
			// Initial read immediately so the UI isn't blank for 5 seconds
			newIsChg, _ := ups.IsCharging()
			newPct, _ := ups.GetBatteryPercentage()
			newV, _ := ups.ReadVoltage()
			newMA, _ := ups.ReadCurrent()
			newW, _ := ups.ReadPower()

			mu.Lock()
			isChg, pct, v, mA, w = newIsChg, newPct, newV, newMA, newW
			mu.Unlock()

			ticker := time.NewTicker(5 * time.Second)
			for range ticker.C {
				newIsChg, _ := ups.IsCharging()
				newPct, _ := ups.GetBatteryPercentage()
				newV, _ := ups.ReadVoltage()
				newMA, _ := ups.ReadCurrent()
				newW, _ := ups.ReadPower()

				mu.Lock()
				isChg, pct, v, mA, w = newIsChg, newPct, newV, newMA, newW
				mu.Unlock()
			}
		}()

		batterySubmenu = &Menu{
			Title:      "Battery",
			IsVertical: true,
			Items: []MenuItem{
				{
					Label:      "Status",
					IsReadOnly: true,
					GetValue: func() string {
						mu.Lock()
						defer mu.Unlock()
						if isChg {
							return "Charging"
						}
						return "On Battery"
					},
				},
				{
					Label:      "Percentage",
					IsReadOnly: true,
					GetValue: func() string {
						mu.Lock()
						defer mu.Unlock()
						return fmt.Sprintf("%.1f%%", pct)
					},
				},
				{
					Label:      "Voltage",
					IsReadOnly: true,
					GetValue: func() string {
						mu.Lock()
						defer mu.Unlock()
						return fmt.Sprintf("%.2f V", v)
					},
				},
				{
					Label:      "Current",
					IsReadOnly: true,
					GetValue: func() string {
						mu.Lock()
						defer mu.Unlock()
						return fmt.Sprintf("%.0f mA", mA)
					},
				},
				{
					Label:      "Power",
					IsReadOnly: true,
					GetValue: func() string {
						mu.Lock()
						defer mu.Unlock()
						return fmt.Sprintf("%.2f W", w)
					},
				},
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

	if batterySubmenu != nil {
		rootItems = append(rootItems, MenuItem{
			Label:   "Battery",
			Icon:    "settings.png",
			Submenu: batterySubmenu,
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
