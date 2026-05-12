package converter

import (
	"image"
	"image/color"
	"math"
)

// ApplyCLAHE applies Contrast Limited Adaptive Histogram Equalization to an 8-bit grayscale image.
// tilesX and tilesY are typically 8. clipLimit is usually between 2.0 and 4.0.
func ApplyCLAHE(src *image.Gray, tilesX, tilesY int, clipLimit float64) *image.Gray {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	out := image.NewGray(bounds)

	tileW := width / tilesX
	tileH := height / tilesY

	// Store CDFs for all tiles
	cdfs := make([][256]int, tilesX*tilesY)

	// Step 1 & 2: Calculate Histograms and clip them
	for ty := 0; ty < tilesY; ty++ {
		for tx := 0; tx < tilesX; tx++ {
			hist := make([]int, 256)
			startX, startY := tx*tileW, ty*tileH
			endX, endY := startX+tileW, startY+tileH

			// Handle edge tiles that might be slightly larger due to integer division
			if tx == tilesX-1 {
				endX = width
			}
			if ty == tilesY-1 {
				endY = height
			}

			tileArea := (endX - startX) * (endY - startY)

			// Build Histogram
			for y := startY; y < endY; y++ {
				for x := startX; x < endX; x++ {
					hist[src.GrayAt(x, y).Y]++
				}
			}

			// Clip Histogram
			limit := int(clipLimit * float64(tileArea) / 256.0)
			clipped := 0
			for i := 0; i < 256; i++ {
				if hist[i] > limit {
					clipped += hist[i] - limit
					hist[i] = limit
				}
			}

			// Redistribute clipped pixels uniformly
			redistribute := clipped / 256
			residual := clipped % 256
			for i := 0; i < 256; i++ {
				hist[i] += redistribute
			}
			// Distribute remaining residual evenly
			step := 256 / (residual + 1)
			for i := 0; i < residual; i++ {
				hist[i*step]++
			}

			// Step 3: Calculate CDF (Cumulative Distribution Function)
			var cdf [256]int
			sum := 0
			for i := 0; i < 256; i++ {
				sum += hist[i]
				// Normalize CDF to 0-255
				cdf[i] = int(math.Round(float64(sum) * 255.0 / float64(tileArea)))
			}
			cdfs[ty*tilesX+tx] = cdf
		}
	}

	// Step 4: Bilinear Interpolation to apply the CDFs smoothly
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelVal := src.GrayAt(x, y).Y

			// Find coordinates of the 4 surrounding tile centers
			tx := float64(x)/float64(tileW) - 0.5
			ty := float64(y)/float64(tileH) - 0.5

			tx1, ty1 := int(math.Floor(tx)), int(math.Floor(ty))
			tx2, ty2 := tx1+1, ty1+1

			// Weights for interpolation
			wx := tx - float64(tx1)
			wy := ty - float64(ty1)

			// Clamp to grid boundaries
			clamp := func(val, max int) int {
				if val < 0 {
					return 0
				}
				if val >= max {
					return max - 1
				}
				return val
			}

			idx11 := clamp(ty1, tilesY)*tilesX + clamp(tx1, tilesX)
			idx12 := clamp(ty1, tilesY)*tilesX + clamp(tx2, tilesX)
			idx21 := clamp(ty2, tilesY)*tilesX + clamp(tx1, tilesX)
			idx22 := clamp(ty2, tilesY)*tilesX + clamp(tx2, tilesX)

			// Fetch mapped values from the 4 surrounding CDFs
			val11 := float64(cdfs[idx11][pixelVal])
			val12 := float64(cdfs[idx12][pixelVal])
			val21 := float64(cdfs[idx21][pixelVal])
			val22 := float64(cdfs[idx22][pixelVal])

			// Interpolate
			interpX1 := val11*(1-wx) + val12*wx
			interpX2 := val21*(1-wx) + val22*wx
			finalVal := interpX1*(1-wy) + interpX2*wy

			// Corrected line:
			out.SetGray(x, y, color.Gray{Y: uint8(finalVal)})
		}
	}

	return out
}
