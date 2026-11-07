package desktop

import (
	"fmt"
	"math/rand/v2"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) phraseGenerator() {
	t := u.selectedTrack()
	if t == nil || !t.Kind.Capabilities().Notes {
		return
	}
	const title = "Phrase generator"
	closed := false
	w, fresh := u.openDrawer(title, func() { closed = true; delete(u.generatorRefresh, title) })
	if !fresh {
		return
	}
	id, slot, session := t.ID, u.session.Project.Pattern, u.session
	bars := 1
	if len(t.Patterns[slot].Steps) > 16 {
		bars = 2
	}
	if len(t.Patterns[slot].Steps) > 32 {
		bars = 4
	}
	options := groove.PhraseOptions{Bars: bars, Root: 36, Style: groove.PhraseAcid, Scale: groove.PhraseMinorPentatonic, Mode: groove.PhraseAll, Density: .5, Movement: .65, Variation: .3, Feel: .65, RhythmSeed: rand.Uint64(), PitchSeed: rand.Uint64(), ArticulationSeed: rand.Uint64()}
	if t.Kind != groove.Acid {
		options.Style = groove.PhraseHypnotic
	}
	if key := u.session.Project.Key; key != nil {
		// Start in the project's key so the phrase fits the other parts.
		options.Root, options.Scale = key.BassRoot(), key.Scale
	}
	preview := newGeneratorPreview(u)
	preview.notes = true
	target, status, description := widget.NewLabel(""), widget.NewLabel(""), widget.NewLabel("")
	target.Wrapping, status.Wrapping, description.Wrapping = fyne.TextWrapWord, fyne.TextWrapWord, fyne.TextWrapWord
	var refresh func()
	var candidate groove.Pattern
	var draftError error
	applying := false
	mode := widget.NewSelect([]string{string(groove.PhraseAll), string(groove.PhrasePitches), string(groove.PhraseFeel)}, nil)
	mode.SetSelected(string(options.Mode))
	mode.OnChanged = func(v string) { options.Mode = groove.PhraseMode(v); refresh() }
	style := widget.NewSelect(groove.PhraseStyles(), nil)
	style.SetSelected(string(options.Style))
	style.OnChanged = func(v string) { options.Style = groove.PhraseStyle(v); refresh() }
	length := widget.NewSelect([]string{"1 bar / 16 steps", "2 bars / 32 steps", "4 bars / 64 steps"}, nil)
	length.SetSelectedIndex(map[int]int{1: 0, 2: 1, 4: 2}[bars])
	length.OnChanged = func(string) { options.Bars = []int{1, 2, 4}[length.SelectedIndex()]; refresh() }
	root := widget.NewSelect([]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}, nil)
	root.SetSelectedIndex(options.Root % 12)
	register := widget.NewSelect([]string{"Octave 1", "Octave 2", "Octave 3", "Octave 4"}, nil)
	register.SetSelectedIndex(options.Root/12 - 2)
	updateRoot := func(string) { options.Root = 24 + register.SelectedIndex()*12 + root.SelectedIndex(); refresh() }
	root.OnChanged, register.OnChanged = updateRoot, updateRoot
	scale := widget.NewSelect(groove.PhraseScales(), nil)
	scale.SetSelected(string(options.Scale))
	scale.OnChanged = func(v string) { options.Scale = groove.PhraseScale(v); refresh() }
	densityRow, density := draftSlider("Notes per bar", options.Density*16, 2, 16, 1, func(v float64) { options.Density = v / 16; refresh() })
	movementRow, movement := draftSlider("Pitch movement", options.Movement, 0, 1, .01, func(v float64) { options.Movement = v; refresh() })
	variationRow, variation := draftSlider("Bar variation", options.Variation, 0, 1, .01, func(v float64) { options.Variation = v; refresh() })
	feelRow, feel := draftSlider("Feel amount", options.Feel, 0, 1, .01, func(v float64) { options.Feel = v; refresh() })
	rerollAll := widget.NewButton("Reroll all", func() {
		options.RhythmSeed, options.PitchSeed, options.ArticulationSeed = rand.Uint64(), rand.Uint64(), rand.Uint64()
		refresh()
	})
	rerollRhythm := widget.NewButton("Rhythm", func() { options.RhythmSeed = rand.Uint64(); refresh() })
	rerollPitches := widget.NewButton("Pitches", func() { options.PitchSeed = rand.Uint64(); refresh() })
	rerollFeel := widget.NewButton("Feel", func() { options.ArticulationSeed = rand.Uint64(); refresh() })
	apply := widget.NewButton("Apply phrase", func() {
		if closed || u.session != session || session.Project.Pattern != slot || session.Project.Track(id) == nil {
			return
		}
		if draftError != nil {
			u.showError(draftError)
			return
		}
		applying = true
		u.page = min(u.page, options.Bars-1)
		if options.Mode != groove.PhraseFeel {
			u.roll.rollKey = "" // refit the piano roll to the new notes
		}
		u.edit("Generate "+string(options.Style)+" phrase", func(p *groove.Project) error { p.Track(id).Patterns[slot] = candidate; return nil })
		applying = false
		status.SetText("Phrase applied · Undo restores the previous pattern")
	})
	refresh = u.watchGenerator(title, w, session, id, slot, func(t *groove.Track) {
		if applying {
			return
		}
		target.SetText(fmt.Sprintf("%s / pattern %s · %d bars in 4/4 · sixteenth notes", t.Name, groove.SlotName(slot), options.Bars))
		setEnabled := func(enabled bool, controls ...fyne.Disableable) {
			for _, control := range controls {
				if enabled {
					control.Enable()
				} else {
					control.Disable()
				}
			}
		}
		setEnabled(options.Mode == groove.PhraseAll, density, rerollAll, rerollRhythm)
		setEnabled(options.Mode != groove.PhraseFeel, root, register, scale, movement, rerollPitches)
		setEnabled(options.Mode != groove.PhrasePitches, feel, rerollFeel)
		setEnabled(options.Bars > 1, variation)
		switch options.Style {
		case groove.PhraseHypnotic:
			description.SetText("A small repeating motif and syncopated pulse; restrained pitch changes with evolving articulation.")
		case groove.PhraseAcid:
			description.SetText("Root-led motifs, controlled jumps and stronger accents; slides connect nearby notes with different pitches.")
		case groove.PhraseRolling:
			description.SetText("An eighth-note foundation with sixteenth-note pickups and stepwise melodic development.")
		}
		candidate, draftError = groove.PhrasePattern(t.Patterns[slot], t.Kind, options)
		if draftError != nil {
			apply.Disable()
			status.SetText(draftError.Error())
			return
		}
		apply.Enable()
		preview.pattern = candidate
		preview.Refresh()
		count, accented, sliding := 0, 0, 0
		for _, step := range candidate.Steps {
			if step.Active {
				count++
				if step.Accent {
					accented++
				}
				if step.Slide {
					sliding++
				}
			}
		}
		status.SetText(fmt.Sprintf("Draft · %d notes · %d accents · %d slides", count, accented, sliding))
	})
	help := widget.NewLabel("Reroll buttons explore rhythm, pitches or feel separately. Pitches only keeps the current pattern’s rhythm and feel; Feel only keeps its rhythm and pitches. Each preview row is one bar; > accent, ~ incoming slide. Apply resizes only this track to the chosen length at 1/16 resolution. Extending partial modes repeats existing notes; locks stay at their original positions. Drafts stay separate until Apply, with one undo.")
	help.Wrapping = fyne.TextWrapWord
	labelled := func(label string, control fyne.CanvasObject) fyne.CanvasObject {
		return container.NewVBox(widget.NewLabel(label), control)
	}
	selectors := container.NewGridWithColumns(2, labelled("Generate", mode), labelled("Style", style), labelled("Length", length), labelled("Scale", scale), labelled("Root", root), labelled("Register", register))
	footer := container.NewVBox(status, container.NewGridWithColumns(4, rerollAll, rerollRhythm, rerollPitches, rerollFeel), container.NewGridWithColumns(2, apply, widget.NewButton("Close", w.Close)))
	w.SetContent(container.NewPadded(container.NewBorder(nil, footer, nil, nil, container.NewVScroll(container.NewVBox(target, preview, selectors, description, container.NewGridWithColumns(2, densityRow, movementRow, variationRow, feelRow), help)))))
	refresh()
}
