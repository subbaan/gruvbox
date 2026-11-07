package desktop

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/subbaan/gruvbox/groove"
)

// shortcut sends a shortcut the way the desktop driver does when nothing
// focused handles it.
func shortcut(t *testing.T, w fyne.Window, s fyne.Shortcut) {
	t.Helper()
	c, ok := w.Canvas().(interface{ TypedShortcut(fyne.Shortcut) })
	if !ok {
		t.Fatal("canvas cannot take shortcuts")
	}
	c.TypedShortcut(s)
}

func TestStandardUndoShortcutsReachTheGroove(t *testing.T) {
	u, w := redesignUI(t)
	muted := u.selectedTrack().Mute
	u.toggleMute()
	shortcut(t, w, &fyne.ShortcutUndo{})
	if u.selectedTrack().Mute != muted {
		t.Fatal("Ctrl+Z (ShortcutUndo) did not undo")
	}
	shortcut(t, w, &fyne.ShortcutRedo{})
	if u.selectedTrack().Mute == muted {
		t.Fatal("Ctrl+Y (ShortcutRedo) did not redo")
	}
}

func TestCopyPasteStepsAndPatterns(t *testing.T) {
	u, w := redesignUI(t)
	id := u.selected
	u.edit("Source", func(p *groove.Project) error {
		s := &p.Track(id).Patterns[p.Pattern].Steps[0]
		*s = groove.Step{Active: true, Note: 50, Velocity: .4, Accent: true, Probability: 1, Locks: map[string]float64{"cutoff": .3}}
		return nil
	})
	u.selectedStep = 0
	shortcut(t, w, &fyne.ShortcutCopy{})
	u.selectedStep = 5
	shortcut(t, w, &fyne.ShortcutPaste{})
	shortcut(t, w, &fyne.ShortcutPaste{})
	steps := u.selectedTrack().Patterns[u.session.Project.Pattern].Steps
	for _, i := range []int{5, 6} {
		if !steps[i].Active || steps[i].Note != 50 || !steps[i].Accent || steps[i].Locks["cutoff"] != .3 {
			t.Fatalf("step %d not pasted: %+v", i, steps[i])
		}
	}
	foreign := groove.Step{Locks: map[string]float64{"cutoff": .3, groove.EffectLockID("missing", "mix"): .5}}
	keepLocks(u.selectedTrack(), &foreign)
	if len(foreign.Locks) != 1 || foreign.Locks["cutoff"] != .3 {
		t.Fatalf("keepLocks kept a lock the track cannot use: %v", foreign.Locks)
	}
	if u.selectedStep != 7 {
		t.Fatal("paste did not move to the next step")
	}
	// Pasted locks are independent copies.
	u.edit("Change", func(p *groove.Project) error {
		p.Track(id).Patterns[p.Pattern].Steps[5].Locks["cutoff"] = .9
		return nil
	})
	if u.selectedTrack().Patterns[u.session.Project.Pattern].Steps[6].Locks["cutoff"] != .3 {
		t.Fatal("pasted steps share a locks map")
	}
	// Whole pattern to another track.
	shortcut(t, w, &desktop.CustomShortcut{KeyName: fyne.KeyC, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift})
	source := u.selectedTrack().Patterns[u.session.Project.Pattern]
	other := u.session.Project.Tracks[1].ID
	if other == id {
		other = u.session.Project.Tracks[0].ID
	}
	u.selectTrack(other)
	shortcut(t, w, &desktop.CustomShortcut{KeyName: fyne.KeyV, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift})
	got := u.selectedTrack().Patterns[u.session.Project.Pattern]
	if len(got.Steps) != len(source.Steps) || got.Steps[5].Note != 50 || got.Resolution != source.Resolution {
		t.Fatal("pattern paste did not copy steps")
	}
	u.undo()
	if u.selectedTrack().Patterns[u.session.Project.Pattern].Steps[5].Note == 50 && !source.Steps[5].Active {
		t.Fatal("pattern paste was not one undo step")
	}
}

