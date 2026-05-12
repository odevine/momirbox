package converter

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"math"
	"os"

	"golang.org/x/image/draw"
)

type DitherMethod string

const (
	MethodFloydSteinberg DitherMethod = "Floyd-Steinberg"
	MethodAtkinson       DitherMethod = "Atkinson"
	MethodBayer          DitherMethod = "Bayer"
	MethodBlueNoise      DitherMethod = "Blue Noise"
)

// FrameStyle mirrors the raw frame-version strings from MTGJSON and Scryfall
// ("1993", "1997", "2003", "2015", "future") so the SauvolaRegions map can
// hold a distinct region list for every frame era.
type FrameStyle string

const (
	Frame1993   FrameStyle = "1993"
	Frame1997   FrameStyle = "1997"
	Frame2003   FrameStyle = "2003"
	Frame2015   FrameStyle = "2015"
	FrameFuture FrameStyle = "future"
)

// MapFrame validates a raw MTGJSON/Scryfall frame version. Unknown or empty
// values fall back to Frame2015 — the current-modern frame — so downstream
// code always has a region key to look up.
func MapFrame(raw string) FrameStyle {
	switch FrameStyle(raw) {
	case Frame1993, Frame1997, Frame2003, Frame2015, FrameFuture:
		return FrameStyle(raw)
	default:
		return Frame2015
	}
}

// CLAHERegion is one rectangular sub-region (as fractions of the resized
// image) where Contrast-Limited Adaptive Histogram Equalization is applied.
// TilesX, TilesY, and ClipLimit, when > 0, override the global CLAHE knobs
// for this region; otherwise the globals apply. Pixels outside any region
// are left untouched.
type CLAHERegion struct {
	XStart    float64 `json:"x_start"`
	XEnd      float64 `json:"x_end"`
	YStart    float64 `json:"y_start"`
	YEnd      float64 `json:"y_end"`
	TilesX    int     `json:"tiles_x,omitempty"`
	TilesY    int     `json:"tiles_y,omitempty"`
	ClipLimit float64 `json:"clip_limit,omitempty"`
}

// SauvolaRegion is one rectangular sub-region (as fractions of the resized
// image) where Sauvola thresholding is applied. Each region has its own
// BackgroundAlpha so a frame can mix, say, a hard-replace textbox with a
// softer typeline overlay. Regions are processed in slice order, so later
// regions overwrite earlier ones where they overlap. Setting WhiteText
// inverts the input grayscale before thresholding so the same algorithm
// captures light strokes on dark backgrounds (old-frame name bars,
// typelines, and P/T boxes). Window and K, when > 0, override the global
// SauvolaWindow / SauvolaK for this region; otherwise the globals apply.
// HaloRadius > 0 paints a ring of bgColor pixels of that pixel-radius
// around every text pixel, synthesizing a drop-shadow-style outline for
// readability on top of a dithered background.
type SauvolaRegion struct {
	XStart          float64 `json:"x_start"`
	XEnd            float64 `json:"x_end"`
	YStart          float64 `json:"y_start"`
	YEnd            float64 `json:"y_end"`
	BackgroundAlpha float64 `json:"background_alpha"`
	WhiteText       bool    `json:"white_text"`
	Window          int     `json:"window,omitempty"`
	K               float64 `json:"k,omitempty"`
	HaloRadius      int     `json:"halo_radius,omitempty"`
}

type DitherSettings struct {
	// Contrast-Limited Adaptive Histogram Equalization, applied per-region
	// (typically just the artwork box) so it doesn't fight the Sauvola
	// passes downstream. Tiles/ClipLimit on the settings are global defaults
	// that each region inherits unless it overrides them. Frames with no
	// regions defined skip CLAHE entirely.
	CLAHEEnabled   bool                         `json:"clahe_enabled"`
	CLAHETilesX    int                          `json:"clahe_tiles_x"`
	CLAHETilesY    int                          `json:"clahe_tiles_y"`
	CLAHEClipLimit float64                      `json:"clahe_clip_limit"`
	CLAHERegions   map[FrameStyle][]CLAHERegion `json:"clahe_regions"`

	// Sauvola adaptive thresholding, merged over the dither inside one or more
	// sub-regions chosen per FrameStyle (old, modern, and Future Sight frames
	// put their fields at different fractions of the card). Each region runs
	// its own Sauvola pass and has its own BackgroundAlpha, so a single frame
	// can mix a hard-replace textbox with a softer name/typeline/P-T overlay.
	// Regions are applied in slice order; later regions overwrite earlier ones
	// where they overlap.
	SauvolaEnabled bool                           `json:"sauvola_enabled"`
	SauvolaWindow  int                            `json:"sauvola_window"`
	SauvolaK       float64                        `json:"sauvola_k"`
	SauvolaRegions map[FrameStyle][]SauvolaRegion `json:"sauvola_regions"`

	// Test-only knobs: where to drop dither-test PNGs and a debug flag that
	// overlays a 2px red rectangle around each Sauvola region on the test PNG
	// so the masked area is easy to locate visually.
	TestOutputDir         string `json:"test_output_dir"`
	TestShowRegionBorders bool   `json:"test_show_region_borders"`
}

