package desktop

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
)

func TestGraphicalEditingHistoryAndTrackIdentity(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	w.Resize(fyne.NewSize(1320, 780))
	id := u.selected
	original := u.selectedTrack().Patterns[0].Steps[0].Active
	test.TapAt(u.grid, fyne.NewPos(nameWidth+8, 46))
	if w.Canvas().Focused() != u.grid {
		t.Fatal("step editing did not focus the keyboard editor")
	}
	if u.selectedTrack().Patterns[0].Steps[0].Active == original {
		t.Fatal("graphical step tap did not edit pattern")
	}
	u.undo()
	if u.selectedTrack().Patterns[0].Steps[0].Active != original {
		t.Fatal("GUI undo did not restore step")
	}
	u.duplicateTrack()
	if len(u.session.Project.Tracks) != 7 || u.selected == id {
		t.Fatal("duplicate did not create independent track identity")
	}
	u.removeTrack()
	if len(u.session.Project.Tracks) != 6 {
		t.Fatal("remove failed")
	}
	u.undo()
	if len(u.session.Project.Tracks) != 7 {
		t.Fatal("track removal cannot be undone")
	}
	u.applyScheme(theme.Bundled()["gruvbox-dark"])
	if u.scheme.Name != "Gruvbox Dark" {
		t.Fatal("GUI theme did not switch")
	}
}

func TestThemePreviewCancelAndLightTheme(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	w.Resize(fyne.NewSize(1320, 780))
	project := u.session.Project.Clone()
	original := u.scheme
	u.settings()
	var selector *widget.Select
	var cancel *widget.Button
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch item := obj.(type) {
		case *widget.PopUp:
			walk(item.Content)
		case *fyne.Container:
			for _, child := range item.Objects {
				walk(child)
			}
		case *widget.Select:
			selector = item
		case *widget.Button:
			if item.Text == "Cancel" {
				cancel = item
			}
		}
	}
	conf := u.panels["Conf / Appearance"]
	if conf == nil || conf == w {
		t.Fatal("Conf did not create a separate desktop window")
	}
	walk(conf.Content())
	if selector == nil || cancel == nil {
		t.Fatal("appearance dialog controls missing")
	}
	selector.SetSelected("gruvbox-dark")
	if u.scheme.Name != "Gruvbox Dark" {
		t.Fatal("preview did not change appearance")
	}
	test.Tap(cancel)
	if u.scheme != original || !reflect.DeepEqual(project, u.session.Project) {
		t.Fatal("cancel did not restore theme or changed musical state")
	}
	data, err := os.ReadFile("../theme/testdata/light.yaml")
	if err != nil {
		t.Fatal(err)
	}
	light, err := theme.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	u.applyScheme(light)
	if !reflect.DeepEqual(project, u.session.Project) {
		t.Fatal("light theme changed musical state")
	}
}

