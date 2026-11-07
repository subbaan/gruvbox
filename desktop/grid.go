package desktop

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

const rowHeight float32 = 44

// The piano roll shows three octaves in compact rows.
const (
	rollRows      = 36
	rollRowHeight = float32(22)
)

// rowH is the row pitch for this grid.
func (g *sequencerGrid) rowH() float32 {
	if g.roll {
		return rollRowHeight
	}
	return rowHeight
}

// cellBox is a step cell's vertical inset and height inside its row.
func (g *sequencerGrid) cellBox() (inset, height float32) {
	if g.roll {
		return 2, rollRowHeight - 4
	}
	return 5, rowHeight - 11
}

const nameWidth float32 = 220

// Hold, mute and solo hit areas in the track-name column.
const holdLeft, muteLeft, soloLeft float32 = 124, 154, 184
const playheadInterval = time.Second / 60

type sequencerGrid struct {
	widget.BaseWidget
	ui               *UI
	roll             bool
	renderer         *gridRenderer
	noteTop          int
	rollKey          string // track and pattern the roll last fitted its range to
	followNote       int    // note to scroll to once laid out, or -1
	drag             *noteDrag
	selection        map[int]bool
	selectionSession *groove.Session
	selectionTrack   string
	selectionPattern int
	suppressTap      bool
}

func newGrid(ui *UI, roll bool) *sequencerGrid {
	g := &sequencerGrid{ui: ui, roll: roll, noteTop: 59, followNote: -1}
	g.ExtendBaseWidget(g)
	return g
}
func (g *sequencerGrid) Tapped(e *fyne.PointEvent) {
	u := g.ui
	u.window.Canvas().Focus(g)
	if e.Position.Y < 34 {
		return
	}
	row := int((e.Position.Y - 34) / g.rowH())
	if g.roll {
		if g.suppressTap {
			g.suppressTap = false
			return
		}
		track := u.selectedTrack()
		if track == nil || !track.Kind.Capabilities().Notes {
			return
		}
		if row >= rollRows || e.Position.X < 60 {
			return
		}
		step := u.page*16 + int((e.Position.X-60)/((g.Size().Width-60)/16))
		if step >= len(track.Patterns[u.session.Project.Pattern].Steps) {
			return
		}
		id, note := track.ID, g.noteTop-row
		if owner := g.noteOwner(step, note); owner >= 0 {
			step = owner
		}
		g.clearSelection()
		u.selectedStep = step
		if u.lockMode {
			u.refresh()
			return
		}
		u.edit("Edit note", func(p *groove.Project) error {
			s := &p.Track(id).Patterns[p.Pattern].Steps[step]
			if s.Active && s.Note == note {
				s.Active = false
			} else {
				s.Active = true
				s.Note = note
			}
			return nil
		})
		return
	}
	if row >= len(u.session.Project.Tracks) {
		return
	}
	track := u.session.Project.Tracks[row]
	u.selected = track.ID
	if e.Position.X < nameWidth {
		if e.Position.X >= holdLeft && e.Position.X < muteLeft {
			u.toggleTrackHold(track.ID)
		} else if e.Position.X >= muteLeft && e.Position.X < soloLeft {
			u.toggleMute()
		} else if e.Position.X >= soloLeft && e.Position.X < soloLeft+30 {
			u.toggleSolo()
		} else {
			u.refresh()
		}
		return
	}
	step := u.page*16 + int((e.Position.X-nameWidth)/((g.Size().Width-nameWidth)/16))
	if step < 0 || step >= len(track.Patterns[u.session.Project.Pattern].Steps) {
		u.refresh()
		return
	}
	u.selectedStep = step
	if u.lockMode {
		// In lock mode a click picks the step to lock rather than toggling it.
		u.refresh()
		return
	}
	u.edit("Toggle step", func(p *groove.Project) error {
		s := &p.Track(track.ID).Patterns[p.Pattern].Steps[step]
		s.Active = !s.Active
		return nil
	})
}
func (g *sequencerGrid) CreateRenderer() fyne.WidgetRenderer {
	r := &gridRenderer{grid: g, content: container.NewWithoutLayout()}
	g.renderer = r
	r.Refresh()
	return r
}

type gridRenderer struct {
	grid    *sequencerGrid
	content *fyne.Container
	heads   []playhead
	column  *canvas.Rectangle // header highlight under the playing step number
}

type playhead struct {
	trackID        string
	marker         *canvas.Rectangle
	left, width, y float32
	length         int
}

