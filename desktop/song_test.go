package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/theme"
	"testing"
)

func songButton(root fyne.CanvasObject, name string) *widget.Button {
	if b, ok := root.(*widget.Button); ok && b.Text == name {
		return b
	}
	if c, ok := root.(*fyne.Container); ok {
		for _, obj := range c.Objects {
			if b := songButton(obj, name); b != nil {
				return b
			}
		}
	}
	return nil
}
func TestSongEditorCommandsAndManualPatternQueue(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("song")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	test.Tap(songButton(u.songPanel, "Add current pattern"))
	if len(u.session.Project.Song) != 1 {
		t.Fatal("add song section failed")
	}
	id := u.session.Project.Song[0].ID
	row := u.songPanel.Objects[1].(*fyne.Container)
	name := row.Objects[1].(*fyne.Container).Objects[0].(*commitEntry)
	name.SetText("Intro")
	name.OnSubmitted(name.Text)
	if u.session.Project.Song[0].Name != "Intro" {
		t.Fatal("song name failed")
	}
	// Entries also apply when focus leaves them, and an unchanged blur adds no history.
	row = u.songPanel.Objects[1].(*fyne.Container)
	name = row.Objects[1].(*fyne.Container).Objects[0].(*commitEntry)
	commit, commits := name.OnCommit, 0
	name.OnCommit = func(v string) { commits++; commit(v) }
	name.FocusLost()
	if commits != 0 {
		t.Fatal("unchanged entry blur created an edit")
	}
	name.SetText("Verse")
	name.FocusLost()
	if u.session.Project.Song[0].Name != "Verse" {
		t.Fatal("entry did not commit on focus loss")
	}
	u.undo()
	test.Tap(songButton(u.songPanel, "Duplicate"))
	if len(u.session.Project.Song) != 2 || u.session.Project.Song[1].ID == id {
		t.Fatal("duplicate aliases section identity")
	}
	u.moveSection(id, 1)
	if u.session.Project.Song[1].ID != id {
		t.Fatal("section reorder failed")
	}
	u.undo()
	if u.session.Project.Song[0].ID != id {
		t.Fatal("reorder undo failed")
	}
	test.Tap(songButton(u.songPanel, "×"))
	if len(u.session.Project.Song) != 1 {
		t.Fatal("remove failed")
	}
	u.undo()
	u.player.SetPlaying(true)
	var output [512]float32
	u.player.Render(output[:])
	u.player.SetSongMode(true)
	u.selectPattern(1)
	u.player.Render(output[:])
	if u.player.SongMode() || u.player.CurrentPattern() != 0 || u.player.QueuedPattern() != 1 {
		t.Fatal("GUI pattern selection was not queued/manual")
	}
}
