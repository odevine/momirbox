package widget

import (
	"image"
	"time"

	"momirbox/internal/hardware"
)

type DialogButton struct {
	Label  string
	Action func()
}

// Dialog is Flipper's modal popup: a title, a message, and 1-3 horizontal
// buttons at the bottom. Left/Right moves between buttons, Select fires the
// highlighted button, Back invokes OnBack.
type Dialog struct {
	title    string
	message  string
	buttons  []DialogButton
	selected int

	OnBack func()
}

func NewDialog(title, message string) *Dialog {
	return &Dialog{title: title, message: message}
}

func (d *Dialog) AddButton(label string, action func()) *Dialog {
	d.buttons = append(d.buttons, DialogButton{Label: label, Action: action})
	return d
}

func (d *Dialog) Render(c *Canvas, dt time.Duration) {
	drawHeader(c, d.title)

	c.DrawString(Theme.VerticalTextX, Theme.VerticalStartY, d.message, ColorWhite)

	if len(d.buttons) == 0 {
		return
	}
	b := c.Bounds()
	btnY := b.Dy() - 2
	sliceW := b.Dx() / len(d.buttons)
	for i, btn := range d.buttons {
		cx := i*sliceW + sliceW/2
		labelW := c.MeasureString(btn.Label)
		if i == d.selected {
			rect := image.Rect(cx-labelW/2-3, btnY-9, cx+labelW/2+3, btnY+1)
			c.FillRect(rect, ColorWhite)
			c.DrawString(cx-labelW/2, btnY-2, btn.Label, ColorBlack)
		} else {
			c.DrawString(cx-labelW/2, btnY-2, btn.Label, ColorWhite)
		}
	}
}

func (d *Dialog) HandleInput(a hardware.InputAction) bool {
	if len(d.buttons) == 0 {
		if a == hardware.InputBack && d.OnBack != nil {
			d.OnBack()
			return true
		}
		return false
	}
	switch a {
	case hardware.InputLeft:
		d.selected--
		if d.selected < 0 {
			d.selected = len(d.buttons) - 1
		}
		return true
	case hardware.InputRight:
		d.selected = (d.selected + 1) % len(d.buttons)
		return true
	case hardware.InputSelect:
		if fn := d.buttons[d.selected].Action; fn != nil {
			fn()
		}
		return true
	case hardware.InputBack:
		if d.OnBack != nil {
			d.OnBack()
			return true
		}
	}
	return false
}
