package desktop

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func TestModeToggleNeedsSongSections(t *testing.T) {
	u, _ := redesignUI(t)
	if !u.songMode.Disabled() || u.patternMode.Importance != widget.HighImportance {
		t.Fatal("Song mode should be disabled without sections and Pattern lit")
	}
	u.edit("Add song section", func(p *groove.Project) error { p.Song = append(p.Song, groove.NewSongSection(0)); return nil })
	u.updatePlaybackLabel()
	test.Tap(u.songMode)
	u.updatePlaybackLabel()
	if !u.player.SongMode() || u.songMode.Importance != widget.HighImportance {
		t.Fatal("Song toggle did not enable song mode")
	}
	test.Tap(u.patternMode)
	if u.player.SongMode() {
		t.Fatal("Pattern toggle did not return to pattern mode")
	}
}

func TestGeneratorsLiveInInspectorHeader(t *testing.T) {
	u, _ := redesignUI(t)
	var acid *widget.Button
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Button:
			if o.Text == "Acid" {
				acid = o
			}
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(u.inspector)
	if acid == nil {
		t.Fatal("303 generator missing from inspector header")
	}
	before := u.selectedTrack().Patterns[0].Steps
	test.Tap(acid)
	if u.session.LastAction != "Acid" {
		t.Fatalf("Acid did not run (%q, %d steps)", u.session.LastAction, len(before))
	}
}

func TestMixerStripsSyncAndToggle(t *testing.T) {
	u, _ := redesignUI(t)
	if len(u.mixer.Objects) != len(u.session.Project.Tracks) {
		t.Fatal("mixer should show one strip per track")
	}
	strip := u.mixer.Objects[2]
	second := u.session.Project.Tracks[2].ID
	var mute *widget.Button
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Button:
			if o.Text == "M" {
				mute = o
			}
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(strip)
	test.Tap(mute)
	if !u.session.Project.Track(second).Mute || mute.Importance != widget.DangerImportance {
		t.Fatal("mixer mute did not toggle its own track")
	}
	if u.mixer.Objects[2] != strip {
		t.Fatal("mute rebuilt the mixer")
	}
	u.undo()
	if mute.Importance == widget.DangerImportance {
		t.Fatal("undo did not clear the mute chip in place")
	}
}

func TestTrackContextMenuAndChips(t *testing.T) {
	u, w := redesignUI(t)
	row := float32(34 + rowHeight + 10)
	test.TapSecondaryAt(u.grid, fyne.NewPos(40, row))
	second := u.session.Project.Tracks[1].ID
	if u.selected != second {
		t.Fatal("right-click did not select the track")
	}
	overlay := w.Canvas().Overlays().Top()
	if overlay == nil {
		t.Fatal("right-click on a track name did not open the track menu")
	}
	w.Canvas().Overlays().Remove(overlay)
	u.renameTrack(second, fyne.NewPos(10, 10))
	entry, ok := w.Canvas().Focused().(*commitEntry)
	if !ok {
		t.Fatal("rename popup did not focus its entry")
	}
	entry.SetText("Bass two")
	entry.OnSubmitted(entry.Text)
	if u.session.Project.Track(second).Name != "Bass two" {
		t.Fatal("rename failed")
	}
	test.TapAt(u.grid, fyne.NewPos(muteLeft+10, row))
	if !u.session.Project.Track(second).Mute {
		t.Fatal("mute chip hit area failed")
	}
	test.TapAt(u.grid, fyne.NewPos(soloLeft+10, row))
	if !u.session.Project.Track(second).Solo {
		t.Fatal("solo chip hit area failed")
	}
	test.TapAt(u.grid, fyne.NewPos(holdLeft+10, row))
	if !u.session.Project.Track(second).Hold || u.session.Project.Track(second).Mute != true {
		t.Fatal("hold chip hit area failed")
	}
}
