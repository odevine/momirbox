package widget

import "time"

// Transition drives an animated swap between two views. The dispatcher calls
// Update once per frame and Render every frame until Update returns true.
//
// Implementations may render `from` (outgoing) and `to` (incoming) at any
// positions/opacities they like. Use Canvas.PushRegion to clip each view to
// its animated rect.
type Transition interface {
	Update(dt time.Duration) (done bool)
	Render(c *Canvas, from, to View, dt time.Duration)
}

// Instant performs no animation — the swap happens in a single frame.
type Instant struct{}

func (Instant) Update(dt time.Duration) bool { return true }
func (Instant) Render(c *Canvas, from, to View, dt time.Duration) {
	if to != nil {
		to.Render(c, dt)
	} else if from != nil {
		from.Render(c, dt)
	}
}
