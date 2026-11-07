package desktop

import (
	"fyne.io/fyne/v2/widget"
)

func (u *UI) updateProjectTitle() {
	dirty := ""
	if u.session.Dirty() {
		dirty = " •"
	}
	u.projectLabel.SetText(u.session.Project.Name + dirty)
	u.window.SetTitle("Gruvbox 3 — " + u.session.Project.Name + dirty)
}
func (u *UI) finishLiveSlider() {
	if u.liveSession != nil {
		u.liveSession.EndGesture()
	}
	u.liveSlider = nil
	u.liveSession = nil
}
func (u *UI) bindLiveSlider(slider *widget.Slider, change func(float64)) {
	slider.OnChanged = func(value float64) {
		if u.liveSlider != slider || u.liveSession != u.session {
			u.finishLiveSlider()
			u.liveSlider = slider
			u.liveSession = u.session
			u.session.BeginGesture()
		}
		u.liveEdit = true
		change(value)
		u.liveEdit = false
	}
	slider.OnChangeEnded = func(float64) {
		if u.liveSlider != slider {
			return
		}
		u.finishLiveSlider()
		u.refresh()
	}
}
