package desktop

import (
	"fyne.io/fyne/v2"
)

// Utility windows have native title bars and are movable by the window manager.
// Keep one instance per purpose, and cancel unfinished work on native close.
func (u *UI) panel(title string, size fyne.Size, closed func()) (fyne.Window, bool) {
	if w := u.panels[title]; w != nil {
		u.showPanel(w)
		return w, false
	}
	w := u.app.NewWindow(title)
	u.panels[title] = w
	w.Resize(size)
	w.SetOnClosed(func() {
		delete(u.panels, title)
		if closed != nil {
			closed()
		}
	})
	return w, true
}

// showPanel shows a utility window above the main window, focused. A bare
// Show can leave it behind the main window when the window manager
// prevents focus stealing (see raiseOver).
func (u *UI) showPanel(w fyne.Window) {
	w.Show()
	raiseOver(w, u.window)
}
