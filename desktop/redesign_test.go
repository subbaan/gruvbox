package desktop

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
)

func redesignUI(t *testing.T) (*UI, fyne.Window) {
	t.Helper()
	a := test.NewApp()
	t.Cleanup(a.Quit)
	w := a.NewWindow("redesign")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	w.Resize(fyne.NewSize(1320, 780))
	return u, w
}

func TestPatternPadsAndNumberKeys(t *testing.T) {
	u, _ := redesignUI(t)
	if len(u.patternPads) != groove.BankSize {
		t.Fatalf("expected %d pattern pads, got %d", groove.BankSize, len(u.patternPads))
	}
	if u.patternPads[0].Importance != widget.HighImportance || u.patternPads[1].Importance != widget.LowImportance {
		t.Fatal("pads do not distinguish current and empty patterns")
	}
	test.Tap(u.patternPads[2])
	if u.session.Project.Pattern != 2 || u.patternPads[2].Importance != widget.HighImportance {
		t.Fatal("pad tap did not select pattern 3")
	}
	u.key(fyne.Key1)
	if u.session.Project.Pattern != 0 {
		t.Fatal("key 1 did not select pattern 1")
	}
	// Patterns with content stay at normal importance when not selected.
	u.key(fyne.Key2)
	if u.patternPads[0].Importance != widget.MediumImportance {
		t.Fatal("filled pattern pad not shown as filled")
	}
	// Alt+3 shows bank C without changing the pattern; 4 then picks C4.
	u.showBank(2)
	if u.bankButton.Text != "C" || u.session.Project.Pattern != 1 || u.patternPads[1].Importance == widget.HighImportance {
		t.Fatal("showing a bank changed the pattern or kept its pad lit")
	}
	u.key(fyne.Key4)
	if u.session.Project.Pattern != 19 || u.patternPads[3].Importance != widget.HighImportance {
		t.Fatalf("key 4 in bank C picked %s", groove.SlotName(u.session.Project.Pattern))
	}
	// Choosing a pattern elsewhere (song, history) moves the view to it.
	u.edit("Elsewhere", func(p *groove.Project) error { return groove.SelectPattern(p, 41) })
	if u.bankButton.Text != "F" || u.patternPads[1].Importance != widget.HighImportance {
		t.Fatalf("view did not follow to F2: bank %s", u.bankButton.Text)
	}
	// Variation stays in its bank: F8 wraps to F1.
	u.edit("To F8", func(p *groove.Project) error { return groove.SelectPattern(p, 47) })
	u.generate("Variation")
	if u.session.Project.Pattern != 40 {
		t.Fatalf("variation from F8 went to %s", groove.SlotName(u.session.Project.Pattern))
	}
}

func TestStepPagesFollowPatternLength(t *testing.T) {
	u, _ := redesignUI(t)
	if u.pageCount != 1 || u.pageButtons[1].Visible() {
		t.Fatal("16-step project should show a single page")
	}
	id := u.selected
	u.edit("Pattern length", func(p *groove.Project) error {
		pattern := &p.Track(id).Patterns[p.Pattern]
		for len(pattern.Steps) < 40 {
			pattern.Steps = append(pattern.Steps, groove.DefaultStep())
		}
		return nil
	})
	if u.pageCount != 3 || !u.pageButtons[2].Visible() || u.pageButtons[3].Visible() {
		t.Fatalf("40 steps should show 3 pages, got %d", u.pageCount)
	}
	u.key(fyne.KeyRightBracket)
	u.key(fyne.KeyRightBracket)
	u.key(fyne.KeyRightBracket)
	if u.page != 2 || u.pageButtons[2].Importance != widget.HighImportance {
		t.Fatal("] did not advance and clamp the step page")
	}
	u.key(fyne.KeyLeftBracket)
	if u.page != 1 {
		t.Fatal("[ did not go back a page")
	}
	u.undo()
	if u.page != 0 || u.pageCount != 1 {
		t.Fatal("page was not clamped after the pattern shrank")
	}
}

func TestSaveWritesKnownPathWithoutPrompt(t *testing.T) {
	u, _ := redesignUI(t)
	path := filepath.Join(t.TempDir(), "groove.gruvbox3")
	u.path = path
	u.toggleMute()
	u.save()
	if u.modals["Save groove as"] != nil {
		t.Fatal("save prompted for a path that is already known")
	}
	deadline := time.Now().Add(3 * time.Second)
	for u.session.Dirty() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(path); err != nil || u.session.Dirty() {
		t.Fatalf("save did not write the known path: %v", err)
	}
	u.path = ""
	u.save()
	if u.modals["Save groove as"] == nil {
		t.Fatal("save of an unnamed project did not ask for a path")
	}
}

