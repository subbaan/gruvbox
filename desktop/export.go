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
	return []*fyne.MenuItem{fyne.NewMenuItem("Selected pattern…", func() { u.exportWAV(false) }), fyne.NewMenuItem("Full song…", func() { u.exportWAV(true) })}
}
func (u *UI) exportWAV(song bool) {
	if song && len(u.session.Project.Song) == 0 {
		u.showError(fmt.Errorf("add song sections before exporting a song"))
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w, fresh := u.panel("Export WAV", fyne.NewSize(600, 280), cancel)
	if !fresh {
		cancel()
		return
	}
	path := widget.NewEntry()
	stem := strings.TrimSuffix(filepath.Base(u.path), filepath.Ext(u.path))
	if stem == "" || stem == "." {
		stem = "untitled"
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
	tailLabel := widget.NewLabel("Effect tail 2.0 s")
	tail.OnChanged = func(v float64) { tailLabel.SetText(fmt.Sprintf("Effect tail %.1f s", v)) }
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
		tailSeconds := tail.Value
		start.Disable()
		path.Disable()
		browse.Disable()
		tail.Disable()
		status.SetText("Rendering… editing and playback remain available")
		go func() {
			err := os.MkdirAll(filepath.Dir(destination), 0755)
			if err == nil {
				err = groove.ExportWAV(ctx, destination, snapshot, song, tailSeconds, func(fraction float64) {
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
				if err != nil {
					status.SetText("Export failed")
					u.showError(err)
					return
				}
				status.SetText("Saved " + destination)
			})
		}()
	}
	help := widget.NewLabel("Export uses a snapshot of your groove. Closing this window cancels the export; a failed or cancelled render leaves an existing file intact.")
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
	w.SetContent(container.NewPadded(container.NewVBox(widget.NewLabel("Destination"), destination, tailLabel, tail, help, progress, status, container.NewHBox(start, widget.NewButton("Close / cancel", w.Close)))))
	u.showPanel(w)
}
