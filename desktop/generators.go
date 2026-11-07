package desktop

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) watchGenerator(title string, window interface{ Close() }, session *groove.Session, trackID string, slot int, update func(*groove.Track)) func() {
	if u.generatorRefresh == nil {
		u.generatorRefresh = map[string]func(){}
	}
	refresh := func() {
		if u.session != session || session.Project.Pattern != slot || session.Project.Track(trackID) == nil {
			window.Close()
			return
		}
		update(session.Project.Track(trackID))
	}
	u.generatorRefresh[title] = refresh
	return refresh
}

func (u *UI) euclideanGenerator() {
	t := u.selectedTrack()
	if t == nil {
		return
	}
	const title = "Euclidean rhythm"
	closed := false
	w, fresh := u.openDrawer(title, func() { closed = true; delete(u.generatorRefresh, title) })
	if !fresh {
		return
	}
	id, slot, session := t.ID, u.session.Project.Pattern, u.session
	options := groove.EuclideanOptions{Steps: len(t.Patterns[slot].Steps), Pulses: min(5, len(t.Patterns[slot].Steps)), Velocity: .8}
	preview := newGeneratorPreview(u)
	status, target := widget.NewLabel(""), widget.NewLabel("")
	status.Wrapping, target.Wrapping = fyne.TextWrapWord, fyne.TextWrapWord
	var candidate groove.Pattern
	var draftError error
	var refresh func()
	syncing, applying := false, false
	stepsRow, steps := draftSlider("Cycle steps", float64(options.Steps), 1, 64, 1, func(v float64) {
		if syncing {
			return
		}
		options.Steps = int(v)
		options.Pulses = min(options.Pulses, options.Steps)
		options.Rotation %= options.Steps
		refresh()
	})
	pulseRow, pulses := draftSlider("Pulses", float64(options.Pulses), 0, float64(options.Steps), 1, func(v float64) {
		if !syncing {
			options.Pulses = int(v)
			refresh()
		}
	})
	rotateRow, rotation := draftSlider("Rotation", 0, 0, float64(max(1, options.Steps-1)), 1, func(v float64) {
		if !syncing {
			options.Rotation = int(v)
			refresh()
		}
	})
	velocityRow, _ := draftSlider("Velocity for new hits", .8, 0, 1, .01, func(v float64) { options.Velocity = v; refresh() })
	mode := widget.NewSelect([]string{"Replace rhythm", "Layer onto existing hits"}, nil)
	mode.SetSelectedIndex(0)
	mode.OnChanged = func(string) { options.Layer = mode.SelectedIndex() == 1; refresh() }
	apply := widget.NewButton("Apply rhythm", func() {
		if closed || u.session != session || session.Project.Pattern != slot || session.Project.Track(id) == nil {
			return
		}
		if draftError != nil {
			u.showError(draftError)
			return
		}
		applying = true
		u.edit("Euclidean rhythm", func(p *groove.Project) error { p.Track(id).Patterns[slot] = candidate; return nil })
		applying = false
		status.SetText("Rhythm applied · Undo restores the previous pattern")
	})
	refresh = u.watchGenerator(title, w, session, id, slot, func(t *groove.Track) {
		if applying {
			return
		}
		target.SetText(fmt.Sprintf("%s / pattern %02d", t.Name, slot+1))
		syncing = true
		pulses.Max = float64(options.Steps)
		pulses.SetValue(float64(options.Pulses))
		pulses.Refresh()
		rotation.Max = float64(max(1, options.Steps-1))
		rotation.SetValue(float64(options.Rotation))
		rotation.Refresh()
		if options.Steps == 1 {
			rotation.Disable()
		} else {
			rotation.Enable()
		}
		steps.SetValue(float64(options.Steps))
		syncing = false
		candidate, draftError = groove.EuclideanPattern(t.Patterns[slot], options)
		if draftError != nil {
			apply.Disable()
			status.SetText(draftError.Error())
			return
		}
		apply.Enable()
		preview.pattern = candidate
		preview.Refresh()
		count := 0
		for _, step := range candidate.Steps {
			if step.Active {
				count++
			}
		}
		status.SetText(fmt.Sprintf("Draft · %d pulses in %d steps · %d total hits", options.Pulses, options.Steps, count))
	})
	help := widget.NewLabel("Preview changes stay here until Apply. Existing note settings and locks stay; new hits use the chosen velocity. Changing cycle length resizes this pattern, with undo.")
	help.Wrapping = fyne.TextWrapWord
	w.SetContent(container.NewPadded(container.NewBorder(nil, container.NewVBox(status, container.NewGridWithColumns(2, apply, widget.NewButton("Close", w.Close))), nil, nil, container.NewVScroll(container.NewVBox(target, preview, stepsRow, pulseRow, rotateRow, mode, velocityRow, help)))))
	refresh()
}

