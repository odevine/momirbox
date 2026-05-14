package ui

import (
	"time"

	"momirbox/internal/hardware"
	"momirbox/internal/ui/widget"
)

// Dispatcher is the Flipper-style view stack manager. It owns the active
// stack of widget.Views, runs any in-flight widget.Transition, and routes
// input to the top of the stack.
//
// Push/Pop/Replace are safe to call from view callbacks (during HandleInput
// or even during Render); changes are committed immediately and any pending
// Transition animates between the outgoing and incoming top views.
type Dispatcher struct {
	stack []widget.View

	// During a transition the outgoing view is held here while `stack`
	// already reflects the post-transition state. Nil when idle.
	outgoing   widget.View
	transition widget.Transition
}

func NewDispatcher() *Dispatcher { return &Dispatcher{} }

func (d *Dispatcher) Top() widget.View {
	if len(d.stack) == 0 {
		return nil
	}
	return d.stack[len(d.stack)-1]
}

func (d *Dispatcher) Depth() int { return len(d.stack) }

func (d *Dispatcher) Push(v widget.View, t widget.Transition) {
	if t == nil {
		t = widget.Instant{}
	}
	d.outgoing = d.Top()
	d.stack = append(d.stack, v)
	d.transition = t
}

func (d *Dispatcher) Pop(t widget.Transition) {
	if len(d.stack) == 0 {
		return
	}
	if t == nil {
		t = widget.Instant{}
	}
	d.outgoing = d.stack[len(d.stack)-1]
	d.stack = d.stack[:len(d.stack)-1]
	d.transition = t
}

func (d *Dispatcher) Replace(v widget.View, t widget.Transition) {
	if t == nil {
		t = widget.Instant{}
	}
	d.outgoing = d.Top()
	if len(d.stack) > 0 {
		d.stack[len(d.stack)-1] = v
	} else {
		d.stack = append(d.stack, v)
	}
	d.transition = t
}

// Render draws the current frame. dt is the time elapsed since the previous
// Render call.
func (d *Dispatcher) Render(c *widget.Canvas, dt time.Duration) {
	c.Clear(widget.ColorBlack)
	if d.transition != nil {
		d.transition.Render(c, d.outgoing, d.Top(), dt)
		if d.transition.Update(dt) {
			d.outgoing = nil
			d.transition = nil
		}
		return
	}
	if v := d.Top(); v != nil {
		v.Render(c, dt)
	}
}

// HandleInput delivers an action to the top of the stack. Inputs are dropped
// during transitions so a double-press can't race the animation.
func (d *Dispatcher) HandleInput(a hardware.InputAction) bool {
	if d.transition != nil {
		return true
	}
	if v := d.Top(); v != nil {
		return v.HandleInput(a)
	}
	return false
}
