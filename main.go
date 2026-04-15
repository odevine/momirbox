package main

import (
	"fmt"
	"path/filepath"

	"momirbox/internal/config"
	"momirbox/internal/hardware"
	"momirbox/internal/printer"
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

	if config.IsRaspberryPi {
		runPhysicalHardware()
	} else {
		runEmulator(themePath)
	}
}

func runPhysicalHardware() {
	fmt.Println("Starting MomirBox on Raspberry Pi Hardware...")

	display, err := hardware.NewPiDisplay()
	if err != nil {
		panic(err)
	}
	defer display.Close()

	input, err := hardware.NewPiInput()
	if err != nil {
		panic(err)
	}
	defer input.Close()

	// Standard serial port for Pi Zero UART communication
	thermalPrinter, err := printer.NewThermalPrinter("/dev/serial0")
	if err != nil {
		panic(err)
	}
	defer thermalPrinter.Close()

	app := ui.NewApp(display, input, thermalPrinter)
	app.Run()
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