// Playback moves retained rectangles. Note cells and text are rebuilt only
// after edits, layout changes or theme changes, not on every animation frame.
func (g *sequencerGrid) advancePlayhead() {
	if r := g.renderer; r != nil {
		r.advancePlayhead()
	}
}
func (r *gridRenderer) advancePlayhead() {
	u := r.grid.ui
	place := func(marker *canvas.Rectangle, trackID string, length int, left, width, y, inset float32) bool {
		index := u.player.Position(trackID)
		column := index - u.page*16
		if !u.player.Playing() || u.player.CurrentPattern() != u.session.Project.Pattern || column < 0 || column >= 16 || index >= length {
			if marker.Visible() {
				marker.Hide()
			}
			return false
		}
		pos := fyne.NewPos(left+float32(column)*width+inset, y)
		if marker.Position() != pos {
			marker.Move(pos)
		}
		if !marker.Visible() {
			marker.Show()
		}
		return true
	}
	header := false
	for _, h := range r.heads {
		shown := place(h.marker, h.trackID, h.length, h.left, h.width, h.y, 3)
		// The step-number highlight follows the selected track.
		if r.column != nil && !header && (h.trackID == u.selected || r.grid.roll) {
			header = place(r.column, h.trackID, h.length, h.left, h.width, 5, 1) && shown
		}
	}
	if r.column != nil && !header && r.column.Visible() {
		r.column.Hide()
	}
}

// Secondary click selects a step for inspection without changing its content.
func (g *sequencerGrid) TappedSecondary(e *fyne.PointEvent) {
	u := g.ui
	u.window.Canvas().Focus(g)
	if e.Position.Y < 34 {
		return
	}
	left := nameWidth
	if g.roll {
		left = 60
	} else {
		row := int((e.Position.Y - 34) / g.rowH())
		if row >= len(u.session.Project.Tracks) {
			return
		}
		u.selected = u.session.Project.Tracks[row].ID
		if e.Position.X < nameWidth {
			u.showTrackMenu(u.selected, e.AbsolutePosition)
			return
		}
	}
	if e.Position.X < left || u.selectedTrack() == nil {
		return
	}
	step := u.page*16 + int((e.Position.X-left)/((g.Size().Width-left)/16))
	if step < 0 || step >= len(u.selectedTrack().Patterns[u.session.Project.Pattern].Steps) {
		return
	}
	u.selectedStep = step
	u.refresh()
}

func (g *sequencerGrid) FocusGained() { g.Refresh() }
func (g *sequencerGrid) FocusLost()   { g.Refresh() }
func (g *sequencerGrid) TypedKey(e *fyne.KeyEvent) {
	if g.roll && len(g.selectedNotes()) > 0 {
		switch e.Name {
		case fyne.KeyLeft:
			g.transformSelection(-1, 0, false)
			return
		case fyne.KeyRight:
			g.transformSelection(1, 0, false)
			return
		case fyne.KeyUp:
			g.transformSelection(0, 1, false)
			return
		case fyne.KeyDown:
			g.transformSelection(0, -1, false)
			return
		case fyne.KeyDelete:
			g.deleteSelection()
			return
		case fyne.KeyEscape:
			g.clearSelection()
			return
		}
	}
	g.ui.key(e.Name)
}
func (g *sequencerGrid) TypedRune(rune) {}