func TestSamplerWaveformTrimAndUndo(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("sampler")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	w.Resize(fyne.NewSize(1320, 780))
	var wav bytes.Buffer
	wav.WriteString("RIFF")
	binary.Write(&wav, binary.LittleEndian, uint32(36+2000))
	wav.WriteString("WAVEfmt ")
	binary.Write(&wav, binary.LittleEndian, uint32(16))
	binary.Write(&wav, binary.LittleEndian, uint16(1))
	binary.Write(&wav, binary.LittleEndian, uint16(1))
	binary.Write(&wav, binary.LittleEndian, uint32(44100))
	binary.Write(&wav, binary.LittleEndian, uint32(88200))
	binary.Write(&wav, binary.LittleEndian, uint16(2))
	binary.Write(&wav, binary.LittleEndian, uint16(16))
	wav.WriteString("data")
	binary.Write(&wav, binary.LittleEndian, uint32(2000))
	for i := 0; i < 1000; i++ {
		binary.Write(&wav, binary.LittleEndian, int16((i%100-50)*500))
	}
	sample, err := groove.LoadSample("wave.wav", &wav)
	if err != nil {
		t.Fatal(err)
	}
	track := groove.NewTrack(groove.Sampler, 6)
	u.edit("Add sampler", func(p *groove.Project) error { p.Tracks = append(p.Tracks, track); u.selected = track.ID; return nil })
	u.setSample(track.ID, sample)
	var wave *sampleWave
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *sampleWave:
			wave = o
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(u.inspectorTabs.Items[0].Content)
	if wave == nil {
		t.Fatal("sampler waveform missing")
	}
	wave.Resize(fyne.NewSize(256, 100))
	renderer := wave.CreateRenderer()
	renderer.Layout(wave.Size())
	content := renderer.Objects()[0].(*fyne.Container)
	endMarker := content.Objects[len(content.Objects)-1].(*canvas.Line)
	if endMarker.Position1.X != 240 || endMarker.Position2.X != 240 {
		t.Fatal("end marker must be inset from the scrollbar edge")
	}
	if wave.sourceAt(16) != 0 || wave.sourceAt(240) != 1 {
		t.Fatal("inset endpoints don't map to the whole sample")
	}
	renderer.Destroy()
	wave.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(72, 50)}, Dragged: fyne.NewDelta(56, 0)})
	if u.selectedTrack().Parameters["start"] != 0 {
		t.Fatal("drag changed project before release")
	}
	wave.DragEnd()
	if u.selectedTrack().Parameters["start"] != .25 {
		t.Fatal("waveform drag did not trim")
	}
	u.undo()
	if u.selectedTrack().Parameters["start"] != 0 {
		t.Fatal("trim cannot be undone")
	}

	wave.setView(sampleView{.25, .75})
	wave.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(72, 50)}, Dragged: fyne.NewDelta(56, 0)})
	wave.DragEnd()
	if u.selectedTrack().Parameters["start"] != .375 {
		t.Fatal("zoomed trim did not map viewport to source")
	}
	u.undo()
	if u.sampleViews[track.ID] != (sampleView{.25, .75}) {
		t.Fatal("inspector refresh lost waveform view")
	}
	wave.pan(1)
	if wave.bounds() != (sampleView{.5, 1}) {
		t.Fatal("waveform pan bounds")
	}
	for i := 0; i < 15; i++ {
		wave.zoom(.5)
	}
	if wave.bounds().end-wave.bounds().start < 32/float64(sample.Frames())-1e-9 {
		t.Fatal("zoom went below useful source resolution")
	}
	u.setSample(track.ID, nil)
	if u.selectedTrack().Sample != nil {
		t.Fatal("clear sample failed")
	}
	u.undo()
	if u.selectedTrack().Sample != sample {
		t.Fatal("clear sample cannot be undone")
	}
	u.applyScheme(theme.Bundled()["gruvbox-dark"])
	if u.selectedTrack().Sample != sample {
		t.Fatal("theme switch changed sample")
	}

	// Sample processing uses the same live gesture and undo boundary as synths.
	panel := u.samplePanel(u.selectedTrack()).(*fyne.Container)
	var fade *widget.Slider
	var normalize *widget.Check
	for _, obj := range panel.Objects {
		if check, ok := obj.(*widget.Check); ok && check.Text == "Normalize (95% peak)" {
			normalize = check
		}
	}
	fade = findSlider(panel, "Fade in (ms)")
	if fade == nil || normalize == nil {
		t.Fatal("sample processing controls missing")
	}
	fade.Resize(fyne.NewSize(300, 36))
	fade.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(100, 18)}, Dragged: fyne.NewDelta(100, 0)})
	if u.selectedTrack().Parameters["fade_in"] != fade.Value || fade.Value == 0 {
		t.Fatal("sample fade slider is not live")
	}
	fade.DragEnd()
	u.undo()
	if u.selectedTrack().Parameters["fade_in"] != 0 {
		t.Fatal("sample fade undo failed")
	}
	normalize.OnChanged(true)
	if u.selectedTrack().Parameters["normalize"] != 1 || u.selectedTrack().Sample != sample {
		t.Fatal("normalization changed original asset")
	}
	u.undo()
	if u.selectedTrack().Parameters["normalize"] != 0 {
		t.Fatal("normalization undo failed")
	}
}

