package desktop

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
)

// dialogUI is a UI whose data root and project folder are temp folders.
func dialogUI(t *testing.T) (*UI, string) {
	t.Helper()
	a := test.NewApp()
	t.Cleanup(a.Quit)
	w := a.NewWindow("Files")
	w.Resize(fyne.NewSize(1200, 800))
	root := t.TempDir()
	u := newUI(a, w, theme.Library{Dir: filepath.Join(root, "themes")})
	w.SetContent(u.content())
	projects := filepath.Join(root, "projects")
	if err := os.MkdirAll(filepath.Join(projects, "older"), 0o755); err != nil {
		t.Fatal(err)
	}
	u.path = filepath.Join(projects, "current.gruvbox3")
	return u, projects
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatal("timed out waiting for " + what)
}

func openDialog(t *testing.T, u *UI, title string) *fileDialog {
	t.Helper()
	m := u.modals[title]
	if m == nil || m.file == nil {
		t.Fatalf("no %q dialog", title)
	}
	return m.file
}

func typeName(d *fileDialog, text string) {
	d.name.SetText(text)
	d.key(fyne.KeyReturn)
}

func TestSaveAsWritesTheFileAndAsksBeforeReplacing(t *testing.T) {
	u, projects := dialogUI(t)
	u.saveAs()
	d := openDialog(t, u, "Save groove as")
	if d.b.dir != projects || d.name.Text != "current.gruvbox3" {
		t.Fatalf("starts in %s with %q", d.b.dir, d.name.Text)
	}
	// The extension is added when it is left off.
	typeName(d, "night drive")
	want := filepath.Join(projects, "night drive.gruvbox3")
	waitFor(t, "the save", func() bool { return u.path == want })
	if u.modals["Save groove as"] != nil {
		t.Fatal("dialog still open")
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatal(err)
	}
	if list := loadRecent(u.recentFilesFile(), "projects"); len(list) != 1 || list[0].Path != want {
		t.Fatalf("recent: %v", list)
	}
	// Saving over it asks first; Cancel keeps the dialog open.
	u.saveAs()
	d = openDialog(t, u, "Save groove as")
	if d.b.dir != projects {
		t.Fatalf("the dialog did not remember its folder: %s", d.b.dir)
	}
	info, _ := os.Stat(want)
	typeName(d, "night drive.gruvbox3")
	confirm := u.modals["Replace file"]
	if confirm == nil {
		t.Fatal("no replace confirm")
	}
	confirm.finish(false)
	if u.modals["Save groove as"] == nil {
		t.Fatal("cancelling the replace closed the dialog")
	}
	typeName(d, "night drive.gruvbox3")
	u.modals["Replace file"].finish(true)
	waitFor(t, "the replace", func() bool {
		now, err := os.Stat(want)
		return err == nil && (now.ModTime() != info.ModTime() || u.modals["Save groove as"] == nil)
	})
}

func TestOpenGrooveByTypedPathAndRecent(t *testing.T) {
	u, projects := dialogUI(t)
	p := u.session.Project.Clone()
	p.BPM = 97
	path := filepath.Join(projects, "older", "slow.gruvbox3")
	if err := groove.Save(path, p); err != nil {
		t.Fatal(err)
	}
	u.open()
	d := openDialog(t, u, "Open groove")
	if d.b.places[0].kind != placeRecent {
		t.Fatal("opening has no Recent place")
	}
	typeName(d, "older/")
	if d.b.dir != filepath.Join(projects, "older") {
		t.Fatalf("slash did not go in: %s", d.b.dir)
	}
	if got := names(d.b); len(got) != 1 || got[0] != "slow.gruvbox3" {
		t.Fatalf("listing: %v", got)
	}
	// The preview reads the project.
	d.refreshPreview()
	if d.previewBody.Text == "" || !strings.Contains(d.previewBody.Text, "97 BPM") {
		t.Fatalf("preview: %q", d.previewBody.Text)
	}
	typeName(d, "sl")
	waitFor(t, "the open", func() bool { return u.path == path })
	if u.session.Project.BPM != 97 {
		t.Fatalf("bpm %v", u.session.Project.BPM)
	}
	// Next time it is in Recent, listed first.
	u.open()
	d = openDialog(t, u, "Open groove")
	d.b.goPlace(0)
	if got := d.b.visible(); len(got) != 1 || got[0].Path != path {
		t.Fatalf("recent: %v", got)
	}
	d.key(fyne.KeyEscape)
	if u.modals["Open groove"] != nil {
		t.Fatal("Escape did not close it")
	}
}

