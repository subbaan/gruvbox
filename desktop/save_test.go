package desktop

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func TestSaveAsNamesTheGrooveAndSelectsTheStem(t *testing.T) {
	u, projects := dialogUI(t)
	// A regenerated groove gets a suggested name even though a file is open
	// (so the test's folder is the one searched).
	u.regenerated = true
	u.session.Project.Style, u.session.Project.BPM = "Acid", 128
	u.session.Project.Key = &groove.Key{Root: 1, Scale: "minor"}
	if err := os.WriteFile(filepath.Join(projects, "acid-c-sharp-minor-128bpm.gruvbox3"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	u.saveAs()
	d := openDialog(t, u, "Save groove as")
	// The first name is taken, so the suggestion is numbered.
	if d.name.Text != "acid-c-sharp-minor-128bpm-2.gruvbox3" {
		t.Fatalf("suggested %q", d.name.Text)
	}
	if got := d.name.SelectedText(); got != "acid-c-sharp-minor-128bpm-2" {
		t.Fatalf("selected %q, want the name without its extension", got)
	}
	// Typing replaces the stem and keeps the extension.
	d.name.TypedRune('x')
	if d.name.Text != "x.gruvbox3" {
		t.Fatalf("after typing: %q", d.name.Text)
	}
}

func TestFileSlug(t *testing.T) {
	for in, want := range map[string]string{"Acid A minor 128bpm": "acid-a-minor-128bpm", "  --  ": "groove", "Dub/Tape: Mix": "dub-tape-mix"} {
		if got := fileSlug(in); got != want {
			t.Errorf("fileSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

// After Randomise all over a saved groove, Ctrl+S asks before overwriting
// the saved file; Save as new offers a fresh name.
func TestSaveAfterANewGrooveAsksBeforeOverwriting(t *testing.T) {
	u, projects := dialogUI(t)
	u.save()
	waitFor(t, "the first save", func() bool { return !u.session.Dirty() })
	saved, err := os.ReadFile(u.path)
	if err != nil {
		t.Fatal(err)
	}

	// An ordinary edit saves straight over the file.
	u.edit("Set BPM", func(p *groove.Project) error { p.BPM = 131; return nil })
	u.save()
	if u.modals["New groove since last save"] != nil || u.session.Dirty() {
		t.Fatal("an ordinary edit should save without asking")
	}
	saved, _ = os.ReadFile(u.path)

	u.randomiseGroove()
	u.save()
	card := u.modals["New groove since last save"]
	if card == nil {
		t.Fatal("no overwrite question after Randomise all")
	}
	if now, _ := os.ReadFile(u.path); string(now) != string(saved) {
		t.Fatal("the saved file changed before the user answered")
	}
	// Escape cancels and leaves the file alone.
	u.modalKey(fyne.KeyEscape)
	if now, _ := os.ReadFile(u.path); string(now) != string(saved) || !u.session.Dirty() {
		t.Fatal("cancel wrote the file")
	}

	// Save as new opens the dialog on a new name, not the saved file's.
	u.save()
	u.modalKey(fyne.KeyReturn)
	d := openDialog(t, u, "Save groove as")
	if d.name.Text == filepath.Base(u.path) || !strings.HasSuffix(d.name.Text, ".gruvbox3") {
		t.Fatalf("Save as new suggested %q", d.name.Text)
	}
	typeName(d, "second")
	want := filepath.Join(projects, "second.gruvbox3")
	waitFor(t, "the new save", func() bool { return u.path == want })
	if now, _ := os.ReadFile(filepath.Join(projects, "current.gruvbox3")); string(now) != string(saved) {
		t.Fatal("Save as new overwrote the old groove")
	}
	if u.regenerated {
		t.Fatal("saving should clear the new-groove mark")
	}

	// Overwrite writes over the file in place.
	u.randomiseGroove()
	u.save()
	card = u.modals["New groove since last save"]
	var overwrite *widget.Button
	walkObjects(card.pop.Content, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == "Overwrite" {
			overwrite = b
		}
	})
	if overwrite == nil {
		t.Fatal("no Overwrite button")
	}
	overwrite.OnTapped()
	waitFor(t, "the overwrite", func() bool { return !u.session.Dirty() })
	if u.path != want {
		t.Fatalf("overwrite saved to %s", u.path)
	}
}

func walkObjects(o fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	visit(o)
	if c, ok := o.(*fyne.Container); ok {
		for _, child := range c.Objects {
			walkObjects(child, visit)
		}
	}
}

func TestTrackMenuRendersOneTrack(t *testing.T) {
	u, projects := dialogUI(t)
	u.path = filepath.Join(projects, "night.gruvbox3")
	id := u.session.Project.Tracks[1].ID
	items := u.exportItemsFor(id)
	name := u.session.Project.Tracks[1].Name
	if len(items) != 2 || items[0].Label != name+" only · selected pattern…" || items[1].Label != name+" only · full song…" {
		t.Fatalf("items %q, %q should say the track is rendered alone", items[0].Label, items[1].Label)
	}
	items[0].Action()
	title := "Export WAV · " + u.session.Project.Tracks[1].Name
	w := u.panels[title]
	if w == nil {
		t.Fatalf("no %q window", title)
	}
	defer w.Close()
	var path *widget.Entry
	said := false
	walkObjects(w.Content(), func(o fyne.CanvasObject) {
		if l, ok := o.(*widget.Label); ok && strings.HasPrefix(l.Text, name+" only:") {
			said = true
		}
	})
	if !said {
		t.Fatal("the export window should say only this track is rendered")
	}
	walkObjects(w.Content(), func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && path == nil {
			path = e
		}
	})
	slot := groove.SlotName(u.session.Project.Pattern)
	if want := "night-" + fileSlug(u.session.Project.Tracks[1].Name) + "-pattern-" + slot + ".wav"; filepath.Base(path.Text) != want {
		t.Fatalf("destination %s, want %s", path.Text, want)
	}
}

// A pattern export loops by default (one loop long, tail wrapped onto the
// start); a song export offers only a tail at the end or none.
func TestExportWindowWrapsPatternTails(t *testing.T) {
	u, projects := dialogUI(t)
	id := u.session.Project.Tracks[0].ID
	u.exportWAV(false, id)
	w := u.panels["Export WAV · "+u.session.Project.Tracks[0].Name]
	var mode *widget.Select
	var path *widget.Entry
	var start *widget.Button
	walkObjects(w.Content(), func(o fyne.CanvasObject) {
		switch o := o.(type) {
		case *widget.Select:
			mode = o
		case *widget.Entry:
			if path == nil {
				path = o
			}
		case *widget.Button:
			if o.Text == "Export" {
				start = o
			}
		}
	})
	if mode == nil || mode.Selected != wrapTail.label || len(mode.Options) != 3 {
		t.Fatalf("pattern tail choice %v of %v, want %q first", mode.Selected, mode.Options, wrapTail.label)
	}
	destination := filepath.Join(projects, "loop.wav")
	path.SetText(destination)
	snapshot := u.session.Project.Clone()
	groove.IsolateTrack(&snapshot, id)
	start.OnTapped()
	waitFor(t, "the export", func() bool { _, err := os.Stat(destination); return err == nil })
	w.Close()
	var want bytes.Buffer
	if err := groove.RenderWAVMode(context.Background(), &want, snapshot, false, 2, groove.TailWrap, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(destination); !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("exported %d bytes, want the wrapped loop's %d", len(got), want.Len())
	}

	if labels := exportTailLabels(exportTailModes(true)); len(labels) != 2 || labels[0] != appendTail.label {
		t.Fatalf("song tail choices %v", labels)
	}
}
