package main

import (
	"fmt"

	"momirbox/internal/config"
	"momirbox/internal/hardware"
	"momirbox/internal/printer"
	"momirbox/internal/ui"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	config.InitPrefs()
	
	if err := ui.LoadFonts(); err != nil {
		panic(fmt.Sprintf("Failed to load fonts: %v", err))
	}

	// Direct execution path based on detected platform
	if config.IsRaspberryPi {
		runPhysicalHardware()
	} else {
		runEmulator()
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

func runEmulator() {
	fmt.Println("Starting MomirBox Emulator...")

	emulator := hardware.NewEmulator()
	mockPrinter := hardware.NewMockPrinter()
	app := ui.NewApp(emulator, emulator, mockPrinter)

	// App logic runs in the background so Ebitengine can own the main thread
	go app.Run()

	if err := ebiten.RunGame(emulator); err != nil {
		fmt.Printf("Emulator closed: %v\n", err)
	}
}
