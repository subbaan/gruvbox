package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// binder holds the sync functions of a long-lived panel. Each one reads the
// current project and pushes values into its widget without firing callbacks,
// so a refresh updates panels in place instead of recreating them. This keeps
// focus, scroll position and open sections stable while editing.
type binder struct{ syncs []func() }

func (b *binder) reset() { b.syncs = b.syncs[:0] }
func (b *binder) sync() {
	for _, f := range b.syncs {
		f()
	}
}
func (b *binder) len() int { return len(b.syncs) }

// bound registers f with the panel currently being built. Outside a build
// (u.bind == nil), for example in utility windows that rebuild themselves, it
// does nothing.
func (u *UI) bound(f func()) {
	if u.bind != nil {
		u.bind.syncs = append(u.bind.syncs, f)
	}
}

// live runs a sync now, so a widget starts in the right state, and binds it
// for later refreshes.
func (u *UI) live(f func()) {
	f()
	u.bound(f)
}

// building runs build with sync registration directed at b.
func (u *UI) building(b *binder, build func()) {
	previous := u.bind
	b.reset()
	u.bind = b
	defer func() { u.bind = previous }()
	build()
}

func setSlider(s *widget.Slider, v float64) {
	if s.Value == v {
		return
	}
	changed, ended := s.OnChanged, s.OnChangeEnded
	s.OnChanged, s.OnChangeEnded = nil, nil
	s.SetValue(v)
	s.OnChanged, s.OnChangeEnded = changed, ended
}
func setCheck(c *widget.Check, v bool) {
	if c.Checked == v {
		return
	}
	changed := c.OnChanged
	c.OnChanged = nil
	c.SetChecked(v)
	c.OnChanged = changed
}
func setSelectIndex(s *widget.Select, i int) {
	if s.SelectedIndex() == i {
		return
	}
	changed := s.OnChanged
	s.OnChanged = nil
	if i < 0 || i >= len(s.Options) {
		s.ClearSelected()
	} else {
		s.SetSelectedIndex(i)
	}
	s.OnChanged = changed
}
func setLabel(l *widget.Label, text string) {
	if l.Text != text {
		l.SetText(text)
	}
}
func setVisible(o fyne.CanvasObject, visible bool) {
	if visible && !o.Visible() {
		o.Show()
	} else if !visible && o.Visible() {
		o.Hide()
	}
}

// focused reports whether o currently holds keyboard focus in its canvas.
func focused(o fyne.Focusable) bool {
	c := fyne.CurrentApp().Driver().CanvasForObject(o.(fyne.CanvasObject))
	return c != nil && c.Focused() == o
}

// setEntry updates an entry unless the user is part-way through editing it.
func setEntry(e *commitEntry, value string) {
	if e.baseline == value {
		return
	}
	if focused(e) && e.Text != e.baseline {
		return
	}
	e.SetValue(value)
}

func (u *UI) boundSlider(name string, min, max, increment float64, get func() float64, commit func(float64)) fyne.CanvasObject {
	return u.boundSliderFormat(name, min, max, increment, get, func(v float64) string { return sliderValue(v, increment) }, commit)
}
func (u *UI) boundSliderFormat(name string, min, max, increment float64, get func() float64, format func(float64) string, commit func(float64)) fyne.CanvasObject {
	row, slider, reading := u.sliderParts(name, get(), min, max, increment, format, commit)
	u.bound(func() {
		if u.liveSlider == slider {
			return
		}
		v := get()
		setSlider(slider, v)
		setLabel(reading, format(slider.Value))
	})
	return row
}
func (u *UI) boundCheck(text string, get func() bool, set func(bool)) *widget.Check {
	check := widget.NewCheck(text, nil)
	check.SetChecked(get())
	check.OnChanged = set
	u.bound(func() { setCheck(check, get()) })
	return check
}

// boundSelect binds a select by option index.
func (u *UI) boundSelect(options []string, get func() int, set func(int)) *widget.Select {
	s := widget.NewSelect(options, nil)
	setSelectIndex(s, get())
	s.OnChanged = func(string) { set(s.SelectedIndex()) }
	u.bound(func() { setSelectIndex(s, get()) })
	return s
}
func (u *UI) boundEntry(get func() string, commit func(string)) *commitEntry {
	e := newCommitEntry(get(), commit)
	u.bound(func() { setEntry(e, get()) })
	return e
}
func (u *UI) boundLabel(get func() string) *widget.Label {
	l := widget.NewLabel(get())
	u.bound(func() { setLabel(l, get()) })
	return l
}
func (u *UI) boundVisible(o fyne.CanvasObject, visible func() bool) fyne.CanvasObject {
	setVisible(o, visible())
	u.bound(func() { setVisible(o, visible()) })
	return o
}
