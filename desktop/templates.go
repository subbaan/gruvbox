package desktop

import (
	"math/rand/v2"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// startupProject makes the groove shown at launch and by New ▸ Random
// groove, with a line for the status bar. Tests replace it (see TestMain) with
// the fixed classic layout so they do not depend on random sounds or FX.
var startupProject = func() (groove.Project, string) {
	return groove.RandomGroove(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
}

func (u *UI) templateItems() []*fyne.MenuItem {
	items := []*fyne.MenuItem{fyne.NewMenuItem("Random groove", u.startRandomGroove), fyne.NewMenuItemSeparator()}
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

// startRandomGroove replaces the project with a new groove in a random style.
func (u *UI) startRandomGroove() {
	u.finishLiveSlider()
	u.discardConfirm(func() {
		p, info := startupProject()
		u.replaceProject(p, info)
		u.recordGroove(info)
	})
}

func (u *UI) replaceProject(p groove.Project, info string) {
	u.player.SetPlaying(false)
	u.player.SetSongMode(false)
	u.player.StopAudition()
	u.player.ClearPerformanceTake()
	u.session = groove.NewSession(p)
	u.session.LastAction = info
	u.path = ""
	u.selected = ""
	if len(p.Tracks) > 0 {
		u.selected = p.Tracks[0].ID
	}
	u.selectedStep, u.page = 0, 0
	u.sampleViews = map[string]sampleView{}
	u.grid.drag, u.roll.drag = nil, nil
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
