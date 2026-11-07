package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
	"testing"
)

func TestEffectModulationWindowLiveUndoAndProjectScope(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("FX")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	track := groove.NewTrack(groove.Sampler, 0)
	u.session.Project.Tracks = []groove.Track{track}
	u.selected = track.ID
	u.refresh()
	u.addEffect(groove.Chorus)
	id := u.selectedTrack().Effects[0].ID
	u.effectModulation(track.ID, id, 0)
	modWindow := u.drawerTitled("FX 1 / Sampler / Chorus modulation")
	if modWindow == nil {
		t.Fatal("FX LFOs did not open in a drawer")
	}
	var enabled *widget.Check
	var depth *widget.Slider
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Check:
			if o.Text == "LFO 1 enabled" {
				enabled = o
			}
		case *widget.Slider:
			if depth == nil {
				depth = o
			}
		case *container.Scroll:
			walk(o.Content)
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(modWindow.Content())
	if enabled == nil || depth == nil {
		t.Fatal("FX LFO controls missing")
	}
	enabled.SetChecked(true)
	depth = nil
	walk(modWindow.Content())
	before := u.selectedTrack().Effect(id).LFOs[0].Depth
	depth.Resize(fyne.NewSize(300, 36))
	depth.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(245, 18)}, Dragged: fyne.NewDelta(100, 0)})
	if u.selectedTrack().Effect(id).LFOs[0].Depth == before {
		t.Fatal("FX LFO depth is not live")
	}
	depth.DragEnd()
	u.undo()
	if u.selectedTrack().Effect(id).LFOs[0].Depth != before {
		t.Fatal("FX LFO gesture undo failed")
	}
	depth = nil
	walk(modWindow.Content())
	if depth.Value != before {
		t.Fatal("FX LFO window did not refresh after undo")
	}
	u.undo()
	if u.selectedTrack().Effect(id).LFOs[0].Enabled {
		t.Fatal("FX LFO enable undo failed")
	}
	u.edit("Remove FX", func(p *groove.Project) error { groove.RemoveEffect(p.Track(track.ID), id); return nil })
	if len(u.effectModulationRefresh) != 0 {
		t.Fatal("removed FX left a modulation window")
	}
}
