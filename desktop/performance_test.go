package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
	"reflect"
	"strings"
	"testing"
)

func TestPerformanceControlsQueueKeepAndUndo(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("performance")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	if !u.performanceActions[0].Disabled() || !u.performanceKeep.Disabled() {
		t.Fatal("stopped controls enabled")
	}
	u.session.Project.BPM = 300
	u.player.Publish(u.session.Project)
	u.player.SetPlaying(true)
	u.player.Render(make([]float32, 2))
	u.updatePlaybackLabel()
	if u.performanceActions[0].Disabled() {
		t.Fatal("playing cues disabled")
	}
	u.key(fyne.KeyF)
	if u.player.QueuedPerformance() != groove.DrumFill || u.performanceCancel.Disabled() {
		t.Fatal("fill button did not queue")
	}
	test.Tap(u.performanceCancel)
	if u.player.QueuedPerformance() != groove.NoPerformance {
		t.Fatal("cancel button failed")
	}
	test.Tap(u.performanceActions[2])
	u.player.Render(make([]float32, (groove.SampleRate*240/300)*2))
	u.updatePlaybackLabel()
	if u.player.RunningPerformance() != groove.GrooveVariation || u.performanceKeep.Disabled() {
		t.Fatal("take did not launch")
	}
	test.Tap(u.performanceKeep)
	if u.session.Project.Pattern != 0 || u.session.Project.Tracks[0].Patterns[1].Steps[0].Note < 0 {
		t.Fatal("keep changed current scene")
	}
	found := false
	for _, track := range u.session.Project.Tracks {
		for _, step := range track.Patterns[1].Steps {
			found = found || step.Active
		}
	}
	if !found {
		t.Fatal("kept slot empty")
	}
	u.undo()
	for _, track := range u.session.Project.Tracks {
		for _, step := range track.Patterns[1].Steps {
			if step.Active {
				t.Fatal("keep undo failed")
			}
		}
	}
}

func TestJamChangesJustBeforeTheBarAndUndoes(t *testing.T) {
	u, _ := redesignUI(t)
	if u.jam.amount.Selected != "Evolving" || u.jam.state.Amount != groove.JamEvolving {
		t.Fatalf("jam amount starts at %q", u.jam.amount.Selected)
	}
	u.jam.amount.SetSelected("Wild")
	if u.jam.state.Amount != groove.JamWild {
		t.Fatal("amount select did not set the jam amount")
	}
	u.jam.every = 4
	u.setJam(true)
	before := u.session.Project.Clone()
	u.player.Publish(u.session.Project)
	u.player.SetPlaying(true)
	var block [256 * 2]float32
	moved := -1.
	for i := 0; i < 4000 && u.player.BarClock() < 4.5; i++ {
		u.player.Render(block[:])
		u.jamTick()
		if moved < 0 && strings.HasPrefix(u.session.LastAction, "Jam · ") {
			moved = u.player.BarClock()
		}
	}
	if moved < 4-jamLead || moved >= 4 {
		t.Fatalf("jam moved at bar clock %.3f", moved)
	}
	if !strings.Contains(u.jam.label.Text, "next change in") {
		t.Fatalf("label %q", u.jam.label.Text)
	}
	u.undo()
	if !reflect.DeepEqual(before, u.session.Project) {
		t.Fatal("undo should step back over the jam move")
	}
	// Song mode pauses the jam.
	u.player.SetSongMode(true)
	u.jamTick()
	if u.jam.next != 0 || !strings.Contains(u.jam.label.Text, "Song") {
		t.Fatalf("label %q", u.jam.label.Text)
	}
	u.player.SetPlaying(false)
	u.key(fyne.KeyJ)
	if u.jam.on {
		t.Fatal("J did not turn jam off")
	}
}
