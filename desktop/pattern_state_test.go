package desktop

import (
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
)

func TestPatternStateShortcutRecallSwitchAndHistory(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("pattern state")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	w.Resize(fyne.NewSize(1320, 780))
	before := u.session.Project.Clone()
	w.Canvas().(interface{ TypedShortcut(fyne.Shortcut) }).TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyP, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift})
	stored := u.session.Project.Clone()
	if count, edited := groove.PatternStateStatus(stored, 0); count != len(stored.Tracks) || edited != 0 || !strings.Contains(u.status.Text, "State: stored") {
		t.Fatal("shortcut did not store all tracks or update the indicator")
	}
	u.undo()
	if !reflect.DeepEqual(u.session.Project, before) {
		t.Fatal("store did not undo once")
	}
	u.redo()
	u.selectPattern(1)
	u.toggleMute()
	u.addEffect(groove.Chorus)
	u.edit("New sound", func(p *groove.Project) error {
		p.Tracks[0].Parameters["cutoff"] = .1
		p.Tracks[0].Level, p.Tracks[0].Pan = .2, .6
		p.Tracks[0].LFOs[0].Enabled, p.Tracks[0].LFOs[0].Target = true, 1
		return nil
	})
	u.storePatternState()
	second := u.session.Project.Clone()
	u.selectPattern(0)
	if !reflect.DeepEqual(groove.CaptureTrackState(u.session.Project.Tracks[0]), groove.CaptureTrackState(stored.Tracks[0])) {
		t.Fatal("pattern return retained previous mutes/sound/mix")
	}
	u.selectPattern(1)
	if !reflect.DeepEqual(groove.CaptureTrackState(u.session.Project.Tracks[0]), groove.CaptureTrackState(second.Tracks[0])) {
		t.Fatal("second pattern did not restore its full state")
	}
	u.toggleMute()
	if !strings.Contains(u.status.Text, "edited") || !u.session.Project.Tracks[0].Patterns[1].State.Mute {
		t.Fatal("live edit overwrote stored mute or status did not indicate it")
	}
	u.selectPattern(0)
	u.selectPattern(1)
	if !u.selectedTrack().Mute {
		t.Fatal("unstored edits survived return to saved pattern")
	}
	if w.Content().MinSize().Width > 1320 {
		t.Fatal("state controls widened main window", w.Content().MinSize())
	}
	u.duplicatePattern(false)
	if u.session.Project.Pattern != 2 || !u.selectedTrack().Mute || u.selectedTrack().Patterns[2].State == nil {
		t.Fatal("whole-groove duplicate lost saved state")
	}
}

func TestSongFollowsStoredControlsWithoutDirtyingSavedProject(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("song state")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	p := u.session.Project.Clone()
	p.BPM = 300
	p.Tracks[0].Parameters["cutoff"] = .2
	groove.StorePatternState(&p, 0, "")
	p.Tracks[0].Parameters["cutoff"], p.Tracks[0].Mute = .8, true
	groove.StorePatternState(&p, 1, "")
	groove.SelectPattern(&p, 0)
	first, second := groove.NewSongSection(0), groove.NewSongSection(1)
	first.Repeats, second.Repeats = 1, 1
	p.Song = []groove.SongSection{first, second}
	u.session = groove.NewSession(p)
	u.player.Publish(p)
	w.SetContent(u.content())
	u.player.SetSongMode(true)
	u.player.SetPlaying(true)
	var out [groove.BlockSize * 2]float32
	render := func(frames int) {
		for frames > 0 {
			count := min(groove.BlockSize, frames)
			u.player.Render(out[:count*2])
			frames -= count
		}
		u.updatePlaybackLabel()
	}
	render(1)
	render(groove.SampleRate * 240 / 300)
	if u.session.Project.Pattern != 1 || u.selectedTrack().Parameters["cutoff"] != .8 || !u.selectedTrack().Mute || u.session.Dirty() {
		t.Fatal("song controls did not follow recalled state cleanly")
	}
	if u.session.Undo() {
		t.Fatal("song playback added undo entries")
	}
	u.edit("Unsaved note", func(p *groove.Project) error { p.Tracks[0].Patterns[0].Steps[0].Note = 70; return nil })
	render(groove.SampleRate * 240 / 300)
	if u.session.Project.Pattern != 0 || u.selectedTrack().Parameters["cutoff"] != .2 || !u.session.Dirty() || u.selectedTrack().Patterns[0].Steps[0].Note != 70 {
		t.Fatal("following song discarded actual edits or failed to recall controls")
	}
}
