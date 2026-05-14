package widget

import (
	"time"

	"momirbox/internal/hardware"
)

// SplashView paints a centered image (falling back to text if the image is
// missing) and calls onDone once `duration` has elapsed. The callback is the
// right place to Replace the splash with the home view.
type SplashView struct {
	asset    string
	duration time.Duration
	elapsed  time.Duration
	onDone   func()
	fired    bool
}

func NewSplashView(asset string, duration time.Duration, onDone func()) *SplashView {
	return &SplashView{asset: asset, duration: duration, onDone: onDone}
}

func (s *SplashView) Render(c *Canvas, dt time.Duration) {
	s.elapsed += dt
	if !s.fired && s.elapsed >= s.duration {
		s.fired = true
		if s.onDone != nil {
			s.onDone()
		}
	}

	if img := getAsset(s.asset); img != nil {
		c.DrawImage(c.Bounds(), img)
		return
	}

	label := "MomirBox"
	b := c.Bounds()
	w := c.MeasureString(label)
	c.DrawString((b.Dx()-w)/2, b.Dy()/2, label, ColorWhite)
}

func (s *SplashView) HandleInput(a hardware.InputAction) bool { return true }
