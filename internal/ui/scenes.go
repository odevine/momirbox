package ui

import (
	"fmt"
	"path/filepath"
	"time"

	"momirbox/internal/config"
	"momirbox/internal/momir"
	"momirbox/internal/ui/widget"
)

// throttled wraps fn so it runs at most once per interval, returning the
// cached result in between. Useful for live readouts (battery telemetry,
// etc.) that would otherwise be fetched every render frame.
func throttled(interval time.Duration, fn func() string) func() string {
	var (
		last  time.Time
		value string
	)
	return func() string {
		if time.Since(last) >= interval {
			value = fn()
			last = time.Now()
		}
		return value
	}
}

// Each build*Scene function constructs a View configured for one screen and
// wires its callbacks against the App's dispatcher. The functions are the
// equivalent of Flipper's scene_on_enter handlers.

func buildHomeScene(app *App) *widget.HomeView {
	var battery widget.BatteryProvider
	if app.ups != nil {
		battery = func() *widget.BatteryStatus {
			app.batteryMu.Lock()
			defer app.batteryMu.Unlock()
			return &widget.BatteryStatus{Percent: app.batteryPct, Charging: app.isCharging}
		}
	}
	h := widget.NewHomeView(battery)
	h.OnOpenMenu = func() {
		app.dispatcher.Push(buildMainMenuScene(app), widget.Instant{})
	}
	return h
}

func buildMainMenuScene(app *App) widget.View {
	m := widget.NewMenu()
	m.OnBack = func() { app.dispatcher.Pop(widget.Instant{}) }
	m.Title = "Main Menu"

	m.AddItem("Momir Basic", "momir.png", func() {
		app.dispatcher.Push(buildCMCSelectorScene(app), widget.Instant{})
	})

	if config.CurrentPrefs.EnableTokens {
		m.AddItem("Tokens", "token.png", func() {
			app.dispatcher.Push(buildTokensScene(app), widget.Instant{})
		})
	}

	if app.ups != nil {
		m.AddItem("Battery", "battery.png", func() {
			app.dispatcher.Push(buildBatteryScene(app), widget.Instant{})
		})
	}

	m.AddItem("Settings", "settings.png", func() {
		app.dispatcher.Push(buildSettingsScene(app), widget.Instant{})
	})

	m.AddItem("Power Off", "power.png", func() {
		app.dispatcher.Push(buildPowerOffConfirmScene(app), widget.Instant{})
	})

	return m
}

func buildCMCSelectorScene(app *App) widget.View {
	m := widget.NewMenu()
	m.OnBack = func() { app.dispatcher.Pop(widget.Instant{}) }
	m.Title = "Select CMC"

	any := false
	for i := 0; i < 17; i++ {
		cmc := i
		if !momir.HasValidImages(cmc) {
			continue
		}
		any = true
		m.AddItem("", fmt.Sprintf("cmc_%d.png", cmc), func() {
			app.dispatcher.Push(buildRollingScene(app, cmc), widget.Instant{})
		})
	}

	if !any {
		d := widget.NewDialog("Momir Basic", "No images synced.")
		d.AddButton("OK", func() { app.dispatcher.Pop(widget.Instant{}) })
		d.OnBack = func() { app.dispatcher.Pop(widget.Instant{}) }
		return d
	}
	return m
}

func buildRollingScene(app *App, cmc int) widget.View {
	lv := widget.NewLoadingView(
		"LET'S GO GAMBLING!",
		func() error {
			if err := momir.Roll(cmc, app.Printer); err != nil {
				return err
			}
			// The printer keeps feeding paper after Roll returns. Hold the
			// loading view (and the GIF) on screen until it's likely done.
			time.Sleep(2 * time.Second)
			return nil
		},
		func(err error) {
			if err != nil {
				dlg := widget.NewDialog("Roll Failed", err.Error())
				dlg.AddButton("OK", func() { app.dispatcher.Pop(widget.Instant{}) })
				dlg.OnBack = func() { app.dispatcher.Pop(widget.Instant{}) }
				app.dispatcher.Replace(dlg, widget.Instant{})
				return
			}
			app.dispatcher.Pop(widget.Instant{})
		},
	)
	if p, err := widget.LoadGIF(
		filepath.Join(config.AssetsDir, "lets_go_gambling.gif"),
		config.ScreenWidth, config.ScreenHeight,
	); err == nil {
		lv.Backdrop = p
	}
	return lv
}

