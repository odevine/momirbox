//go:build !pi

package main

import (
	"fmt"
	"path/filepath"

	"momirbox/internal/config"
	"momirbox/internal/hardware"
	"momirbox/internal/ui"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	config.InitPrefs()

	themePath := filepath.Join(config.AssetsDir, "theme.json")
	if err := ui.LoadTheme(themePath); err != nil {
		fmt.Printf("Warning: Could not load theme.json: %v\n", err)
	}

	ui.LoadFonts()
	runEmulator(themePath)
}

func runEmulator(themePath string) {
	fmt.Println("Starting MomirBox Emulator...")
	// Watch the theme file for changes and reload it automatically
	go ui.WatchTheme(themePath)

	emulator := hardware.NewEmulator()
	mockPrinter := hardware.NewMockPrinter()
	app := ui.NewApp(emulator, emulator, mockPrinter)

	// App logic runs in the background so Ebitengine can own the main thread
	go app.Run()

	if err := ebiten.RunGame(emulator); err != nil {
		fmt.Printf("Emulator closed: %v\n", err)
	}
}