// DefaultDitherSettings returns the baseline settings used when fields are
// missing from the on-disk config.
func DefaultDitherSettings() DitherSettings {
	return DitherSettings{
		CLAHEEnabled:   true,
		CLAHETilesX:    16,
		CLAHETilesY:    16,
		CLAHEClipLimit: 3.0,
		CLAHERegions: map[FrameStyle][]CLAHERegion{
			Frame1993:   {{XStart: 0.08, XEnd: 0.92, YStart: 0.10, YEnd: 0.50}},
			Frame1997:   {{XStart: 0.08, XEnd: 0.92, YStart: 0.10, YEnd: 0.50}},
			Frame2003:   {{XStart: 0.08, XEnd: 0.92, YStart: 0.10, YEnd: 0.50}},
			Frame2015:   {{XStart: 0.075, XEnd: 0.925, YStart: 0.09, YEnd: 0.50}},
			FrameFuture: {{XStart: 0.07, XEnd: 0.93, YStart: 0.10, YEnd: 0.50}},
		},
		SauvolaEnabled: true,
		SauvolaWindow:  19,
		SauvolaK:       0.2,
		SauvolaRegions: map[FrameStyle][]SauvolaRegion{
			Frame1993:   {{XStart: 0.125, XEnd: 0.88, YStart: 0.60, YEnd: 0.887}},
			Frame1997:   {{XStart: 0.125, XEnd: 0.88, YStart: 0.60, YEnd: 0.887}},
			Frame2003:   {{XStart: 0.07, XEnd: 0.93, YStart: 0.625, YEnd: 0.90}},
			Frame2015:   {{XStart: 0.07, XEnd: 0.93, YStart: 0.625, YEnd: 0.90}},
			FrameFuture: {{XStart: 0.10, XEnd: 0.90, YStart: 0.62, YEnd: 0.88}},
		},
		TestOutputDir: "dither_tests",
	}
}

