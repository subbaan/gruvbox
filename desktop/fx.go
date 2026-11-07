package desktop

import (
	"fmt"
	"math/rand/v2"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) addEffect(kind groove.EffectKind) {
	if t := u.selectedTrack(); t != nil {
		id := t.ID
		u.edit("Add "+kind.Name(), func(p *groove.Project) error {
			t := p.Track(id)
			t.Effects = append(t.Effects, groove.NewEffect(kind))
			return nil
		})
	}
}
func (u *UI) randomiseEffects() {
	if t := u.selectedTrack(); t != nil {
		id := t.ID
		u.edit("Randomise effects", func(p *groove.Project) error { t := p.Track(id); groove.RandomiseEffects(t); return nil })
	}
}

// applyPedalboard replaces the selected track's chain with a pedalboard;
// "" picks one suited to the track.
func (u *UI) applyPedalboard(name string) {
	t := u.selectedTrack()
	if t == nil {
		return
	}
	id := t.ID
	r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	board, ok := groove.PedalboardNamed(name)
	if !ok {
		board = groove.RandomPedalboard(t.Kind, r)
	}
	u.edit("Pedalboard "+board.Name, func(p *groove.Project) error {
		groove.ApplyPedalboard(p.Track(id), board, r)
		return nil
	})
}

const randomBoard = "Random board"

