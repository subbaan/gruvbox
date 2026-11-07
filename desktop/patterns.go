package desktop

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) duplicatePattern(variation bool) {
	target := groove.NextEmptyPattern(u.session.Project, u.session.Project.Pattern)
	if target < 0 {
		u.showError(fmt.Errorf("all pattern slots contain notes or locks; choose a replacement slot in Patterns"))
		return
	}
	id := u.selected
	label := "Duplicate groove"
	if variation {
		label = "Groove variation"
	}
	u.player.SetSongMode(false)
	u.selectedStep, u.page = 0, 0
	u.edit(label, func(p *groove.Project) error {
		if err := groove.CopyPattern(p, p.Pattern, target, ""); err != nil {
			return err
		}
		if variation && p.Track(id) != nil {
			groove.GenerateIn(p, p.Track(id), target, "Mutate")
		}
		return groove.SelectPattern(p, target)
	})
}
func (u *UI) patternTools(button *widget.Button) {
	all := fyne.NewMenuItem("Copy whole groove to…", nil)
	one := fyne.NewMenuItem("Copy selected track to…", nil)
	one.Disabled = u.selectedTrack() == nil
	all.ChildMenu = fyne.NewMenu("Whole groove")
	one.ChildMenu = fyne.NewMenu("Track")
	source := u.session.Project.Pattern
	id := u.selected
	for slot := 0; slot < groove.PatternCount; slot++ {
		if slot == source {
			continue
		}
		for _, pair := range []struct {
			menu  *fyne.Menu
			track string
		}{{all.ChildMenu, ""}, {one.ChildMenu, id}} {
			slot, track := slot, pair.track
			pair.menu.Items = append(pair.menu.Items, fyne.NewMenuItem(fmt.Sprintf("Replace pattern %02d", slot+1), func() {
				u.edit("Copy pattern", func(p *groove.Project) error { return groove.CopyPattern(p, source, slot, track) })
			}))
		}
	}
	euclidean := fyne.NewMenuItem("Euclidean rhythm…", u.euclideanGenerator)
	locks := fyne.NewMenuItem("Parameter-lock generator…", u.lockGenerator)
	phrase := fyne.NewMenuItem("Phrase generator…", u.phraseGenerator)
	phrase.Disabled = u.selectedTrack() == nil || !u.selectedTrack().Kind.Capabilities().Notes
	euclidean.Disabled, locks.Disabled = u.selectedTrack() == nil, u.selectedTrack() == nil
	grow := fyne.NewMenuItem("Grow into a track…", u.growTrack)
	grow.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyG, Modifier: fyne.KeyModifierControl}
	menu := fyne.NewMenu("Patterns", grow, fyne.NewMenuItemSeparator(), euclidean, locks, phrase, fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Duplicate groove → empty slot", func() { u.duplicatePattern(false) }), fyne.NewMenuItem("Variation → empty slot", func() { u.duplicatePattern(true) }), all, one)
	stored, _ := groove.PatternStateStatus(u.session.Project, source)
	recall := fyne.NewMenuItem("Recall stored state", func() {
		u.edit("Recall pattern state", func(p *groove.Project) error { return groove.RecallPatternState(p, p.Pattern, "") })
	})
	clear := fyne.NewMenuItem("Clear stored state", func() {
		u.edit("Clear pattern state", func(p *groove.Project) error { return groove.ClearPatternState(p, p.Pattern) })
	})
	recall.Disabled, clear.Disabled = stored == 0, stored == 0
	menu.Items = append(menu.Items, fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Store state · Ctrl+Shift+P", u.storePatternState), recall, clear)
	widget.NewPopUpMenu(menu, u.window.Canvas()).ShowAtPosition(fyne.CurrentApp().Driver().AbsolutePositionForObject(button).Add(fyne.NewPos(0, button.Size().Height)))
}

func (u *UI) storePatternState() {
	u.edit("Store pattern state", func(p *groove.Project) error { return groove.StorePatternState(p, p.Pattern, "") })
}
