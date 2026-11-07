package desktop

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
)

func findNamedCheck(root fyne.CanvasObject, text string) *widget.Check {
	var found *widget.Check
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		if found != nil {
			return
		}
		switch o := obj.(type) {
		case *widget.Check:
			if o.Text == text {
				found = o
			}
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(root)
	return found
}

// Step edits from the grid must update the inspector in place.
func TestInspectorUpdatesInPlace(t *testing.T) {
	u, _ := redesignUI(t)
	builds, tabs := u.inspectorBuilds, u.inspectorTabs
	cutoff := findSlider(u.inspectorTabs.Items[0].Content, "Cutoff")
	active := findNamedCheck(u.inspectorTabs.Items[1].Content, "Active")
	if cutoff == nil || active == nil {
		t.Fatal("inspector controls missing")
	}
	test.TapAt(u.grid, fyne.NewPos(nameWidth+8+float32(1)*((u.grid.Size().Width-nameWidth)/16), 46))
	step := u.selectedTrack().Patterns[0].Steps[1]
	if u.selectedStep != 1 || active.Checked != step.Active {
		t.Fatal("step tab did not follow the tapped step")
	}
	if u.inspectorBuilds != builds || u.inspectorTabs != tabs || findSlider(u.inspectorTabs.Items[0].Content, "Cutoff") != cutoff {
		t.Fatal("step toggle rebuilt the inspector")
	}
	// Undo of a sound edit pushes the old value back into the same slider.
	before := cutoff.Value
	id := u.selected
	u.edit("Set Cutoff", func(p *groove.Project) error { p.Track(id).Parameters["cutoff"] = .1; return nil })
	if cutoff.Value != .1 {
		t.Fatal("sound slider did not show the new value")
	}
	u.undo()
	if cutoff.Value != before || u.inspectorBuilds != builds {
		t.Fatal("undo did not restore the slider in place")
	}
	// A new lock reveals its value row without a rebuild.
	u.setLock(id, "cutoff", .3)
	if _, ok := u.selectedTrack().Patterns[0].Steps[1].Locks["cutoff"]; !ok || u.inspectorBuilds != builds {
		t.Fatal("lock failed or rebuilt the inspector")
	}
	if row := findSlider(u.inspectorTabs.Items[1].Content, "Cutoff"); row == nil || row.Value != .3 {
		t.Fatal("step tab did not show the new lock row")
	}
	// Changing track does rebuild, because the controls differ.
	u.key(fyne.KeyDown)
	if u.inspectorBuilds == builds {
		t.Fatal("track change did not rebuild the inspector")
	}
}

// A focused entry keeps its focus and unfinished text across unrelated edits.
func TestFocusedEntrySurvivesRefresh(t *testing.T) {
	u, w := redesignUI(t)
	u.inspectorTabs.SelectIndex(2)
	var length *commitEntry
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *commitEntry:
			length = o
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(u.inspectorTabs.Items[2].Content)
	w.Canvas().Focus(length)
	length.SetText("32")
	id := u.selected
	u.edit("Toggle step", func(p *groove.Project) error {
		s := &p.Track(id).Patterns[0].Steps[3]
		s.Active = !s.Active
		return nil
	})
	if w.Canvas().Focused() != length || length.Text != "32" {
		t.Fatal("refresh stole focus or discarded unfinished text")
	}
	w.Canvas().Unfocus()
	if len(u.selectedTrack().Patterns[0].Steps) != 32 {
		t.Fatal("blur did not commit the pattern length")
	}
}

func TestSongRowsUpdateInPlace(t *testing.T) {
	u, _ := redesignUI(t)
	u.edit("Add song section", func(p *groove.Project) error { p.Song = append(p.Song, groove.NewSongSection(0)); return nil })
	row := u.songPanel.Objects[1]
	id := u.session.Project.Song[0].ID
	u.edit("Section bars", func(p *groove.Project) error { p.Section(id).Repeats = 8; return nil })
	bars := row.(*fyne.Container).Objects[5].(*fyne.Container).Objects[0].(*commitEntry)
	if u.songPanel.Objects[1] != row || bars.Text != "8" {
		t.Fatal("song row was rebuilt or not updated")
	}
	u.undo()
	if bars.Text != "4" {
		t.Fatal("undo did not update the bars entry in place")
	}
}

func TestEffectEditsKeepPanelAddsRebuild(t *testing.T) {
	u, _ := redesignUI(t)
	u.addEffect(groove.Delay)
	builds := u.inspectorBuilds
	id := u.selected
	effect := u.selectedTrack().Effects[0].ID
	u.edit("Effect Mix", func(p *groove.Project) error { p.Track(id).Effect(effect).Parameters["mix"] = .9; return nil })
	if u.inspectorBuilds != builds {
		t.Fatal("effect value edit rebuilt the inspector")
	}
	// Delay sync swaps the time slider for a division select.
	u.edit("Delay sync", func(p *groove.Project) error { p.Track(id).Effect(effect).Parameters["sync"] = 0; return nil })
	if u.inspectorBuilds == builds {
		t.Fatal("structural FX change did not rebuild")
	}
}

// BenchmarkStepToggle compares an in-place refresh with a forced inspector
// and song rebuild (the pre-alpha.25 behaviour).
func BenchmarkStepToggle(b *testing.B) {
	for _, rebuild := range []bool{false, true} {
		name := "in-place"
		if rebuild {
			name = "rebuild"
		}
		b.Run(name, func(b *testing.B) {
			a := test.NewApp()
			defer a.Quit()
			w := a.NewWindow("bench")
			u := newUI(a, w, theme.Library{Dir: b.TempDir()})
			w.SetContent(u.content())
			w.Resize(fyne.NewSize(1320, 780))
			u.addEffect(groove.Delay)
			u.addEffect(groove.Reverb)
			id := u.selected
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if rebuild {
					u.inspectorKey, u.songKey, u.mixerKey = "", "", ""
				}
				u.edit("Toggle step", func(p *groove.Project) error {
					s := &p.Track(id).Patterns[0].Steps[i%16]
					s.Active = !s.Active
					return nil
				})
			}
		})
	}
}
