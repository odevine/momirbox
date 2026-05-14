package widget

import (
	"image"
	"image/color"
	"os"
	"path/filepath"

	"momirbox/internal/config"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

var (
	ColorBlack = color.RGBA{0, 0, 0, 255}
	ColorWhite = color.RGBA{255, 255, 255, 255}

	PixelFont  font.Face
	imageCache = make(map[string]image.Image)
)

// LoadFonts initializes PixelFont from the asset directory. Must be called
// after LoadTheme so font size/DPI are populated.
func LoadFonts() error {
	fontPath := filepath.Join(config.AssetsDir, "04B_03.TTF")
	data, err := os.ReadFile(fontPath)
	if err != nil {
		return err
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return err
	}
	PixelFont, err = opentype.NewFace(f, &opentype.FaceOptions{
		Size:    float64(Theme.FontSize),
		DPI:     float64(Theme.FontDPI),
		Hinting: font.HintingFull,
	})
	return err
}

func loadImageCached(path string) image.Image {
	if img, ok := imageCache[path]; ok {
		return img
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil
	}
	imageCache[path] = img
	return img
}

// getIcon resolves a filename inside config.IconsDir.
func getIcon(filename string) image.Image {
	if filename == "" {
		return nil
	}
	return loadImageCached(filepath.Join(config.IconsDir, filename))
}

// getAsset resolves a filename inside config.AssetsDir.
func getAsset(filename string) image.Image {
	if filename == "" {
		return nil
	}
	return loadImageCached(filepath.Join(config.AssetsDir, filename))
}

// drawHeader renders the standard module title bar — a title string with a
// horizontal underline. Used by Submenu, VariableItemList, and Dialog.
func drawHeader(c *Canvas, title string) {
	w := c.Bounds().Dx()
	c.FillRect(image.Rect(0, Theme.HeaderLineY1, w, Theme.HeaderLineY2), ColorWhite)
	c.DrawString(Theme.HeaderTextX, Theme.HeaderTextY, title, ColorWhite)
}
