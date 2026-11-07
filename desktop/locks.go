package desktop

import (
	"fmt"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// Lock mode: while it is on, moving a lockable Sound or FX control writes a
// parameter lock for the selected step instead of changing the base sound,
// and the controls show that step's values. Clicking the grid selects steps
// instead of toggling them, so you can move between steps while locking.

func (u *UI) setLockMode(on bool) {
	if u.lockMode == on {
		return
	}
	u.finishLiveSlider()
	u.lockMode = on
	u.refresh()
}

// currentStep is the selected step of track id in the edited pattern.
func (u *UI) currentStep(id string) *groove.Step {
	t := u.session.Project.Track(id)
	if t == nil {
		return nil
	}
	steps := t.Patterns[u.session.Project.Pattern].Steps
	if len(steps) == 0 {
		return nil
	}
	return &steps[max(0, min(u.selectedStep, len(steps)-1))]
}

// editStep changes the selected step, capturing the slot and index now so
// a later selection change cannot redirect the edit.
func (u *UI) editStep(id, label string, fn func(*groove.Step)) {
	slot, index := u.session.Project.Pattern, u.selectedStep
	u.edit(label, func(p *groove.Project) error {
		if t := p.Track(id); t != nil && index < len(t.Patterns[slot].Steps) {
			fn(&t.Patterns[slot].Steps[index])
		}
		return nil
	})
}
func (u *UI) setLock(id, lock string, value float64) {
	u.editStep(id, "Lock value", func(s *groove.Step) {
		if s.Locks == nil {
			s.Locks = map[string]float64{}
		}
		s.Locks[lock] = value
	})
}
func (u *UI) clearLock(id, lock string) {
	u.editStep(id, "Clear lock", func(s *groove.Step) { delete(s.Locks, lock) })
}
func (u *UI) locked(id, lock string) (float64, bool) {
	if lock == "" {
		return 0, false
	}
	s := u.currentStep(id)
	if s == nil {
		return 0, false
	}
	v, ok := s.Locks[lock]
	return v, ok
}

// paramControl describes a Sound or FX control. Values are in model units;
// toSlider/fromSlider map them to slider positions for non-linear controls.
type paramControl struct {
	track, name, lock    string // lock is the step-lock ID, or "" if not lockable
	min, max, increment  float64
	def                  float64
	base                 func() float64
	setBase              func(float64)
	format               func(float64) string
	toSlider, fromSlider func(float64) float64
}

// paramName is a slider's name label. Double-clicking it clears the shown
// lock, or resets an unlocked control to its default.
type paramName struct {
	widget.Label
	reset func()
}

func newParamName(text string, reset func()) *paramName {
	n := &paramName{reset: reset}
	n.Text = text
	n.Truncation = fyne.TextTruncateEllipsis
	n.ExtendBaseWidget(n)
	return n
}
func (n *paramName) DoubleTapped(*fyne.PointEvent) {
	if n.reset != nil {
		n.reset()
	}
}

// styleParamName shows whether the selected step locks this control.
func styleParamName(n *widget.Label, locked, dim bool) {
	importance, style := widget.MediumImportance, fyne.TextStyle{}
	if locked {
		importance, style = widget.HighImportance, fyne.TextStyle{Bold: true}
	} else if dim {
		importance = widget.LowImportance
	}
	if n.Importance != importance || n.TextStyle != style {
		n.Importance, n.TextStyle = importance, style
		n.Refresh()
	}
}

func (u *UI) paramRow(c paramControl) fyne.CanvasObject {
	to, from := c.toSlider, c.fromSlider
	sliderMin, sliderMax, step := c.min, c.max, c.increment
	if to == nil {
		to, from = func(v float64) float64 { return v }, func(v float64) float64 { return v }
	} else {
		sliderMin, sliderMax, step = to(c.min), to(c.max), (to(c.max)-to(c.min))/1000
	}
	if c.format == nil {
		c.format = func(v float64) string { return sliderValue(v, c.increment) }
	}
	effective := func() float64 {
		if u.lockMode {
			if v, ok := u.locked(c.track, c.lock); ok {
				return v
			}
		}
		return c.base()
	}
	name := newParamName(c.name, func() {
		if _, ok := u.locked(c.track, c.lock); ok && u.lockMode {
			u.clearLock(c.track, c.lock)
			return
		}
		c.setBase(c.def)
	})
	reading := widget.NewLabel(c.format(effective()))
	reading.Alignment = fyne.TextAlignTrailing
	slider := widget.NewSlider(sliderMin, sliderMax)
	slider.Step = step
	slider.SetValue(to(effective()))
	u.bindLiveSlider(slider, func(position float64) {
		v := min(c.max, max(c.min, from(position)))
		reading.SetText(c.format(v))
		if u.lockMode && c.lock != "" {
			u.setLock(c.track, c.lock, v)
		} else {
			c.setBase(v)
		}
	})
	sync := func() {
		_, isLocked := u.locked(c.track, c.lock)
		styleParamName(&name.Label, isLocked, u.lockMode && c.lock == "")
		if u.liveSlider == slider {
			return
		}
		v := effective()
		setSlider(slider, to(v))
		setLabel(reading, c.format(v))
	}
	sync()
	u.bound(sync)
	height := slider.MinSize().Height
	return container.NewBorder(nil, nil, container.NewGridWrap(fyne.NewSize(sliderNameWidth, height), name), container.NewGridWrap(fyne.NewSize(sliderValueWidth, height), reading), slider)
}

// paramSelect is the lock-aware counterpart of paramRow for stepped choices
// stored as numbers (e.g. delay division).
func (u *UI) paramSelect(c paramControl, options []string, values []float64) fyne.CanvasObject {
	effective := func() float64 {
		if u.lockMode {
			if v, ok := u.locked(c.track, c.lock); ok {
				return v
			}
		}
		return c.base()
	}
	name := newParamName(c.name, func() {
		if _, ok := u.locked(c.track, c.lock); ok && u.lockMode {
			u.clearLock(c.track, c.lock)
			return
		}
		c.setBase(c.def)
	})
	choice := u.boundSelect(options, func() int { return slices.Index(values, effective()) }, func(i int) {
		if u.lockMode && c.lock != "" {
			u.setLock(c.track, c.lock, values[i])
		} else {
			c.setBase(values[i])
		}
	})
	sync := func() {
		_, isLocked := u.locked(c.track, c.lock)
		styleParamName(&name.Label, isLocked, u.lockMode && c.lock == "")
	}
	sync()
	u.bound(sync)
	return container.NewBorder(nil, nil, container.NewGridWrap(fyne.NewSize(sliderNameWidth, choice.MinSize().Height), name), nil, choice)
}

// lockToggle is the inspector header's Lock switch; its label names the step.
func (u *UI) lockToggle() fyne.CanvasObject {
	check := widget.NewCheck("", func(on bool) { u.setLockMode(on) })
	sync := func() {
		setCheck(check, u.lockMode)
		if text := fmt.Sprintf("Lock step %02d", u.selectedStep+1); check.Text != text {
			check.Text = text
			check.Refresh()
		}
	}
	sync()
	u.bound(sync)
	return check
}

// stepLocks lists the selected step's locks with value sliders and remove
// buttons, plus an Add menu for any lockable parameter.
func (u *UI) stepLocks(t *groove.Track) fyne.CanvasObject {
	id := t.ID
	panel := container.NewVBox()
	params := []groove.Parameter{}
	for _, p := range groove.LockParameters(*t) {
		if p.Lockable {
			params = append(params, p)
		}
	}
	any := func() bool {
		s := u.currentStep(id)
		return s != nil && len(s.Locks) > 0
	}
	var add *widget.Button
	add = widget.NewButton("Add lock", func() {
		var items []*fyne.MenuItem
		for _, p := range params {
			p := p
			if _, ok := u.locked(id, p.ID); ok {
				continue
			}
			items = append(items, fyne.NewMenuItem(p.Name, func() {
				u.setLock(id, p.ID, u.session.Project.Track(id).ParameterValue(p.ID))
			}))
		}
		if len(items) == 0 {
			return
		}
		pop := widget.NewPopUpMenu(fyne.NewMenu("Add lock", items...), u.window.Canvas())
		pop.ShowAtPosition(fyne.CurrentApp().Driver().AbsolutePositionForObject(add).Add(fyne.NewPos(0, add.Size().Height)))
	})
	clear := widget.NewButton("Clear locks", func() {
		u.editStep(id, "Clear step locks", func(s *groove.Step) { s.Locks = nil })
	})
	u.live(func() { setButtonEnabled(clear, any()) })
	panel.Add(container.NewBorder(nil, nil, widget.NewLabel("LOCKS"), container.NewHBox(add, clear)))
	hint := widget.NewLabel("No locks on this step. Tick Lock step above and move any Sound or FX control, or use Add lock.")
	hint.Wrapping = fyne.TextWrapWord
	panel.Add(u.boundVisible(hint, func() bool { return !any() }))
	for _, p := range params {
		p := p
		row := u.boundSlider(shortLockName(p.Name), p.Min, p.Max, p.Increment, func() float64 {
			if v, ok := u.locked(id, p.ID); ok {
				return v
			}
			return u.session.Project.Track(id).ParameterValue(p.ID)
		}, func(v float64) { u.setLock(id, p.ID, v) })
		remove := widget.NewButton("×", func() { u.clearLock(id, p.ID) })
		remove.Importance = widget.LowImportance
		item := container.NewBorder(nil, nil, nil, remove, row)
		panel.Add(u.boundVisible(item, func() bool { _, ok := u.locked(id, p.ID); return ok }))
	}
	return panel
}

// shortLockName fits lock names into the inspector: "FX 1 · Delay · Time"
// becomes "FX1 Time"; instrument names are unchanged.
func shortLockName(name string) string {
	parts := strings.Split(name, " · ")
	if len(parts) == 3 && strings.HasPrefix(parts[0], "FX ") {
		return strings.ReplaceAll(parts[0], " ", "") + " " + parts[2]
	}
	return name
}
