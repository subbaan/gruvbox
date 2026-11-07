package desktop

import (
	"fyne.io/fyne/v2/test"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
	"reflect"
	"testing"
)

func TestRecordedTakeTrimUndoAndProjectIdentity(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("recording")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	track := groove.NewTrack(groove.Sampler, 0)
	track.Parameters["gain_db"], track.Parameters["fade_in"] = -3, 200
	track.Effects = []groove.Effect{groove.NewEffect(groove.Delay)}
	track.Patterns[0].Steps[0].Active = true
	u.edit("Add sampler", func(p *groove.Project) error { p.Tracks = append(p.Tracks, track); u.selected = track.ID; return nil })
	before := u.session.Project.Clone()
	sample, err := groove.SampleFromPCM("take.wav", []byte{0, 32, 0, 224, 0, 16, 0, 240}, 44100, 2)
	if err != nil {
		t.Fatal(err)
	}
	session := u.session
	if err := u.useRecordedSample(session, track.ID, sample, .25, .75); err != nil {
		t.Fatal(err)
	}
	got := u.selectedTrack()
	if got.Sample != sample || got.Parameters["start"] != .25 || got.Parameters["end"] != .75 || got.Parameters["gain_db"] != 0 || got.Parameters["fade_in"] != 0 || !reflect.DeepEqual(got.Patterns, track.Patterns) || !reflect.DeepEqual(got.Effects, track.Effects) {
		t.Fatal("use take lost trim, retained old processing, or changed notes/FX")
	}
	u.undo()
	if !reflect.DeepEqual(u.session.Project, before) {
		t.Fatal("use take did not undo as one command")
	}
	u.session = groove.NewSession(groove.NewProject())
	if err := u.useRecordedSample(session, track.ID, sample, 0, 1); err == nil {
		t.Fatal("recording leaked into a replacement project")
	}
	if !reflect.DeepEqual(session.Project, before) {
		t.Fatal("stale use take mutated the old session")
	}
}
