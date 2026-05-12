package converter

import (
	"image"
	"image/color"
	"math"
)

// SauvolaThreshold produces a binary image (0 = text/foreground, 255 = background)
// using Sauvola's adaptive thresholding. window is the side length of the local
// neighborhood (odd values around 15-25 work well for ~5px text strokes); k is the
// sensitivity (typical range 0.2-0.5 — lower picks up more text, higher is stricter).
// When invert is true the input pixels are flipped (255 - v) before the integral
// images and threshold comparison so the same algorithm captures light strokes
// on dark backgrounds.
//
// Local mean and stddev are computed via integral images for O(1) per pixel.
func SauvolaThreshold(src *image.Gray, window int, k float64, invert bool) *image.Gray {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	pixel := func(x, y int) int64 {
		v := int64(src.GrayAt(x+bounds.Min.X, y+bounds.Min.Y).Y)
		if invert {
			return 255 - v
		}
		return v
	}

	stride := w + 1
	integral := make([]int64, stride*(h+1))
	integralSq := make([]int64, stride*(h+1))

	for y := 0; y < h; y++ {
		rowSum := int64(0)
		rowSumSq := int64(0)
		for x := 0; x < w; x++ {
			v := pixel(x, y)
			rowSum += v
			rowSumSq += v * v
			integral[(y+1)*stride+x+1] = integral[y*stride+x+1] + rowSum
			integralSq[(y+1)*stride+x+1] = integralSq[y*stride+x+1] + rowSumSq
		}
	}

	half := window / 2
	const R = 128.0
	out := image.NewGray(bounds)

	for y := 0; y < h; y++ {
		y0 := y - half
		if y0 < 0 {
			y0 = 0
		}
		y1 := y + half + 1
		if y1 > h {
			y1 = h
		}
		for x := 0; x < w; x++ {
			x0 := x - half
			if x0 < 0 {
				x0 = 0
			}
			x1 := x + half + 1
			if x1 > w {
				x1 = w
			}

			area := float64((x1 - x0) * (y1 - y0))
			sum := integral[y1*stride+x1] - integral[y0*stride+x1] - integral[y1*stride+x0] + integral[y0*stride+x0]
			sumSq := integralSq[y1*stride+x1] - integralSq[y0*stride+x1] - integralSq[y1*stride+x0] + integralSq[y0*stride+x0]

			mean := float64(sum) / area
			variance := float64(sumSq)/area - mean*mean
			if variance < 0 {
				variance = 0
			}
			stdDev := math.Sqrt(variance)
			threshold := mean * (1 + k*(stdDev/R-1))

			v := float64(pixel(x, y))
			if v < threshold {
				out.SetGray(x+bounds.Min.X, y+bounds.Min.Y, color.Gray{Y: 0})
			} else {
				out.SetGray(x+bounds.Min.X, y+bounds.Min.Y, color.Gray{Y: 255})
			}
		}
	}

	return out
}
