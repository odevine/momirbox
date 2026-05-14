package widget

import (
	"image"
	"time"

	"momirbox/internal/config"
	"momirbox/internal/hardware"
)

// MenuItem is one entry in a Menu — an icon with an optional label and a
// single action fired on Select.
type MenuItem struct {
	Label  string
	Icon   string // filename in config.IconsDir
	Action func()
}

// Menu is Flipper's icon-carousel module: a horizontally scrolling list of
// icons with the active item's label shown below.
type Menu struct {
	// Title is an optional label centered at the top of the view. Empty
	// string draws nothing.
	Title string

	items    []MenuItem
	selected int
	visual   float64

	// OnBack fires on InputBack. Leave nil for the root home menu (which has
	// no parent to return to).
	OnBack func()
}

func NewMenu() *Menu { return &Menu{} }

func (m *Menu) AddItem(label, icon string, action func()) *Menu {
	m.items = append(m.items, MenuItem{Label: label, Icon: icon, Action: action})
	return m
}

func (m *Menu) Selected() int { return m.selected }
func (m *Menu) Count() int    { return len(m.items) }

func (m *Menu) Render(c *Canvas, dt time.Duration) {
	// Lerp the animated index toward the selection. Speed is per-frame
	// (matches the original behavior); for frame-rate independence later,
	// switch to 1 - exp(-k*dt).
	m.visual += (float64(m.selected) - m.visual) * config.CurrentPrefs.AnimSpeed

	b := c.Bounds()

	if m.Title != "" {
		tw := c.MeasureString(m.Title)
		c.DrawString((b.Dx()-tw)/2, Theme.HeaderTextY, m.Title, ColorWhite)
	}

	baseX := (b.Dx() / 2) - (Theme.CarouselIconSize / 2)

	for i, item := range m.items {
		offset := float64(i) - m.visual
		x := int(float64(baseX) + offset*float64(Theme.CarouselItemSpacing))
		if x <= -Theme.CarouselIconSize || x >= b.Dx() {
			continue
		}

		rect := image.Rect(
			x, Theme.CarouselIconY,
			x+Theme.CarouselIconSize, Theme.CarouselIconY+Theme.CarouselIconSize,
		)
		if icon := getIcon(item.Icon); icon != nil {
			c.DrawImage(rect, icon)
		} else if i != m.selected {
			c.FillRect(rect, ColorWhite)
			inner := image.Rect(rect.Min.X+1, rect.Min.Y+1, rect.Max.X-1, rect.Max.Y-1)
			c.FillRect(inner, ColorBlack)
		}

		if i == m.selected && item.Label != "" {
			tx := x + (Theme.CarouselIconSize / 2) - c.MeasureString(item.Label)/2
			c.DrawString(tx, Theme.CarouselTextY, item.Label, ColorWhite)
		}
	}
}

func (m *Menu) HandleInput(a hardware.InputAction) bool {
	if len(m.items) == 0 {
		if a == hardware.InputBack && m.OnBack != nil {
			m.OnBack()
			return true
		}
		return false
	}
	switch a {
	case hardware.InputRight:
		m.selected++
		if m.selected >= len(m.items) {
			m.selected = len(m.items) - 1
		}
		return true
	case hardware.InputLeft:
		m.selected--
		if m.selected < 0 {
			m.selected = 0
		}
		return true
	case hardware.InputSelect:
		if fn := m.items[m.selected].Action; fn != nil {
			fn()
		}
		return true
	case hardware.InputBack:
		if m.OnBack != nil {
			m.OnBack()
			return true
		}
	}
	return false
}
