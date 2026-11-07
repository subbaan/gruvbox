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

func findCheck(root fyne.CanvasObject) *widget.Check {
	if scroll, ok := root.(*container.Scroll); ok {
		return findCheck(scroll.Content)
	}
	if check, ok := root.(*widget.Check); ok {
		return check
	}
	if c, ok := root.(*fyne.Container); ok {
		for _, obj := range c.Objects {
			if check := findCheck(obj); check != nil {
				return check
			}
		}
	}
	return nil
}
func TestSectionTracksAndGrooveDuplication(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("arrange")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	u.edit("Section", func(p *groove.Project) error { p.Song = append(p.Song, groove.NewSongSection(0)); return nil })
	sectionID, id := u.session.Project.Song[0].ID, u.selected
	u.sectionTracks(sectionID)
	panel := u.panels["Section tracks"]
	check := findCheck(panel.Content())
	if check == nil {
		t.Fatal("section track controls missing")
	}
	test.TapAt(check, fyne.NewPos(12, check.Size().Height/2))
	if !u.session.Project.Song[0].Mutes[id] || u.selectedTrack().Mute {
		t.Fatalf("section mute state: id=%s, map=%v, global=%v, check=%s/%v", id, u.session.Project.Song[0].Mutes, u.selectedTrack().Mute, check.Text, check.Checked)
	}
	test.Tap(songButton(u.songPanel, "Duplicate"))
	if !u.session.Project.Song[1].Mutes[id] {
		t.Fatal("duplicate lost section mute overrides")
	}
	u.removeTrack()
	if u.session.Project.Validate() != nil || len(u.session.Project.Song[0].Mutes) != 0 {
		t.Fatal("removing track left dangling section references")
	}
	u.undo()
	if !u.session.Project.Song[0].Mutes[id] {
		t.Fatal("remove undo lost section mute")
	}
	panel.Close()
	u.selected = id
	original := u.session.Project.Clone()
	u.duplicatePattern(false)
	if u.session.Project.Pattern != 1 {
		t.Fatal("duplicate did not select empty pattern")
	}
	for i, track := range u.session.Project.Tracks {
		if len(track.Patterns[1].Steps) != len(original.Tracks[i].Patterns[0].Steps) {
			t.Fatal("whole groove duplicate omitted a track")
		}
		for n, step := range track.Patterns[1].Steps {
			if step.Note != original.Tracks[i].Patterns[0].Steps[n].Note || step.Active != original.Tracks[i].Patterns[0].Steps[n].Active {
				t.Fatal("duplicate changed groove")
			}
		}
	}
	u.undo()
	if u.session.Project.Pattern != 0 {
		t.Fatal("duplicate undo did not restore pattern slot")
	}
	u.exportWAV(false)
	export := u.panels["Export WAV"]
	if export == nil || songButton(export.Content(), "Export") == nil {
		t.Fatal("native export controls missing")
	}
	export.Close()
}
