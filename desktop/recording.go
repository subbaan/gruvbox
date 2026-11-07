package desktop

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/capture"
	"github.com/subbaan/gruvbox/config"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) recordSample(trackID string) {
	t := u.session.Project.Track(trackID)
	if t == nil || t.Kind != groove.Sampler {
		return
	}
	session := u.session
	ctx, cancel := context.WithCancel(context.Background())
	closed, starting := false, false
	actionGeneration := 0
	var recording *capture.Recording
	w, fresh := u.panel("Record sample", fyne.NewSize(600, 660), func() {
		closed = true
		if recording != nil {
			recording.Stop()
		}
		cancel()
		u.player.StopAudition()
		u.recordingRefresh = nil
	})
	if !fresh {
		cancel()
		return
	}
	sources := capture.DefaultSources()
	labels := func() []string {
		var values []string
		for i, source := range sources {
			label := source.Description
			if i >= 2 {
				if runes := []rune(label); len(runes) > 25 {
					label = string(runes[:25]) + "…"
				}
				mode := "input"
				if source.Monitor {
					mode = "output"
				}
				label = fmt.Sprintf("%s · %s (%d)", label, mode, i-1)
			}
			values = append(values, label)
		}
		return values
	}
	source := widget.NewSelect(labels(), nil)
	source.SetSelectedIndex(0)
	channels := widget.NewSelect([]string{"Stereo", "Mono"}, nil)
	channels.SetSelectedIndex(0)
	limit := widget.NewSelect([]string{"5", "15", "30", "60", "90"}, nil)
	limit.SetSelected("30")
	name := widget.NewEntry()
	name.SetText("Recording " + time.Now().Format("2006-01-02 15-04-05"))
	status := widget.NewLabel("Ready · choose system output or an input, then Record.")
	status.Wrapping = fyne.TextWrapWord
	level := widget.NewProgressBar()
	levelLabel := widget.NewLabel("0.00 s · 44.1 kHz · no signal yet")
	levelLabel.Wrapping = fyne.TextWrapWord
	clip := widget.NewLabel("")
	clip.Wrapping = fyne.TextWrapWord
	clip.Hide()
	var take *groove.Sample
	var recordedChannels int
	var wave *sampleWave
	waveBox := container.NewMax(widget.NewLabel("Your take appears here after stopping."))
	record := widget.NewButton("Record", nil)
	stop := widget.NewButton("Stop", nil)
	audition := widget.NewButton("Audition", nil)
	use := widget.NewButton("Use take", nil)
	save := widget.NewButton("Save WAV…", nil)
	refreshSources := widget.NewButton("Refresh sources", nil)
	stop.Disable()
	audition.Disable()
	use.Disable()
	save.Disable()
	setBusy := func(busy bool) {
		if busy {
			record.Disable()
			source.Disable()
			channels.Disable()
			limit.Disable()
			name.Disable()
			refreshSources.Disable()
			audition.Disable()
			use.Disable()
			save.Disable()
			stop.Enable()
		} else {
			record.Enable()
			source.Enable()
			channels.Enable()
			limit.Enable()
			name.Enable()
			refreshSources.Enable()
			stop.Disable()
			if take != nil {
				audition.Enable()
				use.Enable()
				save.Enable()
			}
		}
	}
	refreshSources.OnTapped = func() {
		refreshSources.Disable()
		requestGeneration := actionGeneration
		go func() {
			discoveryCtx, discoveryCancel := context.WithTimeout(ctx, 3*time.Second)
			defer discoveryCancel()
			found, err := capture.Sources(discoveryCtx)
			fyne.Do(func() {
				if closed {
					return
				}
				if starting || recording != nil {
					return
				}
				refreshSources.Enable()
				if err != nil {
					if requestGeneration == actionGeneration {
						status.SetText("Could not list devices; default sources remain available. " + err.Error())
					}
					return
				}
				selectedName := sources[max(0, source.SelectedIndex())].Name
				sources = append(capture.DefaultSources(), found...)
				source.Options = labels()
				source.Refresh()
				source.SetSelectedIndex(0)
				for i, candidate := range sources {
					if candidate.Name == selectedName {
						source.SetSelectedIndex(i)
						break
					}
				}
				if requestGeneration == actionGeneration {
					status.SetText("Ready · sources refreshed")
				}
			})
		}()
	}
	stop.OnTapped = func() {
		stop.Disable()
		status.SetText("Stopping and preparing the take…")
		if recording != nil {
			recording.Stop()
		}
		// If process startup is still in flight, its callback observes this flag.
		starting = false
	}
	record.OnTapped = func() {
		if starting || recording != nil {
			return
		}
		selected := source.SelectedIndex()
		if selected < 0 || selected >= len(sources) {
			return
		}
		seconds, _ := strconv.Atoi(limit.Selected)
		count := 2
		if channels.SelectedIndex() == 1 {
			count = 1
		}
		takeName := strings.TrimSpace(name.Text)
		if takeName == "" {
			status.SetText("Name the take before recording.")
			return
		}
		if !strings.HasSuffix(strings.ToLower(takeName), ".wav") {
			takeName += ".wav"
		}
		chosen := sources[selected]
		actionGeneration++
		starting = true
		setBusy(true)
		u.player.StopAudition()
		status.SetText("Starting " + chosen.Description + "…")
		clip.SetText("")
		clip.Hide()
		level.SetValue(0)
		go func() {
			r, err := capture.Start(ctx, chosen.Name, count, seconds)
			fyne.Do(func() {
				if closed {
					if r != nil {
						r.Stop()
					}
					return
				}
				stopRequested := !starting
				starting = false
				if err != nil {
					setBusy(false)
					status.SetText(err.Error())
					return
				}
				recording = r
				if stopRequested {
					r.Stop()
				} else {
					status.SetText("Recording " + chosen.Description + " · Stop or wait for the limit")
				}
				go func() {
					pcm, err := r.Wait()
					var sample *groove.Sample
					if err == nil {
						sample, err = groove.SampleFromPCM(takeName, pcm, capture.Rate, count)
					}
					fyne.Do(func() {
						if closed {
							return
						}
						recording = nil
						if err != nil {
							setBusy(false)
							status.SetText(err.Error())
							return
						}
						take, recordedChannels = sample, count
						wave = &sampleWave{ui: u, sample: take, start: 0, end: 1}
						wave.ExtendBaseWidget(wave)
						wave.commit = func(float64, float64) {} // preview edits stay local until Use take
						waveBox.Objects = []fyne.CanvasObject{container.NewPadded(wave)}
						waveBox.Refresh()
						setBusy(false)
						level.SetValue(0)
						levelLabel.SetText(fmt.Sprintf("%.2f s · 44.1 kHz · %d channel(s)", float64(take.Frames())/capture.Rate, recordedChannels))
						message := "Take ready · audition, save, or use it on " + t.Name
						if r.Stats().Limited {
							message = "Duration limit reached. " + message
						}
						if r.Stats().Clipped {
							clip.SetText("Full-scale peaks detected · lower the source’s input level")
							clip.Show()
						}
						status.SetText(message)
					})
				}()
			})
		}()
	}
	audition.OnTapped = func() {
		if take == nil {
			return
		}
		if u.audioError != nil {
			u.showError(u.audioError)
			return
		}
		preview := groove.NewTrack(groove.Sampler, 0)
		preview.Sample = take
		preview.Parameters["start"], preview.Parameters["end"] = wave.start, wave.end
		if err := u.player.Audition(preview); err != nil {
			status.SetText(err.Error())
		}
	}
	use.OnTapped = func() {
		if take == nil {
			return
		}
		if err := u.useRecordedSample(session, trackID, take, wave.start, wave.end); err != nil {
			status.SetText(err.Error())
			return
		}
		status.SetText("Take loaded into " + session.Project.Track(trackID).Name + " · Undo restores the previous sample")
	}
	save.OnTapped = func() {
		if take != nil {
			u.saveRecordedWAV(take, w.Canvas())
		}
	}
	help := widget.NewLabel("System output captures everything playing through that output, including Gruvbox. Inputs capture microphones or line-in. Drag the preview markers to trim before audition/use; Save WAV keeps the complete original take.")
	help.Wrapping = fyne.TextWrapWord
	controls := container.NewVBox(widget.NewLabel("Record into "+t.Name), container.NewBorder(nil, nil, nil, refreshSources, source), container.NewGridWithColumns(2, channels, container.NewBorder(nil, nil, widget.NewLabel("Limit (s)"), nil, limit)), container.NewBorder(nil, nil, widget.NewLabel("Take name"), nil, name), container.NewGridWithColumns(2, record, stop), level, levelLabel, clip)
	actions := container.NewVBox(status, help, container.NewGridWithColumns(3, audition, widget.NewButton("Stop preview", u.player.StopAudition), use), container.NewGridWithColumns(2, save, widget.NewButton("Close", w.Close)))
	w.SetContent(container.NewPadded(container.NewBorder(controls, actions, nil, nil, waveBox)))
	lastUpdate := time.Time{}
	u.recordingRefresh = func() {
		if u.session != session || session.Project.Track(trackID) == nil {
			w.Close()
			return
		}
		if recording == nil || time.Since(lastUpdate) < 100*time.Millisecond {
			return
		}
		lastUpdate = time.Now()
		stats := recording.Stats()
		level.SetValue(stats.Peak)
		db := "−∞"
		if stats.Peak > 0 {
			db = fmt.Sprintf("%.1f", 20*math.Log10(stats.Peak))
		}
		levelLabel.SetText(fmt.Sprintf("%.2f s · %s dBFS · 44.1 kHz", float64(stats.Frames)/capture.Rate, db))
		if stats.Clipped {
			clip.SetText("Full-scale peaks detected · lower the source’s input level")
			clip.Show()
		}
	}
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name == fyne.KeyEscape {
			w.Close()
		}
	})
	u.showPanel(w)
	refreshSources.OnTapped()
}