func (r *gridRenderer) Destroy()                     {}
func (r *gridRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.content} }
func (r *gridRenderer) MinSize() fyne.Size {
	if r.grid.roll {
		return fyne.NewSize(760, 34+rollRows*rollRowHeight)
	}
	rows := len(r.grid.ui.session.Project.Tracks)
	return fyne.NewSize(760, 34+float32(max(rows, 6))*rowHeight)
}
func (r *gridRenderer) Layout(size fyne.Size) { r.content.Resize(size); r.draw(size) }
func (r *gridRenderer) Refresh()              { r.draw(r.grid.Size()); r.content.Refresh() }
func (r *gridRenderer) draw(size fyne.Size) {
	u, g := r.grid.ui, r.grid
	if g.roll {
		g.ensureSelection()
	}
	roles := u.scheme.Roles()
	var objects []fyne.CanvasObject
	r.heads = nil
	box := func(x, y, w, h float32, fill, border string) {
		rect := canvas.NewRectangle(colour(fill))
		if border != "" {
			rect.StrokeColor = colour(border)
			rect.StrokeWidth = 1
		}
		rect.Move(fyne.NewPos(x, y))
		rect.Resize(fyne.NewSize(w, h))
		objects = append(objects, rect)
	}
	text := func(value string, x, y float32, c color.Color, fontSize float32) {
		t := canvas.NewText(value, c)
		t.TextSize = fontSize
		t.TextStyle.Monospace = true
		t.Move(fyne.NewPos(x, y))
		objects = append(objects, t)
	}
	left := nameWidth
	if g.roll {
		left = 60
	}
	width := (max(size.Width, 760) - left) / 16
	r.column = canvas.NewRectangle(translucent(roles.Playback, 60))
	r.column.CornerRadius = 3
	r.column.Resize(fyne.NewSize(width-2, 22))
	r.column.Hide()
	objects = append(objects, r.column)
	text("TRACK", 12, 9, colour(roles.Secondary), 11)
	for n := 0; n < 16; n++ {
		text(fmt.Sprintf("%02d", u.page*16+n+1), left+float32(n)*width+6, 9, colour(roles.Secondary), 11)
	}
	tracks := u.session.Project.Tracks
	if g.roll && (u.selectedTrack() == nil || !u.selectedTrack().Kind.Capabilities().Notes) {
		text("Select a melodic or sampler track to edit notes.", 12, 50, colour(roles.Secondary), 13)
		r.content.Objects = objects
		return
	}
	rows := len(tracks)
	if g.roll {
		rows = rollRows
		g.fitNotes()
	}
	rh := g.rowH()
	inset, cellHeight := g.cellBox()
	for row := 0; row < rows; row++ {
		y := 34 + float32(row)*rh
		var track *groove.Track
		if g.roll {
			track = u.selectedTrack()
		} else {
			track = &tracks[row]
		}
		if track == nil {
			break
		}
		pattern := track.Patterns[u.session.Project.Pattern]
		tc := roles.Tracks[track.Colour]
		background := roles.Background
		if row%2 != 0 {
			background = roles.Alternate
		}
		if g.roll {
			// Rows follow the keyboard: black keys darker, a line above each B/C boundary.
			background = roles.Background
			if blackKey(g.noteTop - row) {
				background = roles.Alternate
			}
		} else if track.ID == u.selected {
			background = roles.Selection
		}
		box(0, y, max(size.Width, 760), rh-1, background, "")
		if g.roll {
			note := g.noteTop - row
			if note%12 == 11 && row > 0 {
				box(0, y, max(size.Width, 760), 1, roles.Border, "")
			}
			label := colour(roles.Disabled)
			if note%12 == 0 {
				label = colour(roles.Secondary)
			}
			text(noteName(note), 6, y+4, label, 11)
		} else {
			box(4, y+7, 3, rowHeight-14, tc, "")
			text(track.Name, 13, y+9, colour(roles.Text), 13)
			text(fmt.Sprintf("%d steps · 1/%d", len(pattern.Steps), pattern.Resolution), 13, y+27, colour(roles.Secondary), 9)
			// Hold/mute/solo chips sit over their hit areas (see Tapped).
			chip := func(label string, x float32, on bool, fill string) {
				background, foreground, border := roles.InactiveStep, roles.Secondary, roles.Border
				if on {
					background, foreground, border = fill, u.scheme.TextOn(fill), fill
				}
				box(x, y+10, 24, rowHeight-20, background, border)
				text(label, x+7, y+14, colour(foreground), 13)
			}
			chip("H", holdLeft+2, track.Hold, roles.Success)
			chip("M", muteLeft+2, track.Mute, roles.Error)
			chip("S", soloLeft+2, track.Solo, roles.Focus)
		}
		for n := 0; n < 16; n++ {
			index := u.page*16 + n
			if index >= len(pattern.Steps) {
				continue
			}
			step := pattern.Steps[index]
			active := step.Active && (!g.roll || step.Note == g.noteTop-row)
			fill := roles.InactiveStep
			if active {
				fill = velocityShade(tc, roles.InactiveStep, step.Velocity)
			}
			border := ""
			if index == u.selectedStep && track.ID == u.selected {
				border = roles.Focus
			}
			x := left + float32(n)*width + 3
			if g.roll {
				// Notes are drawn by drawNote below; this is the empty cell.
				empty := roles.InactiveStep
				if blackKey(g.noteTop - row) {
					empty = mixHex(roles.InactiveStep, roles.Background, .55)
				}
				box(x, y+inset, width-6, cellHeight, empty, "")
				continue
			}
			box(x, y+inset, width-6, cellHeight, fill, border)
			if !active && len(step.Locks) > 0 {
				// Mark locks on rests too; FX locks there can still shape effect tails.
				text("L", x+4, y+26, colour(roles.Secondary), 9)
			}

			if active {
				foreground := colour(u.scheme.TextOn(fill))
				label := "•"
				if track.Kind.Capabilities().Notes {
					label = noteName(step.Note)
				}
				text(label, x+4, y+9, foreground, 11)
				marks := ""
				if step.Accent {
					marks += "A"
				}
				if step.Slide {
					marks += "~"
				}
				if len(step.Locks) > 0 {
					marks += "L"
				}
				if step.Probability < 1 || step.Every > 1 {
					marks += "?"
				}
				text(marks, x+4, y+26, foreground, 9)
			}
		}
		if g.roll {
			drawNote := func(index int, step groove.Step, preview bool) {
				if !step.Active || step.Note != g.noteTop-row {
					return
				}
				length := 1
				if track.Kind.Capabilities().NoteLengths {
					length = step.GateSteps()
				}
				if d := g.drag; !preview && d != nil && d.moved && d.valid && d.resize && d.source == index {
					length = d.length
				}
				end := min(len(pattern.Steps), index+length)
				if !preview {
					for next := index + 1; next < end; next++ {
						if pattern.Steps[next].Active {
							end = next
							break
						}
					}
				}
				start := max(u.page*16, index)
				end = min(u.page*16+16, end)
				if start >= end {
					return
				}
				x := left + float32(start-u.page*16)*width + 3
				border := ""
				if g.selection[index] || preview {
					border = roles.Focus
				}
				fill := velocityShade(tc, roles.InactiveStep, step.Velocity)
				if !preview && index == u.selectedStep && border == "" {
					border = roles.Focus
				}
				box(x, y+inset, float32(end-start)*width-6, cellHeight, fill, border)
				text(noteName(step.Note), x+4, y+3, colour(u.scheme.TextOn(fill)), 10)
				marks := ""
				if step.Accent {
					marks += "A"
				}
				if step.Slide {
					marks += "~"
				}
				if len(step.Locks) > 0 {
					marks += "L"
				}
				if step.Probability < 1 || step.Every > 1 {
					marks += "?"
				}
				// Marks share the line with the note name in the compact roll.
				text(marks, x+28, y+4, colour(u.scheme.TextOn(fill)), 9)
				if track.Kind.Capabilities().NoteLengths && index+length == end {
					box(x+float32(end-start)*width-10, y+6, 2, rh-12, roles.Focus, "")
				}
			}
			for index, step := range pattern.Steps {
				drawNote(index, step, false)
			}
			if d := g.drag; d != nil && d.moved && d.valid && !d.resize {
				for _, index := range d.indices {
					step := pattern.Steps[index]
					step.Note += d.note - d.step.Note
					drawNote(index+d.target-d.source, step, true)
				}
			}
		}
		// The playing cell gets a lit overlay and outline.
		marker := canvas.NewRectangle(translucent(roles.Playback, 70))
		marker.StrokeColor, marker.StrokeWidth = colour(roles.Playback), 2
		marker.Resize(fyne.NewSize(width-6, cellHeight))
		marker.Hide()
		objects = append(objects, marker)
		r.heads = append(r.heads, playhead{trackID: track.ID, marker: marker, left: left, width: width, y: y + inset, length: len(pattern.Steps)})
	}
	r.content.Objects = objects
	r.advancePlayhead()
}

