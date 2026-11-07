package desktop

import (
	"fmt"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) soundPresets() {
	if u.selectedTrack() == nil {
		return
	}
	closed := false
	w, fresh := u.openDrawer("Sound library", func() {
		closed = true
		u.soundLibraryRefresh = nil
	})
	if !fresh {
		return
	}
	target := widget.NewLabel("")
	target.Wrapping = fyne.TextWrapWord
	search := widget.NewEntry()
	search.SetPlaceHolder("Find a sound…")
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	description := widget.NewLabel("Select a preset to see its sound and FX.")
	description.Wrapping = fyne.TextWrapWord
	details := container.NewVScroll(description)
	details.SetMinSize(fyne.NewSize(0, 100))
	includeFX := widget.NewCheck("Recall FX chain too", nil)
	includeFX.SetChecked(true)
	help := widget.NewLabel("Recall is live and undoable. FX recall replaces the chain and clears its step locks. Notes, instrument locks and mixer settings stay.")
	help.Wrapping = fyne.TextWrapWord
	var all, visible []groove.SoundPresetEntry
	var loaded *groove.SoundPreset
	selectedID := ""
	importNotice := ""
	var kind groove.Kind
	var generation, loadGeneration int
	apply := widget.NewButton("Apply", nil)
	export := widget.NewButton("Export…", nil)
	remove := widget.NewButton("Delete…", nil)
	apply.Disable()
	export.Disable()
	remove.Disable()
	list := widget.NewList(func() int { return len(visible) }, func() fyne.CanvasObject {
		label := widget.NewLabel("")
		label.Truncation = fyne.TextTruncateEllipsis
		return label
	}, func(i widget.ListItemID, obj fyne.CanvasObject) {
		e := visible[i]
		source := "Saved"
		if e.Factory {
			source = "Built-in"
		}
		obj.(*widget.Label).SetText(e.Name + "  ·  " + source)
	})
	list.OnSelected = func(i widget.ListItemID) {
		if i < 0 || i >= len(visible) {
			return
		}
		e := visible[i]
		selectedID = e.ID
		loaded = nil
		apply.Disable()
		export.Disable()
		remove.Disable()
		loadGeneration++
		request := loadGeneration
		status.SetText("Loading " + e.Name + "…")
		go func() {
			p, err := u.soundLibrary.Load(e.ID)
			fyne.Do(func() {
				if closed || request != loadGeneration {
					return
				}
				if err != nil {
					status.SetText(err.Error())
					return
				}
				loaded = &p
				details := p.Description
				if p.Sample != nil {
					details += "\nSample: " + p.Sample.Name()
				}
				fxNames := []string{}
				for _, e := range p.Effects {
					name := e.Kind.Name()
					if !e.Enabled {
						name += " (off)"
					}
					fxNames = append(fxNames, name)
				}
				if len(fxNames) == 0 {
					fxNames = []string{"None"}
				}
				details += "\nFX: " + strings.Join(fxNames, " → ")
				description.SetText(strings.TrimSpace(details))
				apply.Enable()
				export.Enable()
				if !e.Factory {
					remove.Enable()
				}
				status.SetText("Selected " + p.Name)
			})
		}()
	}
	filter := func() {
		loadGeneration++
		loaded = nil
		apply.Disable()
		export.Disable()
		remove.Disable()
		list.UnselectAll()
		visible = nil
		query := strings.ToLower(strings.TrimSpace(search.Text))
		for _, e := range all {
			if strings.Contains(strings.ToLower(e.Name+" "+e.Description), query) {
				visible = append(visible, e)
			}
		}
		list.Refresh()
		for i, e := range visible {
			if e.ID == selectedID {
				list.Select(i)
				return
			}
		}
		description.SetText("Select a preset to see its sound and FX.")
		if len(visible) == 0 {
			status.SetText("No matching presets. Save the current sound or import a .gruvsound file.")
		} else {
			status.SetText(fmt.Sprintf("%d presets for %s", len(visible), kind.Name()))
		}
	}
	search.OnChanged = func(string) { filter() }
	reload := func(selectID string) {
		generation++
		loadGeneration++
		request, requestedKind := generation, kind
		selectedID = selectID
		loaded = nil
		apply.Disable()
		export.Disable()
		remove.Disable()
		all, visible = nil, nil
		list.UnselectAll()
		list.Refresh()
		description.SetText("Select a preset to see its sound and FX.")
		status.SetText("Reading sound library…")
		go func() {
			entries, err := u.soundLibrary.List(requestedKind)
			fyne.Do(func() {
				if closed || request != generation {
					return
				}
				all = entries
				filter()
				if err != nil {
					status.SetText(err.Error())
				} else if importNotice != "" {
					status.SetText(importNotice)
					importNotice = ""
				}
			})
		}()
	}
	apply.OnTapped = func() {
		if loaded == nil || u.selectedTrack() == nil {
			return
		}
		p, id := *loaded, u.selected
		u.recallSound(id, p, includeFX.Checked)
	}
	remove.OnTapped = func() {
		id := selectedID
		if loaded == nil {
			return
		}
		name := loaded.Name
		u.confirm("Delete sound preset", "Delete", "Cancel", widget.NewLabel("Delete "+name+" from the sound library?"), func(ok bool) {
			if !ok || closed {
				return
			}
			go func() {
				err := u.soundLibrary.Delete(id)
				fyne.Do(func() {
					if closed {
						return
					}
					if err != nil {
						u.showError(err)
						return
					}
					reload("")
				})
			}()
		})
	}
	export.OnTapped = func() {
		if loaded != nil {
			u.exportSoundPreset(*loaded, nil)
		}
	}
	save := widget.NewButton("Save current…", func() {
		t := u.selectedTrack()
		if t == nil {
			return
		}
		id, session := t.ID, u.session
		name := widget.NewEntry()
		name.SetPlaceHolder("Name this sound")
		name.SetText(t.Name + " sound")
		u.confirm("Save sound preset", "Save", "Cancel", container.NewVBox(widget.NewLabel("Current sound on "+t.Name), name), func(ok bool) {
			if !ok || closed {
				return
			}
			t := session.Project.Track(id)
			if u.session != session || t == nil {
				u.showError(fmt.Errorf("the track changed before saving; reopen Save current"))
				return
			}
			u.finishLiveSlider()
			p := groove.CaptureSound(*t, name.Text)
			go func() {
				id, err := u.soundLibrary.Save(p)
				fyne.Do(func() {
					if err != nil {
						u.showError(err)
						return
					}
					if !closed {
						search.SetText("")
						reload(id)
					}
				})
			}()
		})
	})
	importSound := widget.NewButton("Import…", func() {
		u.chooseFile(fileSpec{title: "Import sound preset", accept: "Import", mode: openFile, kinds: []fileKind{soundFiles}, dir: u.soundLibrary.Dir}, func(path string) {
			go func() {
				var p groove.SoundPreset
				var id string
				f, err := os.Open(path)
				if err == nil {
					p, err = groove.DecodeSoundPreset(f)
					f.Close()
				}
				if err == nil {
					id, err = u.soundLibrary.Save(p)
				}
				fyne.Do(func() {
					if err != nil {
						u.showError(err)
						return
					}
					if !closed {
						search.SetText("")
						if p.Kind != kind {
							importNotice = fmt.Sprintf("Imported %s. Select a %s track to recall it.", p.Name, p.Kind.Name())
						}
						reload(id)
					}
				})
			}()
		})
	})
	u.soundLibraryRefresh = func() {
		t := u.selectedTrack()
		if t == nil {
			w.Close()
			return
		}
		target.SetText("Sounds for " + t.Name + " / " + t.Kind.Name())
		if kind != t.Kind {
			kind = t.Kind
			reload("")
		}
	}
	w.SetContent(container.NewPadded(container.NewBorder(container.NewVBox(target, search), container.NewVBox(details, includeFX, help, container.NewGridWithColumns(3, apply, save, importSound), container.NewGridWithColumns(3, export, remove, widget.NewButton("Close", w.Close)), status), nil, nil, list)))
	u.soundLibraryRefresh()
}

func (u *UI) recallSound(id string, p groove.SoundPreset, includeFX bool) {
	u.edit("Recall "+p.Name, func(project *groove.Project) error {
		err := groove.ApplySound(project.Track(id), p, includeFX)
		if err == nil && p.Kind == groove.Sampler {
			delete(u.sampleViews, id)
			u.player.StopAudition()
		}
		return err
	})
}

// soundFiles is the type filter for sound presets.
var soundFiles = fileKind{Name: "Sound presets", Exts: []string{".gruvsound"}}

func (u *UI) exportSoundPreset(p groove.SoundPreset, canvas fyne.Canvas) {
	name := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(p.Name), " ", "-"))
	if name == "" {
		name = "export-" + string(p.Kind)
	}
	u.chooseFile(fileSpec{title: "Export sound preset", accept: "Export", mode: saveFile, kinds: []fileKind{soundFiles}, dir: u.soundLibrary.Dir, name: name + ".gruvsound", canvas: canvas}, func(destination string) {
		go func() {
			err := groove.SaveSoundPreset(destination, p)
			fyne.Do(func() {
				if err != nil {
					u.showError(err)
				} else {
					u.status.SetText("Exported sound preset: " + destination)
				}
			})
		}()
	})
}
