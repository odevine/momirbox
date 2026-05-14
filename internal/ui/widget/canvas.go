package widget

import (
	"image"
	"image/color"
	"image/draw"
	_ "image/png"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Canvas wraps an *image.RGBA and tracks a stack of nested regions so child
// components can draw using coordinates local to their container.
//
// The bottom of the stack is the full framebuffer. PushRegion pushes a new
// sub-rect (in the parent's local space); draw calls translate local
// coordinates into the absolute backbuffer and clip to the active region.
type Canvas struct {
	img     *image.RGBA
	regions []image.Rectangle
}

func NewCanvas(img *image.RGBA) *Canvas {
	return &Canvas{
		img:     img,
		regions: []image.Rectangle{img.Bounds()},
	}
}

func (c *Canvas) PushRegion(local image.Rectangle) {
	parent := c.region()
	abs := local.Add(parent.Min).Intersect(parent)
	c.regions = append(c.regions, abs)
}

func (c *Canvas) PopRegion() {
	if len(c.regions) > 1 {
		c.regions = c.regions[:len(c.regions)-1]
	}
}

// Bounds returns the current region's size in local coordinates.
func (c *Canvas) Bounds() image.Rectangle {
	r := c.region()
	return image.Rect(0, 0, r.Dx(), r.Dy())
}

// Raw is an escape hatch for code that needs the underlying buffer (e.g. the
// dispatcher reading the final frame before pushing it to the display).
func (c *Canvas) Raw() *image.RGBA { return c.img }

func (c *Canvas) region() image.Rectangle { return c.regions[len(c.regions)-1] }

func (c *Canvas) toAbs(local image.Rectangle) image.Rectangle {
	r := c.region()
	return local.Add(r.Min).Intersect(r)
}

// Clear fills the current region with col.
func (c *Canvas) Clear(col color.Color) {
	r := c.region()
	draw.Draw(c.img, r, &image.Uniform{C: col}, image.Point{}, draw.Src)
}

// FillRect paints a local-coordinate rectangle.
func (c *Canvas) FillRect(local image.Rectangle, col color.Color) {
	draw.Draw(c.img, c.toAbs(local), &image.Uniform{C: col}, image.Point{}, draw.Src)
}

// DrawImage scales src into a local destination rect (nearest-neighbor).
func (c *Canvas) DrawImage(local image.Rectangle, src image.Image) {
	if src == nil {
		return
	}
	abs := c.toAbs(local)
	if abs.Empty() {
		return
	}
	xdraw.NearestNeighbor.Scale(c.img, abs, src, src.Bounds(), draw.Over, nil)
}

// DrawString writes label at a local baseline point, clipped to the region.
func (c *Canvas) DrawString(x, y int, label string, col color.Color) {
	if PixelFont == nil {
		return
	}
	r := c.region()
	sub := c.img.SubImage(r).(*image.RGBA)
	d := &font.Drawer{
		Dst:  sub,
		Src:  image.NewUniform(col),
		Face: PixelFont,
		Dot:  fixed.Point26_6{X: fixed.I(x + r.Min.X), Y: fixed.I(y + r.Min.Y)},
	}
	d.DrawString(label)
}

func (c *Canvas) MeasureString(label string) int {
	if PixelFont == nil {
		return 0
	}
	return font.MeasureString(PixelFont, label).Ceil()
}
