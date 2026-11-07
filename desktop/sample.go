package desktop

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/config"
	"github.com/subbaan/gruvbox/groove"
)

// wavFiles is the type filter for audio.
var wavFiles = fileKind{Name: "WAV audio", Exts: []string{".wav", ".wave"}}

func (u *UI) loadSample(id string) {
	session := u.session
	u.chooseFile(fileSpec{title: "Load sample", accept: "Load", mode: openFile, kinds: []fileKind{wavFiles}, dir: config.SamplesDir, recent: "samples"}, func(path string) {
		u.status.SetText("Loading WAV…")
		go func() {
			f, err := os.Open(path)
			var sample *groove.Sample
			if err == nil {
				sample, err = groove.LoadSample(filepath.Base(path), f)
				f.Close()
			}
			fyne.Do(func() {
				if session != u.session || session.Project.Track(id) == nil {
					return
				}
				if err != nil {
					u.status.SetText("Sample load failed")
					u.showError(err)
					return
				}
				u.setSample(id, sample)
				u.rememberFile("samples", path)
			})
		}()
	})
}

func (u *UI) setSample(id string, sample *groove.Sample) {
	u.edit("Load sample", func(p *groove.Project) error {
		t := p.Track(id)
		if t == nil || t.Kind != groove.Sampler {
			return fmt.Errorf("sampler track no longer exists")
		}
		t.Sample = sample
		delete(u.sampleViews, id)
		t.Parameters["start"], t.Parameters["end"] = 0, 1
		t.Parameters["gain_db"], t.Parameters["normalize"], t.Parameters["fade_in"], t.Parameters["fade_out"] = 0, 0, 0, 0
		return nil
	})
}

func (u *UI) samplePanel(t *groove.Track) fyne.CanvasObject {
	id := t.ID
	panel := container.NewVBox(container.NewGridWithColumns(2, widget.NewButton("Load WAV…", func() { u.loadSample(id) }), widget.NewButton("Record…", func() { u.recordSample(id) })))
	if t.Sample == nil {
		panel.Add(widget.NewLabel("No sample loaded · WAV up to 16 MiB"))
		return panel
	}
	panel.Add(container.NewHBox(widget.NewButton("Audition", func() {
		if u.audioError != nil {
			u.showError(u.audioError)
			return
		}
		if err := u.player.Audition(*u.session.Project.Track(id)); err != nil {
			u.showError(err)
		}
	}), widget.NewButton("Stop preview", u.player.StopAudition)))
	label := widget.NewLabel(t.Sample.Name())
	label.Wrapping = fyne.TextWrapWord
	panel.Add(label)
	panel.Add(widget.NewLabel(fmt.Sprintf("%.2f s · %d Hz · %d channel(s)", float64(t.Sample.Frames())/float64(t.Sample.Rate()), t.Sample.Rate(), t.Sample.Channels())))
	wave := &sampleWave{ui: u, sample: t.Sample, start: t.Parameters["start"], end: t.Parameters["end"]}
	wave.ExtendBaseWidget(wave)
	wave.viewID = id
	wave.setView(u.sampleViews[id])
	panel.Add(container.NewPadded(wave))
	panel.Add(container.NewHBox(widget.NewButton("Zoom +", func() { wave.zoom(.5) }), widget.NewButton("Zoom −", func() { wave.zoom(2) }), widget.NewButton("‹", func() { wave.pan(-1) }), widget.NewButton("›", func() { wave.pan(1) })))
	panel.Add(container.NewHBox(widget.NewButton("Fit trim", func() { wave.setView(sampleView{wave.start, wave.end}); wave.Refresh() }), widget.NewButton("Full", func() { wave.setView(sampleView{0, 1}); wave.Refresh() })))
	commit := func(param string, value float64) {
		u.edit("Sample "+param, func(p *groove.Project) error { p.Track(id).Parameters[param] = value; return nil })
	}
	param := func(key string) float64 { return u.session.Project.Track(id).Parameters[key] }
	// Keep the waveform's markers in step with undo/redo and slider edits.
	u.bound(func() {
		if wave.dragging {
			return
		}
		if start, end := param("start"), param("end"); wave.start != start || wave.end != end {
			wave.start, wave.end = start, end
			wave.Refresh()
		}
	})
	gap := 1 / float64(t.Sample.Frames())
	wave.commit = func(start, end float64) {
		u.edit("Sample trim", func(p *groove.Project) error {
			t := p.Track(id)
			t.Parameters["start"], t.Parameters["end"] = start, end
			return nil
		})
	}
	panel.Add(widget.NewLabel("Drag the nearest marker to trim"))
	panel.Add(u.boundSlider("Start (%)", 0, 100, .01, func() float64 { return param("start") * 100 }, func(v float64) {
		value := min(v/100, max(0, param("end")-gap))
		commit("start", value)
		wave.start = value
		wave.Refresh()
	}))
	panel.Add(u.boundSlider("End (%)", 0, 100, .01, func() float64 { return param("end") * 100 }, func(v float64) {
		value := max(v/100, min(1, param("start")+gap))
		commit("end", value)
		wave.end = value
		wave.Refresh()
	}))
	flag := func(on bool) float64 {
		if on {
			return 1
		}
		return 0
	}
	panel.Add(container.NewBorder(nil, nil, widget.NewLabel("Playback"), nil, u.boundSelect([]string{"One-shot", "Gate", "Loop"}, func() int { return int(param("mode")) }, func(i int) { commit("mode", float64(i)) })))
	panel.Add(u.boundCheck("Reverse", func() bool { return param("reverse") >= .5 }, func(v bool) { commit("reverse", flag(v)) }))
	panel.Add(u.boundCheck("Normalize (95% peak)", func() bool { return param("normalize") >= .5 }, func(v bool) { commit("normalize", flag(v)) }))
	for _, definition := range groove.Parameters(groove.Sampler) {
		if definition.ID != "gain_db" && definition.ID != "fade_in" && definition.ID != "fade_out" {
			continue
		}
		panel.Add(u.instrumentParam(id, definition, nil, nil, nil))
	}
	panel.Add(u.withHelp(widget.NewButton("Reset gain / fades", func() {
		u.edit("Reset sample processing", func(p *groove.Project) error {
			params := p.Track(id).Parameters
			params["gain_db"], params["normalize"], params["fade_in"], params["fade_out"] = 0, 0, 0, 0
			return nil
		})
	}), "Fades follow the trimmed region, including reverse. Times are at the sample’s original pitch. The original WAV is preserved."))
	panel.Add(widget.NewLabel("Root note C2 · samples saved inside project"))
	panel.Add(widget.NewButton("Reset trim", func() {
		u.edit("Reset sample trim", func(p *groove.Project) error {
			t := p.Track(id)
			t.Parameters["start"], t.Parameters["end"] = 0, 1
			return nil
		})
	}))
	panel.Add(widget.NewButton("Clear sample", func() { u.setSample(id, nil) }))
	return panel
}