func TestVelocityKeysAndTabCycling(t *testing.T) {
	u, w := redesignUI(t)
	u.selectedStep = 0
	start := u.currentStep(u.selected).Velocity
	u.key(fyne.KeyComma)
	if got := u.currentStep(u.selected).Velocity; got >= start && start > .1 {
		t.Fatalf("comma did not lower velocity: %v -> %v", start, got)
	}
	for i := 0; i < 15; i++ {
		u.key(fyne.KeyPeriod)
	}
	if u.currentStep(u.selected).Velocity != 1 {
		t.Fatal("velocity not clamped at 1")
	}
	for i := 0; i < 15; i++ {
		u.key(fyne.KeyComma)
	}
	if u.currentStep(u.selected).Velocity != .1 {
		t.Fatal("velocity not clamped at 0.1")
	}
	shortcut(t, w, &desktop.CustomShortcut{KeyName: fyne.KeyTab, Modifier: fyne.KeyModifierControl})
	if u.editorTabs.SelectedIndex() != 1 {
		t.Fatal("Ctrl+Tab did not move to the next editor tab")
	}
	shortcut(t, w, &desktop.CustomShortcut{KeyName: fyne.KeyTab, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift})
	shortcut(t, w, &desktop.CustomShortcut{KeyName: fyne.KeyTab, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift})
	if u.editorTabs.SelectedIndex() != len(u.editorTabs.Items)-1 {
		t.Fatal("Ctrl+Shift+Tab did not wrap to the last tab")
	}
}

func TestRandomGrooveMenu(t *testing.T) {
	u, _ := redesignUI(t)
	saved := startupProject
	startupProject = randomStartup
	defer func() { startupProject = saved }()
	u.startRandomGroove()
	p := u.session.Project
	if p.Key == nil || len(p.Tracks) < 6 || u.session.Dirty() {
		t.Fatal("random groove not loaded cleanly")
	}
	if u.session.LastAction == "" || !strings.Contains(u.status.Text, "groove ·") {
		t.Fatalf("status does not describe the groove: %q", u.status.Text)
	}
	items := u.templateItems()
	if items[0].Label != "Random groove" {
		t.Fatal("New menu does not offer Random groove first")
	}
}

func TestPhraseDrawerStartsInProjectKey(t *testing.T) {
	u, _ := redesignUI(t)
	key := groove.Key{Root: 9, Scale: groove.PhraseDorian}
	u.session.Project.Key = &key
	u.phraseGenerator()
	c := findGeneratorControls(u.drawerTitled("Phrase generator"))
	var root, scale bool
	for _, s := range c.selects {
		if s.Selected == "A" {
			root = true
		}
		if s.Selected == string(groove.PhraseDorian) {
			scale = true
		}
	}
	if !root || !scale {
		t.Fatal("phrase drawer did not start in the project's key")
	}
}

func TestRandomiseAllRebuildsPatternAndUndoes(t *testing.T) {
	u, _ := redesignUI(t)
	u.selectPattern(1)
	before := u.session.Project.Clone()
	u.randomiseGroove()
	p := u.session.Project
	if !strings.HasPrefix(u.session.LastAction, "Randomise all · ") || !strings.Contains(u.session.LastAction, "BPM") {
		t.Fatalf("status %q", u.session.LastAction)
	}
	if u.bpm.Text != fmt.Sprintf("%.0f", p.BPM) {
		t.Fatalf("tempo field shows %q for %v BPM", u.bpm.Text, p.BPM)
	}
	active := 0
	for i := range p.Tracks {
		if !reflect.DeepEqual(p.Tracks[i].Patterns[0], before.Tracks[i].Patterns[0]) {
			t.Fatal("randomise all changed another pattern")
		}
		for _, s := range p.Tracks[i].Patterns[1].Steps {
			if s.Active {
				active++
			}
		}
	}
	if active == 0 {
		t.Fatal("selected pattern still empty")
	}
	u.undo()
	if !reflect.DeepEqual(before, u.session.Project) {
		t.Fatal("one undo should restore everything")
	}
}

