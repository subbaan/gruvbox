package desktop

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// showTrackMenu is the right-click menu on a track name in the step grid.
func (u *UI) showTrackMenu(id string, at fyne.Position) {
	t := u.session.Project.Track(id)
	if t == nil {
		return
	}
	u.selected = id
	u.refresh()
	index := 0
	for i, track := range u.session.Project.Tracks {
		if track.ID == id {
			index = i
		}
	}
	up := fyne.NewMenuItem("Move up", func() { u.moveTrack(-1) })
	down := fyne.NewMenuItem("Move down", func() { u.moveTrack(1) })
	up.Disabled, down.Disabled = index == 0, index == len(u.session.Project.Tracks)-1
	colours := fyne.NewMenuItem("Colour", nil)
	colours.ChildMenu = fyne.NewMenu("Colour")
	roles := u.scheme.Roles()
	for slot, hex := range roles.Tracks {
		slot := slot
		item := fyne.NewMenuItem(fmt.Sprintf("Colour %d", slot+1), func() {
			u.edit("Track colour", func(p *groove.Project) error {
				if t := p.Track(id); t != nil {
					t.Colour = slot
				}
				return nil
			})
		})
		item.Icon = swatch(hex)
		item.Checked = slot == t.Colour
		colours.ChildMenu.Items = append(colours.ChildMenu.Items, item)
	}
	hold := fyne.NewMenuItem("Hold (keep when randomising)", func() { u.toggleTrackHold(id) })
	hold.Checked = t.Hold
	mute := fyne.NewMenuItem("Mute", u.toggleMute)
	mute.Checked = t.Mute
	solo := fyne.NewMenuItem("Solo", u.toggleSolo)
	solo.Checked = t.Solo
	menu := fyne.NewMenu(t.Name,
		fyne.NewMenuItem("Rename…", func() { u.renameTrack(id, at) }),
		fyne.NewMenuItem("Duplicate", u.duplicateTrack),
		colours,
		fyne.NewMenuItemSeparator(), up, down,
		fyne.NewMenuItemSeparator(), hold, mute, solo,
		fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Remove track", u.removeTrack),
	)
	widget.NewPopUpMenu(menu, u.window.Canvas()).ShowAtPosition(at)
}

// renameTrack edits a track name in a small popup; Enter or clicking away
// applies it.
func (u *UI) renameTrack(id string, at fyne.Position) {
	t := u.session.Project.Track(id)
	if t == nil {
		return
	}
	var pop *widget.PopUp
	entry := newCommitEntry(t.Name, func(name string) {
		if name == "" {
			return
		}
		u.edit("Rename track", func(p *groove.Project) error {
			if t := p.Track(id); t != nil {
				t.Name = name
			}
			return nil
		})
	})
	entry.OnSubmitted = func(string) {
		entry.commit()
		pop.Hide()
	}
	pop = widget.NewPopUp(container.NewGridWrap(fyne.NewSize(220, entry.MinSize().Height), entry), u.window.Canvas())
	pop.ShowAtPosition(at)
	u.window.Canvas().Focus(entry)
}

// swatch returns a small square icon in the given colour for menus.
func swatch(hex string) fyne.Resource {
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect x="1" y="1" width="14" height="14" rx="2" fill="%s"/></svg>`, hex)
	return fyne.NewStaticResource("swatch-"+hex[1:]+".svg", []byte(svg))
}
