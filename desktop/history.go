package desktop

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// historySize is how many recent grooves are kept in this run.
const historySize = 20

const historyTitle = "Groove history"

// grooveHistory holds recent randomise results (this run only) and the
// favourites file, read the first time it is needed and written only when a
// favourite is added or removed.
type grooveHistory struct {
	recent     []groove.GrooveSnapshot // oldest first
	favourites []groove.GrooveSnapshot
	loaded     bool
}

func (u *UI) favouritesPath() string {
	return filepath.Join(filepath.Dir(u.library.Dir), "favourite-grooves.json")
}

// recordGroove adds the selected pattern's groove to the recent list.
func (u *UI) recordGroove(label string) {
	if label == "" {
		label = u.plainGrooveLabel()
	}
	u.history.recent = append(u.history.recent, groove.CaptureGroove(u.session.Project, label))
	if len(u.history.recent) > historySize {
		u.history.recent = u.history.recent[len(u.history.recent)-historySize:]
	}
	if d := u.drawerTitled(historyTitle); d != nil {
		u.fillHistory(d)
	}
}

func (u *UI) loadFavourites() {
	if u.history.loaded {
		return
	}
	list, err := groove.LoadFavourites(u.favouritesPath())
	if err != nil {
		u.showError(err)
		return
	}
	u.history.favourites, u.history.loaded = list, true
}

func (u *UI) saveFavourites() {
	if err := groove.SaveFavourites(u.favouritesPath(), u.history.favourites); err != nil {
		u.showError(err)
	}
}

func (u *UI) isFavourite(id string) bool {
	for _, s := range u.history.favourites {
		if s.ID == id {
			return true
		}
	}
	return false
}

func (u *UI) toggleFavourite(s groove.GrooveSnapshot) {
	u.loadFavourites()
	for i, f := range u.history.favourites {
		if f.ID == s.ID {
			u.history.favourites = append(u.history.favourites[:i], u.history.favourites[i+1:]...)
			u.saveFavourites()
			u.status.SetText("Removed from favourites")
			return
		}
	}
	u.history.favourites = append(u.history.favourites, s)
	u.saveFavourites()
	u.status.SetText("Kept in favourites: " + shortGrooveLabel(s.Label))
}

// keepCurrentGroove stars the selected pattern as it is now.
func (u *UI) keepCurrentGroove() {
	label := u.session.LastAction
	if !strings.Contains(label, " groove · ") {
		label = u.plainGrooveLabel()
	}
	u.toggleFavourite(groove.CaptureGroove(u.session.Project, label))
}

// plainGrooveLabel names the selected pattern by number, tempo and key.
func (u *UI) plainGrooveLabel() string {
	p := u.session.Project
	label := fmt.Sprintf("Pattern %s · %.0f BPM", groove.SlotName(p.Pattern), p.BPM)
	if p.Key != nil {
		label += " · " + p.Key.Name()
	}
	return label
}

func (u *UI) restoreGroove(s groove.GrooveSnapshot) {
	u.finishLiveSlider()
	preview := u.session.Project.Clone()
	matched, err := groove.RestoreGroove(&preview, s)
	if err != nil {
		u.showError(err)
		return
	}
	label := "Restore groove · " + shortGrooveLabel(s.Label)
	if matched < len(s.Parts) {
		label += fmt.Sprintf(" (%d of %d parts matched)", matched, len(s.Parts))
	}
	u.edit(label, func(p *groove.Project) error {
		_, err := groove.RestoreGroove(p, s)
		return err
	})
}

// shortGrooveLabel drops "Randomise all · " and shortens the part rerolls
// to "Drums: " and "303s: ", which matter when telling entries apart.
func shortGrooveLabel(label string) string {
	label = strings.TrimPrefix(label, "Randomise all · ")
	label = strings.Replace(label, "Randomise drums · ", "Drums: ", 1)
	return strings.Replace(label, "Randomise 303s · ", "303s: ", 1)
}

func (u *UI) showHistory() {
	u.loadFavourites()
	d, fresh := u.openDrawer(historyTitle, nil)
	if fresh {
		u.fillHistory(d)
	}
}

func (u *UI) fillHistory(d *drawer) {
	row := func(s groove.GrooveSnapshot, action fyne.CanvasObject) fyne.CanvasObject {
		label := widget.NewLabel(shortGrooveLabel(s.Label))
		label.Truncation = fyne.TextTruncateEllipsis
		when := widget.NewLabel(s.Saved.Format("15:04"))
		if time.Since(s.Saved) > 20*time.Hour {
			when.SetText(s.Saved.Format("2 Jan"))
		}
		when.Importance = widget.LowImportance
		restore := widget.NewButton("Restore", func() { u.restoreGroove(s) })
		return container.NewBorder(nil, nil, when, container.NewHBox(restore, action), label)
	}
	list := container.NewVBox()
	keep := widget.NewButtonWithIcon("Keep this pattern's groove", fynetheme.ContentAddIcon(), func() { u.keepCurrentGroove(); u.fillHistory(d) })
	help := u.helpButton("Recent lists the last 20 grooves made by Randomise (all, drums or 303s) in this run, newest first. Restore puts one back in the selected pattern as an undoable edit: tempo, key, notes and sounds. Held tracks stay as they are.\n\nStar a groove (☆) to keep it in Favourites, which are saved and available in any project: parts go to tracks of the same kind, in order.")
	list.Add(container.NewBorder(nil, nil, nil, help, container.NewHBox(keep)))
	list.Add(widget.NewLabelWithStyle("Recent", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if len(u.history.recent) == 0 {
		list.Add(widget.NewLabel("Nothing yet: Randomise all (Ctrl+R) adds each groove here."))
	}
	for i := len(u.history.recent) - 1; i >= 0; i-- {
		s := u.history.recent[i]
		star := widget.NewButton("☆", nil)
		if u.isFavourite(s.ID) {
			star.SetText("★")
			star.Importance = widget.HighImportance
		}
		star.OnTapped = func() { u.toggleFavourite(s); u.fillHistory(d) }
		list.Add(row(s, star))
	}
	list.Add(widget.NewSeparator())
	list.Add(widget.NewLabelWithStyle("Favourites", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if len(u.history.favourites) == 0 {
		list.Add(widget.NewLabel("Star a recent groove, or keep this pattern's, to save it here."))
	}
	for i := len(u.history.favourites) - 1; i >= 0; i-- {
		s := u.history.favourites[i]
		remove := widget.NewButtonWithIcon("", fynetheme.DeleteIcon(), func() { u.toggleFavourite(s); u.fillHistory(d) })
		remove.Importance = widget.LowImportance
		list.Add(row(s, remove))
	}
	d.SetContent(container.NewVScroll(list))
}
