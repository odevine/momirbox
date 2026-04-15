package ui

import (
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"os"
	"path/filepath"
	"time"

	"momirbox/internal/config"
	"momirbox/internal/momir"

	xdraw "golang.org/x/image/draw"
)

// PlayGamblingSequence runs the cinematic animation when a card is rolled.
// This recreates the multi-stage visual sequence from the original Python implementation.
func (app *App) PlayGamblingSequence(cmc int) {
	app.currentState = StateGambling // Prevent menu interaction during animation

	// 1. Shutter Close: Black bars slide in from top and bottom
	stepSize := 64 / 30
	if stepSize < 2 {
		stepSize = 2
	}

	for step := 0; step <= 34; step += stepSize {
		img := image.NewRGBA(image.Rect(0, 0, 128, 64))
		app.renderMenuToImage(img)

		draw.Draw(img, image.Rect(0, 0, 128, step), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(0, 64-step, 128, 64), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)

		app.display.DrawFrame(img)
		time.Sleep(config.FrameDelay)
	}

	// 2. Text Sequence: Flash the catchphrase words sequentially
	words := []string{"LETS", "GO...", "GAMBLING!!!"}
	for _, word := range words {
		img := image.NewRGBA(image.Rect(0, 0, 128, 64))
		draw.Draw(img, img.Bounds(), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)
		
		x := 64 - (len(word) * 3) 
		drawString(img, x, 35, word, ColorWhite)
		
		app.display.DrawFrame(img)
		time.Sleep(500 * time.Millisecond)
	}

	// 3. Roll card in background while the GIF plays
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- momir.Roll(cmc, app.Printer)
	}()

	// 4. Play the background GIF loop
	gifPath := filepath.Join(config.AssetsDir, "lets_go_gambling.gif")
	app.playGIF(gifPath)

	if err := <-doneChan; err != nil {
		fmt.Println("Roll error:", err)
		img := image.NewRGBA(image.Rect(0, 0, 128, 64))
		draw.Draw(img, img.Bounds(), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)
		drawString(img, 10, 35, "MISSING IMAGES!", ColorWhite)
		app.display.DrawFrame(img)
		time.Sleep(2 * time.Second)
	}

	// 5. Shutter Open: Bars retract to reveal the menu
	for step := 34; step >= 0; step -= stepSize {
		img := image.NewRGBA(image.Rect(0, 0, 128, 64))
		app.renderMenuToImage(img)
		draw.Draw(img, image.Rect(0, 0, 128, step), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(0, 64-step, 128, 64), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)
		
		app.display.DrawFrame(img)
		time.Sleep(config.FrameDelay)
	}

	app.currentState = StateMenu
}

// playGIF renders the specified GIF file to the OLED display.
func (app *App) playGIF(path string) {
	file, err := os.Open(path)
	if err != nil {
		return 
	}
	defer file.Close()

	g, err := gif.DecodeAll(file)
	if err != nil {
		return
	}

	// Pre-scale frames to the native 128x64 resolution to optimize playback speed
	var scaledFrames []*image.RGBA
	for _, frame := range g.Image {
		scaled := image.NewRGBA(image.Rect(0, 0, 128, 64))
		xdraw.NearestNeighbor.Scale(scaled, scaled.Bounds(), frame, frame.Bounds(), draw.Over, nil)
		scaledFrames = append(scaledFrames, scaled)
	}

	// Repeat the GIF for a fixed duration of loops
	for loop := 0; loop < 10; loop++ {
		for i, frame := range scaledFrames {
			app.display.DrawFrame(frame)
			
			delay := g.Delay[i]
			if delay < 2 {
				delay = 10 
			}
			time.Sleep(time.Duration(delay) * 10 * time.Millisecond)
		}
	}
}