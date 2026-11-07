package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) queuePerformance(kind groove.PerformanceKind) {
	u.finishLiveSlider()
	if err := u.player.QueuePerformance(u.session.Project, kind, u.selected); err != nil {
		u.showError(err)
		return
	}
	u.updatePlaybackLabel()
}
func (u *UI) keepPerformance() {
	slot := groove.NextEmptyPattern(u.session.Project, u.session.Project.Pattern)
	applied := false
	u.edit("Keep performance take", func(p *groove.Project) error {
		err := u.player.KeepPerformance(p, slot)
		applied = err == nil
		return err
	})
	if applied {
		u.status.SetText("Kept performance in pattern " + groove.SlotName(slot))
	}
}
func (u *UI) performancePanel() fyne.CanvasObject {
	u.performanceLabel = widget.NewLabel("Start playback to queue a fill or variation.")
	u.performanceLabel.Wrapping = fyne.TextWrapWord
	fill := widget.NewButton("Fill drums", func() { u.queuePerformance(groove.DrumFill) })
	track := widget.NewButton("Vary selected track", func() { u.queuePerformance(groove.TrackVariation) })
	all := widget.NewButton("Vary whole groove", func() { u.queuePerformance(groove.GrooveVariation) })
	u.performanceActions = []*widget.Button{fill, track, all}
	u.performanceCancel = widget.NewButton("Cancel queued cue", func() { u.player.CancelPerformance(); u.updatePlaybackLabel() })
	u.performanceKeep = widget.NewButton("Keep last take → empty pattern", u.keepPerformance)
	help := u.helpButton("Cues launch at the next 4/4 bar, play for one bar, then return. A new cue replaces the queued one. Song sections keep advancing underneath. Pattern cycles restart on launch and return; effects keep their tails.\n\nFills embellish the final beat of the drum parts. Variations mutate rhythm and melodic notes. Performance does not change saved patterns until you keep a take; keeping supports undo.")
	u.updatePerformanceControls()
	return container.NewPadded(container.NewVBox(container.NewBorder(nil, nil, nil, help, u.performanceLabel), container.NewHBox(fill, track, all), container.NewHBox(u.performanceCancel, u.performanceKeep), widget.NewSeparator(), u.jamPanel()))
}
func setButtonEnabled(button *widget.Button, on bool) {
	if button == nil {
		return
	}
	if on && button.Disabled() {
		button.Enable()
	} else if !on && !button.Disabled() {
		button.Disable()
	}
}
func (u *UI) updatePerformanceControls() {
	if u.performanceLabel == nil {
		return
	}
	playing := u.player.Playing()
	for _, button := range u.performanceActions {
		setButtonEnabled(button, playing)
	}
	if len(u.performanceActions) > 1 {
		setButtonEnabled(u.performanceActions[1], playing && u.selectedTrack() != nil)
	}
	setButtonEnabled(u.performanceCancel, u.player.QueuedPerformance() != groove.NoPerformance)
	setButtonEnabled(u.performanceKeep, u.player.HasPerformanceTake())
	state := "Start playback to queue a fill or variation."
	if playing {
		state = "Ready · cues launch at the next bar."
	}
	if running := u.player.RunningPerformance(); running != groove.NoPerformance {
		state = running.Name() + " playing · returns at the next bar."
	}
	if queued := u.player.QueuedPerformance(); queued != groove.NoPerformance {
		state += "\nQueued: " + queued.Name() + " · next bar."
	}
	u.performanceLabel.SetText(state)
}
