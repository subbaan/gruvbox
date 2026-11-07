package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
	"math"
	"sort"
)

// Gestures preview without changing the project until release.
type noteDrag struct {
	session                       *groove.Session
	trackID                       string
	pattern, source, target, note int
	anchor                        int
	step                          groove.Step
	indices                       []int
	resize                        bool
	length                        int
	valid, moved                  bool
}

func (g *sequencerGrid) ensureSelection() {
	u := g.ui
	if g.selectionSession != u.session || g.selectionTrack != u.selected || g.selectionPattern != u.session.Project.Pattern {
		g.selection = map[int]bool{}
		g.selectionSession, g.selectionTrack, g.selectionPattern = u.session, u.selected, u.session.Project.Pattern
	}
	if track := u.selectedTrack(); track != nil {
		steps := track.Patterns[u.session.Project.Pattern].Steps
		for index := range g.selection {
			if index >= len(steps) || !steps[index].Active {
				delete(g.selection, index)
			}
		}
	}
}
func (g *sequencerGrid) selectedNotes() []int {
	g.ensureSelection()
	indices := []int{}
	for i := range g.selection {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	return indices
}
func (g *sequencerGrid) selectAll() {
	g.ensureSelection()
	if t := g.ui.selectedTrack(); t != nil && t.Kind.Capabilities().Notes {
		for i, step := range t.Patterns[g.ui.session.Project.Pattern].Steps {
			if step.Active {
				g.selection[i] = true
			}
		}
	}
	g.Refresh()
}
func (g *sequencerGrid) clearSelection() { g.selection = map[int]bool{}; g.Refresh() }
func (g *sequencerGrid) transformSelection(offset, semitones int, copyNotes bool) {
	indices := g.selectedNotes()
	if len(indices) == 0 {
		return
	}
	u := g.ui
	id, slot := u.selected, u.session.Project.Pattern
	applied := false
	u.edit("Edit note selection", func(p *groove.Project) error {
		err := groove.TransformNotes(&p.Track(id).Patterns[slot], indices, offset, semitones, copyNotes)
		applied = err == nil
		if applied {
			u.selectedStep = indices[0] + offset
		}
		return err
	})
	if applied {
		g.selection = map[int]bool{}
		for _, i := range indices {
			g.selection[i+offset] = true
		}
	}
	g.Refresh()
}
func (g *sequencerGrid) duplicateSelection() {
	indices := g.selectedNotes()
	if len(indices) == 0 {
		return
	}
	steps := g.ui.selectedTrack().Patterns[g.ui.session.Project.Pattern].Steps
	end := indices[len(indices)-1] + 1
	for _, i := range indices {
		end = max(end, i+steps[i].GateSteps())
	}
	g.transformSelection(end-indices[0], 0, true)
}
func (g *sequencerGrid) deleteSelection() {
	indices := g.selectedNotes()
	if len(indices) == 0 {
		return
	}
	u := g.ui
	id, slot := u.selected, u.session.Project.Pattern
	u.edit("Delete selected notes", func(p *groove.Project) error {
		for _, i := range indices {
			p.Track(id).Patterns[slot].Steps[i] = groove.DefaultStep()
		}
		return nil
	})
	g.clearSelection()
}
func (g *sequencerGrid) shiftOctave(direction int) {
	g.drag = nil
	g.noteTop = min(127, max(rollRows-1, g.noteTop+12*direction))
	g.Refresh()
}
func (g *sequencerGrid) noteCell(pos fyne.Position) (int, int, bool) {
	t := g.ui.selectedTrack()
	if !g.roll || t == nil || !t.Kind.Capabilities().Notes || pos.X < 60 || pos.X >= g.Size().Width || pos.Y < 34 || pos.Y >= 34+rollRows*rollRowHeight || g.Size().Width <= 60 {
		return 0, 0, false
	}
	step := g.ui.page*16 + int((pos.X-60)/((g.Size().Width-60)/16))
	return step, g.noteTop - int((pos.Y-34)/rollRowHeight), step < len(t.Patterns[g.ui.session.Project.Pattern].Steps)
}

// Resolve the body of a long note to its onset, including earlier pages.
func (g *sequencerGrid) noteOwner(index, note int) int {
	t := g.ui.selectedTrack()
	steps := t.Patterns[g.ui.session.Project.Pattern].Steps
	for i := index; i >= 0; i-- {
		step := steps[i]
		if !step.Active {
			continue
		}
		length := 1
		if t.Kind.Capabilities().NoteLengths {
			length = step.GateSteps()
		}
		if step.Note == note && index < i+length {
			return i
		}
		// A newer onset cuts off the previous monophonic note.
		return -1
	}
	return -1
}
func (g *sequencerGrid) MouseDown(e *desktop.MouseEvent) {
	g.drag = nil
	g.suppressTap = false
	if e.Button != desktop.MouseButtonPrimary {
		return
	}
	index, note, ok := g.noteCell(e.Position)
	if !ok {
		return
	}
	g.ensureSelection()
	owner := g.noteOwner(index, note)
	if e.Modifier&fyne.KeyModifierShift != 0 {
		g.suppressTap = true
		if owner >= 0 {
			if g.selection[owner] {
				delete(g.selection, owner)
			} else {
				g.selection[owner] = true
			}
			g.ui.selectedStep = owner
		}
		g.ui.refresh()
		return
	}
	if owner < 0 {
		return
	}
	u := g.ui
	u.finishLiveSlider()
	step := u.selectedTrack().Patterns[u.session.Project.Pattern].Steps[owner]
	indices := []int{owner}
	if g.selection[owner] {
		indices = g.selectedNotes()
	} else {
		g.selection = map[int]bool{owner: true}
	}
	width := (g.Size().Width - 60) / 16
	endX := 60 + float32(owner-u.page*16+step.GateSteps())*width
	resize := u.selectedTrack().Kind.Capabilities().NoteLengths && e.Position.X >= endX-10 && e.Position.X <= endX
	g.drag = &noteDrag{session: u.session, trackID: u.selected, pattern: u.session.Project.Pattern, source: owner, anchor: index, target: owner, note: note, step: step, indices: indices, resize: resize, length: step.GateSteps(), valid: true}
}
func (g *sequencerGrid) MouseUp(*desktop.MouseEvent) {}
func (g *sequencerGrid) Dragged(e *fyne.DragEvent) {
	d := g.drag
	if d == nil {
		return
	}
	d.moved = true
	index, note, valid := g.noteCell(e.Position)
	d.valid = valid
	if d.resize {
		width := (g.Size().Width - 60) / 16
		endpoint := g.ui.page*16 + int(math.Round(float64((e.Position.X-60)/width)))
		d.length = min(64, max(1, endpoint-d.source))
	} else {
		d.target, d.note = d.source+index-d.anchor, note
	}
	g.Refresh()
}
func (g *sequencerGrid) DragEnd() {
	d := g.drag
	g.drag = nil
	if d == nil {
		return
	}
	u := g.ui
	if !d.moved || !d.valid || u.session != d.session || u.selected != d.trackID || u.session.Project.Pattern != d.pattern {
		g.Refresh()
		return
	}
	if d.resize {
		if d.length == d.step.GateSteps() {
			g.Refresh()
			return
		}
		u.edit("Resize note", func(p *groove.Project) error {
			p.Track(d.trackID).Patterns[d.pattern].Steps[d.source].Length = d.length
			return nil
		})
		return
	}
	offset, semitones := d.target-d.source, d.note-d.step.Note
	if offset == 0 && semitones == 0 {
		g.Refresh()
		return
	}
	g.transformSelection(offset, semitones, false)
}

func (g *sequencerGrid) selectionTools() fyne.CanvasObject {
	return container.NewHBox(
		widget.NewButton("Select all", g.selectAll), widget.NewButton("Clear selection", g.clearSelection),
		widget.NewButton("Duplicate", g.duplicateSelection),
		widget.NewButton("−12", func() { g.transformSelection(0, -12, false) }),
		widget.NewButton("−1", func() { g.transformSelection(0, -1, false) }),
		widget.NewButton("+1", func() { g.transformSelection(0, 1, false) }),
		widget.NewButton("+12", func() { g.transformSelection(0, 12, false) }),
		widget.NewButton("Delete", g.deleteSelection))
}