func TestNativeWindowCloseAndSoundDiscovery(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	before := u.session.Project.Clone()
	u.key(fyne.KeyR)
	if reflect.DeepEqual(before.Tracks[0].Parameters, u.selectedTrack().Parameters) || !reflect.DeepEqual(before.Tracks[0].Patterns, u.selectedTrack().Patterns) {
		t.Fatal("R failed to discover a sound independently of pattern")
	}
	u.undo()
	if !reflect.DeepEqual(before, u.session.Project) {
		t.Fatal("sound undo failed")
	}
	original := u.scheme
	u.settings()
	conf := u.panels["Conf / Appearance"]
	u.settings()
	if u.panels["Conf / Appearance"] != conf {
		t.Fatal("duplicate Conf window")
	}
	u.applyScheme(theme.Bundled()["gruvbox-dark"])
	conf.Close()
	if u.scheme != original || u.panels["Conf / Appearance"] != nil {
		t.Fatal("native Conf close did not cancel preview")
	}
	calls := 0
	u.confirm("confirm", "Apply", "Cancel", widget.NewLabel("test"), func(ok bool) {
		calls++
		if ok {
			t.Fatal("native close accepted confirmation")
		}
	})
	u.modals["confirm"].Close()
	if calls != 1 {
		t.Fatal("confirmation close did not cancel exactly once")
	}
}

func TestAddTrackDropdownAddsDirectlyAndUndoes(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	w.Resize(fyne.NewSize(1320, 780))
	test.Tap(u.addTrackButton)
	if len(u.panels) != 0 {
		t.Fatal("Add Track opened a utility window")
	}
	overlays := w.Canvas().Overlays().List()
	if len(overlays) == 0 {
		t.Fatal("instrument menu missing")
	}
	var popup *widget.PopUpMenu
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.PopUpMenu:
			popup = o
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		case fyne.Widget:
			for _, child := range test.WidgetRenderer(o).Objects() {
				walk(child)
			}
		}
	}
	walk(overlays[len(overlays)-1])
	if popup == nil {
		t.Fatal("Add Track did not create a dropdown menu")
	}
	items := popup.Menu.Items
	test.Tap(items[len(items)-1].(fyne.Tappable))
	if len(u.session.Project.Tracks) != 7 || u.selectedTrack().Kind != groove.Sampler {
		t.Fatal("instrument selection did not add sampler directly")
	}
	u.undo()
	if len(u.session.Project.Tracks) != 6 {
		t.Fatal("direct add cannot be undone")
	}
}

func TestPlayheadFollowsAudioWithoutRebuildingGrid(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	w.Resize(fyne.NewSize(1320, 780))
	p := u.session.Project.Clone()
	p.BPM = 300
	for i := range p.Tracks {
		p.Tracks[i].Patterns[0].Resolution = 32
	}
	u.session = groove.NewSession(p)
	u.player.Publish(p)
	u.refresh()
	u.player.SetPlaying(true)
	r := u.grid.renderer
	if r == nil || len(r.heads) != len(p.Tracks) {
		t.Fatal("playhead renderer missing")
	}
	firstObject := r.content.Objects[0]
	firstMarker := r.heads[0].marker
	seen := map[int]bool{}
	var block [735 * 2]float32 // 44100 Hz / 60 GUI frames per second
	for frame := 0; frame < 60; frame++ {
		u.player.Render(block[:])
		u.grid.advancePlayhead()
		position := u.player.Position(p.Tracks[0].ID)
		seen[position] = true
		expected := fyne.NewPos(r.heads[0].left+float32(position)*r.heads[0].width+3, r.heads[0].y)
		if !firstMarker.Visible() || firstMarker.Position() != expected {
			t.Fatal("marker did not follow audio position")
		}
		if r.column == nil || !r.column.Visible() {
			t.Fatal("step-number column highlight not shown while playing")
		}
		if firstObject != r.content.Objects[0] || firstMarker != r.heads[0].marker {
			t.Fatal("animation rebuilt static grid")
		}
	}
	if len(seen) != 16 {
		t.Fatalf("playhead skipped steps: %v", seen)
	}
	u.page = 1
	u.grid.advancePlayhead()
	if firstMarker.Visible() {
		t.Fatal("off-page marker remained visible")
	}
	u.page = 0
	u.player.SetPlaying(false)
	u.grid.advancePlayhead()
	if firstMarker.Visible() {
		t.Fatal("stopped marker remained visible")
	}
}

