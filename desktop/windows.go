package desktop

import (
	"fyne.io/fyne/v2"
)

// Utility windows have native title bars and are movable by the window manager.
// Keep one instance per purpose, and cancel unfinished work on native close.
func (u *UI) panel(title string, size fyne.Size, closed func()) (fyne.Window, bool) {
	if w := u.panels[title]; w != nil {
		w.Show()
		w.RequestFocus()
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
