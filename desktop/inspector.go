package desktop

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

const sliderNameWidth, sliderValueWidth float32 = 100, 48

// sliderValue formats a slider reading at the precision its increment implies.
func sliderValue(v, increment float64) string {
	if increment >= 1 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

// sliderRow lays out one parameter on a single line: name | slider | value.
func (u *UI) sliderRow(name string, value, min, max, increment float64, commit func(float64)) fyne.CanvasObject {
	return u.sliderRowFormat(name, value, min, max, increment, func(v float64) string { return sliderValue(v, increment) }, commit)
}
func (u *UI) sliderRowFormat(name string, value, min, max, increment float64, format func(float64) string, commit func(float64)) fyne.CanvasObject {
	row, _, _ := u.sliderParts(name, value, min, max, increment, format, commit)
	return row
}
func (u *UI) sliderParts(name string, value, min, max, increment float64, format func(float64) string, commit func(float64)) (fyne.CanvasObject, *widget.Slider, *widget.Label) {
	label := widget.NewLabel(name)
	label.Truncation = fyne.TextTruncateEllipsis
	reading := widget.NewLabel(format(value))
	reading.Alignment = fyne.TextAlignTrailing
	slider := widget.NewSlider(min, max)
	slider.Step = increment
	slider.SetValue(value)
	u.bindLiveSlider(slider, func(v float64) { reading.SetText(format(v)); commit(v) })
	height := slider.MinSize().Height
	row := container.NewBorder(nil, nil, container.NewGridWrap(fyne.NewSize(sliderNameWidth, height), label), container.NewGridWrap(fyne.NewSize(sliderValueWidth, height), reading), slider)
	return row, slider, reading
}

// inspectorShape names everything that changes which widgets the inspector
// contains. Edits that keep the shape only push new values into the existing
// widgets; anything else rebuilds.
func (u *UI) inspectorShape(t *groove.Track) string {
	if t == nil {
		return "none"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%s|%p|lfo %d %d", u.scheme.Name, t.ID, t.Kind, t.Sample, t.LFOs[0].Sync, t.LFOs[1].Sync)
	for _, e := range t.Effects {
		fmt.Fprintf(&b, "|%s:%s", e.ID, e.Kind)
		if e.Kind.TempoSynced() {
			fmt.Fprintf(&b, ":%t", e.Parameters["sync"] >= .5)
		}
	}
	return b.String()
}

var resolutions = []int{4, 8, 16, 32, 64}

// instrumentParam is a lock-aware slider for an instrument parameter.
// toSlider/fromSlider/format may be nil for a linear control.
// instrumentChoice is a lock-aware menu for a stepped instrument control.
func (u *UI) instrumentChoice(id string, param groove.Parameter, choices []string) fyne.CanvasObject {
	lock := ""
	if param.Lockable {
		lock = param.ID
	}
	values := make([]float64, len(choices))
	for i := range values {
		values[i] = param.Min + float64(i)
	}
	return u.paramSelect(paramControl{
		track: id, name: param.Name, lock: lock, def: param.Default,
		base: func() float64 { return u.session.Project.Track(id).Parameters[param.ID] },
		setBase: func(v float64) {
			u.edit("Set "+param.Name, func(p *groove.Project) error {
				if t := p.Track(id); t != nil {
					t.Parameters[param.ID] = v
				}
				return nil
			})
		},
	}, choices, values)
}

func (u *UI) instrumentParam(id string, param groove.Parameter, toSlider, fromSlider func(float64) float64, format func(float64) string) fyne.CanvasObject {
	lock := ""
	if param.Lockable {
		lock = param.ID
	}
	return u.paramRow(paramControl{
		track: id, name: param.Name, lock: lock,
		min: param.Min, max: param.Max, increment: param.Increment, def: param.Default,
		base: func() float64 { return u.session.Project.Track(id).Parameters[param.ID] },
		setBase: func(v float64) {
			u.edit("Set "+param.Name, func(p *groove.Project) error {
				if t := p.Track(id); t != nil {
					t.Parameters[param.ID] = v
				}
				return nil
			})
		},
		format: format, toSlider: toSlider, fromSlider: fromSlider,
	})
}

// generatorHeader puts the selected track's generators beside its sound.
// Clear and the generator windows live under ⋯ to keep the header compact.
func (u *UI) generatorHeader(t *groove.Track) fyne.CanvasObject {
	grid := container.NewGridWithColumns(4)
	var more []*fyne.MenuItem
	for _, action := range groove.Actions(t.Kind) {
		action := action
		if action == "Clear" {
			more = append(more, fyne.NewMenuItem("Clear pattern", func() { u.generate(action) }), fyne.NewMenuItemSeparator())
			continue
		}
		grid.Add(widget.NewButton(action, func() { u.generate(action) }))
	}
	more = append(more, fyne.NewMenuItem("Euclidean…", u.euclideanGenerator), fyne.NewMenuItem("Parameter locks…", u.lockGenerator))
	if t.Kind.Capabilities().Notes {
		more = append(more, fyne.NewMenuItem("Phrase…", u.phraseGenerator))
	}
	var moreButton *widget.Button
	moreButton = widget.NewButton("⋯", func() {
		pop := widget.NewPopUpMenu(fyne.NewMenu("Generate", more...), u.window.Canvas())
		pop.ShowAtPosition(fyne.CurrentApp().Driver().AbsolutePositionForObject(moreButton).Add(fyne.NewPos(0, moreButton.Size().Height)))
	})
	grid.Add(moreButton)
	return grid
}

// refreshInspector updates the inspector in place when its shape is unchanged.
func (u *UI) refreshInspector() {
	t := u.selectedTrack()
	if shape := u.inspectorShape(t); shape == u.inspectorKey && u.inspectorTabs != nil {
		u.inspectorBinder.sync()
		return
	} else {
		u.inspectorKey = shape
	}
	u.inspectorBuilds++
	u.building(&u.inspectorBinder, u.rebuildInspector)
}
func (u *UI) rebuildInspector() {
	previous := 0
	if u.inspectorTabs != nil {
		previous = u.inspectorTabs.SelectedIndex()
	}
	u.inspector.Objects = nil
	t := u.selectedTrack()
	if t == nil {
		u.inspectorTabs = nil
		u.inspector.Add(widget.NewLabel("Add a track to begin."))
		u.inspector.Refresh()
		return
	}
	id := t.ID
	// Accessors read the live project, so callbacks and syncs never act on a
	// stale step, pattern or track captured when the widgets were built.
	track := func() *groove.Track { return u.session.Project.Track(id) }
	pattern := func() *groove.Pattern { return &track().Patterns[u.session.Project.Pattern] }
	step := func() *groove.Step {
		steps := pattern().Steps
		return &steps[max(0, min(u.selectedStep, len(steps)-1))]
	}
	edit := func(label string, fn func(*groove.Track)) {
		u.edit(label, func(p *groove.Project) error {
			if t := p.Track(id); t != nil {
				fn(t)
			}
			return nil
		})
	}
	stepEdit := func(label string, fn func(*groove.Step)) {
		slot, index := u.session.Project.Pattern, u.selectedStep
		edit(label, func(t *groove.Track) {
			if steps := t.Patterns[slot].Steps; index < len(steps) {
				fn(&steps[index])
			}
		})
	}
	title := u.boundLabel(func() string { return track().Name + " · " + track().Kind.Name() })
	title.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	u.inspector.Add(container.NewBorder(nil, nil, nil, u.withHelp(u.lockToggle(), "Lock (K): moving a Sound or FX control writes a parameter lock for the selected step instead of changing the sound. Highlighted names are locked on this step. Clicking the grid selects steps while Lock is on. Double-click a name to clear its lock, or to reset an unlocked control. Escape turns Lock off."), title))
	u.inspector.Add(u.generatorHeader(t))
	sound := container.NewVBox(container.NewGridWithColumns(2, widget.NewButton("Randomise sound", u.randomiseSound), widget.NewButton("Presets…", u.soundPresets)))
	if t.Kind == groove.Tom {
		sound.Add(u.withHelp(widget.NewButton("Bass tone", func() {
			u.edit("Tom bass tone", func(p *groove.Project) error { return groove.TomBassTone(p.Track(id)) })
		}), "Piano roll notes transpose the root pitch. C2 keeps the root unchanged; Tune, LFOs and pitch locks still apply."))
	}
	if t.Kind == groove.Sampler {
		sound.Add(u.samplePanel(t))
	}
	if t.Kind == groove.Dual {
		sound.Add(u.dualSoundPanel(t))
	}
	for _, param := range groove.Parameters(t.Kind) {
		if t.Kind == groove.Dual {
			continue
		}
		if t.Kind == groove.Sampler && param.ID != "pitch" {
			continue
		}
		if choices := groove.ParameterChoices(t.Kind, param.ID); choices != nil {
			sound.Add(u.instrumentChoice(id, param, choices))
			continue
		}
		sound.Add(u.instrumentParam(id, param, nil, nil, nil))
	}
	stepPanel := container.NewVBox(u.boundLabel(func() string { return fmt.Sprintf("STEP %02d", u.selectedStep+1) }))
	stepPanel.Add(u.boundCheck("Active", func() bool { return step().Active }, func(v bool) { stepEdit("Step active", func(s *groove.Step) { s.Active = v }) }))
	if t.Kind.Capabilities().Notes {
		stepPanel.Add(u.boundSliderFormat("Note", 0, 127, 1, func() float64 { return float64(step().Note) }, func(v float64) string { return noteName(int(v)) }, func(v float64) { stepEdit("Note", func(s *groove.Step) { s.Note = int(v) }) }))
	}
	if t.Kind.Capabilities().NoteLengths {
		stepPanel.Add(u.withHelp(u.boundSlider("Length (steps)", 1, 64, 1, func() float64 { return float64(step().GateSteps()) }, func(v float64) { stepEdit("Note length", func(s *groove.Step) { s.Length = int(v) }) }), "Length controls the gate; a later note retriggers the voice. Sampler length applies in Gate mode. Dual synth releases its envelopes at gate end; the 303 rings into its decay unless its Gate control is below 1, when it releases after Gate × the length."))
	}
	stepPanel.Add(u.boundSlider("Velocity", 0, 1, .01, func() float64 { return step().Velocity }, func(v float64) { stepEdit("Velocity", func(s *groove.Step) { s.Velocity = v }) }))
	stepPanel.Add(u.boundSlider("Probability", 0, 1, .01, func() float64 { return step().Probability }, func(v float64) { stepEdit("Probability", func(s *groove.Step) { s.Probability = v }) }))
	stepPanel.Add(u.boundCheck("Accent", func() bool { return step().Accent }, func(v bool) { stepEdit("Accent", func(s *groove.Step) { s.Accent = v }) }))
	if t.Kind.Capabilities().Slide {
		stepPanel.Add(u.boundCheck("Slide", func() bool { return step().Slide }, func(v bool) { stepEdit("Slide", func(s *groove.Step) { s.Slide = v }) }))
	}
	condition := u.boundSelect([]string{"Always", "Every 2nd", "Every 3rd", "Every 4th"}, func() int {
		if every := step().Every; every >= 2 && every <= 4 {
			return every - 1
		}
		return 0
	}, func(i int) { stepEdit("Condition", func(s *groove.Step) { s.Every = i + 1 }) })
	stepPanel.Add(container.NewBorder(nil, nil, widget.NewLabel("Play"), nil, condition))
	stepPanel.Add(widget.NewSeparator())
	stepPanel.Add(u.stepLocks(t))
	sequence := container.NewVBox(widget.NewLabel("PATTERN SETTINGS"))
	var length *commitEntry
	length = u.boundEntry(func() string { return strconv.Itoa(len(pattern().Steps)) }, func(text string) {
		n, err := strconv.Atoi(text)
		if err != nil || n < 1 || n > 64 {
			length.SetValue(strconv.Itoa(len(pattern().Steps)))
			return
		}
		slot := u.session.Project.Pattern
		edit("Pattern length", func(t *groove.Track) {
			pattern := &t.Patterns[slot]
			if n < len(pattern.Steps) {
				pattern.Steps = pattern.Steps[:n]
			} else {
				for len(pattern.Steps) < n {
					pattern.Steps = append(pattern.Steps, groove.DefaultStep())
				}
			}
		})
	})
	sequence.Add(container.NewBorder(nil, nil, widget.NewLabel("Length (1–64 steps)"), nil, length))
	resolution := u.boundSelect([]string{"1/4", "1/8", "1/16", "1/32", "1/64"}, func() int { return slices.Index(resolutions, pattern().Resolution) }, func(i int) {
		slot := u.session.Project.Pattern
		edit("Step resolution", func(t *groove.Track) { t.Patterns[slot].Resolution = resolutions[i] })
	})
	sequence.Add(u.withHelp(container.NewBorder(nil, nil, widget.NewLabel("Step size"), nil, resolution), "Step size is the note value of one step: 1/16 is the usual groovebox step, 1/32 runs twice as fast, 1/8 half as fast."))
	u.inspectorTabs = container.NewAppTabs(container.NewTabItem("Sound", sound), container.NewTabItem("Step", stepPanel), container.NewTabItem("Seq", sequence), container.NewTabItem("LFO", u.modulationPanel(t)), container.NewTabItem("FX", u.effectsPanel(t)))
	u.inspectorTabs.SelectIndex(previous)
	u.inspector.Add(u.inspectorTabs)
	u.inspector.Refresh()
}
