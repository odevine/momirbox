package widget

import (
	"time"

	"momirbox/internal/hardware"
)

// LoadingView shows a label while a background job runs. The job is started
// in a goroutine the moment the view is constructed; when it completes,
// onResult is invoked from the render loop (single-threaded) with the work
// function's return value. onResult is the right place to pop the loading
// view and surface success/failure.
//
// When Backdrop is set, the GIF is played behind the label and the label is
// drawn near the top of the view instead of centered.
type LoadingView struct {
	label    string
	done     chan error
	fired    bool
	onResult func(error)

	Backdrop *GIFPlayer
}

func NewLoadingView(label string, work func() error, onResult func(error)) *LoadingView {
	l := &LoadingView{
		label:    label,
		done:     make(chan error, 1),
		onResult: onResult,
	}
	go func() { l.done <- work() }()
	return l
}

func (l *LoadingView) Render(c *Canvas, dt time.Duration) {
	if !l.fired {
		select {
		case err := <-l.done:
			l.fired = true
			if l.onResult != nil {
				l.onResult(err)
			}
		default:
		}
	}

	if l.Backdrop != nil {
		l.Backdrop.Tick(dt)
		if f := l.Backdrop.Frame(); f != nil {
			c.DrawImage(c.Bounds(), f)
		}
	}

	b := c.Bounds()
	w := c.MeasureString(l.label)
	y := b.Dy() / 2
	if l.Backdrop != nil {
		y = Theme.HeaderTextY
	}
	c.DrawString((b.Dx()-w)/2, y, l.label, ColorWhite)
}

// HandleInput swallows all input — the user can't interrupt the background
// job. This is a deliberate choice; once we have a Popup/cancel pattern we
// can revisit.
func (l *LoadingView) HandleInput(a hardware.InputAction) bool { return true }