func (u *UI) effectsPanel(t *groove.Track) fyne.CanvasObject {
	id := t.ID
	track := func() *groove.Track { return u.session.Project.Track(id) }
	panel := container.NewVBox()
	bypass := u.boundCheck("Bypass entire chain", func() bool { return track().EffectsBypassed }, func(v bool) {
		u.edit("Bypass effects", func(p *groove.Project) error { p.Track(id).EffectsBypassed = v; return nil })
	})
	boards := []string{randomBoard}
	for _, b := range groove.Pedalboards {
		boards = append(boards, b.Name)
	}
	board := widget.NewSelect(boards, nil)
	board.PlaceHolder = "Pedalboard…"
	board.OnChanged = func(name string) {
		if name == "" {
			return
		}
		if name == randomBoard {
			name = ""
		}
		u.applyPedalboard(name)
	}
	panel.Add(container.NewBorder(nil, nil, bypass, container.NewHBox(board, widget.NewButton("Randomise", u.randomiseEffects))))
	kinds := []string{}
	for _, k := range groove.EffectKinds {
		kinds = append(kinds, k.Name())
	}
	choose := widget.NewSelect(kinds, nil)
	choose.SetSelectedIndex(0)
	add := widget.NewButtonWithIcon("Add", fynetheme.ContentAddIcon(), func() { u.addEffect(groove.EffectKinds[choose.SelectedIndex()]) })
	panel.Add(container.NewBorder(nil, nil, nil, add, choose))
	if len(t.Effects) == 0 {
		panel.Add(widget.NewLabel("Add effects or randomise a new chain."))
	}
	// The inspector shape includes effect IDs, kinds and order, so index and
	// kind are fixed for these widgets; values are read live.
	for index, e := range t.Effects {
		index, effectID, kind := index, e.ID, e.Kind
		effect := func() *groove.Effect { return track().Effect(effectID) }
		edit := func(label string, fn func(*groove.Effect)) {
			u.edit(label, func(p *groove.Project) error {
				if t := p.Track(id); t != nil && t.Effect(effectID) != nil {
					fn(t.Effect(effectID))
				}
				return nil
			})
		}
		// Each effect is a card: a header with enable, name, LFO, order,
		// remove and collapse, over its parameters.
		title := widget.NewLabelWithStyle(fmt.Sprintf("%d · %s", index+1, kind.Name()), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		title.Truncation = fyne.TextTruncateEllipsis
		on := u.boundCheck("", func() bool { return effect().Enabled }, func(v bool) { edit("Effect enabled", func(e *groove.Effect) { e.Enabled = v }) })
		u.live(func() {
			importance := widget.MediumImportance
			if !effect().Enabled || track().EffectsBypassed {
				importance = widget.LowImportance
			}
			if title.Importance != importance {
				title.Importance = importance
				title.Refresh()
			}
		})
		move := func(delta int) {
			u.edit("Move effect", func(p *groove.Project) error {
				t := p.Track(id)
				next := index + delta
				if next >= 0 && next < len(t.Effects) {
					t.Effects[index], t.Effects[next] = t.Effects[next], t.Effects[index]
				}
				return nil
			})
		}
		up := widget.NewButtonWithIcon("", fynetheme.MoveUpIcon(), func() { move(-1) })
		down := widget.NewButtonWithIcon("", fynetheme.MoveDownIcon(), func() { move(1) })
		if index == 0 {
			up.Disable()
		}
		if index == len(t.Effects)-1 {
			down.Disable()
		}
		remove := widget.NewButtonWithIcon("", fynetheme.DeleteIcon(), func() {
			u.edit("Remove effect", func(p *groove.Project) error { groove.RemoveEffect(p.Track(id), effectID); return nil })
		})
		lfo := widget.NewButton("LFO", func() { u.effectModulation(id, effectID, index) })
		u.live(func() {
			// The LFO button lights when either LFO is running.
			importance := widget.LowImportance
			if e := effect(); e.LFOs[0].Enabled || e.LFOs[1].Enabled {
				importance = widget.HighImportance
			}
			if lfo.Importance != importance {
				lfo.Importance = importance
				lfo.Refresh()
			}
		})
		for _, b := range []*widget.Button{up, down, remove} {
			b.Importance = widget.LowImportance
		}
		body := container.NewVBox()
		var fold *widget.Button
		setFolded := func(folded bool) {
			if folded {
				u.fxFolded[effectID] = true
				fold.SetIcon(fynetheme.MenuExpandIcon())
				body.Hide()
			} else {
				delete(u.fxFolded, effectID)
				fold.SetIcon(fynetheme.MenuDropDownIcon())
				body.Show()
			}
		}
		fold = widget.NewButtonWithIcon("", fynetheme.MenuDropDownIcon(), func() { setFolded(!u.fxFolded[effectID]) })
		fold.Importance = widget.LowImportance
		header := container.NewBorder(nil, nil, container.NewHBox(fold, on), container.NewHBox(lfo, up, down, remove), title)
		card := container.NewVBox(header, body)
		roles := u.scheme.Roles()
		background := canvas.NewRectangle(colour(roles.Panel))
		background.StrokeColor, background.StrokeWidth, background.CornerRadius = colour(roles.Border), 1, 4
		panel.Add(container.NewStack(background, container.NewPadded(card)))
		setFolded(u.fxFolded[effectID])
		for _, param := range groove.EffectParameters(kind) {
			param := param
			value := func() float64 { return effect().Parameters[param.ID] }
			if choices := groove.EffectChoices(kind, param.ID); choices != nil {
				pick := u.boundSelect(choices, func() int { return int(value()) }, func(i int) {
					edit("Effect "+param.Name, func(e *groove.Effect) { e.Parameters[param.ID] = float64(i) })
				})
				body.Add(container.NewBorder(nil, nil, widget.NewLabel(param.Name), nil, pick))
				continue
			}
			if param.ID == "sync" {
				body.Add(u.boundCheck("Tempo sync", func() bool { return value() >= .5 }, func(v bool) {
					edit(kind.Name()+" sync", func(e *groove.Effect) {
						e.Parameters["sync"] = 0
						if v {
							e.Parameters["sync"] = 1
						}
					})
				}))
				continue
			}
			if kind.TempoSynced() && ((param.ID == "time" && e.Parameters["sync"] >= .5) || (param.ID == "division" && e.Parameters["sync"] < .5)) {
				continue
			}
			lock := ""
			if param.Lockable {
				lock = groove.EffectLockID(effectID, param.ID)
			}
			control := paramControl{
				track: id, name: param.Name, lock: lock,
				min: param.Min, max: param.Max, increment: param.Increment, def: param.Default,
				base:    value,
				setBase: func(v float64) { edit("Effect "+param.Name, func(e *groove.Effect) { e.Parameters[param.ID] = v }) },
			}
			if param.ID == "division" {
				values := []float64{1. / 64, 1. / 32, 1. / 16, 1. / 8, 3. / 16, 1. / 4, 3. / 8, 1. / 2, 3. / 4, 1}
				control.name = "Division"
				body.Add(u.paramSelect(control, []string{"1/64", "1/32", "1/16", "1/8", "Dotted 1/8", "1/4", "Dotted 1/4", "1/2", "Dotted 1/2", "1 bar"}, values))
				continue
			}
			body.Add(u.paramRow(control))
		}
	}
	return panel
}
