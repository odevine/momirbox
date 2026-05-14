package widget

import (
	"image"
	"path/filepath"
	"time"

	"momirbox/internal/config"
	"momirbox/internal/hardware"
)

// BatteryStatus is the snapshot HomeView renders into the status corner.
type BatteryStatus struct {
	Percent  float64
	Charging bool
}

// BatteryProvider returns the current status, or nil if no UPS is present.
type BatteryProvider func() *BatteryStatus

// HomeView is the idle/desktop screen — the Flipper-style resting state. It
// paints a battery indicator in the corner; background art comes later. Any
// Select or Back press opens the main menu via OnOpenMenu.
type HomeView struct {
	battery BatteryProvider

	backdrop     *GIFPlayer
	lastGIFCheck time.Time

	OnOpenMenu func()
}

func NewHomeView(battery BatteryProvider) *HomeView {
	return &HomeView{battery: battery}
}

func (h *HomeView) Render(c *Canvas, dt time.Duration) {
	h.drawBackdrop(c, dt)

	x, y := c.Bounds().Dx()-24, 6
	if h.battery != nil {
		if bat := h.battery(); bat != nil {
			drawBatteryIcon(c, x, y, bat.Percent, bat.Charging)
			return
		}
	}
}

// drawBackdrop paints the home_splash.gif if it can be loaded, otherwise
// falls back to home_splash.png. The GIF is loaded lazily and retried at
// most every 2s so dropping the file in mid-run picks it up without
// spamming file opens.
func (h *HomeView) drawBackdrop(c *Canvas, dt time.Duration) {
	if h.backdrop == nil && time.Since(h.lastGIFCheck) > 2*time.Second {
		h.lastGIFCheck = time.Now()
		if p, err := LoadGIF(filepath.Join(config.AssetsDir, "home_splash.gif"), c.Bounds().Dx(), c.Bounds().Dy()); err == nil {
			h.backdrop = p
		}
	}

	if h.backdrop != nil {
		h.backdrop.Tick(dt)
		if f := h.backdrop.Frame(); f != nil {
			c.DrawImage(c.Bounds(), f)
		}
		return
	}

	if img := getAsset("home_splash.png"); img != nil {
		c.DrawImage(c.Bounds(), img)
	}
}

func (h *HomeView) HandleInput(a hardware.InputAction) bool {
	switch a {
	case hardware.InputSelect, hardware.InputBack:
		if h.OnOpenMenu != nil {
			h.OnOpenMenu()
		}
		return true
	}
	return false
}

func drawBatteryIcon(c *Canvas, x, y int, pct float64, charging bool) {
	const w, hgt = 15, 4
	c.FillRect(image.Rect(x, y, x+w, y+hgt), ColorWhite)
	c.FillRect(image.Rect(x+1, y+1, x+w-1, y+hgt-1), ColorBlack)
	c.FillRect(image.Rect(x+w, y+2, x+w+2, y+hgt-2), ColorWhite)

	fillWidth := int((pct / 100.0) * float64(w-2))
	if fillWidth > 0 {
		c.FillRect(image.Rect(x+1, y+1, x+1+fillWidth, y+hgt-1), ColorWhite)
	}
	if charging {
		cx, cy := x-5, y+(hgt/2)
		c.FillRect(image.Rect(cx-1, cy, cx+2, cy+1), ColorWhite)
		c.FillRect(image.Rect(cx, cy-1, cx+1, cy+2), ColorWhite)
	}
}