func TestEffectsDiscoveryHistoryAndControls(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	w.Resize(fyne.NewSize(1320, 780))
	before := u.session.Project.Clone()
	u.inspectorTabs.SelectIndex(4)
	u.randomiseEffects()
	if len(u.selectedTrack().Effects) == 0 || u.inspectorTabs.SelectedIndex() != 4 {
		t.Fatal("FX discovery did not populate selected chain")
	}
	if !reflect.DeepEqual(before.Tracks[0].Patterns, u.selectedTrack().Patterns) {
		t.Fatal("FX discovery changed notes")
	}
	var bypass *widget.Check
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Check:
			if o.Text == "Bypass entire chain" {
				bypass = o
			}
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(u.inspectorTabs.Items[4].Content)
	if bypass == nil {
		t.Fatal("FX bypass control missing")
	}
	test.TapAt(bypass, fyne.NewPos(10, bypass.Size().Height/2))
	if !u.selectedTrack().EffectsBypassed {
		t.Fatal("FX bypass was not saved")
	}
	u.undo()
	if u.selectedTrack().EffectsBypassed {
		t.Fatal("FX bypass undo failed")
	}
	u.undo()
	if !reflect.DeepEqual(before, u.session.Project) {
		t.Fatal("FX discovery undo failed")
	}
	u.addEffect(groove.Delay)
	if len(u.selectedTrack().Effects) != 1 || u.selectedTrack().Effects[0].Kind != groove.Delay {
		t.Fatal("FX Add failed")
	}
}

func TestLiveResonanceSliderPublishesBeforeReleaseAndGroupsUndo(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	w.Resize(fyne.NewSize(1320, 780))
	var sliders []*widget.Slider
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Slider:
			sliders = append(sliders, o)
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(u.inspectorTabs.Items[0].Content)
	if len(sliders) < 2 {
		t.Fatal("303 resonance slider missing")
	}
	slider := sliders[1]
	tabs := u.inspectorTabs
	initial := u.selectedTrack().Parameters["resonance"]
	u.player.SetPlaying(true)
	var out [groove.BlockSize * 2]float32
	for _, fraction := range []float32{.2, .4, .7, .9} {
		slider.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(slider.Size().Width*fraction, slider.Size().Height/2)}})
		value := slider.Value
		if math.Abs(u.selectedTrack().Parameters["resonance"]-value) > 1e-9 {
			t.Fatal("resonance waited for mouse release")
		}
		if u.inspectorTabs != tabs {
			t.Fatal("live sweep rebuilt slider under pointer")
		}
		u.player.Render(out[:])
		if u.player.Playing() == false {
			t.Fatal("sweep interrupted playback")
		}
	}
	final := slider.Value
	slider.DragEnd()
	u.undo()
	if u.selectedTrack().Parameters["resonance"] != initial {
		t.Fatal("sweep undo restored an intermediate value")
	}
	u.redo()
	if u.selectedTrack().Parameters["resonance"] != final {
		t.Fatal("sweep redo failed")
	}
	// Keyboard/tap adjustments also apply immediately, and toolbar refresh must
	// not manufacture a swing gesture by setting the control programmatically.
	u.swing.SetValue(20)
	if u.session.Project.Swing != .2 {
		t.Fatal("swing slider is not live")
	}
	u.swing.OnChangeEnded(u.swing.Value)
	u.refresh()
	if u.liveSlider != nil {
		t.Fatal("refresh started an unintended gesture")
	}
}
