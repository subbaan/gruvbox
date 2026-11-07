package desktop

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) sectionTracks(id string) {
	if old := u.panels["Section tracks"]; old != nil {
		old.Close()
	}
	w, _ := u.panel("Section tracks", fyne.NewSize(460, 420), nil)
	session := u.session
	var rebuild func()
	change := func(label string, fn func(*groove.Project, *groove.SongSection)) {
		if u.session != session || session.Project.Section(id) == nil {
			w.Close()
			return
		}
		u.edit(label, func(p *groove.Project) error { fn(p, p.Section(id)); return nil })
		rebuild()
	}
	rebuild = func() {
		section := u.session.Project.Section(id)
		if section == nil {
			w.Close()
			return
		}
		w.SetTitle("Section tracks — " + section.Name)
		rows := container.NewVBox()
		help := widget.NewLabel("Section overrides can mute or unmute tracks. Live uses this pattern’s stored mute, or the live mixer when no state is stored. Track solo still applies.")
		help.Wrapping = fyne.TextWrapWord
		rows.Add(help)
		for _, track := range u.session.Project.Tracks {
			track := track
			muted, overridden := section.Mutes[track.ID]
			if !overridden {
				muted = track.Mute
				if state := track.Patterns[section.Pattern].State; state != nil {
					muted = state.Mute
				}
			}
			label := "Mute " + track.Name
			if overridden {
				label += " · override"
			}
			check := widget.NewCheck(label, nil)
			check.SetChecked(muted)
			check.OnChanged = func(on bool) {
				change("Section mute", func(p *groove.Project, s *groove.SongSection) {
					if p.Track(track.ID) == nil {
						return
					}
					if s.Mutes == nil {
						s.Mutes = map[string]bool{}
					}
					s.Mutes[track.ID] = on
				})
			}
			rows.Add(container.NewBorder(nil, nil, nil, widget.NewButton("Live", func() {
				change("Use live track mute", func(_ *groove.Project, s *groove.SongSection) { delete(s.Mutes, track.ID) })
			}), check))
		}
		footer := container.NewHBox(widget.NewButton("Capture mixer mutes", func() {
			change("Capture section mutes", func(p *groove.Project, s *groove.SongSection) {
				s.Mutes = map[string]bool{}
				for _, t := range p.Tracks {
					s.Mutes[t.ID] = t.Mute
				}
			})
		}), widget.NewButton("Use live mutes", func() {
			change("Clear section overrides", func(_ *groove.Project, s *groove.SongSection) { s.Mutes = nil })
		}), widget.NewButton("Close", w.Close))
		w.SetContent(container.NewPadded(container.NewBorder(widget.NewLabel(fmt.Sprintf("Pattern %02d · %d bars", section.Pattern+1, section.Repeats)), footer, nil, nil, container.NewVScroll(rows))))
	}
	rebuild()
	w.Show()
}
