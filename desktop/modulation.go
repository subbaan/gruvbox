package desktop

import (
	"math/rand/v2"

	"fmt"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) randomiseSound() {
	if t := u.selectedTrack(); t != nil {
		id := t.ID
		// Preview with a seed to name the archetype, then make the same
		// change as one undoable edit.
		seed1, seed2 := rand.Uint64(), rand.Uint64()
		preview := u.session.Project.Clone()
		label := "Randomise sound"
		if name := groove.RandomiseSoundFrom(preview.Track(id), rand.New(rand.NewPCG(seed1, seed2))); name != "" {
			label += " · " + name
		}
		u.edit(label, func(p *groove.Project) error {
			groove.RandomiseSoundFrom(p.Track(id), rand.New(rand.NewPCG(seed1, seed2)))
			return nil
		})
	}
}
func (u *UI) modulationPanel(t *groove.Track) fyne.CanvasObject {
	if len(groove.LFOTargets(t.Kind)) == 0 {
		label := widget.NewLabel("Instrument LFOs are available on synth and drum tracks. FX LFOs work on every track via FX → LFOs….")
		label.Wrapping = fyne.TextWrapWord
		return label
	}
	id := t.ID
	return u.lfoControls(func() [2]groove.LFO { return u.session.Project.Track(id).LFOs }, groove.LFOTargets(t.Kind), func(index int, label string, fn func(*groove.LFO)) {
		u.edit(fmt.Sprintf("LFO %d %s", index+1, label), func(p *groove.Project) error { fn(&p.Track(id).LFOs[index]); return nil })
	})
}

var lfoDivisions = []float64{1. / 64, 1. / 32, 1. / 16, 1. / 8, 1. / 4, 1. / 2, 1, 2, 4}

// lfoControls builds both LFOs. Each LFO's sync mode decides which rate
// control appears, so callers must rebuild when a sync mode changes; all
// other values are read live through get.
func (u *UI) lfoControls(get func() [2]groove.LFO, targets []string, apply func(int, string, func(*groove.LFO))) fyne.CanvasObject {
	panel := container.NewVBox()
	for index, c := range get() {
		index := index
		lfo := func() groove.LFO { return get()[index] }
		edit := func(label string, fn func(*groove.LFO)) { apply(index, label, fn) }
		panel.Add(u.boundCheck(fmt.Sprintf("LFO %d enabled", index+1), func() bool { return lfo().Enabled }, func(v bool) { edit("enabled", func(l *groove.LFO) { l.Enabled = v }) }))
		target := u.boundSelect(targets, func() int { return lfo().Target }, func(i int) { edit("target", func(l *groove.LFO) { l.Target = i }) })
		wave := u.boundSelect([]string{"Sine", "Triangle", "Square", "Saw up", "Saw down", "Sample & hold"}, func() int { return lfo().Waveform }, func(i int) { edit("waveform", func(l *groove.LFO) { l.Waveform = i }) })
		panel.Add(container.NewBorder(nil, nil, widget.NewLabel("Target"), nil, target))
		panel.Add(container.NewBorder(nil, nil, widget.NewLabel("Wave"), nil, wave))
		panel.Add(u.boundSlider("Depth", 0, 1, .01, func() float64 { return lfo().Depth }, func(v float64) { edit("depth", func(l *groove.LFO) { l.Depth = v }) }))
		sync := u.boundSelect([]string{"Free (Hz)", "Tempo division", "Track steps"}, func() int { return lfo().Sync }, func(i int) { edit("sync", func(l *groove.LFO) { l.Sync = i }) })
		panel.Add(container.NewBorder(nil, nil, widget.NewLabel("Sync"), nil, sync))
		switch c.Sync {
		case 0:
			panel.Add(u.boundSlider("Rate (Hz)", .01, 100, .01, func() float64 { return lfo().Rate }, func(v float64) { edit("rate", func(l *groove.LFO) { l.Rate = v }) }))
		case 1:
			division := u.boundSelect([]string{"1/64", "1/32", "1/16", "1/8", "1/4", "1/2", "1 bar", "2 bars", "4 bars"}, func() int { return slices.Index(lfoDivisions, lfo().Division) }, func(i int) {
				edit("division", func(l *groove.LFO) { l.Division = lfoDivisions[i] })
			})
			panel.Add(container.NewBorder(nil, nil, widget.NewLabel("Division"), nil, division))
		case 2:
			panel.Add(u.boundSlider("Steps per cycle", 1, 128, 1, func() float64 { return float64(lfo().Steps) }, func(v float64) { edit("steps", func(l *groove.LFO) { l.Steps = int(v) }) }))
		}
		panel.Add(u.boundCheck("Retrigger on note", func() bool { return lfo().Retrigger }, func(v bool) { edit("retrigger", func(l *groove.LFO) { l.Retrigger = v }) }))
		panel.Add(u.boundSlider("Start phase", 0, 1, .01, func() float64 { return lfo().Phase }, func(v float64) { edit("phase", func(l *groove.LFO) { l.Phase = v }) }))
		panel.Add(widget.NewSeparator())
	}
	return panel
}
