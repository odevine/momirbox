package converter

import (
	"bytes"
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"
)

type DitherMethod string

const (
	MethodFloydSteinberg DitherMethod = "Floyd-Steinberg"
	MethodAtkinson       DitherMethod = "Atkinson"
	MethodBayer          DitherMethod = "Bayer"
	MethodBlueNoise      DitherMethod = "Blue Noise"
)

type DitherSettings struct {
	Brightness int          `json:"brightness"`
	Contrast   float64      `json:"contrast"`
	Method     DitherMethod `json:"method"`
}

var bayer8x8 = [8][8]float64{
	{0, 48, 12, 60, 3, 51, 15, 63},
	{32, 16, 44, 28, 35, 19, 47, 31},
	{8, 56, 4, 52, 11, 59, 7, 55},
	{40, 24, 36, 20, 43, 27, 39, 23},
	{2, 50, 14, 62, 1, 49, 13, 61},
	{34, 18, 46, 30, 33, 17, 45, 29},
	{10, 58, 6, 54, 9, 57, 5, 53},
	{42, 26, 38, 22, 41, 25, 37, 21},
}

func srgbToLinear(c float64) float64 {
	c = c / 255.0
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func DitherImage(img image.Image, targetWidth int, settings DitherSettings, noiseImg image.Image) *image.Gray {
	bounds := img.Bounds()
	aspectRatio := float64(bounds.Dy()) / float64(bounds.Dx())
	newHeight := int(float64(targetWidth) * aspectRatio)

	resizedImg := image.NewRGBA(image.Rect(0, 0, targetWidth, newHeight))
	draw.ApproxBiLinear.Scale(resizedImg, resizedImg.Bounds(), img, bounds, draw.Over, nil)

	width := targetWidth
	height := newHeight

	linearGray := make([]float64, width*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y * resizedImg.Stride) + (x * 4)
			r := float64(resizedImg.Pix[offset])
			g := float64(resizedImg.Pix[offset+1])
			b := float64(resizedImg.Pix[offset+2])

			gray := (r*0.299 + g*0.587 + b*0.114)
			gray = (gray-128.0)*settings.Contrast + 128.0 + float64(settings.Brightness)

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

	switch settings.Method {
	case MethodFloydSteinberg:
		applyFloydSteinberg(linearGray, width, height, out)
	case MethodAtkinson:
		applyAtkinson(linearGray, width, height, out)
	case MethodBayer:
		applyBayer(linearGray, width, height, out)
	case MethodBlueNoise:
		applyBlueNoise(linearGray, width, height, out, noiseImg)
	default:
		applyFloydSteinberg(linearGray, width, height, out)
	}

	return out
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

			quantError := oldPixel - newPixel
			weight := quantError / 8.0

			if x+1 < width {
				levels[y*width+(x+1)] += weight
			}
			if x+2 < width {
				levels[y*width+(x+2)] += weight
			}
			if y+1 < height {
				if x-1 >= 0 {
					levels[(y+1)*width+(x-1)] += weight
				}
				levels[(y+1)*width+x] += weight
				if x+1 < width {
					levels[(y+1)*width+(x+1)] += weight
				}
			}
			if y+2 < height {
				levels[(y+2)*width+x] += weight
			}
		}
	}
}

func applyBayer(levels []float64, width, height int, out *image.Gray) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			threshold := (bayer8x8[y%8][x%8] + 0.5) / 64.0
			if levels[idx] < threshold {
				out.SetGray(x, y, color.Gray{Y: 0})
			} else {
				out.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
}

func applyBlueNoise(levels []float64, width, height int, out *image.Gray, noiseImg image.Image) {
	if noiseImg == nil {
		applyFloydSteinberg(levels, width, height, out)
		return
	}

	noiseBounds := noiseImg.Bounds()
	nw := noiseBounds.Dx()
	nh := noiseBounds.Dy()

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x

			nx := x % nw
			ny := y % nh
			r, _, _, _ := noiseImg.At(nx, ny).RGBA()
			threshold := float64(r>>8) / 255.0

			if levels[idx] < threshold {
				out.SetGray(x, y, color.Gray{Y: 0})
			} else {
				out.SetGray(x, y, color.Gray{Y: 255})
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
