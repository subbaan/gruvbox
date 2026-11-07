package desktop

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
	"strconv"
	"strings"
)

func (u *UI) updatePlaybackLabel() {
	if u.playbackLabel == nil {
		return
	}
	if u.player.Playing() && u.player.SongMode() && u.player.SongSection() >= 0 {
		pattern, launch := u.player.LaunchedPattern()
		if launch > 0 && (u.followSession != u.session || u.followLaunch != launch) {
			u.followSession, u.followLaunch = u.session, launch
			u.finishLiveSlider()
			if err := u.session.FollowPattern(pattern); err != nil {
				u.showError(err)
				return
			}
			u.selectedStep, u.page = 0, 0
			u.player.Publish(u.session.Project)
			u.refresh()
		}
	} else {
		u.followSession, u.followLaunch = nil, 0
	}
	state := "Stopped"
	if u.player.Playing() {
		state = fmt.Sprintf("Playing %02d", u.player.CurrentPattern()+1)
		if section := u.player.SongSection(); section >= 0 {
			state += fmt.Sprintf(" · Section %02d / bar %d", section+1, u.player.SongBar())
		}
		if queued := u.player.QueuedPattern(); queued >= 0 {
			state += fmt.Sprintf(" → %02d next bar", queued+1)
		}
	}
	if running := u.player.RunningPerformance(); running != groove.NoPerformance {
		state += " · " + running.Name()
	}
	if queued := u.player.QueuedPerformance(); queued != groove.NoPerformance {
		state += " · " + queued.Name() + " next bar"
	}
	u.updatePerformanceControls()
	u.updateModeButtons()
	u.updatePatternPads()
	u.updatePageButtons()
	if u.songTimeline != nil {
		u.songTimeline.advance()
	}
	if u.player.Auditioning() {
		state += " · Preview"
	}
	u.playbackLabel.SetText(state)
}

// refreshSong rebuilds the section list only when sections are added,
// removed or reordered; other edits update the existing rows in place.
func (u *UI) refreshSong() {
	if u.songPanel == nil {
		return
	}
	if song := u.session.Project.Song; u.session.Project.Section(u.selectedSection) == nil {
		u.selectedSection = ""
		if len(song) > 0 {
			u.selectedSection = song[0].ID
		}
	}
	if u.songTimeline != nil {
		u.songTimeline.Refresh()
	}
	var key strings.Builder
	for _, section := range u.session.Project.Song {
		key.WriteString(section.ID + "|")
	}
	if key.String() == u.songKey && len(u.songPanel.Objects) > 0 {
		u.songBinder.sync()
		return
	}
	u.songKey = key.String()
	u.building(&u.songBinder, u.rebuildSong)
}
func (u *UI) rebuildSong() {
	u.songPanel.Objects = nil
	grow := widget.NewButton("Grow into a track…", u.growTrack)
	grow.Importance = widget.HighImportance
	u.songPanel.Add(container.NewHBox(widget.NewButton("Add current pattern", func() {
		u.edit("Add song section", func(p *groove.Project) error { p.Song = append(p.Song, groove.NewSongSection(p.Pattern)); return nil })
	}), grow, u.helpButton("The strip above shows the song: one block per section, as wide as its bars. Click a block (or a row number) to select it. Sections count 4/4 bars and loop at the end. Choose Song in the top bar before Play to start at section 01; while playing it starts at the next bar. Selecting a pattern returns to Pattern mode at the next bar.")))
	for i, section := range u.session.Project.Song {
		id := section.ID
		current := func() groove.SongSection {
			if s := u.session.Project.Section(id); s != nil {
				return *s
			}
			return section
		}
		edit := func(label string, fn func(*groove.SongSection)) {
			u.edit(label, func(p *groove.Project) error {
				if section := p.Section(id); section != nil {
					fn(section)
				}
				return nil
			})
		}
		name := u.boundEntry(func() string { return current().Name }, func(value string) { edit("Name song section", func(s *groove.SongSection) { s.Name = value }) })
		slots := []string{}
		for n := 0; n < groove.PatternCount; n++ {
			slots = append(slots, fmt.Sprintf("%02d", n+1))
		}
		pattern := u.boundSelect(slots, func() int { return current().Pattern }, func(i int) {
			edit("Section pattern", func(s *groove.SongSection) { s.Pattern = i })
		})
		var bars *commitEntry
		bars = u.boundEntry(func() string { return strconv.Itoa(current().Repeats) }, func(value string) {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 {
				bars.SetValue(strconv.Itoa(current().Repeats))
				return
			}
			edit("Section bars", func(s *groove.SongSection) { s.Repeats = n })
		})
		number := widget.NewButton(fmt.Sprintf("%02d", i+1), func() { u.selectSection(id) })
		u.live(func() {
			importance := widget.LowImportance
			if id == u.selectedSection {
				importance = widget.HighImportance
			}
			if number.Importance != importance {
				number.Importance = importance
				number.Refresh()
			}
		})
		row := container.NewHBox(number, container.NewGridWrap(fyne.NewSize(150, 36), name), widget.NewLabel("Pattern"), pattern, widget.NewLabel("Bars"), container.NewGridWrap(fyne.NewSize(45, 36), bars), widget.NewButton("Duplicate", func() {
			u.edit("Duplicate section", func(p *groove.Project) error {
				for n, s := range p.Song {
					if s.ID == id {
						duplicate := groove.NewSongSection(s.Pattern)
						duplicate.Name = s.Name
						duplicate.Repeats = s.Repeats
						duplicate.Mutes = groove.CloneSection(s).Mutes
						p.Song = append(p.Song, duplicate)
						copy(p.Song[n+2:], p.Song[n+1:len(p.Song)-1])
						p.Song[n+1] = duplicate
						break
					}
				}
				return nil
			})
		}), widget.NewButton("↑", func() { u.moveSection(id, -1) }), widget.NewButton("↓", func() { u.moveSection(id, 1) }), widget.NewButton("×", func() {
			u.edit("Remove section", func(p *groove.Project) error {
				for n, s := range p.Song {
					if s.ID == id {
						p.Song = append(p.Song[:n], p.Song[n+1:]...)
						break
					}
				}
				return nil
			})
		}))
		row.Add(widget.NewButton("Tracks", func() { u.sectionTracks(id) }))
		u.songPanel.Add(row)
	}
	u.songPanel.Refresh()
}
func (u *UI) moveSection(id string, delta int) {
	u.edit("Move section", func(p *groove.Project) error {
		for n, s := range p.Song {
			if s.ID == id {
				to := n + delta
				if to >= 0 && to < len(p.Song) {
					p.Song[n], p.Song[to] = p.Song[to], p.Song[n]
				}
				break
			}
		}
		return nil
	})
}
