package desktop

import (
	"math/rand/v2"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// startupProject makes the groove shown at launch and by New groove
// (Ctrl+N), with a line for the status bar. Tests replace it (see TestMain) with
// the fixed classic layout so they do not depend on random sounds or FX.
var startupProject = func() (groove.Project, string) {
	return groove.RandomGroove(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
}

// newGrooveItem is the ≡ menu's "start again" entry: a fresh random groove,
// as at launch.
func (u *UI) newGrooveItem() *fyne.MenuItem {
	item := fyne.NewMenuItem("New groove", u.startRandomGroove)
	item.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierControl}
	return item
}

// templateItems lists the New from template submenu.
func (u *UI) templateItems() []*fyne.MenuItem {
	var items []*fyne.MenuItem
	for _, template := range groove.Templates {
		template := template
		items = append(items, fyne.NewMenuItem(string(template), func() { u.startTemplate(template) }))
	}
	return items
}
func (u *UI) startTemplate(template groove.Template) {
	u.finishLiveSlider()
	u.discardConfirm(func() {
		p, err := groove.NewFromTemplate(template)
		if err != nil {
			u.showError(err)
			return
		}
		u.replaceProject(p, "")
	})
}

// startRandomGroove replaces the project with a new groove in a random style,
// as if the app had just started (New groove, Ctrl+N). Ignored while a card
// is open, so the shortcut cannot stack a second confirmation.
func (u *UI) startRandomGroove() {
	if len(u.modalOrder) > 0 {
		return
	}
	u.finishLiveSlider()
	u.discardConfirm(func() {
		p, info := startupProject()
		u.replaceProject(p, info)
		u.recordGroove(info)
	})
}

// replaceProject swaps in a new project with the editor and playback reset
// as at launch: stopped, pattern mode, Jam off, first track selected.
func (u *UI) replaceProject(p groove.Project, info string) {
	u.player.SetPlaying(false)
	u.player.SetSongMode(false)
	u.player.StopAudition()
	u.player.ClearPerformanceTake()
	u.session = groove.NewSession(p)
	u.session.LastAction = info
	u.path, u.regenerated = "", false
	u.selected = ""
	if len(p.Tracks) > 0 {
		u.selected = p.Tracks[0].ID
	}
	u.selectedStep, u.page = 0, 0
	u.sampleViews = map[string]sampleView{}
	u.grid.drag, u.roll.drag = nil, nil
	u.jam.on, u.jam.next = false, 0
	u.updateJam()
	u.player.Publish(p)
	u.refresh()
}

// randomiseGroove turns the selected pattern into a fresh groove, as if the
// app had just started, keeping the project's tracks, other patterns, mixer
// and song. It is one undoable edit; a seeded preview names it first.
func (u *UI) randomiseGroove() { u.randomiseParts(groove.AllParts) }

var partLabels = map[groove.GroovePart]string{groove.AllParts: "Randomise all", groove.DrumParts: "Randomise drums", groove.SynthParts: "Randomise 303s"}

// randomiseParts rerolls the tracks part selects, leaving held tracks (and,
// for drums or 303s only, the other group) as they are.
func (u *UI) randomiseParts(part groove.GroovePart) {
	u.finishLiveSlider()
	if groove.RerolledTracks(u.session.Project, part) == 0 {
		msg := "Every track is held: press H on a track to release it"
		if part != groove.AllParts {
			msg = "No unheld tracks of that kind to randomise"
		}
		u.status.SetText(msg)
		return
	}
	seed1, seed2 := rand.Uint64(), rand.Uint64()
	preview := u.session.Project.Clone()
	info := groove.RandomiseGrooveParts(&preview, rand.New(rand.NewPCG(seed1, seed2)), part)
	u.edit(partLabels[part]+" · "+info, func(p *groove.Project) error {
		groove.RandomiseGrooveParts(p, rand.New(rand.NewPCG(seed1, seed2)), part)
		return nil
	})
	u.regenerated = true
	if strings.HasPrefix(u.session.LastAction, partLabels[part]+" · ") {
		u.recordGroove(u.session.LastAction)
	}
}

// randomiseItems lists the randomise actions for the ▾ and ≡ menus.
func (u *UI) randomiseItems() []*fyne.MenuItem {
	all := fyne.NewMenuItem("Randomise all", u.randomiseGroove)
	all.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl}
	drums := fyne.NewMenuItem("Drums only", func() { u.randomiseParts(groove.DrumParts) })
	drums.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}
	synths := fyne.NewMenuItem("303s only", func() { u.randomiseParts(groove.SynthParts) })
	synths.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl | fyne.KeyModifierAlt}
	history := fyne.NewMenuItem("Groove history…", u.showHistory)
	history.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyH, Modifier: fyne.KeyModifierControl}
	return []*fyne.MenuItem{all, drums, synths, fyne.NewMenuItemSeparator(), history}
}

func (u *UI) showRandomiseMenu(button fyne.CanvasObject) {
	pop := widget.NewPopUpMenu(fyne.NewMenu("Randomise", u.randomiseItems()...), u.window.Canvas())
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(button).Add(fyne.NewPos(button.Size().Width-pop.MinSize().Width, button.Size().Height))
	pop.ShowAtPosition(fyne.NewPos(max(0, pos.X), pos.Y))
}

// fitPart rewrites the selected track's pattern as a part that suits the
// groove (G, or the Fit groove button). A seeded preview names it first, as
// for randomise, so the status line says what was written.
func (u *UI) fitPart() {
	id := u.selected
	if u.selectedTrack() == nil {
		return
	}
	u.finishLiveSlider()
	seed1, seed2 := rand.Uint64(), rand.Uint64()
	preview := u.session.Project.Clone()
	info := groove.FitPart(&preview, preview.Track(id), preview.Pattern, rand.New(rand.NewPCG(seed1, seed2)))
	u.edit(info, func(p *groove.Project) error {
		groove.FitPart(p, p.Track(id), p.Pattern, rand.New(rand.NewPCG(seed1, seed2)))
		return nil
	})
}

// addTrackOf adds a track set up to join the groove: its length, and in a
// styled groove the style's sound for the part.
func (u *UI) addTrackOf(kind groove.Kind) {
	u.finishLiveSlider()
	seed1, seed2 := rand.Uint64(), rand.Uint64()
	preview := u.session.Project.Clone()
	_, info := groove.AddTrack(&preview, kind, rand.New(rand.NewPCG(seed1, seed2)))
	u.edit(info, func(p *groove.Project) error {
		i, _ := groove.AddTrack(p, kind, rand.New(rand.NewPCG(seed1, seed2)))
		u.selected = p.Tracks[i].ID
		return nil
	})
}
