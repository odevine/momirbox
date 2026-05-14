package widget

import (
	"image"
	"time"

	"momirbox/internal/hardware"
)

// SubmenuItem is one row in a Submenu — a text label with a single action.
type SubmenuItem struct {
	Label  string
	Action func()
}

// Submenu is Flipper's vertical, scrollable list of text items with a header.
// Up/Down navigates rows, Select fires the action, Back invokes OnBack.
type Submenu struct {
	header   string
	items    []SubmenuItem
	selected int

	OnBack func()
}

func NewSubmenu(header string) *Submenu {
	return &Submenu{header: header}
}

func (s *Submenu) AddItem(label string, action func()) *Submenu {
	s.items = append(s.items, SubmenuItem{Label: label, Action: action})
	return s
}

func (s *Submenu) Render(c *Canvas, dt time.Duration) {
	drawHeader(c, s.header)

	start := 0
	if s.selected >= Theme.VerticalMaxVisible {
		start = s.selected - Theme.VerticalMaxVisible + 1
	}

	for i := 0; i < Theme.VerticalMaxVisible; i++ {
		idx := start + i
		if idx >= len(s.items) {
			break
		}
		item := s.items[idx]
		y := Theme.VerticalStartY + (i * Theme.VerticalRowHeight)

		fg := ColorWhite
		if idx == s.selected {
			rect := image.Rect(0, y-Theme.VerticalHighlightTop, c.Bounds().Dx(), y+Theme.VerticalHighlightBot)
			c.FillRect(rect, ColorWhite)
			fg = ColorBlack
		}
		c.DrawString(Theme.VerticalTextX, y, item.Label, fg)
	}
}

func (s *Submenu) HandleInput(a hardware.InputAction) bool {
	if len(s.items) == 0 {
		if a == hardware.InputBack && s.OnBack != nil {
			s.OnBack()
			return true
		}
		return false
	}
	switch a {
	case hardware.InputDown:
		s.selected = (s.selected + 1) % len(s.items)
		return true
	case hardware.InputUp:
		s.selected--
		if s.selected < 0 {
			s.selected = len(s.items) - 1
		}
		return true
	case hardware.InputSelect:
		if fn := s.items[s.selected].Action; fn != nil {
			fn()
		}
		return true
	case hardware.InputBack:
		if s.OnBack != nil {
			s.OnBack()
			return true
		}
	}
	return false
}
