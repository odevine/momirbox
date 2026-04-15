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

const (
	ScreenWidth  = 128
	ScreenHeight = 64
)

var (
	ColorBlack = color.RGBA{0, 0, 0, 255}
	ColorWhite = color.RGBA{255, 255, 255, 255}

	PixelFont font.Face
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
		// Note: We cast to float64 here as opentype expects it
		Size:    float64(Theme.FontSize),
		DPI:     float64(Theme.FontDPI),
		Hinting: font.HintingFull,
	})
	return err
}

func (app *App) fallbackSplash(img *image.RGBA) {
	drawString(img, Theme.SplashTextX, Theme.SplashTextY, "MomirBox", ColorWhite)
	app.display.DrawFrame(img)
}

func (app *App) renderSplash() {
	img := image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))
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

	xdraw.NearestNeighbor.Scale(img, img.Bounds(), splashImg, splashImg.Bounds(), draw.Over, nil)
	app.display.DrawFrame(img)
}

func (app *App) renderVerticalList(img *image.RGBA) {
	startIndex := 0
	if app.currentIndex >= Theme.VerticalMaxVisible {
		startIndex = app.currentIndex - Theme.VerticalMaxVisible + 1
	}

	for i := 0; i < Theme.VerticalMaxVisible; i++ {
		itemIndex := startIndex + i
		if itemIndex >= len(app.currentMenu.Items) {
			break
		}

		item := app.currentMenu.Items[itemIndex]
		yPos := Theme.VerticalStartY + (i * Theme.VerticalRowHeight)
		
		if itemIndex == app.currentIndex {
			bgRect := image.Rect(0, yPos-Theme.VerticalHighlightTop, ScreenWidth, yPos+Theme.VerticalHighlightBot)
			draw.Draw(img, bgRect, &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
			
			valStr := item.GetValue()
			if app.IsEditing {
				valStr = "< " + valStr + " >"
			}
			
			drawString(img, Theme.VerticalTextX, yPos, item.Label, ColorBlack)
			
			valWidth := font.MeasureString(PixelFont, valStr).Ceil()
			valX := Theme.VerticalRightMargin - valWidth
			drawString(img, valX, yPos, valStr, ColorBlack)

		} else {
			drawString(img, Theme.VerticalTextX, yPos, item.Label, ColorWhite)
			
			if item.GetValue != nil {
				valStr := item.GetValue()
				valWidth := font.MeasureString(PixelFont, valStr).Ceil()
				valX := Theme.VerticalRightMargin - valWidth
				drawString(img, valX, yPos, valStr, ColorWhite)
			}
		}
	}
}

func (app *App) renderHorizontalCarousel(img *image.RGBA) {
	baseX := (ScreenWidth / 2) - (Theme.CarouselIconSize / 2)

	for i, item := range app.currentMenu.Items {
		offset := float64(i) - app.visualIndex
		xPosFloat := float64(baseX) + (offset * float64(Theme.CarouselItemSpacing))
		xPos := int(xPosFloat)

		if xPos > -Theme.CarouselIconSize && xPos < ScreenWidth {
			iconRect := image.Rect(xPos, Theme.CarouselIconY, xPos+Theme.CarouselIconSize, Theme.CarouselIconY+Theme.CarouselIconSize)
			iconImg := getIcon(item.Icon)

			if i == app.currentIndex {
				if iconImg != nil {
					xdraw.NearestNeighbor.Scale(img, iconRect, iconImg, iconImg.Bounds(), draw.Over, nil)
				} else {
					draw.Draw(img, iconRect, &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
				}

				textX := xPos + (Theme.CarouselIconSize / 2)
				if PixelFont != nil {
					textWidth := font.MeasureString(PixelFont, item.Label).Ceil()
					textX -= (textWidth / 2)
				}
				
				drawString(img, textX, Theme.CarouselTextY, item.Label, ColorWhite)
			} else {
				if iconImg != nil {
					xdraw.NearestNeighbor.Scale(img, iconRect, iconImg, iconImg.Bounds(), draw.Over, nil)
				} else {
					innerRect := image.Rect(xPos+1, Theme.CarouselIconY+1, xPos+(Theme.CarouselIconSize-1), Theme.CarouselIconY+(Theme.CarouselIconSize-1))
					draw.Draw(img, iconRect, &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
					draw.Draw(img, innerRect, &image.Uniform{ColorBlack}, image.Point{}, draw.Src)
				}
			}
		}
	}
}

func (app *App) renderMenuToImage(img *image.RGBA) {
	draw.Draw(img, img.Bounds(), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)

	// Animate the visual index 
	app.visualIndex += (float64(app.currentIndex) - app.visualIndex) * config.CurrentPrefs.AnimSpeed

	draw.Draw(img, image.Rect(0, Theme.HeaderLineY1, ScreenWidth, Theme.HeaderLineY2), &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
	drawString(img, Theme.HeaderTextX, Theme.HeaderTextY, app.currentMenu.Title, ColorWhite)

	if app.currentMenu.IsVertical {
		app.renderVerticalList(img)
	} else {
		app.renderHorizontalCarousel(img)
	}
}

func (app *App) renderMenu() {
	img := image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	app.renderMenuToImage(img)
	app.display.DrawFrame(img)
}

func (app *App) renderStatus(status StatusUpdate) {
	img := image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{ColorBlack}, image.Point{}, draw.Src)

	drawString(img, Theme.HeaderTextX, Theme.HeaderTextY, status.Title, ColorWhite)
	draw.Draw(img, image.Rect(0, Theme.HeaderLineY1, ScreenWidth, Theme.HeaderLineY2), &image.Uniform{ColorWhite}, image.Point{}, draw.Src)

	drawString(img, Theme.HeaderTextX, Theme.StatusRow1Y, status.Row1, ColorWhite)
	drawString(img, Theme.HeaderTextX, Theme.StatusRow2Y, status.Row2, ColorWhite)

	if status.Progress > 0 {
		barWidth := int(float64(Theme.ProgressBarWidth) * status.Progress)
		
		outerRect := image.Rect(Theme.ProgressBarX, Theme.ProgressBarY, Theme.ProgressBarX+Theme.ProgressBarWidth+2, Theme.ProgressBarY+Theme.ProgressBarHeight)
		innerRect := image.Rect(Theme.ProgressBarX+1, Theme.ProgressBarY+1, Theme.ProgressBarX+Theme.ProgressBarWidth+1, Theme.ProgressBarY+Theme.ProgressBarHeight-1)
		
		draw.Draw(img, outerRect, &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
		draw.Draw(img, innerRect, &image.Uniform{ColorBlack}, image.Point{}, draw.Src)
		
		if barWidth > 0 {
			fillRect := image.Rect(Theme.ProgressBarX+1, Theme.ProgressBarY+1, Theme.ProgressBarX+1+barWidth, Theme.ProgressBarY+Theme.ProgressBarHeight-1)
			draw.Draw(img, fillRect, &image.Uniform{ColorWhite}, image.Point{}, draw.Src)
		}
	}

	app.display.DrawFrame(img)
}

func drawString(img *image.RGBA, x, y int, label string, col color.Color) {
	if PixelFont == nil {
		return 
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: PixelFont,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(label)
}

func getIcon(filename string) image.Image {
	if filename == "" {
		return nil
	}
	
	if img, ok := iconCache[filename]; ok {
		return img
	}

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

	iconCache[filename] = img
	return img
}