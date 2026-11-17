package desktop

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// findSelects lists the selects under root.
func findSelects(root fyne.CanvasObject) []*widget.Select {
	var out []*widget.Select
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Select:
			out = append(out, o)
		case *container.Scroll:
			walk(o.Content)
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(root)
	return out
}

// The SH-101 gets its own panel: front-panel sections, switches as selects
// that edit (and undo) the track, and log-scaled envelope times.
func TestSH101PanelSwitchesAndEnvelope(t *testing.T) {
	u, _ := redesignUI(t)
	u.addTrackOf(groove.SH101)
	s := u.selectedTrack()
	if s == nil || s.Kind != groove.SH101 || u.dualAccordion == nil {
		t.Fatal("SH-101 panel missing")
	}
	titles := []string{}
	for _, item := range u.dualAccordion.Items {
		titles = append(titles, item.Title)
	}
	if len(titles) != 5 || titles[0] != "VCF" || titles[1] != "VCO and mixer" {
		t.Fatalf("SH-101 sections %v", titles)
	}
	u.dualAccordion.Open(1)
	selects := findSelects(u.dualAccordion.Items[1].Detail)
	var rangeSelect *widget.Select
	for _, sel := range selects {
		if len(sel.Options) == 4 && sel.Options[0] == "16'" {
			rangeSelect = sel
		}
	}
	if rangeSelect == nil || rangeSelect.Selected != "8'" {
		t.Fatalf("range switch missing or not on 8' (%d selects)", len(selects))
	}
	rangeSelect.SetSelected("4'")
	if u.selectedTrack().Parameters["range"] != 2 {
		t.Fatal("range switch did not edit the track")
	}
	u.undo()
	if u.selectedTrack().Parameters["range"] != 1 {
		t.Fatal("range switch undo failed")
	}
	u.dualAccordion.Open(2)
	decay := findSlider(u.dualAccordion.Items[2].Detail, "Decay (s)")
	if decay == nil {
		t.Fatal("decay slider missing")
	}
	// Log scale: the middle of the slider is the geometric middle of 2 ms–10 s.
	decay.Resize(fyne.NewSize(300, 36))
	decay.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(150, 18)}})
	decay.DragEnd()
	if v := u.selectedTrack().Parameters["decay"]; v < .05 || v > .4 {
		t.Fatalf("decay at mid-slider is %.3f s, not log-scaled", v)
	}
}
