package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
	"testing"
)

func TestPianoDragPreservesDetailsAndOneUndo(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("notes")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	g := u.roll
	g.Resize(fyne.NewSize(860, 1100))
	g.noteTop = 59 // positions below assume this range
	source := groove.Step{Active: true, Note: 48, Velocity: .37, Accent: true, Slide: true, Probability: .6, Every: 3, Locks: map[string]float64{"cutoff": .72}}
	u.session.Project.Tracks[0].Patterns[0].Steps[2] = source
	u.session.Project.Tracks[0].Patterns[0].Steps[6] = groove.DefaultStep()
	down := fyne.NewPos(60+2*50+20, 34+11*rollRowHeight+10)
	dest := fyne.NewPos(60+6*50+20, 34+8*rollRowHeight+10)
	g.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: down}, Button: desktop.MouseButtonPrimary})
	g.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: dest}})
	if !u.selectedTrack().Patterns[0].Steps[2].Active {
		t.Fatal("preview mutated source")
	}
	g.DragEnd()
	steps := u.selectedTrack().Patterns[0].Steps
	got := steps[6]
	if steps[2].Active || !got.Active || got.Note != 51 || got.Velocity != source.Velocity || !got.Accent || !got.Slide || got.Probability != source.Probability || got.Every != 3 || got.Locks["cutoff"] != .72 {
		t.Fatalf("lost note details: %+v", got)
	}
	u.undo()
	if u.selectedTrack().Patterns[0].Steps[2].Note != 48 || u.selectedTrack().Patterns[0].Steps[6].Active {
		t.Fatal("undo failed")
	}
	if u.session.Undo() {
		t.Fatal("drag added multiple history entries")
	}
	u.redo()
	if u.selectedTrack().Patterns[0].Steps[6].Note != 51 {
		t.Fatal("redo failed")
	}
	// Dropping outside the grid must leave the project untouched.
	g.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: dest}, Button: desktop.MouseButtonPrimary})
	g.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(-5, 40)}})
	g.DragEnd()
	if !u.selectedTrack().Patterns[0].Steps[6].Active {
		t.Fatal("outside drop deleted note")
	}
	for i := 0; i < 20; i++ {
		g.shiftOctave(1)
	}
	if g.noteTop != 127 {
		t.Fatal("high range exceeds MIDI")
	}
	for i := 0; i < 20; i++ {
		g.shiftOctave(-1)
	}
	if g.noteTop != rollRows-1 {
		t.Fatal("low range exceeds MIDI")
	}
	if _, _, ok := g.noteCell(fyne.NewPos(60, 34+rollRows*rollRowHeight)); ok {
		t.Fatal("accepted row below visible range")
	}
}

func TestTomPianoRollDragAndUndo(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("tom notes")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	track := groove.NewTrack(groove.Tom, 0)
	u.session.Project.Tracks = []groove.Track{track}
	u.selected = track.ID
	w.SetContent(u.content())
	u.refresh()
	g := u.roll
	g.Resize(fyne.NewSize(860, 1100))
	g.noteTop = 59 // positions below assume this range
	u.selectedTrack().Patterns[0].Steps[2] = groove.Step{Active: true, Note: 48, Velocity: .6, Probability: 1, Every: 1, Locks: map[string]float64{"pitch_decay": .2}}
	down := fyne.NewPos(60+2*50+20, 34+11*rollRowHeight+10)
	dest := fyne.NewPos(60+6*50+20, 34+4*rollRowHeight+10)
	g.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: down}, Button: desktop.MouseButtonPrimary})
	g.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: dest}})
	g.DragEnd()
	got := u.selectedTrack().Patterns[0].Steps[6]
	if !got.Active || got.Note != 55 || got.Velocity != .6 || got.Locks["pitch_decay"] != .2 || u.selectedTrack().Patterns[0].Steps[2].Active {
		t.Fatalf("tom note drag failed: %+v", got)
	}
	u.undo()
	if u.selectedTrack().Patterns[0].Steps[2].Note != 48 || u.selectedTrack().Patterns[0].Steps[6].Active {
		t.Fatal("tom drag undo failed")
	}
}

