package ui

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"

	"momirbox/internal/config"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	ColorBlack = color.RGBA{0, 0, 0, 255}
	ColorWhite = color.RGBA{255, 255, 255, 255}
	PixelFont  font.Face
	iconCache = make(map[string]image.Image)
)

func LoadFonts() error {
	fontPath := filepath.Join(config.AssetsDir, "04B_03.TTF")
	fontData, err := os.ReadFile(fontPath)
	if err != nil {
		return err
	}

	f, err := opentype.Parse(fontData)
	if err != nil {
		return err
	}

	PixelFont, err = opentype.NewFace(f, &opentype.FaceOptions{
		Size:    8,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	return err
}

func (app *App) fallbackSplash(img *image.RGBA) {
	drawString(img, 35, 35, "MomirBox", ColorWhite)
	app.display.DrawFrame(img)
}

func (app *App) renderSplash() {
	// Create the native OLED-sized buffer (128x64)
	img := image.NewRGBA(image.Rect(0, 0, 128, 64))
	draw.Draw(img, img.Bounds(), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)

	splashPath := filepath.Join(config.AssetsDir, "momir_splash.png")
	file, err := os.Open(splashPath)
	if err != nil {
		app.fallbackSplash(img)
		return
	}
	defer file.Close()

	splashImg, _, err := image.Decode(file)
	if err != nil {
		app.fallbackSplash(img)
		return
	}

	// Scale the splash image down to 128x64
	// Using NearestNeighbor to keep the 1-bit style since we can't utilize anti-aliasing
	xdraw.NearestNeighbor.Scale(img, img.Bounds(), splashImg, splashImg.Bounds(), draw.Over, nil)

	app.display.DrawFrame(img)
}

// renderMenuToImage draws the current menu state onto a provided RGBA canvas.
func (app *App) renderMenuToImage(img *image.RGBA) {
	draw.Draw(img, img.Bounds(), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)

	app.visualIndex += (float64(app.currentIndex) - app.visualIndex) * 0.15

	// Header bar with menu title
	draw.Draw(img, image.Rect(0, 14, 128, 15), &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
	drawString(img, 2, 11, app.currentMenu.Title, ColorWhite)

	// Horizontal scrolling icons
	itemSpacing := 40
	baseX := (128 / 2) - 12

	for i, item := range app.currentMenu.Items {
		offset := float64(i) - app.visualIndex
		xPosFloat := float64(baseX) + (offset * float64(itemSpacing))
		
		// Convert back to int for each frame to remain pixel-perfect
		xPos := int(xPosFloat)

		if xPos > -24 && xPos < 128 {
			iconRect := image.Rect(xPos, 22, xPos+24, 22+24)
			iconImg := getIcon(item.Icon)

			if i == app.currentIndex {
				if iconImg != nil {
					xdraw.NearestNeighbor.Scale(img, iconRect, iconImg, iconImg.Bounds(), draw.Over, nil)
				} else {
					// Fallback: Solid white block
					draw.Draw(img, iconRect, &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
				}

				textX := xPos + 12
				if PixelFont != nil {
					textWidth := font.MeasureString(PixelFont, item.Label).Ceil()
					textX = xPos + 12 - (textWidth / 2)
				}
				
				drawString(img, textX, 58, item.Label, ColorWhite)
			} else {
				if iconImg != nil {
					xdraw.NearestNeighbor.Scale(img, iconRect, iconImg, iconImg.Bounds(), draw.Over, nil)
				} else {
					// Fallback: Hollow block
					innerRect := image.Rect(xPos+1, 23, xPos+23, 22+23)
					draw.Draw(img, iconRect, &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
					draw.Draw(img, innerRect, &image.Uniform{ColorBlack}, image.Point{}, draw.Src)
				}
			}
		}
	}
}

func (app *App) renderMenu() {
	img := image.NewRGBA(image.Rect(0, 0, 128, 64))
	app.renderMenuToImage(img)
	app.display.DrawFrame(img)
}

// renderStatus draws a standardized overlay for progress bars and system messages.
func (app *App) renderStatus(status StatusUpdate) {
	img := image.NewRGBA(image.Rect(0, 0, 128, 64))
	draw.Draw(img, img.Bounds(), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)

	drawString(img, 2, 11, status.Title, ColorWhite)
	draw.Draw(img, image.Rect(0, 14, 128, 15), &image.Uniform{ColorWhite}, image.Point{}, draw.Src)

	drawString(img, 2, 28, status.Row1, ColorWhite)
	drawString(img, 2, 40, status.Row2, ColorWhite)

	if status.Progress > 0 {
		barWidth := int(124 * status.Progress)
		// Draw progress bar border
		draw.Draw(img, image.Rect(2, 50, 126, 54), &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
		// Hollow out the center
		draw.Draw(img, image.Rect(3, 51, 125, 53), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)
		// Fill current progress
		if barWidth > 0 {
			draw.Draw(img, image.Rect(3, 51, 3+barWidth, 53), &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
		}
	}

	app.display.DrawFrame(img)
}

// drawString is a helper for rendering text using our custom TTF font.
func drawString(img *image.RGBA, x, y int, label string, col color.Color) {
	// Fallback check in case LoadFonts() failed or wasn't called
	if PixelFont == nil {
		return 
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: PixelFont, // Swapped to our TTF font!
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(label)
}

// getIcon loads an image from disk and caches it, returning the cached version on subsequent calls.
func getIcon(filename string) image.Image {
	if filename == "" {
		return nil
	}
	
	// Return from cache if we already loaded it
	if img, ok := iconCache[filename]; ok {
		return img
	}

	// Otherwise, load it from the assets directory
	iconPath := filepath.Join(config.IconsDir, filename)
	file, err := os.Open(iconPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil
	}

	// Save to cache and return
	iconCache[filename] = img
	return img
}