func TestHeldTracksAndPartRandomise(t *testing.T) {
	u, _ := redesignUI(t)
	held := u.session.Project.Tracks[0].ID
	u.selected = held
	u.key(fyne.KeyH)
	if !u.session.Project.Track(held).Hold {
		t.Fatal("H did not hold the track")
	}
	before := u.session.Project.Clone()
	u.randomiseGroove()
	if !reflect.DeepEqual(*before.Track(held), *u.session.Project.Track(held)) {
		t.Fatal("randomise all changed a held track")
	}
	if !strings.Contains(u.session.LastAction, "held: ") {
		t.Fatalf("status %q", u.session.LastAction)
	}
	before = u.session.Project.Clone()
	u.randomiseParts(groove.DrumParts)
	if !strings.HasPrefix(u.session.LastAction, "Randomise drums · ") {
		t.Fatalf("status %q", u.session.LastAction)
	}
	for i, track := range u.session.Project.Tracks {
		if track.Kind == groove.Acid && !reflect.DeepEqual(track, before.Tracks[i]) {
			t.Fatal("drums only changed a 303")
		}
	}
	for i := range u.session.Project.Tracks {
		u.session.Project.Tracks[i].Hold = true
	}
	last := u.session.LastAction
	u.randomiseGroove()
	if u.session.LastAction != last || !strings.Contains(u.status.Text, "held") {
		t.Fatalf("all held should only explain: %q", u.status.Text)
	}
}

func TestGrowIntoATrackBuildsTheSongAndUndoes(t *testing.T) {
	u, _ := redesignUI(t)
	u.growTrack()
	card := u.modals["Grow into a track"]
	if card == nil {
		t.Fatal("no confirmation card")
	}
	before := u.session.Project.Clone()
	card.finish(true)
	p := u.session.Project
	if !strings.HasPrefix(u.session.LastAction, "Grow into a track · 10 sections") {
		t.Fatalf("status %q", u.session.LastAction)
	}
	if len(p.Song) != 10 || !u.player.SongMode() {
		t.Fatalf("song %d sections, song mode %v", len(p.Song), u.player.SongMode())
	}
	if u.editorTabs.Selected().Text != "Song" || u.selectedSection != p.Song[0].ID {
		t.Fatal("song tab not shown")
	}
	for slot := 0; slot < groove.PatternCount; slot++ {
		if groove.PatternEmpty(p, slot) == (slot < groove.BankSize) {
			t.Fatalf("pattern %s: Grow should fill bank A only", groove.SlotName(slot))
		}
	}
	// A second grow says what it will replace.
	u.growTrack()
	if u.modals["Grow into a track"] == nil {
		t.Fatal("no card")
	}
	u.modals["Grow into a track"].Close()
	u.undo()
	if !reflect.DeepEqual(before, u.session.Project) {
		t.Fatal("one undo should restore everything")
	}
	// Nothing to grow from an empty pattern.
	u.selectPattern(7)
	u.growTrack()
	if u.modals["Grow into a track"] != nil || !strings.Contains(u.status.Text, "first") {
		t.Fatalf("empty pattern: %q", u.status.Text)
	}
}

func TestGrooveHistoryRestoresAndKeepsFavourites(t *testing.T) {
	u, w := redesignUI(t)
	first := u.session.Project.Clone()
	u.randomiseGroove()
	u.randomiseParts(groove.DrumParts)
	if len(u.history.recent) != 3 {
		t.Fatalf("%d recent grooves", len(u.history.recent))
	}
	u.showHistory()
	if u.drawerTitled(historyTitle) == nil {
		t.Fatal("drawer not open")
	}
	u.restoreGroove(u.history.recent[0])
	if !strings.HasPrefix(u.session.LastAction, "Restore groove · ") {
		t.Fatalf("status %q", u.session.LastAction)
	}
	for i := range first.Tracks {
		if !reflect.DeepEqual(first.Tracks[i].Patterns[0].Steps, u.session.Project.Tracks[i].Patterns[0].Steps) {
			t.Fatal("startup groove not restored")
		}
	}
	u.toggleFavourite(u.history.recent[1])
	u.keepCurrentGroove()
	if len(u.history.favourites) != 2 {
		t.Fatalf("%d favourites", len(u.history.favourites))
	}
	// A new window on the same data root finds them.
	w2 := test.NewApp().NewWindow("again")
	u2 := newUI(fyne.CurrentApp(), w2, u.library)
	u2.loadFavourites()
	if len(u2.history.favourites) != 2 || u2.history.favourites[0].ID != u.history.recent[1].ID {
		t.Fatal("favourites not saved")
	}
	u.toggleFavourite(u.history.favourites[0])
	if len(u.history.favourites) != 1 {
		t.Fatal("unstar failed")
	}
	_ = w
}