// velocityShade fades a step's colour toward the empty-step colour as its
// velocity drops, keeping quiet hits visible.
func velocityShade(fill, empty string, velocity float64) string {
	return mixHex(fill, empty, (1-min(1, max(0, velocity)))*.6)
}

func blackKey(note int) bool {
	switch note % 12 {
	case 1, 3, 6, 8, 10:
		return true
	}
	return false
}

// fitNotes centres the roll's range on the selected track's notes whenever
// the track or pattern changes; octave buttons move it freely after that.
func (g *sequencerGrid) fitNotes() {
	u := g.ui
	t := u.selectedTrack()
	if t == nil {
		return
	}
	key := fmt.Sprintf("%s/%d", t.ID, u.session.Project.Pattern)
	if key == g.rollKey {
		return
	}
	g.rollKey = key
	low, high := 128, -1
	for _, step := range t.Patterns[u.session.Project.Pattern].Steps {
		if step.Active {
			low, high = min(low, step.Note), max(high, step.Note)
		}
	}
	if high < 0 {
		return
	}
	top := high
	if span := high - low + 1; span < rollRows {
		top = high + (rollRows-span)/2
	}
	g.noteTop = min(127, max(rollRows-1, top))
	g.followNote = (low + high) / 2
}

// followRoll scrolls the piano roll so the notes found by the last fit sit
// mid-view. It waits until the roll has been laid out (its tab shown).
func (u *UI) followRoll() {
	g, scroll := u.roll, u.rollScroll
	if g == nil || scroll == nil || g.followNote < 0 || scroll.Size().Height <= 0 {
		return
	}
	middle := 34 + (float32(g.noteTop-g.followNote)+.5)*rollRowHeight
	limit := max(0, 34+rollRows*rollRowHeight-scroll.Size().Height)
	scroll.Offset.Y = min(limit, max(0, middle-scroll.Size().Height/2))
	g.followNote = -1
	scroll.Refresh()
}

func noteName(note int) string {
	return fmt.Sprintf("%s%d", []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}[note%12], note/12-1)
}
