//go:build pi

package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"momirbox/internal/config"
	"momirbox/internal/hardware"
	"momirbox/internal/printer"
	"momirbox/internal/ui"
)

func main() {
	config.InitPrefs()

	themePath := filepath.Join(config.AssetsDir, "theme.json") 
	if err := ui.LoadTheme(themePath); err != nil {
		fmt.Printf("Warning: Could not load theme.json: %v\n", err)
	}

	ui.LoadFonts()
	runPhysicalHardware()
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

  sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down gracefully...")
		app.PowerOff()
	}()

	app.Run()
}

