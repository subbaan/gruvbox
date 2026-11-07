package desktop

import (
	"time"

	"fyne.io/fyne/v2"
	fynecanvas "fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const modalWidth, toastDuration = 460, 10 * time.Second

// modal is an in-window confirmation card. Closing it any way other than the
// accept button reports false, exactly once.
type modal struct {
	pop    *widget.PopUp
	finish func(bool)
	key    func(fyne.KeyName) // if set, takes every key instead (the file dialog)
	file   *fileDialog
}

func (m *modal) Close() { m.finish(false) }

// confirm asks a question over the main window.
func (u *UI) confirm(title, accept, cancel string, content fyne.CanvasObject, callback func(bool)) {
	u.confirmOn(u.window.Canvas(), title, accept, cancel, content, callback)
}

// confirmOn shows the card over canvas (a utility window may pass its own).
// Enter in a field accepts; Escape or Cancel declines. One card per title.
// An empty cancel label leaves out the cancel button (an information card).
func (u *UI) confirmOn(canvas fyne.Canvas, title, accept, cancel string, content fyne.CanvasObject, callback func(bool)) {
	if u.modals == nil {
		u.modals = map[string]*modal{}
	}
	if u.modals[title] != nil {
		return
	}
	m := &modal{}
	done := false
	m.finish = func(ok bool) {
		if done {
			return
		}
		done = true
		m.pop.Hide()
		delete(u.modals, title)
		u.modalOrder = deleteModal(u.modalOrder, m)
		callback(ok)
	}
	acceptButton := widget.NewButton(accept, func() { m.finish(true) })
	acceptButton.Importance = widget.HighImportance
	cancelButton := widget.NewButton(cancel, func() { m.finish(false) })
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	var firstEntry *widget.Entry
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Entry:
			if firstEntry == nil {
				firstEntry = o
			}
			if o.OnSubmitted == nil {
				o.OnSubmitted = func(string) { m.finish(true) }
			}
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(content)
	buttons := container.NewHBox(layout.NewSpacer(), cancelButton, acceptButton)
	if cancel == "" {
		buttons = container.NewHBox(layout.NewSpacer(), acceptButton)
		acceptButton.Importance = widget.MediumImportance
	}
	card := container.NewBorder(container.NewVBox(heading, widget.NewSeparator()), buttons, nil, nil, content)
	roles := u.scheme.Roles()
	background := fynecanvas.NewRectangle(colour(roles.Panel))
	background.StrokeColor, background.StrokeWidth, background.CornerRadius = colour(roles.Border), 1, 4
	m.pop = widget.NewModalPopUp(container.NewStack(background, container.NewPadded(card)), canvas)
	if label, ok := content.(*widget.Label); ok && label.Wrapping != fyne.TextWrapOff {
		label.Resize(fyne.NewSize(modalWidth-4*fynetheme.Padding(), label.MinSize().Height))
	}
	m.pop.Resize(fyne.NewSize(modalWidth, m.pop.Content.MinSize().Height))
	u.modals[title] = m
	if canvas == u.window.Canvas() {
		u.modalOrder = append(u.modalOrder, m)
	}
	m.pop.Show()
	if firstEntry != nil {
		canvas.Focus(firstEntry)
	}
}

func deleteModal(list []*modal, m *modal) []*modal {
	for i, item := range list {
		if item == m {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// modalKey handles keys while a card is open over the main window: Escape
// declines, Enter accepts, and everything else is swallowed so shortcuts do
// not edit the groove behind the card.
func (u *UI) modalKey(key fyne.KeyName) bool {
	if len(u.modalOrder) == 0 {
		return false
	}
	top := u.modalOrder[len(u.modalOrder)-1]
	if top.key != nil {
		top.key(key)
		return true
	}
	switch key {
	case fyne.KeyEscape:
		top.finish(false)
	case fyne.KeyReturn, fyne.KeyEnter:
		top.finish(true)
	}
	return true
}

// showError reports a failure in a banner above the status line that fades
// after a while or on ×; it never blocks the window.
func (u *UI) showError(err error) {
	if u.toast == nil {
		label := widget.NewLabel(err.Error())
		label.Wrapping = fyne.TextWrapWord
		u.confirm("Gruvbox / Error", "OK", "Close", label, func(bool) {})
		return
	}
	roles := u.scheme.Roles()
	u.toastBackground.FillColor, u.toastBackground.StrokeColor = colour(roles.Panel), colour(roles.Error)
	u.toastBackground.Refresh()
	u.toastLabel.SetText(err.Error())
	u.toast.Show()
	u.relayout()
	u.toastGeneration++
	generation := u.toastGeneration
	time.AfterFunc(toastDuration, func() {
		fyne.Do(func() {
			if u.toastGeneration == generation {
				u.hideToast()
			}
		})
	})
}

func (u *UI) errorToast() fyne.CanvasObject {
	u.toastLabel = widget.NewLabel("")
	u.toastLabel.Wrapping = fyne.TextWrapWord
	u.toastLabel.Importance = widget.DangerImportance
	u.toastBackground = fynecanvas.NewRectangle(colour(u.scheme.Roles().Panel))
	u.toastBackground.StrokeWidth, u.toastBackground.CornerRadius = 1, 4
	dismiss := widget.NewButtonWithIcon("", fynetheme.CancelIcon(), u.hideToast)
	dismiss.Importance = widget.LowImportance
	icon := widget.NewIcon(fynetheme.ErrorIcon())
	u.toast = container.NewStack(u.toastBackground, container.NewBorder(nil, nil, icon, dismiss, u.toastLabel))
	u.toast.Hide()
	return u.toast
}

func (u *UI) hideToast() {
	u.toastGeneration++
	u.toast.Hide()
	u.relayout()
}

// relayout re-runs the window's top-level layout after the footer changes
// height; showing a hidden child does not resize its parents by itself.
func (u *UI) relayout() {
	if u.root != nil {
		u.root.Layout.Layout(u.root.Objects, u.root.Size())
	}
}