// LoadDitherSettings reads JSON from path and overlays it on top of the
// defaults. Missing fields keep their defaults; a missing file returns
// defaults with no error so the caller can edit-and-retry freely.
func LoadDitherSettings(path string) (DitherSettings, error) {
	cfg := DefaultDitherSettings()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Bayer 8x8 ordered-dither threshold matrix (values 0..63). Used to pick a
// spatially-stable subset of background pixels that retain the underlying
// dither when SauvolaBackgroundAlpha is between 0 and 1.
var bayer8x8 = [8][8]uint8{
	{0, 32, 8, 40, 2, 34, 10, 42},
	{48, 16, 56, 24, 50, 18, 58, 26},
	{12, 44, 4, 36, 14, 46, 6, 38},
	{60, 28, 52, 20, 62, 30, 54, 22},
	{3, 35, 11, 43, 1, 33, 9, 41},
	{51, 19, 59, 27, 49, 17, 57, 25},
	{15, 47, 7, 39, 13, 45, 5, 37},
	{63, 31, 55, 23, 61, 29, 53, 21},
}

func srgbToLinear(c float64) float64 {
	c = c / 255.0
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// DitherImage runs the resize/CLAHE/dither/Sauvola pipeline for one card.
// The frame argument selects which SauvolaRegion list to apply; pass
// Frame2015 when the frame is unknown.
func DitherImage(img image.Image, targetWidth int, settings DitherSettings, frame FrameStyle, noiseImg image.Image) *image.Gray {
	bounds := img.Bounds()
	aspectRatio := float64(bounds.Dy()) / float64(bounds.Dx())
	newHeight := int(float64(targetWidth) * aspectRatio)

	resizedImg := image.NewRGBA(image.Rect(0, 0, targetWidth, newHeight))
	draw.ApproxBiLinear.Scale(resizedImg, resizedImg.Bounds(), img, bounds, draw.Over, nil)

	width := targetWidth
	height := newHeight

	standardGray := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y * resizedImg.Stride) + (x * 4)
			r := float64(resizedImg.Pix[offset])
			g := float64(resizedImg.Pix[offset+1])
			b := float64(resizedImg.Pix[offset+2])

			grayVal := uint8(r*0.299 + g*0.587 + b*0.114)
			standardGray.SetGray(x, y, color.Gray{Y: grayVal})
		}
	}

	workingGray := standardGray
	if settings.CLAHEEnabled {
		workingGray = image.NewGray(image.Rect(0, 0, width, height))
		copy(workingGray.Pix, standardGray.Pix)
		for _, region := range settings.CLAHERegions[frame] {
			tilesX := region.TilesX
			if tilesX <= 0 {
				tilesX = settings.CLAHETilesX
			}
			tilesY := region.TilesY
			if tilesY <= 0 {
				tilesY = settings.CLAHETilesY
			}
			if tilesX <= 0 || tilesY <= 0 {
				continue
			}
			clip := region.ClipLimit
			if clip <= 0 {
				clip = settings.CLAHEClipLimit
			}

			xMin := int(region.XStart * float64(width))
			xMax := int(region.XEnd * float64(width))
			yMin := int(region.YStart * float64(height))
			yMax := int(region.YEnd * float64(height))
			if xMin < 0 {
				xMin = 0
			}
			if yMin < 0 {
				yMin = 0
			}
			if xMax > width {
				xMax = width
			}
			if yMax > height {
				yMax = height
			}
			if xMax <= xMin || yMax <= yMin {
				continue
			}

			rw, rh := xMax-xMin, yMax-yMin
			sub := image.NewGray(image.Rect(0, 0, rw, rh))
			for ry := 0; ry < rh; ry++ {
				for rx := 0; rx < rw; rx++ {
					sub.SetGray(rx, ry, standardGray.GrayAt(xMin+rx, yMin+ry))
				}
			}
			equalized := ApplyCLAHE(sub, tilesX, tilesY, clip)
			for ry := 0; ry < rh; ry++ {
				for rx := 0; rx < rw; rx++ {
					workingGray.SetGray(xMin+rx, yMin+ry, equalized.GrayAt(rx, ry))
				}
			}
		}
	}

	linearGray := make([]float64, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			gray := float64(workingGray.GrayAt(x, y).Y)
			gray = (gray - 128.0) + 128.0

			if gray < 0 {
				gray = 0
			}
			if gray > 255 {
				gray = 255
			}

			linearGray[y*width+x] = srgbToLinear(gray)
		}
	}

	out := image.NewGray(image.Rect(0, 0, width, height))
	applyFloydSteinberg(linearGray, width, height, out)

	if settings.SauvolaEnabled {
		for _, region := range settings.SauvolaRegions[frame] {
			window := region.Window
			if window <= 0 {
				window = settings.SauvolaWindow
			}
			if window <= 0 {
				continue
			}
			k := region.K
			if k <= 0 {
				k = settings.SauvolaK
			}

			xMin := int(region.XStart * float64(width))
			xMax := int(region.XEnd * float64(width))
			yMin := int(region.YStart * float64(height))
			yMax := int(region.YEnd * float64(height))
			if xMin < 0 {
				xMin = 0
			}
			if yMin < 0 {
				yMin = 0
			}
			if xMax > width {
				xMax = width
			}
			if yMax > height {
				yMax = height
			}
			if xMax <= xMin || yMax <= yMin {
				continue
			}
			subGray := standardGray.SubImage(image.Rect(xMin, yMin, xMax, yMax)).(*image.Gray)
			textMask := SauvolaThreshold(subGray, window, k, region.WhiteText)
			alpha := region.BackgroundAlpha
			if alpha < 0 {
				alpha = 0
			}
			if alpha > 1 {
				alpha = 1
			}
			textColor, bgColor := uint8(0), uint8(255)
			if region.WhiteText {
				textColor, bgColor = 255, 0
			}

			// Precompute a halo mask: any background pixel within HaloRadius of
			// a text pixel. Forces bgColor regardless of alpha so the outline
			// punches through transparent backgrounds.
			halo := region.HaloRadius
			if halo < 0 {
				halo = 0
			}
			var haloMask *image.Gray
			if halo > 0 {
				haloMask = image.NewGray(image.Rect(xMin, yMin, xMax, yMax))
				for ty := yMin; ty < yMax; ty++ {
					for tx := xMin; tx < xMax; tx++ {
						if textMask.GrayAt(tx, ty).Y != 0 {
							continue
						}
						for dy := -halo; dy <= halo; dy++ {
							ny := ty + dy
							if ny < yMin || ny >= yMax {
								continue
							}
							for dx := -halo; dx <= halo; dx++ {
								nx := tx + dx
								if nx < xMin || nx >= xMax {
									continue
								}
								haloMask.SetGray(nx, ny, color.Gray{Y: 255})
							}
						}
					}
				}
			}

			for y := yMin; y < yMax; y++ {
				for x := xMin; x < xMax; x++ {
					v := textMask.GrayAt(x, y).Y
					if v == 0 {
						out.SetGray(x, y, color.Gray{Y: textColor})
						continue
					}
					inHalo := haloMask != nil && haloMask.GrayAt(x, y).Y == 255
					if inHalo || float64(bayer8x8[y&7][x&7])/64.0 >= alpha {
						out.SetGray(x, y, color.Gray{Y: bgColor})
					}
				}
			}
		}
	}

	return out
}

