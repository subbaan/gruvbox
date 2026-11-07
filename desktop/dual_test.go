package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
	"testing"
)

func TestDualEnvelopeSliderLiveUndoAndSectionRetention(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("Dual")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	track := groove.NewTrack(groove.Dual, 0)
	u.edit("Add Dual", func(p *groove.Project) error { p.Tracks = append(p.Tracks, track); u.selected = track.ID; return nil })
	u.dualAccordion.Open(2)
	u.dualAccordion.Close(0)
	before := u.selectedTrack().Parameters["amp_attack"]
	attack := findSlider(u.dualAccordion.Items[2].Detail, "Attack (s)")
	if attack == nil {
		t.Fatal("amp attack slider missing")
	}
	attack.Resize(fyne.NewSize(300, 36))
	attack.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(200, 18)}})
	if u.selectedTrack().Parameters["amp_attack"] == before {
		t.Fatal("envelope slider is not live")
	}
	attack.DragEnd()
	if !u.dualAccordion.Items[2].Open || u.dualAccordion.Items[0].Open {
		t.Fatal("release lost the open sound section")
	}
	u.undo()
	if u.selectedTrack().Parameters["amp_attack"] != before || !u.dualAccordion.Items[2].Open {
		t.Fatal("envelope undo or section retention failed")
	}
	if !u.selectedTrack().Kind.Capabilities().NoteLengths || !u.selectedTrack().Kind.Capabilities().Slide {
		t.Fatal("Dual piano-roll controls missing")
	}
}