func TestPianoPhraseSelectionResizeAndUndo(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("phrases")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	track := u.selectedTrack()
	for i := range track.Patterns[0].Steps {
		track.Patterns[0].Steps[i] = groove.DefaultStep()
	}
	for _, i := range []int{1, 3} {
		track.Patterns[0].Steps[i] = groove.Step{Active: true, Note: 48, Velocity: .6, Probability: 1, Locks: map[string]float64{"cutoff": .5}}
	}
	g := u.roll
	g.Resize(fyne.NewSize(860, 1100))
	g.noteTop = 59 // positions below assume this range
	for _, i := range []int{1, 3} {
		pos := fyne.NewPos(60+float32(i)*50+20, 34+11*rollRowHeight+10)
		g.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: pos}, Button: desktop.MouseButtonPrimary, Modifier: fyne.KeyModifierShift})
		g.Tapped(&fyne.PointEvent{Position: pos})
	}
	if len(g.selectedNotes()) != 2 || !u.selectedTrack().Patterns[0].Steps[1].Active {
		t.Fatal("shift selection toggled notes")
	}
	down := fyne.NewPos(60+50+20, 34+11*rollRowHeight+10)
	dest := fyne.NewPos(60+7*50+20, 34+8*rollRowHeight+10)
	g.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: down}, Button: desktop.MouseButtonPrimary})
	g.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: dest}})
	g.DragEnd()
	steps := u.selectedTrack().Patterns[0].Steps
	if steps[1].Active || steps[3].Active || steps[7].Note != 51 || steps[9].Note != 51 || steps[7].Locks["cutoff"] != .5 || len(g.selectedNotes()) != 2 {
		t.Fatal("group drag failed")
	}
	u.undo()
	if !u.selectedTrack().Patterns[0].Steps[1].Active || u.selectedTrack().Patterns[0].Steps[7].Active {
		t.Fatal("group drag undo failed")
	}
	if u.session.Undo() {
		t.Fatal("group drag added multiple undo entries")
	}
	g.clearSelection()
	// Resize the right edge of step 1 from one to four steps.
	down = fyne.NewPos(60+2*50-5, 34+11*rollRowHeight+10)
	dest = fyne.NewPos(60+5*50-1, 34+11*rollRowHeight+10)
	g.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: down}, Button: desktop.MouseButtonPrimary})
	if g.drag == nil || !g.drag.resize {
		t.Fatal("right edge did not begin resize")
	}
	g.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: dest}})
	if u.selectedTrack().Patterns[0].Steps[1].Length != 0 {
		t.Fatal("resize preview changed project")
	}
	g.DragEnd()
	if u.selectedTrack().Patterns[0].Steps[1].Length != 4 {
		t.Fatal("resize did not save length")
	}
	u.undo()
	if u.selectedTrack().Patterns[0].Steps[1].Length != 0 {
		t.Fatal("resize undo failed")
	}
	g.selectAll()
	g.transformSelection(0, 12, false)
	if u.selectedTrack().Patterns[0].Steps[1].Note != 60 || u.selectedTrack().Patterns[0].Steps[3].Note != 60 {
		t.Fatal("phrase transpose failed")
	}
	g.duplicateSelection()
	if !u.selectedTrack().Patterns[0].Steps[4].Active || !u.selectedTrack().Patterns[0].Steps[6].Active || len(g.selectedNotes()) != 2 {
		t.Fatal("phrase duplicate failed")
	}
	g.deleteSelection()
	if u.selectedTrack().Patterns[0].Steps[4].Active || !u.selectedTrack().Patterns[0].Steps[1].Active {
		t.Fatal("phrase delete changed original")
	}
	u.undo()
	if !u.selectedTrack().Patterns[0].Steps[4].Active {
		t.Fatal("phrase delete undo failed")
	}
	u.session.Project.Pattern = 1
	g.ensureSelection()
	if len(g.selectedNotes()) != 0 {
		t.Fatal("selection leaked across patterns")
	}
}
