package desktop

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	timelineHeight   float32 = 62
	timelineMinBar   float32 = 14
	timelineMaxBar   float32 = 48
	timelineBlockGap float32 = 2
)

// songTimeline draws the arrangement as one block per section, its width
// proportional to the section's bars. Clicking a block selects that section's
// row; while the song plays, the current section is lit and a marker shows the
// bar.
type songTimeline struct {
	widget.BaseWidget
	ui       *UI
	renderer *timelineRenderer
}

func newSongTimeline(u *UI) *songTimeline {
	t := &songTimeline{ui: u}
	t.ExtendBaseWidget(t)
	return t
}

// barWidth fits the whole song into width when it can, within limits.
func (t *songTimeline) barWidth(width float32) float32 {
	bars := 0
	for _, s := range t.ui.session.Project.Song {
		bars += max(1, s.Repeats)
	}
	if bars == 0 {
		return timelineMaxBar
	}
	return min(timelineMaxBar, max(timelineMinBar, (width-8)/float32(bars)))
}

// sectionAt returns the index of the block under x, or -1.
func (t *songTimeline) sectionAt(x float32) int {
	bar := t.barWidth(t.Size().Width)
	left := float32(4)
	for i, s := range t.ui.session.Project.Song {
		w := float32(max(1, s.Repeats)) * bar
		if x >= left && x < left+w {
			return i
		}
		left += w
	}
	return -1
}

func (t *songTimeline) Tapped(e *fyne.PointEvent) {
	if i := t.sectionAt(e.Position.X); i >= 0 {
		t.ui.selectSection(t.ui.session.Project.Song[i].ID)
	}
}

func (t *songTimeline) CreateRenderer() fyne.WidgetRenderer {
	r := &timelineRenderer{timeline: t, content: container.NewWithoutLayout()}
	t.renderer = r
	r.draw(t.Size())
	return r
}

// advance moves the playing overlay; it runs on the 60 Hz tick and only
// touches retained objects.
func (t *songTimeline) advance() {
	if t.renderer != nil {
		t.renderer.advance()
	}
}

type timelineRenderer struct {
	timeline *songTimeline
	content  *fyne.Container
	lit      *canvas.Rectangle
	marker   *canvas.Rectangle
	lefts    []float32
	bar      float32
}

func (r *timelineRenderer) Destroy()                     {}
func (r *timelineRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.content} }
func (r *timelineRenderer) Layout(size fyne.Size)        { r.content.Resize(size); r.draw(size) }
func (r *timelineRenderer) Refresh()                     { r.draw(r.timeline.Size()); r.content.Refresh() }
func (r *timelineRenderer) MinSize() fyne.Size {
	bars := 0
	for _, s := range r.timeline.ui.session.Project.Song {
		bars += max(1, s.Repeats)
	}
	return fyne.NewSize(8+float32(bars)*timelineMinBar, timelineHeight)
}

func (r *timelineRenderer) draw(size fyne.Size) {
	u := r.timeline.ui
	roles := u.scheme.Roles()
	var objects []fyne.CanvasObject
	add := func(o fyne.CanvasObject, x, y, w, h float32) {
		o.Move(fyne.NewPos(x, y))
		o.Resize(fyne.NewSize(w, h))
		objects = append(objects, o)
	}
	label := func(value string, x, y, width float32, c string, size float32, bold bool) {
		style := fyne.TextStyle{Bold: bold}
		if fyne.MeasureText(value, size, style).Width > width-8 {
			return
		}
		text := canvas.NewText(value, colour(c))
		text.TextSize, text.TextStyle = size, style
		text.Move(fyne.NewPos(x+5, y))
		objects = append(objects, text)
	}
	add(canvas.NewRectangle(colour(roles.Panel)), 0, 0, size.Width, timelineHeight)
	song := u.session.Project.Song
	r.bar = r.timeline.barWidth(size.Width)
	r.lefts = r.lefts[:0]
	if len(song) == 0 {
		label("No sections yet. Add current pattern to start the song.", 4, 20, size.Width, roles.Secondary, 13, false)
	}
	left := float32(4)
	for i, s := range song {
		bars := max(1, s.Repeats)
		w := float32(bars) * r.bar
		r.lefts = append(r.lefts, left)
		fill := mixHex(roles.Tracks[s.Pattern%len(roles.Tracks)], roles.Panel, .25)
		block := canvas.NewRectangle(colour(fill))
		block.CornerRadius = 3
		if s.ID == u.selectedSection {
			block.StrokeColor, block.StrokeWidth = colour(roles.Focus), 2
		}
		add(block, left+1, 6, w-timelineBlockGap, timelineHeight-12)
		// Bar ticks help read lengths at a glance.
		for b := 1; b < bars; b++ {
			tick := canvas.NewRectangle(translucent(u.scheme.TextOn(fill), 50))
			add(tick, left+float32(b)*r.bar, timelineHeight-14, 1, 6)
		}
		ink := u.scheme.TextOn(fill)
		label(fmt.Sprintf("%02d %s", i+1, s.Name), left, 10, w, ink, 12, true)
		label(fmt.Sprintf("P%02d · %d bar%s", s.Pattern+1, bars, map[bool]string{true: "", false: "s"}[bars == 1]), left, 28, w, ink, 10, false)
		left += w
	}
	r.lit = canvas.NewRectangle(translucent(roles.Playback, 60))
	r.lit.StrokeColor, r.lit.StrokeWidth = colour(roles.Playback), 2
	r.lit.CornerRadius = 3
	r.lit.Hide()
	r.marker = canvas.NewRectangle(colour(roles.Playback))
	r.marker.Resize(fyne.NewSize(3, timelineHeight-8))
	r.marker.Hide()
	objects = append(objects, r.lit, r.marker)
	r.content.Objects = objects
	r.advance()
}

func (r *timelineRenderer) advance() {
	u := r.timeline.ui
	section := u.player.SongSection()
	song := u.session.Project.Song
	if !u.player.Playing() || section < 0 || section >= len(song) || section >= len(r.lefts) || r.lit == nil {
		if r.lit != nil && r.lit.Visible() {
			r.lit.Hide()
			r.marker.Hide()
		}
		return
	}
	bars := max(1, song[section].Repeats)
	left, w := r.lefts[section], float32(bars)*r.bar
	if pos, size := fyne.NewPos(left+1, 6), fyne.NewSize(w-timelineBlockGap, timelineHeight-12); r.lit.Position() != pos || r.lit.Size() != size {
		r.lit.Move(pos)
		r.lit.Resize(size)
	}
	bar := min(bars, max(1, u.player.SongBar()))
	if pos := fyne.NewPos(left+float32(bar-1)*r.bar+1, 4); r.marker.Position() != pos {
		r.marker.Move(pos)
	}
	if !r.lit.Visible() {
		r.lit.Show()
		r.marker.Show()
	}
}

// selectSection highlights a section in the timeline and its row, and
// scrolls the rows so it is in view.
func (u *UI) selectSection(id string) {
	if u.session.Project.Section(id) == nil {
		return
	}
	u.selectedSection = id
	u.refreshSong()
	if u.songScroll == nil {
		return
	}
	for i, s := range u.session.Project.Song {
		if s.ID == id && i+1 < len(u.songPanel.Objects) {
			row := u.songPanel.Objects[i+1]
			view := u.songScroll.Size().Height
			if y := row.Position().Y; y < u.songScroll.Offset.Y || y+row.Size().Height > u.songScroll.Offset.Y+view {
				u.songScroll.ScrollToOffset(fyne.NewPos(0, max(0, y-view/3)))
			}
		}
	}
}