func buildSettingsScene(app *App) widget.View {
	list := widget.NewVariableItemList("Settings")
	list.OnBack = func() { app.dispatcher.Pop(widget.Instant{}) }

	list.Add(widget.VariableItem{
		Label: "Anim Speed",
		GetValue: func() string {
			return fmt.Sprintf("%.2f", config.CurrentPrefs.AnimSpeed)
		},
		OnChange: func(delta int) {
			s := config.CurrentPrefs.AnimSpeed + float64(delta)*0.05
			if s < 0.05 {
				s = 0.05
			}
			if s > 1.00 {
				s = 1.00
			}
			config.CurrentPrefs.AnimSpeed = s
		},
	})

	list.Add(widget.VariableItem{
		Label: "Screen Timeout",
		GetValue: func() string {
			if config.CurrentPrefs.ScreenTimeoutSec == 0 {
				return "Off"
			}
			return fmt.Sprintf("%ds", config.CurrentPrefs.ScreenTimeoutSec)
		},
		OnChange: func(delta int) {
			config.SetScreenTimeout(config.CurrentPrefs.ScreenTimeoutSec + delta*10)
		},
	})
	return list
}

func buildBatteryScene(app *App) widget.View {
	list := widget.NewVariableItemList("Battery")
	list.OnBack = func() { app.dispatcher.Pop(widget.Instant{}) }

	list.Add(widget.VariableItem{
		Label: "Status",
		GetValue: throttled(time.Second, func() string {
			app.batteryMu.Lock()
			defer app.batteryMu.Unlock()
			if app.isCharging {
				return "Charging"
			}
			return "On Battery"
		}),
	})
	list.Add(widget.VariableItem{
		Label: "Percent",
		GetValue: throttled(time.Second, func() string {
			app.batteryMu.Lock()
			defer app.batteryMu.Unlock()
			return fmt.Sprintf("%.1f%%", app.batteryPct)
		}),
	})
	list.Add(widget.VariableItem{
		Label: "Voltage",
		GetValue: throttled(time.Second, func() string {
			v, _ := app.ups.ReadVoltage()
			return fmt.Sprintf("%.2fV", v)
		}),
	})
	list.Add(widget.VariableItem{
		Label: "Current",
		GetValue: throttled(time.Second, func() string {
			mA, _ := app.ups.ReadCurrent()
			return fmt.Sprintf("%.0fmA", mA)
		}),
	})
	list.Add(widget.VariableItem{
		Label: "Power",
		GetValue: throttled(time.Second, func() string {
			w, _ := app.ups.ReadPower()
			return fmt.Sprintf("%.2fW", w)
		}),
	})
	return list
}

func buildTokensScene(app *App) widget.View {
	sm := widget.NewSubmenu("Tokens")
	sm.OnBack = func() { app.dispatcher.Pop(widget.Instant{}) }
	sm.AddItem("Placeholder", func() { fmt.Println("Tokens!") })
	return sm
}

func buildPowerOffConfirmScene(app *App) widget.View {
	d := widget.NewDialog("Power Off", "Shut down?")
	d.AddButton("Cancel", func() { app.dispatcher.Pop(widget.Instant{}) })
	d.AddButton("OK", func() { app.PowerOff() })
	d.OnBack = func() { app.dispatcher.Pop(widget.Instant{}) }
	return d
}