type sampleWave struct {
	widget.BaseWidget
	viewID              string
	view                sampleView
	peaks               [groove.WaveformBins][2]float32
	ui                  *UI
	sample              *groove.Sample
	start, end          float64
	commit              func(float64, float64)
	dragging, dragStart bool
}

// Keep endpoint markers away from both the widget edge and the inspector scrollbar.
const sampleWaveInset float32 = 16

func (w *sampleWave) plotBounds(width float32) (float32, float32) {
	inset := min(sampleWaveInset, max(0, width/4))
	return inset, max(1, width-2*inset)
}
func (w *sampleWave) sourceAt(x float32) float64 {
	left, width := w.plotBounds(w.Size().Width)
	view := w.bounds()
	return view.start + float64((x-left)/width)*(view.end-view.start)
}

func (w *sampleWave) Dragged(e *fyne.DragEvent) {
	if w.Size().Width <= 0 {
		return
	}
	pos := max(0, min(1, w.sourceAt(e.Position.X)))
	if !w.dragging {
		initial := w.sourceAt(e.Position.X - e.Dragged.DX)
		w.dragStart = math.Abs(initial-w.start) <= math.Abs(initial-w.end)
		w.dragging = true
	}
	gap := 1 / float64(w.sample.Frames())
	if w.dragStart {
		w.start = min(pos, max(0, w.end-gap))
	} else {
		w.end = max(pos, min(1, w.start+gap))
	}
	w.Refresh()
}
func (w *sampleWave) DragEnd() {
	if w.dragging && w.commit != nil {
		w.dragging = false
		w.commit(w.start, w.end)
	}
}

func (w *sampleWave) CreateRenderer() fyne.WidgetRenderer {
	r := &sampleWaveRenderer{wave: w, content: container.NewWithoutLayout()}
	r.Refresh()
	return r
}

type sampleWaveRenderer struct {
	wave    *sampleWave
	content *fyne.Container
}

func (r *sampleWaveRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
	r.draw(size)
}
func (r *sampleWaveRenderer) MinSize() fyne.Size           { return fyne.NewSize(240, 100) }
func (r *sampleWaveRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.content} }
func (r *sampleWaveRenderer) Destroy()                     {}
func (r *sampleWaveRenderer) Refresh()                     { r.draw(r.wave.Size()); r.content.Refresh() }
func (r *sampleWaveRenderer) draw(size fyne.Size) {
	w := r.wave
	roles := w.ui.scheme.Roles()
	r.content.Objects = nil
	bg := canvas.NewRectangle(colour(roles.Panel))
	bg.Resize(size)
	r.content.Add(bg)
	view := w.bounds()
	left, width := w.plotBounds(size.Width)
	peaks := w.peaks
	if w.view.end == 0 {
		peaks = w.sample.Peaks()
	}
	for i, p := range peaks {
		x := left + float32(i)*width/groove.WaveformBins
		c := roles.Waveform
		position := view.start + float64(i)/groove.WaveformBins*(view.end-view.start)
		if position < w.start || position >= w.end {
			c = roles.Disabled
		}
		line := canvas.NewLine(colour(c))
		line.StrokeWidth = 1
		line.Position1 = fyne.NewPos(x, size.Height*(.5-p[1]*.45))
		line.Position2 = fyne.NewPos(x, size.Height*(.5-p[0]*.45))
		r.content.Add(line)
	}
	for _, pos := range []float64{w.start, w.end} {
		if pos < view.start || pos > view.end {
			continue
		}
		pos = (pos - view.start) / (view.end - view.start)
		line := canvas.NewLine(colour(roles.Marker))
		line.StrokeWidth = 2
		line.Position1 = fyne.NewPos(left+float32(pos)*width, 0)
		line.Position2 = fyne.NewPos(left+float32(pos)*width, size.Height)
		r.content.Add(line)
	}
}
