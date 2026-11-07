package desktop

import (
	"errors"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestConfirmCardKeys(t *testing.T) {
	u, _ := redesignUI(t)
	results := []bool{}
	ask := func() {
		u.confirm("Question", "Yes", "No", widget.NewLabel("Sure?"), func(ok bool) { results = append(results, ok) })
	}
	ask()
	muted := u.selectedTrack().Mute
	u.key(fyne.KeyM)
	if u.selectedTrack().Mute != muted {
		t.Fatal("shortcut edited the groove behind an open card")
	}
	u.key(fyne.KeyEscape)
	ask()
	u.key(fyne.KeyReturn)
	if len(results) != 2 || results[0] || !results[1] || len(u.modals) != 0 || len(u.modalOrder) != 0 {
		t.Fatalf("Escape/Enter did not decline/accept once each: %v", results)
	}
	entry := widget.NewEntry()
	accepted := false
	u.confirm("Name", "Save", "Cancel", entry, func(ok bool) { accepted = ok })
	if u.window.Canvas().Focused() != entry {
		t.Fatal("card did not focus its field")
	}
	test.Type(entry, "x")
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if !accepted || u.modals["Name"] != nil {
		t.Fatal("Enter in the field did not accept")
	}
}

func TestErrorToast(t *testing.T) {
	u, _ := redesignUI(t)
	if u.toast.Visible() {
		t.Fatal("toast visible before any error")
	}
	u.showError(errors.New("disk full"))
	if !u.toast.Visible() || u.toastLabel.Text != "disk full" || len(u.modals) != 0 {
		t.Fatal("error did not appear as a non-blocking toast")
	}
	generation := u.toastGeneration
	u.hideToast()
	if u.toast.Visible() || u.toastGeneration == generation {
		t.Fatal("dismiss should invalidate the pending fade")
	}
}
