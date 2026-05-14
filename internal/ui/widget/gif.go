package widget

import (
	"image"
	"image/draw"
	"image/gif"
	"os"
	"time"

	xdraw "golang.org/x/image/draw"
)

// GIFPlayer plays a decoded GIF, pre-scaled to a target size for cheap
// per-frame draws. Advance with Tick(dt); read the current frame with
// Frame().
//
// Frames are pre-scaled independently — GIFs that rely on inter-frame
// compositing (delta-only payloads) will not look right. Export full-frame
// GIFs from your authoring tool to avoid this.
type GIFPlayer struct {
	frames []*image.RGBA
	delays []time.Duration
	cur    int
	accum  time.Duration
}

// LoadGIF reads a GIF from path and pre-scales every frame to (w, h) with
// nearest-neighbor. Returns an error if the file is missing or invalid.
func LoadGIF(path string, w, h int) (*GIFPlayer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	g, err := gif.DecodeAll(file)
	if err != nil {
		return nil, err
	}

	p := &GIFPlayer{
		frames: make([]*image.RGBA, len(g.Image)),
		delays: make([]time.Duration, len(g.Image)),
	}
	bounds := image.Rect(0, 0, w, h)
	for i, frame := range g.Image {
		scaled := image.NewRGBA(bounds)
		xdraw.NearestNeighbor.Scale(scaled, bounds, frame, frame.Bounds(), draw.Src, nil)
		p.frames[i] = scaled

		// GIF delays are in centiseconds. Floor absurdly-low values so a 0
		// or 1 doesn't pin playback at the frame rate.
		d := time.Duration(g.Delay[i]) * 10 * time.Millisecond
		if d < 20*time.Millisecond {
			d = 100 * time.Millisecond
		}
		p.delays[i] = d
	}
	return p, nil
}

// Tick advances the current frame by dt, wrapping back to frame 0.
func (p *GIFPlayer) Tick(dt time.Duration) {
	if len(p.frames) == 0 {
		return
	}
	p.accum += dt
	for p.accum >= p.delays[p.cur] {
		p.accum -= p.delays[p.cur]
		p.cur = (p.cur + 1) % len(p.frames)
	}
}

func (p *GIFPlayer) Frame() image.Image {
	if len(p.frames) == 0 {
		return nil
	}
	return p.frames[p.cur]
}