func (u *UI) useRecordedSample(session *groove.Session, id string, sample *groove.Sample, start, end float64) error {
	if u.session != session {
		return fmt.Errorf("the project changed; record into the current project")
	}
	u.finishLiveSlider()
	if err := session.Do("Use recorded sample", func(p *groove.Project) error {
		t := p.Track(id)
		if t == nil || t.Kind != groove.Sampler {
			return fmt.Errorf("the sampler track no longer exists")
		}
		if sample == nil {
			return fmt.Errorf("record a take first")
		}
		t.Sample = sample
		t.Parameters["start"], t.Parameters["end"] = start, end
		t.Parameters["gain_db"], t.Parameters["normalize"], t.Parameters["fade_in"], t.Parameters["fade_out"] = 0, 0, 0, 0
		return nil
	}); err != nil {
		return err
	}
	delete(u.sampleViews, id)
	u.player.StopAudition()
	u.player.Publish(session.Project)
	u.refresh()
	return nil
}

func (u *UI) saveRecordedWAV(sample *groove.Sample, canvas fyne.Canvas) {
	name := filepath.Base(sample.Name())
	if !strings.HasSuffix(strings.ToLower(name), ".wav") {
		name += ".wav"
	}
	u.chooseFile(fileSpec{title: "Save recording WAV", accept: "Save", mode: saveFile, kinds: []fileKind{wavFiles}, dir: config.RecordingsDir, name: name, canvas: canvas}, func(destination string) {
		go func() {
			err := groove.SaveSampleWAV(destination, sample)
			fyne.Do(func() {
				if err != nil {
					u.showError(err)
				} else {
					u.status.SetText("Saved recording: " + destination)
				}
			})
		}()
	})
}
