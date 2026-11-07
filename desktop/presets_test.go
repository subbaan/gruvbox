package desktop

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
)

func TestSoundRecallPublishesLiveAndUndoRestoresTrack(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("preset recall")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	before := u.session.Project.Clone()
	u.player.SetPlaying(true)
	preset := groove.FactorySounds(groove.Acid)[2]
	u.recallSound(u.selected, preset, true)
	if !u.player.Playing() || u.selectedTrack().Parameters["resonance"] != preset.Parameters["resonance"] || len(u.selectedTrack().Effects) != 2 || !reflect.DeepEqual(u.selectedTrack().Patterns, before.Tracks[0].Patterns) {
		t.Fatal("recall interrupted playback or failed to recall the sound independently of notes")
	}
	var out [groove.BlockSize * 2]float32
	u.player.Render(out[:])
	u.undo()
	if !reflect.DeepEqual(u.session.Project, before) {
		t.Fatal("GUI preset recall did not restore the whole sound in one undo")
	}
	u.redo()
	if len(u.selectedTrack().Effects) != 2 {
		t.Fatal("GUI preset recall redo lost effects")
	}
}