type lockGroup struct {
	name       string
	parameters []groove.Parameter
}

func lockGroups(t groove.Track) []lockGroup {
	groups := []lockGroup{}
	if t.Kind == groove.Dual {
		groups = []lockGroup{{name: "Synth / Oscillators"}, {name: "Synth / Filter"}, {name: "Synth / Amp envelope"}, {name: "Synth / Filter envelope"}, {name: "Synth / Glide & accent"}}
	} else {
		groups = append(groups, lockGroup{name: "Instrument"})
	}
	for _, p := range groove.Parameters(t.Kind) {
		if !p.Lockable {
			continue
		}
		index := 0
		if t.Kind == groove.Dual {
			switch {
			case strings.HasPrefix(p.ID, "amp_"):
				index = 2
			case strings.HasPrefix(p.ID, "filter_"):
				index = 3
			case p.ID == "glide" || p.ID == "accent":
				index = 4
			case p.ID == "cutoff" || p.ID == "resonance" || p.ID == "drive" || p.ID == "env_amount" || p.ID == "keytrack":
				index = 1
			}
		}
		groups[index].parameters = append(groups[index].parameters, p)
	}
	for i, effect := range t.Effects {
		group := lockGroup{name: fmt.Sprintf("FX %d / %s", i+1, effect.Kind.Name())}
		for _, p := range groove.EffectParameters(effect.Kind) {
			if p.Lockable {
				p.ID = groove.EffectLockID(effect.ID, p.ID)
				group.parameters = append(group.parameters, p)
			}
		}
		groups = append(groups, group)
	}
	return groups
}

