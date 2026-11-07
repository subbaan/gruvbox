package desktop

import (
	"testing"

	"fyne.io/fyne/v2"

	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func TestVelocityShade(t *testing.T) {
	if velocityShade("#ff0000", "#000000", 1) != "#ff0000" {
		t.Fatal("full velocity should keep the track colour")
	}
	if got := velocityShade("#ff0000", "#000000", 0); got != "#660000" {
		t.Fatalf("silent step shade = %s", got)
	}
	if velocityShade("#ff0000", "#000000", .5) == velocityShade("#ff0000", "#000000", 1) {
		t.Fatal("velocity does not change the shade")
	}
}

func TestEmptyPagesAreDimmed(t *testing.T) {
	u, _ := redesignUI(t)
	id := u.selected
	u.edit("Pattern length", func(p *groove.Project) error {
		pattern := &p.Track(id).Patterns[p.Pattern]
		for len(pattern.Steps) < 48 {
			pattern.Steps = append(pattern.Steps, groove.DefaultStep())
		}
		pattern.Steps[40].Active = true
		return nil
	})
	u.updatePageButtons()
	if u.pageButtons[1].Importance != widget.LowImportance || u.pageButtons[2].Importance != widget.MediumImportance {
		t.Fatal("empty page 2 should be dimmed and page 3 with a note should not")
	}
}

func TestRollFitsAndFollowsNotes(t *testing.T) {
	u, _ := redesignUI(t)
	id := u.selected
	u.edit("High notes", func(p *groove.Project) error {
		steps := p.Track(id).Patterns[p.Pattern].Steps
		for i := range steps {
			steps[i].Active = i%4 == 0
			steps[i].Note = 84 + i%3
		}
		return nil
	})
	// The edit alone keeps the range; switching track or pattern refits it.
	u.roll.rollKey = ""
	u.refresh()
	if u.roll.noteTop < 86 || u.roll.noteTop-(rollRows-1) > 84 {
		t.Fatalf("range top %d does not cover notes 84–86", u.roll.noteTop)
	}
	u.editorTabs.SelectIndex(1)
	if u.roll.followNote != -1 {
		t.Fatal("scroll follow still pending after the roll was shown")
	}
	before := u.roll.noteTop
	u.roll.shiftOctave(1)
	u.refresh()
	if u.roll.noteTop != before+12 {
		t.Fatal("refresh undid a manual octave shift")
	}
	if _, _, ok := u.roll.noteCell(fyne.NewPos(70, 34+rollRows*rollRowHeight-1)); !ok {
		t.Fatal("bottom roll row not hit-testable")
	}
}
