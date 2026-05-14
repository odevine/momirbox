package widget

import (
	"image"
	"time"

	"momirbox/internal/hardware"
)

// VariableItem is a single row in a VariableItemList. The row shows a label
// on the left and a value on the right.
//
//   - GetValue (optional): returns the display string. Called every render so
//     read-only items can show live data (battery readouts, etc.).
//   - OnChange (optional): fired when Left/Right is pressed on this row.
//     delta is -1 or +1. If nil, the row is read-only.
//   - OnEnter (optional): fired when Select is pressed on this row.
type VariableItem struct {
	Label    string
	GetValue func() string
	OnChange func(delta int)
	OnEnter  func()
}

// VariableItemList is Flipper's vertical list of label+value rows. Up/Down
// navigates rows, Left/Right cycles the selected item's value, Select fires
// OnEnter, Back invokes OnBack.
type VariableItemList struct {
	header   string
	items    []VariableItem
	selected int

	OnBack func()
}

func NewVariableItemList(header string) *VariableItemList {
	return &VariableItemList{header: header}
}

func (v *VariableItemList) Add(item VariableItem) *VariableItemList {
	v.items = append(v.items, item)
	return v
}

func (v *VariableItemList) Render(c *Canvas, dt time.Duration) {
	drawHeader(c, v.header)

	start := 0
	if v.selected >= Theme.VerticalMaxVisible {
		start = v.selected - Theme.VerticalMaxVisible + 1
	}

	for i := 0; i < Theme.VerticalMaxVisible; i++ {
		idx := start + i
		if idx >= len(v.items) {
			break
		}
		item := v.items[idx]
		y := Theme.VerticalStartY + (i * Theme.VerticalRowHeight)

		fg := ColorWhite
		if idx == v.selected {
			rect := image.Rect(0, y-Theme.VerticalHighlightTop, c.Bounds().Dx(), y+Theme.VerticalHighlightBot)
			c.FillRect(rect, ColorWhite)
			fg = ColorBlack
		}
		c.DrawString(Theme.VerticalTextX, y, item.Label, fg)

		if item.GetValue != nil {
			val := item.GetValue()
			if idx == v.selected && item.OnChange != nil {
				val = "< " + val + " >"
			}
			w := c.MeasureString(val)
			c.DrawString(Theme.VerticalRightMargin-w, y, val, fg)
		}
	}
}

func (v *VariableItemList) HandleInput(a hardware.InputAction) bool {
	if len(v.items) == 0 {
		if a == hardware.InputBack && v.OnBack != nil {
			v.OnBack()
			return true
		}
		return false
	}
	item := &v.items[v.selected]
	switch a {
	case hardware.InputDown:
		v.selected = (v.selected + 1) % len(v.items)
		return true
	case hardware.InputUp:
		v.selected--
		if v.selected < 0 {
			v.selected = len(v.items) - 1
		}
		return true
	case hardware.InputLeft:
		if item.OnChange != nil {
			item.OnChange(-1)
			return true
		}
		return false
	case hardware.InputRight:
		if item.OnChange != nil {
			item.OnChange(1)
			return true
		}
		return false
	case hardware.InputSelect:
		if item.OnEnter != nil {
			item.OnEnter()
			return true
		}
		return false
	case hardware.InputBack:
		if v.OnBack != nil {
			v.OnBack()
			return true
		}
	}
	return false
}
