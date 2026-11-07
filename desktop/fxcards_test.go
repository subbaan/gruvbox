package desktop

import (
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// fxButtons finds buttons in the FX tab by text (LFO) or by order of icon-only
// buttons within each card header.
func fxButtons(root fyne.CanvasObject) (lfo []*widget.Button, all []*widget.Button) {
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Button:
			all = append(all, o)
			if o.Text == "LFO" {
				lfo = append(lfo, o)
			}
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(root)
	return
}

func TestFXCardsFoldAndShowLFOs(t *testing.T) {
	u, _ := redesignUI(t)
	u.inspectorTabs.SelectIndex(4)
	u.addEffect(groove.Delay)
	u.addEffect(groove.Reverb)
	id, first := u.selected, u.selectedTrack().Effects[0].ID
	lfo, all := fxButtons(u.inspectorTabs.Items[4].Content)
	if len(lfo) != 2 || lfo[0].Importance != widget.LowImportance {
		t.Fatalf("each card should have an unlit LFO button: %d buttons, effects %+v", len(lfo), u.selectedTrack().Effects)
	}
	// The first icon-only button in a card header is its fold toggle.
	var fold *widget.Button
	for _, b := range all {
		if b.Text == "" && b.Icon != nil {
			fold = b
			break
		}
	}
	sliders := func() int {
		n := 0
		var walk func(fyne.CanvasObject)
		walk = func(obj fyne.CanvasObject) {
			if !obj.Visible() {
				return
			}
			if _, ok := obj.(*widget.Slider); ok {
				n++
			}
			if c, ok := obj.(*fyne.Container); ok {
				for _, child := range c.Objects {
					walk(child)
				}
			}
		}
		walk(u.inspectorTabs.Items[4].Content)
		return n
	}
	before := sliders()
	test.Tap(fold)
	if !u.fxFolded[first] || sliders() >= before {
		t.Fatal("folding did not hide the first card's controls")
	}
	// Fold state survives a rebuild (adding an effect changes the shape).
	u.addEffect(groove.Chorus)
	if !u.fxFolded[first] {
		t.Fatal("fold state lost on rebuild")
	}
	u.edit("FX LFO", func(p *groove.Project) error {
		p.Track(id).Effect(first).LFOs[0] = groove.DefaultLFOs()[0]
		p.Track(id).Effect(first).LFOs[0].Enabled = true
		return nil
	})
	lfo, _ = fxButtons(u.inspectorTabs.Items[4].Content)
	if lfo[0].Importance != widget.HighImportance || lfo[1].Importance != widget.LowImportance {
		t.Fatal("LFO button does not show the running LFO")
	}
}

// Widgets styled by a bound sync must be right as soon as they are built,
// not only after the next refresh.
func TestBoundWidgetsStartInState(t *testing.T) {
	u, _ := redesignUI(t)
	u.toggleMute()
	u.mixerKey = ""
	u.refreshMixer()
	_, buttons := fxButtons(u.mixer)
	muted := false
	for _, b := range buttons {
		if b.Text == "M" && b.Importance == widget.DangerImportance {
			muted = true
		}
	}
	if !muted {
		t.Fatal("rebuilt mixer strip does not show the muted track")
	}
	u.inspectorTabs.SelectIndex(1)
	_, buttons = fxButtons(u.inspectorTabs.Items[1].Content)
	for _, b := range buttons {
		if b.Text == "Clear locks" && !b.Disabled() {
			t.Fatal("Clear locks enabled on a step without locks")
		}
	}
}

func TestPedalboardPickerReplacesChainWithUndo(t *testing.T) {
	u, _ := redesignUI(t)
	u.inspectorTabs.SelectIndex(4)
	before := u.session.Project.Clone()
	u.applyPedalboard("Dub tape")
	effects := u.selectedTrack().Effects
	if len(effects) == 0 {
		t.Fatal("pedalboard added nothing")
	}
	hasTape := false
	for _, e := range effects {
		hasTape = hasTape || e.Kind == groove.TapeEcho
	}
	if !hasTape {
		t.Fatalf("Dub tape board has no tape echo: %+v", effects)
	}
	if u.session.LastAction != "Pedalboard Dub tape" {
		t.Fatalf("status %q", u.session.LastAction)
	}
	var picker *widget.Select
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Select:
			if o.PlaceHolder == "Pedalboard…" {
				picker = o
			}
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(u.inspectorTabs.Items[4].Content)
	if picker == nil || len(picker.Options) != len(groove.Pedalboards)+1 {
		t.Fatal("pedalboard picker missing")
	}
	picker.SetSelected(randomBoard)
	if len(u.selectedTrack().Effects) == 0 {
		t.Fatal("random board added nothing")
	}
	u.undo()
	u.undo()
	if !reflect.DeepEqual(before, u.session.Project) {
		t.Fatal("pedalboard undo failed")
	}
}

func TestRandomiseSoundNamesItsArchetypeAndUndoes(t *testing.T) {
	u, _ := redesignUI(t)
	named := false
	for n := 0; n < 20 && !named; n++ {
		before := u.session.Project.Clone()
		u.randomiseSound()
		if !strings.HasPrefix(u.session.LastAction, "Randomise sound") {
			t.Fatalf("status %q", u.session.LastAction)
		}
		if name, ok := strings.CutPrefix(u.session.LastAction, "Randomise sound · "); ok {
			if _, found := groove.FactorySoundNamed(u.selectedTrack().Kind, name); !found {
				t.Fatalf("status names unknown sound %q", name)
			}
			named = true
		}
		u.undo()
		if !reflect.DeepEqual(before, u.session.Project) {
			t.Fatal("randomise sound undo failed")
		}
	}
	if !named {
		t.Fatal("twenty randomisations never named an archetype")
	}
}
