package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// drawerOffset is the editor/side split while a drawer is open; drawers hold
// wider tools than the inspector, so the side pane grows and is restored on
// close.
const drawerOffset = .56

// A drawer is an in-window tool panel (generators, sound library, FX LFOs)
// shown in place of the inspector. Only one is open at a time: opening another
// closes the current one, which runs its closed callback so background work
// and refresh hooks are cancelled exactly as when a utility window closed.
type drawer struct {
	u      *UI
	title  string
	body   *fyne.Container
	root   fyne.CanvasObject
	closed func()
}

func (d *drawer) SetContent(content fyne.CanvasObject) {
	d.body.Objects = []fyne.CanvasObject{content}
	d.body.Refresh()
}
func (d *drawer) Content() fyne.CanvasObject {
	if len(d.body.Objects) == 0 {
		return nil
	}
	return d.body.Objects[0]
}
func (d *drawer) Close() { d.u.closeDrawer(d) }

// openDrawer shows the drawer named title, reusing it if it is already open.
// fresh reports whether the caller must fill it.
func (u *UI) openDrawer(title string, closed func()) (d *drawer, fresh bool) {
	if d := u.activeDrawer; d != nil && d.title == title {
		return d, false
	}
	if u.activeDrawer != nil {
		u.activeDrawer.Close()
	}
	d = &drawer{u: u, title: title, closed: closed, body: container.NewStack()}
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	heading.Truncation = fyne.TextTruncateEllipsis
	shut := widget.NewButtonWithIcon("", fynetheme.CancelIcon(), d.Close)
	shut.Importance = widget.LowImportance
	d.root = container.NewBorder(container.NewVBox(container.NewBorder(nil, nil, nil, shut, heading), widget.NewSeparator()), nil, nil, nil, d.body)
	u.activeDrawer = d
	if u.drawerHost != nil {
		u.drawerHost.Objects = []fyne.CanvasObject{d.root}
		u.drawerHost.Show()
		u.drawerHost.Refresh()
		u.sidePane.Hide()
	}
	if u.workspace != nil {
		u.splitOffset = u.workspace.Offset
		u.workspace.SetOffset(min(u.workspace.Offset, drawerOffset))
	}
	return d, true
}

func (u *UI) closeDrawer(d *drawer) {
	if d == nil || u.activeDrawer != d {
		return
	}
	u.activeDrawer = nil
	if u.drawerHost != nil {
		u.drawerHost.Objects = nil
		u.drawerHost.Hide()
		u.sidePane.Show()
	}
	if u.workspace != nil && u.splitOffset > 0 {
		u.workspace.SetOffset(u.splitOffset)
	}
	if d.closed != nil {
		d.closed()
	}
}

// drawerTitled returns the open drawer if it has this title.
func (u *UI) drawerTitled(title string) *drawer {
	if u.activeDrawer != nil && u.activeDrawer.title == title {
		return u.activeDrawer
	}
	return nil
}
