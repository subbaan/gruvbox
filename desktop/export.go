package desktop

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/config"
	"github.com/subbaan/gruvbox/groove"
	"os"
	"path/filepath"
	"strings"
)

func (u *UI) exportItems() []*fyne.MenuItem {
	return u.exportItemsFor("")
}

// exportItemsFor is the export menu for the whole mix (trackID "") or one
// track.
func (u *UI) exportItemsFor(trackID string) []*fyne.MenuItem {
	pattern, song := "Selected pattern…", "Full song…"
	if t := u.session.Project.Track(trackID); t != nil {
		pattern, song = t.Name+" only · selected pattern…", t.Name+" only · full song…"
	}
	return []*fyne.MenuItem{fyne.NewMenuItem(pattern, func() { u.exportWAV(false, trackID) }), fyne.NewMenuItem(song, func() { u.exportWAV(true, trackID) })}
}

// exportWAV renders the selected pattern or the song to a WAV file: the whole
// mix, or with trackID set, that track alone (soloed, through the master bus).
func (u *UI) exportWAV(song bool, trackID string) {
	if song && len(u.session.Project.Song) == 0 {
		u.showError(fmt.Errorf("add song sections before exporting a song"))
		return
	}
	title := "Export WAV"
	var track *groove.Track
	if trackID != "" {
		if track = u.session.Project.Track(trackID); track == nil {
			return
		}
		title = "Export WAV · " + track.Name
	}
	ctx, cancel := context.WithCancel(context.Background())
	w, fresh := u.panel(title, fyne.NewSize(660, 380), cancel)
	if !fresh {
		cancel()
		return
	}
	path := widget.NewEntry()
	stem := strings.TrimSuffix(filepath.Base(u.path), filepath.Ext(u.path))
	if stem == "" || stem == "." {
		stem = projectStem(u.session.Project)
	}
	if track != nil {
		stem += "-" + fileSlug(track.Name)
	}
	if song {
		stem += "-song"
	} else {
		stem += "-pattern-" + groove.SlotName(u.session.Project.Pattern)
	}
	path.SetText(filepath.Join(config.ProjectsDir, stem+".wav"))
	tail := widget.NewSlider(0, 10)
	tail.Step = .5
	tail.SetValue(2)
	tailLabel := widget.NewLabel("")
	// A pattern loops with its tail wrapped onto the start; a song rings out.
	modes := exportTailModes(song)
	tailMode := widget.NewSelect(exportTailLabels(modes), nil)
	mode := func() exportTailMode { return modes[max(0, tailMode.SelectedIndex())] }
	showTail := func() {
		m := mode()
		if m.slider == "" {
			tailLabel.SetText(m.help)
			tail.Hide()
		} else {
			tailLabel.SetText(fmt.Sprintf(m.slider, tail.Value) + " · " + m.help)
			tail.Show()
		}
	}
	tail.OnChanged = func(float64) { showTail() }
	tailMode.OnChanged = func(string) { showTail() }
	tailMode.SetSelectedIndex(0)
	tailLabel.Wrapping = fyne.TextWrapWord
	status := widget.NewLabel("44.1 kHz · stereo · 16-bit WAV")
	progress := widget.NewProgressBar()
	start := widget.NewButton("Export", nil)
	browse := widget.NewButton("Browse…", nil)
	start.OnTapped = func() {
		destination := path.Text
		if !strings.EqualFold(filepath.Ext(destination), ".wav") {
			destination += ".wav"
		}
		u.finishLiveSlider()
		snapshot := u.session.Project.Clone()
		if trackID != "" && !groove.IsolateTrack(&snapshot, trackID) {
			u.showError(fmt.Errorf("that track has been removed"))
			return
		}
		m := mode()
		tailSeconds := tail.Value
		if m.slider == "" {
			tailSeconds = 0
		}
		start.Disable()
		path.Disable()
		browse.Disable()
		tail.Disable()
		tailMode.Disable()
		status.SetText("Rendering… editing and playback remain available")
		go func() {
			err := os.MkdirAll(filepath.Dir(destination), 0755)
			if err == nil {
				err = groove.ExportWAVMode(ctx, destination, snapshot, song, tailSeconds, m.mode, func(fraction float64) {
					fyne.Do(func() {
						if ctx.Err() == nil {
							progress.SetValue(fraction)
						}
					})
				})
			}
			fyne.Do(func() {
				if ctx.Err() != nil {
					return
				}
				start.Enable()
				path.Enable()
				browse.Enable()
				tail.Enable()
				tailMode.Enable()
				if err != nil {
					status.SetText("Export failed")
					u.showError(err)
					return
				}
				status.SetText("Saved " + destination)
			})
		}()
	}
	help := widget.NewLabel("Renders a snapshot of the groove. Closing this window cancels; a failed render keeps the old file.")
	help.Wrapping = fyne.TextWrapWord
	browse.OnTapped = func() {
		// The dialog needs more room than this small window has.
		if size := w.Canvas().Size(); size.Width < 900 || size.Height < 600 {
			w.Resize(fyne.NewSize(max(900, size.Width), max(600, size.Height)))
		}
		dir, name := filepath.Split(path.Text)
		if dir == "" {
			dir = config.ProjectsDir
		}
		u.chooseFile(fileSpec{title: "Export WAV to", accept: "Choose", mode: saveFile, kinds: []fileKind{wavFiles}, dir: filepath.Clean(dir), name: name, canvas: w.Canvas()}, func(destination string) {
			path.SetText(destination)
		})
	}
	destination := container.NewBorder(nil, nil, nil, browse, path)
	top := []fyne.CanvasObject{}
	if track != nil {
		// Say plainly that this is a solo render: no muting needed.
		only := widget.NewLabelWithStyle(track.Name+" only: the other tracks are left out of this file, as if "+track.Name+" were soloed.", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		only.Wrapping = fyne.TextWrapWord
		top = append(top, only)
	}
	w.SetContent(container.NewPadded(container.NewVBox(append(top, widget.NewLabel("Destination"), destination, container.NewBorder(nil, nil, widget.NewLabel("Effect tail"), nil, tailMode), tailLabel, tail, help, progress, status, container.NewHBox(start, widget.NewButton("Close / cancel", w.Close)))...)))
	u.showPanel(w)
}

// exportTailMode is one choice of the export window's Effect tail select.
type exportTailMode struct {
	label  string
	mode   groove.TailMode
	slider string // the slider's label (a %.1f format), or "" for no slider
	help   string
}

var (
	wrapTail = exportTailMode{"Loop (wrap tail onto the start)", groove.TailWrap, "Longest tail %.1f s",
		"plays silently first, so reverb and delay carry into the start; one loop long, loops seamlessly"}
	appendTail = exportTailMode{"Add tail at end", groove.TailAppend, "Tail %.1f s", "the effects ring out after the last note"}
	noTail     = exportTailMode{"No tail", groove.TailAppend, "", "the file stops at the end of the music"}
)

// exportTailModes lists the tail choices, the default first: a pattern
// loops, a song rings out (a song can't be wrapped).
func exportTailModes(song bool) []exportTailMode {
	if song {
		return []exportTailMode{appendTail, noTail}
	}
	return []exportTailMode{wrapTail, appendTail, noTail}
}

func exportTailLabels(modes []exportTailMode) []string {
	labels := make([]string, len(modes))
	for i, m := range modes {
		labels[i] = m.label
	}
	return labels
}
