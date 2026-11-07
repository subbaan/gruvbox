package desktop

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

const mixerStripWidth, mixerFaderHeight float32 = 86, 170

// refreshMixer rebuilds the strips only when tracks are added, removed or
// reordered; level, pan, mute, solo and names sync in place.
func (u *UI) refreshMixer() {
	if u.mixer == nil {
		return
	}
	var key strings.Builder
	key.WriteString(u.scheme.Name)
	for _, t := range u.session.Project.Tracks {
		key.WriteString("|" + t.ID)
	}
	if key.String() == u.mixerKey && len(u.mixer.Objects) > 0 {
		u.mixerBinder.sync()
		return
	}
	u.mixerKey = key.String()
	u.building(&u.mixerBinder, u.rebuildMixer)
}
func (u *UI) rebuildMixer() {
	u.mixer.Objects = nil
	if len(u.session.Project.Tracks) == 0 {
		u.mixer.Add(widget.NewLabel("Add a track to begin."))
	}
	for _, t := range u.session.Project.Tracks {
		u.mixer.Add(u.mixerStrip(t.ID))
	}
	u.mixer.Refresh()
}
func (u *UI) mixerStrip(id string) fyne.CanvasObject {
	track := func() *groove.Track { return u.session.Project.Track(id) }
	edit := func(label string, fn func(*groove.Track)) {
		u.edit(label, func(p *groove.Project) error {
			if t := p.Track(id); t != nil {
				fn(t)
			}
			return nil
		})
	}
	name := widget.NewButton(track().Name, func() { u.selectTrack(id) })
	u.live(func() {
		t := track()
		importance := widget.LowImportance
		if id == u.selected {
			importance = widget.HighImportance
		}
		if name.Text != t.Name || name.Importance != importance {
			name.Text, name.Importance = t.Name, importance
			name.Refresh()
		}
	})
	toggle := func(text string, on func(*groove.Track) bool, flip func()) *widget.Button {
		button := widget.NewButton(text, flip)
		u.live(func() {
			importance := widget.MediumImportance
			if on(track()) {
				importance = widget.DangerImportance
				if text == "S" {
					importance = widget.WarningImportance
				}
			}
			if button.Importance != importance {
				button.Importance = importance
				button.Refresh()
			}
		})
		return button
	}
	mute := toggle("M", func(t *groove.Track) bool { return t.Mute }, func() { u.toggleTrackMute(id) })
	solo := toggle("S", func(t *groove.Track) bool { return t.Solo }, func() { u.toggleTrackSolo(id) })
	level := widget.NewSlider(0, 1)
	level.Orientation = widget.Vertical
	level.Step = .01
	level.SetValue(track().Level)
	reading := widget.NewLabel(fmt.Sprintf("%.2f", level.Value))
	reading.Alignment = fyne.TextAlignCenter
	u.bindLiveSlider(level, func(v float64) {
		reading.SetText(fmt.Sprintf("%.2f", v))
		edit("Level", func(t *groove.Track) { t.Level = v })
	})
	pan := widget.NewSlider(-1, 1)
	pan.Step = .01
	pan.SetValue(track().Pan)
	u.bindLiveSlider(pan, func(v float64) { edit("Pan", func(t *groove.Track) { t.Pan = v }) })
	u.live(func() {
		if u.liveSlider != level {
			setSlider(level, track().Level)
			setLabel(reading, fmt.Sprintf("%.2f", level.Value))
		}
		if u.liveSlider != pan {
			setSlider(pan, track().Pan)
		}
	})
	fader := container.NewCenter(container.NewGridWrap(fyne.NewSize(40, mixerFaderHeight), level))
	strip := container.NewVBox(name, fader, reading, widget.NewLabelWithStyle("Pan", fyne.TextAlignCenter, fyne.TextStyle{}), pan, container.NewGridWithColumns(2, mute, solo))
	return container.NewGridWrap(fyne.NewSize(mixerStripWidth, strip.MinSize().Height), strip)
}

func (u *UI) selectTrack(id string) {
	if u.session.Project.Track(id) == nil {
		return
	}
	u.selected = id
	u.refresh()
}