func TestFileDialogKeysAndDoubleClick(t *testing.T) {
	u, projects := dialogUI(t)
	u.open()
	d := openDialog(t, u, "Open groove")
	d.shortcut(&desktop.CustomShortcut{KeyName: fyne.KeyUp, Modifier: fyne.KeyModifierAlt})
	if d.b.dir != filepath.Dir(projects) {
		t.Fatalf("Alt+Up: %s", d.b.dir)
	}
	if e, _ := d.b.selected(); e.Name != "projects" {
		t.Fatalf("Alt+Up should select projects, got %s", e.Name)
	}
	d.shortcut(&desktop.CustomShortcut{KeyName: fyne.KeyLeft, Modifier: fyne.KeyModifierAlt})
	if d.b.dir != projects {
		t.Fatalf("Alt+Left: %s", d.b.dir)
	}
	d.key(fyne.KeyBackspace) // empty name: up
	if d.b.dir != filepath.Dir(projects) {
		t.Fatalf("Backspace: %s", d.b.dir)
	}
	// Two clicks on a folder row go into it.
	row := -1
	for i, e := range d.b.visible() {
		if e.Name == "projects" {
			row = i
		}
	}
	d.clickRow(row)
	if d.b.dir != filepath.Dir(projects) {
		t.Fatal("one click went in")
	}
	d.clickRow(row)
	if d.b.dir != projects {
		t.Fatalf("double click: %s", d.b.dir)
	}
	// Tab moves to the places; Enter there goes to one.
	d.key(fyne.KeyTab)
	if !d.b.placesFocus {
		t.Fatal("Tab did not focus the places")
	}
	d.b.placeSel = 0 // Recent
	d.key(fyne.KeyReturn)
	if !d.b.recentOn || d.b.placesFocus {
		t.Fatal("Enter on a place did not go there")
	}
	d.key(fyne.KeyEscape)
}

func TestFileDialogBookmarksLiveInTheDataRoot(t *testing.T) {
	u, projects := dialogUI(t)
	u.loadSample("none")
	d := openDialog(t, u, "Load sample")
	d.b.goTo(projects)
	d.shortcut(&desktop.CustomShortcut{KeyName: fyne.KeyD, Modifier: fyne.KeyModifierControl})
	file := filepath.Join(u.dataRoot(), "bookmarks.json")
	if got := loadBookmarks(file); len(got) != 1 || got[0] != projects {
		t.Fatalf("bookmarks: %v", got)
	}
	if d.message.Text == "" {
		t.Fatal("no message for the bookmark")
	}
	d.key(fyne.KeyEscape)
	// Another dialog shows it as a place.
	u.saveAs()
	d = openDialog(t, u, "Save groove as")
	last := d.b.places[len(d.b.places)-1]
	if last.kind != placeBookmark || last.dir != projects {
		t.Fatalf("place: %+v", last)
	}
	d.key(fyne.KeyEscape)
}

// TestFileDialogScreenshots writes the Open and Save dialogs; see
// TestScreenshots.
func TestFileDialogScreenshots(t *testing.T) {
	dir := os.Getenv("GRUVBOX_SHOTS")
	if dir == "" {
		t.Skip("set GRUVBOX_SHOTS to a directory to write UI screenshots")
	}
	u, projects := dialogUI(t)
	for _, name := range []string{"acid-friday", "warehouse", "night drive", "sketch 4"} {
		p := u.session.Project.Clone()
		p.BPM = 118 + float64(len(name))
		if err := groove.Save(filepath.Join(projects, name+".gruvbox3"), p); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(projects, "notes.txt"), []byte("ideas"), 0o644)
	u.rememberFile("projects", filepath.Join(projects, "warehouse.gruvbox3"))
	shot := func(name string) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, u.window.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	for _, scheme := range []string{"gruvbox-dark", "warm-dark"} {
		u.applyScheme(theme.Bundled()[scheme])
		u.open()
		d := openDialog(t, u, "Open groove")
		d.b.selectPath(filepath.Join(projects, "night drive.gruvbox3"))
		d.refresh()
		d.refreshPreview()
		shot("file-open-" + scheme)
		d.name.SetText("ware")
		d.refresh()
		shot("file-open-filter-" + scheme)
		d.key(fyne.KeyEscape)
		u.saveAs()
		d = openDialog(t, u, "Save groove as")
		d.key(fyne.KeyTab)
		shot("file-save-" + scheme)
		d.key(fyne.KeyEscape)
	}
	u.window.Resize(fyne.NewSize(760, 520))
	u.open()
	shot("file-open-small")
	openDialog(t, u, "Open groove").key(fyne.KeyEscape)
}

func TestCrumbBarKeepsTheCurrentFolderWhenNarrow(t *testing.T) {
	u, projects := dialogUI(t)
	u.open()
	d := openDialog(t, u, "Open groove")
	defer d.key(fyne.KeyEscape)
	objects := d.crumbs.Objects
	last := objects[len(objects)-1].(*widget.Label)
	if last.Text != filepath.Base(projects) {
		t.Fatalf("last crumb %q", last.Text)
	}
	d.crumbs.Resize(fyne.NewSize(200, 36))
	if !objects[0].Visible() || objects[1].Visible() || !last.Visible() || last.Size().Width < 40 {
		t.Fatal("narrow bar should show … and the current folder")
	}
	d.crumbs.Resize(fyne.NewSize(4000, 36))
	if objects[0].Visible() || !objects[1].Visible() {
		t.Fatal("wide bar should show every crumb and no …")
	}
}