func applyAtkinson(levels []float64, width, height int, out *image.Gray) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			oldPixel := levels[idx]
			var newPixel float64
			if oldPixel < 0.5 {
				newPixel = 0.0
				out.SetGray(x, y, color.Gray{Y: 0})
			} else {
				newPixel = 1.0
				out.SetGray(x, y, color.Gray{Y: 255})
			}
			quantError := (oldPixel - newPixel) / 8.0
			if x+1 < width {
				levels[y*width+(x+1)] += quantError
			}
			if x+2 < width {
				levels[y*width+(x+2)] += quantError
			}
			if y+1 < height {
				if x-1 >= 0 {
					levels[(y+1)*width+(x-1)] += quantError
				}
				levels[(y+1)*width+x] += quantError
				if x+1 < width {
					levels[(y+1)*width+(x+1)] += quantError
				}
			}
			if y+2 < height {
				levels[(y+2)*width+x] += quantError
			}
		}
	}
}

func applyFloydSteinberg(levels []float64, width, height int, out *image.Gray) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			oldPixel := levels[idx]
			var newPixel float64
			if oldPixel < 0.5 {
				newPixel = 0.0
				out.SetGray(x, y, color.Gray{Y: 0})
			} else {
				newPixel = 1.0
				out.SetGray(x, y, color.Gray{Y: 255})
			}

			quantError := oldPixel - newPixel

			if x+1 < width {
				levels[y*width+(x+1)] += quantError * (7.0 / 16.0)
			}
			if y+1 < height {
				if x-1 >= 0 {
					levels[(y+1)*width+(x-1)] += quantError * (3.0 / 16.0)
				}
				levels[(y+1)*width+x] += quantError * (5.0 / 16.0)
				if x+1 < width {
					levels[(y+1)*width+(x+1)] += quantError * (1.0 / 16.0)
				}
			}
		}
	}
}

func ImageToESCPOS(img *image.Gray) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	widthBytes := (width + 7) / 8

	var buffer bytes.Buffer

	buffer.Write([]byte{0x1D, 0x76, 0x30, 0x00})
	buffer.WriteByte(byte(widthBytes % 256))
	buffer.WriteByte(byte(widthBytes / 256))
	buffer.WriteByte(byte(height % 256))
	buffer.WriteByte(byte(height / 256))

	for y := 0; y < height; y++ {
		for xByte := 0; xByte < widthBytes; xByte++ {
			var b byte = 0
			for bit := 0; bit < 8; bit++ {
				x := xByte*8 + bit
				if x < width {
					if img.GrayAt(x, y).Y == 0 {
						b |= 1 << (7 - bit)
					}
				}
			}
			buffer.WriteByte(b)
		}
	}

	return buffer.Bytes()
}
