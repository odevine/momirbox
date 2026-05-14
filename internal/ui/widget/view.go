package widget

import (
	"time"

	"momirbox/internal/hardware"
)

// View is a single screen managed by the Dispatcher.
//
// Render is called once per frame. dt is the time elapsed since the previous
// frame and lets views advance their own time-based animations (the
// dispatcher does NOT provide a separate tick step).
//
// HandleInput returns true to consume an action; false lets the dispatcher
// continue searching for a handler (currently only the top view is queried,
// but the contract leaves room for transparent overlays).
type View interface {
	Render(c *Canvas, dt time.Duration)
	HandleInput(a hardware.InputAction) bool
}