func TestSinglePlayToggle(t *testing.T) {
	u, _ := redesignUI(t)
	stops := 0
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		if b, ok := obj.(*widget.Button); ok && b.Text == "Stop" {
			stops++
		}
		if c, ok := obj.(*fyne.Container); ok {
			for _, child := range c.Objects {
				walk(child)
			}
		}
	}
	u.player.SetPlaying(true)
	u.refresh()
	walk(u.window.Content())
	u.player.SetPlaying(false)
	if stops != 1 {
		t.Fatalf("expected one Stop control while playing, found %d", stops)
	}
}

// TestScreenshots renders the main window with Fyne's software painter.
// Run with GRUVBOX_SHOTS=<dir> go test ./desktop -run TestScreenshots to
// inspect the UI without opening a window on the real display.
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("GRUVBOX_SHOTS")
	if dir == "" {
		t.Skip("set GRUVBOX_SHOTS to a directory to write UI screenshots")
	}
	u, w := redesignUI(t)
	u.applyScheme(theme.Bundled()["gruvbox-dark"])
	shot := func(name string) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, w.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	shot("main")
	u.inspectorTabs.SelectIndex(1)
	shot("step")
	u.inspectorTabs.SelectIndex(2)
	shot("seq")
	u.inspectorTabs.SelectIndex(0)
	u.showHelp(u.play, shortcutHelp)
	shot("shortcuts")
	w.Canvas().Overlays().Remove(w.Canvas().Overlays().Top())
	u.showMainMenu()
	shot("menu")
	w.Canvas().Overlays().Remove(w.Canvas().Overlays().Top())
	u.showAbout()
	shot("about")
	u.modals[aboutTitle].Close()
	u.showTrackMenu(u.session.Project.Tracks[1].ID, fyne.NewPos(40, 140))
	shot("trackmenu")
	w.Canvas().Overlays().Remove(w.Canvas().Overlays().Top())
	u.editorTabs.SelectIndex(3)
	shot("mixer")
	u.editorTabs.SelectIndex(0)
	u.setLockMode(true)
	u.selectedStep = 2
	u.setLock(u.selected, "cutoff", .2)
	u.setLock(u.selected, "decay", .8)
	shot("lock")
	u.inspectorTabs.SelectIndex(1)
	shot("steplocks")
	u.inspectorTabs.SelectIndex(0)
	u.setLockMode(false)
	u.euclideanGenerator()
	shot("drawer-euclid")
	u.soundPresets()
	shot("drawer-presets")
	u.phraseGenerator()
	shot("drawer-phrase")
	u.activeDrawer.Close()
	u.showError(errors.New("could not write /nowhere/groove.gruvbox3: permission denied"))
	u.toggleMute()
	u.discardConfirm(func() {})
	shot("confirm")
	u.modals["Unsaved groove"].Close()
	u.hideToast()
	u.player.Publish(u.session.Project)
	u.player.SetPlaying(true)
	var block [735 * 2]float32
	for i := 0; i < 20; i++ {
		u.player.Render(block[:])
	}
	u.updatePageButtons()
	u.grid.advancePlayhead()
	shot("playing")
	u.player.SetPlaying(false)
	u.editorTabs.SelectIndex(1)
	shot("roll")
	u.edit("Song", func(p *groove.Project) error {
		for i, bars := range []int{4, 8, 2, 8} {
			s := groove.NewSongSection(i % 3)
			s.Repeats = bars
			s.Name = []string{"Intro", "Drop", "Break", "Outro"}[i]
			p.Song = append(p.Song, s)
		}
		return nil
	})
	u.editorTabs.SelectIndex(2)
	u.selectSection(u.session.Project.Song[1].ID)
	shot("song")
	u.growTrack()
	shot("grow-card")
	u.modals["Grow into a track"].finish(true)
	shot("grow-song")
	u.editorTabs.SelectIndex(4)
	u.setJam(true)
	shot("jam")
	u.setJam(false)
	u.editorTabs.SelectIndex(0)
	u.randomiseGroove()
	u.randomiseParts(groove.DrumParts)
	u.showHistory()
	u.toggleFavourite(u.history.recent[1])
	u.fillHistory(u.activeDrawer)
	shot("history")
	u.activeDrawer.Close()
	u.editorTabs.SelectIndex(0)
	u.inspectorTabs.SelectIndex(4)
	u.addEffect(groove.Delay)
	u.addEffect(groove.Reverb)
	u.addEffect(groove.Chorus)
	u.fxFolded[u.selectedTrack().Effects[1].ID] = true
	u.inspectorKey = ""
	u.refresh()
	shot("fx")
	u.applyPedalboard("Squelch wah")
	u.addEffect(groove.Phaser)
	u.inspectorKey = ""
	u.refresh()
	shot("pedals")
	u.inspectorTabs.SelectIndex(0)
	saved := startupProject
	startupProject = randomStartup
	for n := 1; n <= 3; n++ {
		u.session.MarkSaved()
		u.startRandomGroove()
		shot(fmt.Sprintf("random-groove-%d", n))
	}
	startupProject = saved
}
