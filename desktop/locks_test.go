package desktop

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// findParamName returns the name label of a Sound/FX control.
func findParamName(root fyne.CanvasObject, text string) *paramName {
	var found *paramName
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *paramName:
			if o.Text == text && found == nil {
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

func drag(s *widget.Slider, v float64) {
	s.OnChanged(v)
	s.OnChangeEnded(v)
}

func TestLockModeWritesStepLocks(t *testing.T) {
	u, _ := redesignUI(t)
	id := u.selected
	sound := u.inspectorTabs.Items[0].Content
	cutoff := findSlider(sound, "Cutoff")
	base := u.selectedTrack().Parameters["cutoff"]
	builds := u.inspectorBuilds
	u.key(fyne.KeyK)
	if !u.lockMode {
		t.Fatal("K did not enter lock mode")
	}
	// In lock mode a grid click selects the step without toggling it.
	before := u.selectedTrack().Patterns[0].Steps[2].Active
	test.TapAt(u.grid, fyne.NewPos(nameWidth+8+2*((u.grid.Size().Width-nameWidth)/16), 46))
	if u.selectedStep != 2 || u.selectedTrack().Patterns[0].Steps[2].Active != before {
		t.Fatal("lock-mode click toggled the step or did not select it")
	}
	drag(cutoff, .2)
	step := u.selectedTrack().Patterns[0].Steps[2]
	if step.Locks["cutoff"] != .2 || u.selectedTrack().Parameters["cutoff"] != base {
		t.Fatal("lock-mode slider changed the base sound instead of locking the step")
	}
	name := findParamName(sound, "Cutoff")
	if name.Importance != widget.HighImportance {
		t.Fatal("locked parameter is not highlighted")
	}
	// Another step shows the base value and no highlight.
	u.selectedStep = 3
	u.refresh()
	if cutoff.Value != base || name.Importance == widget.HighImportance {
		t.Fatal("unlocked step showed another step's lock")
	}
	// Non-lockable controls are dimmed while locking.
	if findParamName(sound, "Waveform").Importance != widget.LowImportance {
		t.Fatal("non-lockable control not dimmed in lock mode")
	}
	// Double-click clears the shown lock.
	u.selectedStep = 2
	u.refresh()
	name.DoubleTapped(nil)
	if _, ok := u.selectedTrack().Patterns[0].Steps[2].Locks["cutoff"]; ok {
		t.Fatal("double-click did not clear the lock")
	}
	u.key(fyne.KeyEscape)
	if u.lockMode {
		t.Fatal("Escape did not leave lock mode")
	}
	// Outside lock mode, double-click resets the base value to its default.
	u.edit("Set Cutoff", func(p *groove.Project) error { p.Track(id).Parameters["cutoff"] = .9; return nil })
	name.DoubleTapped(nil)
	var def float64
	for _, p := range groove.Parameters(groove.Acid) {
		if p.ID == "cutoff" {
			def = p.Default
		}
	}
	if u.selectedTrack().Parameters["cutoff"] != def {
		t.Fatal("double-click did not reset to the default")
	}
	if u.inspectorBuilds != builds {
		t.Fatal("lock editing rebuilt the inspector")
	}
}

func TestFXLockAndStepLockList(t *testing.T) {
	u, _ := redesignUI(t)
	u.addEffect(groove.Delay)
	effect := u.selectedTrack().Effects[0].ID
	u.setLockMode(true)
	mix := findSlider(u.inspectorTabs.Items[4].Content, "Mix")
	drag(mix, .8)
	lock := groove.EffectLockID(effect, "mix")
	if u.selectedTrack().Patterns[0].Steps[0].Locks[lock] != .8 || u.selectedTrack().Effects[0].Parameters["mix"] == .8 {
		t.Fatal("FX slider did not lock in lock mode")
	}
	// The Step tab lists it with a short name and can clear all locks.
	if findSlider(u.inspectorTabs.Items[1].Content, "FX1 Mix") == nil {
		t.Fatal("step lock list does not show the FX lock")
	}
	var clear *widget.Button
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Button:
			if o.Text == "Clear locks" {
				clear = o
			}
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(u.inspectorTabs.Items[1].Content)
	test.Tap(clear)
	if len(u.selectedTrack().Patterns[0].Steps[0].Locks) != 0 || !clear.Disabled() {
		t.Fatal("Clear locks did not clear or stay enabled")
	}
}
