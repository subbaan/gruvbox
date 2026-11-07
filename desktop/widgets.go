package desktop

import (
	"fyne.io/fyne/v2"
	fynecanvas "fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// commitEntry applies its value on Enter and when focus leaves it, so edits
// are never silently lost. It commits only when the text differs from the last
// committed or programmatically set value, which keeps undo history clean when
// the inspector is rebuilt underneath a focused entry.
type commitEntry struct {
	widget.Entry
	baseline string
	OnCommit func(string)
}

func newCommitEntry(value string, commit func(string)) *commitEntry {
	e := &commitEntry{OnCommit: commit}
	e.ExtendBaseWidget(e)
	e.SetValue(value)
	e.OnSubmitted = func(string) { e.commit() }
	return e
}

// SetValue replaces the text and the committed baseline.
func (e *commitEntry) SetValue(value string) {
	e.baseline = value
	e.SetText(value)
}
func (e *commitEntry) commit() {
	if e.Text == e.baseline || e.OnCommit == nil {
		return
	}
	e.baseline = e.Text
	e.OnCommit(e.Text)
}
func (e *commitEntry) FocusLost() {
	e.Entry.FocusLost()
	e.commit()
}

const helpWidth float32 = 380

// helpButton replaces inline help paragraphs with a small "?" popover.
// Fyne 2.7 has no tooltips, so this is the project's tooltip substitute.
func (u *UI) helpButton(text string) *widget.Button {
	var button *widget.Button
	button = widget.NewButtonWithIcon("", fynetheme.QuestionIcon(), func() { u.showHelp(button, text) })
	button.Importance = widget.LowImportance
	return button
}
func (u *UI) showHelp(anchor fyne.CanvasObject, text string) {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	canvas := fyne.CurrentApp().Driver().CanvasForObject(anchor)
	if canvas == nil {
		canvas = u.window.Canvas()
	}
	roles := u.scheme.Roles()
	panel := fynecanvas.NewRectangle(colour(roles.Panel))
	panel.StrokeColor, panel.StrokeWidth = colour(roles.Border), 1
	pop := widget.NewPopUp(container.NewStack(panel, container.NewPadded(label)), canvas)
	// A wrapping label reports its height for the width it was last laid out at.
	label.Resize(fyne.NewSize(helpWidth, label.MinSize().Height))
	size := fyne.NewSize(helpWidth+2*fynetheme.Padding(), label.MinSize().Height+2*fynetheme.Padding())
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor).Add(fyne.NewPos(0, anchor.Size().Height))
	if limit := canvas.Size().Width - size.Width - 8; pos.X > limit {
		pos.X = max(0, limit)
	}
	pop.Resize(size)
	pop.ShowAtPosition(pos)
}

// withHelp places a "?" popover after a heading or control.
func (u *UI) withHelp(object fyne.CanvasObject, text string) fyne.CanvasObject {
	return container.NewBorder(nil, nil, nil, u.helpButton(text), object)
}