func (u *UI) lockGenerator() {
	t := u.selectedTrack()
	if t == nil {
		return
	}
	const title = "Parameter-lock generator"
	closed := false
	w, fresh := u.openDrawer(title, func() { closed = true; delete(u.generatorRefresh, title) })
	if !fresh {
		return
	}
	id, slot, session := t.ID, u.session.Project.Pattern, u.session
	groups := lockGroups(*t)
	parameter := groups[0].parameters[0]
	options := groove.LockOptions{Parameter: parameter.ID, Shape: groove.LockUp, Last: len(t.Patterns[slot].Steps) - 1, ActiveOnly: true, Seed: rand.Uint64()}
	setBounds := func(t *groove.Track) {
		base := t.ParameterValue(parameter.ID)
		span := (parameter.Max - parameter.Min) * .25
		options.Low = max(parameter.Min, base-span)
		options.High = min(parameter.Max, base+span)
	}
	setBounds(t)
	syncing, applying := false, false
	var refresh func()
	var candidate groove.Pattern
	var draftError error
	preview := newGeneratorPreview(u)
	status, target, description := widget.NewLabel(""), widget.NewLabel(""), widget.NewLabel("")
	status.Wrapping, target.Wrapping, description.Wrapping = fyne.TextWrapWord, fyne.TextWrapWord, fyne.TextWrapWord
	groupSelect := widget.NewSelect(nil, nil)
	parameterSelect := widget.NewSelect(nil, nil)
	shapes := []string{}
	for _, shape := range groove.LockShapes {
		shapes = append(shapes, string(shape))
	}
	shape := widget.NewSelect(shapes, nil)
	shape.SetSelected(string(options.Shape))
	shape.OnChanged = func(v string) {
		if !syncing {
			options.Shape = groove.LockShape(v)
			refresh()
		}
	}
	var low, high, first, last *widget.Slider
	lowRow, low := draftSlider("Low", options.Low, parameter.Min, parameter.Max, parameter.Increment, func(v float64) {
		if syncing {
			return
		}
		options.Low = v
		options.High = max(options.High, v)
		refresh()
	})
	highRow, high := draftSlider("High", options.High, parameter.Min, parameter.Max, parameter.Increment, func(v float64) {
		if syncing {
			return
		}
		options.High = v
		options.Low = min(options.Low, v)
		refresh()
	})
	firstRow, first := draftSlider("First step", 1, 1, float64(max(2, len(t.Patterns[slot].Steps))), 1, func(v float64) {
		if syncing {
			return
		}
		options.First = int(v) - 1
		options.Last = max(options.Last, options.First)
		refresh()
	})
	lastRow, last := draftSlider("Last step", float64(options.Last+1), 1, float64(max(2, len(t.Patterns[slot].Steps))), 1, func(v float64) {
		if syncing {
			return
		}
		options.Last = int(v) - 1
		options.First = min(options.First, options.Last)
		refresh()
	})
	active := widget.NewCheck("Active notes only", nil)
	active.SetChecked(true)
	active.OnChanged = func(v bool) { options.ActiveOnly = v; refresh() }
	reroll := widget.NewButton("Reroll", func() { options.Seed = rand.Uint64(); refresh() })
	apply := widget.NewButton("Apply locks", func() {
		if closed || u.session != session || session.Project.Pattern != slot || session.Project.Track(id) == nil {
			return
		}
		if draftError != nil {
			u.showError(draftError)
			return
		}
		applying = true
		u.edit("Generate "+string(options.Shape), func(p *groove.Project) error { p.Track(id).Patterns[slot] = candidate; return nil })
		applying = false
		status.SetText("Locks applied · Undo restores the previous values")
	})
	groupSelect.OnChanged = func(string) {
		if syncing {
			return
		}
		i := groupSelect.SelectedIndex()
		if i < 0 || i >= len(groups) {
			return
		}
		parameter = groups[i].parameters[0]
		options.Parameter = parameter.ID
		setBounds(session.Project.Track(id))
		refresh()
	}
	parameterSelect.OnChanged = func(string) {
		if syncing {
			return
		}
		g, i := groupSelect.SelectedIndex(), parameterSelect.SelectedIndex()
		if g < 0 || g >= len(groups) || i < 0 || i >= len(groups[g].parameters) {
			return
		}
		parameter = groups[g].parameters[i]
		options.Parameter = parameter.ID
		setBounds(session.Project.Track(id))
		refresh()
	}
	refresh = u.watchGenerator(title, w, session, id, slot, func(t *groove.Track) {
		if applying {
			return
		}
		target.SetText(fmt.Sprintf("%s / pattern %02d", t.Name, slot+1))
		groups = lockGroups(*t)
		g, index, found := 0, 0, false
		for i, group := range groups {
			for j, p := range group.parameters {
				if p.ID == options.Parameter {
					g, index, parameter, found = i, j, p, true
				}
			}
		}
		if !found {
			w.Close()
			return
		}
		syncing = true
		groupSelect.Options = nil
		for _, group := range groups {
			groupSelect.Options = append(groupSelect.Options, group.name)
		}
		groupSelect.Refresh()
		groupSelect.SetSelectedIndex(g)
		parameterSelect.Options = nil
		for _, p := range groups[g].parameters {
			parameterSelect.Options = append(parameterSelect.Options, p.Name)
		}
		parameterSelect.Refresh()
		parameterSelect.SetSelectedIndex(index)
		length := len(t.Patterns[slot].Steps)
		options.Last = min(options.Last, length-1)
		options.First = min(options.First, options.Last)
		first.Max, last.Max = float64(max(2, length)), float64(max(2, length))
		first.SetValue(float64(options.First + 1))
		last.SetValue(float64(options.Last + 1))
		first.Refresh()
		last.Refresh()
		if length == 1 {
			first.Disable()
			last.Disable()
		} else {
			first.Enable()
			last.Enable()
		}
		for _, slider := range []*widget.Slider{low, high} {
			slider.Min, slider.Max, slider.Step = parameter.Min, parameter.Max, parameter.Increment
		}
		low.SetValue(options.Low)
		high.SetValue(options.High)
		low.Refresh()
		high.Refresh()
		if options.Shape == groove.LockClear {
			low.Disable()
			high.Disable()
		} else {
			low.Enable()
			high.Enable()
		}
		syncing = false
		if options.Shape.Random() {
			reroll.Enable()
		} else {
			reroll.Disable()
		}
		description.SetText(options.Shape.Description())
		candidate, draftError = groove.LockPattern(*t, slot, options)
		if draftError != nil {
			apply.Disable()
			status.SetText(draftError.Error())
			return
		}
		apply.Enable()
		preview.pattern, preview.parameter, preview.low, preview.high = candidate, parameter.ID, parameter.Min, parameter.Max
		preview.Refresh()
		count := 0
		for i := options.First; i <= options.Last; i++ {
			if !options.ActiveOnly || candidate.Steps[i].Active {
				count++
			}
		}
		status.SetText(fmt.Sprintf("Draft · %d eligible steps · %s", count, parameter.Name))
		if count == 0 {
			apply.Disable()
		} else {
			apply.Enable()
		}
	})
	help := widget.NewLabel("Low/High use the parameter’s actual units. Shapes follow step positions, including rests; Active notes only chooses where to write. Other locks and note settings stay. Filled bars show lock values; outlined cells mark notes. Apply works during playback and supports undo.")
	help.Wrapping = fyne.TextWrapWord
	selectors := container.NewGridWithColumns(2, container.NewVBox(widget.NewLabel("Source"), groupSelect), container.NewVBox(widget.NewLabel("Parameter"), parameterSelect))
	w.SetContent(container.NewPadded(container.NewBorder(nil, container.NewVBox(status, container.NewGridWithColumns(3, reroll, apply, widget.NewButton("Close", w.Close))), nil, nil, container.NewVScroll(container.NewVBox(target, selectors, shape, description, preview, container.NewGridWithColumns(2, lowRow, highRow), container.NewGridWithColumns(2, firstRow, lastRow), active, help)))))
	refresh()
}
