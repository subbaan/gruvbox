package desktop

import (
	"fyne.io/fyne/v2/test"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
	"testing"
)

func TestTemplateChangeResetsEditorAndPlayback(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("templates")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	u.selectedStep, u.page = 15, 3
	u.path = "old.gruvbox3"
	u.sampleViews["old"] = sampleView{.2, .7}
	u.player.SetPlaying(true)
	u.player.SetSongMode(true)
	u.startTemplate(groove.EmptyTemplate)
	if len(u.session.Project.Tracks) != 0 || u.selected != "" || u.selectedStep != 0 || u.page != 0 || u.path != "" || len(u.sampleViews) != 0 || u.player.Playing() || u.player.SongMode() {
		t.Fatal("template retained previous session state")
	}
	u.startTemplate(groove.TomBassTemplate)
	if u.selectedTrack() == nil || u.selectedTrack().Kind != groove.Tom || len(u.session.Project.Tracks) != 5 || u.session.Dirty() {
		t.Fatal("tom template not initialized")
	}
	// Changes are protected by the usual discard confirmation.
	u.edit("Change root", func(p *groove.Project) error { p.Tracks[0].Parameters["pitch"] = 100; return nil })
	u.startTemplate(groove.SamplerTemplate)
	if u.selectedTrack().Kind != groove.Tom || u.modals["Unsaved groove"] == nil {
		t.Fatal("template bypassed unsaved changes")
	}
	u.modals["Unsaved groove"].Close()
	if u.selectedTrack().Kind != groove.Tom {
		t.Fatal("closing confirmation discarded groove")
	}
}
