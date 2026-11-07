package desktop

import (
	"fmt"
	"maps"
	"math"

	"github.com/subbaan/gruvbox/groove"
)

// Step and pattern clipboard (Ctrl+C/V for the selected step, Ctrl+Shift+C/V
// for the selected track's pattern). It is internal to Gruvbox: text fields
// keep using the system clipboard because the focused entry gets the shortcut.

func cloneStep(s groove.Step) groove.Step {
	s.Locks = maps.Clone(s.Locks)
	return s
}

// keepLocks drops locks that the target track cannot use, such as FX locks
// copied from another track's chain.
func keepLocks(t *groove.Track, s *groove.Step) {
	if len(s.Locks) == 0 {
		return
	}
	valid := map[string]bool{}
	for _, p := range groove.LockParameters(*t) {
		if p.Lockable {
			valid[p.ID] = true
		}
	}
	for id := range s.Locks {
		if !valid[id] {
			delete(s.Locks, id)
		}
	}
	if len(s.Locks) == 0 {
		s.Locks = nil
	}
}

func (u *UI) copyStep() {
	s := u.currentStep(u.selected)
	if s == nil {
		return
	}
	copied := cloneStep(*s)
	u.stepClip = &copied
	u.status.SetText(fmt.Sprintf("Copied step %02d", u.selectedStep+1))
}

// pasteStep writes the copied step over the selected one and moves to the
// next step, so repeated pastes stamp a run of steps.
func (u *UI) pasteStep() {
	t := u.selectedTrack()
	if u.stepClip == nil || t == nil {
		return
	}
	pasted := cloneStep(*u.stepClip)
	keepLocks(t, &pasted)
	u.editStep(t.ID, "Paste step", func(s *groove.Step) { *s = cloneStep(pasted) })
	if t := u.selectedTrack(); t != nil {
		steps := len(t.Patterns[u.session.Project.Pattern].Steps)
		u.selectedStep = (u.selectedStep + 1) % steps
		u.page = u.selectedStep / 16
		u.refresh()
	}
}

func (u *UI) copyPattern() {
	t := u.selectedTrack()
	if t == nil {
		return
	}
	pattern := t.Patterns[u.session.Project.Pattern]
	copied := groove.Pattern{Resolution: pattern.Resolution, Steps: make([]groove.Step, len(pattern.Steps))}
	for i, s := range pattern.Steps {
		copied.Steps[i] = cloneStep(s)
	}
	u.patternClip = &copied
	u.status.SetText(fmt.Sprintf("Copied %s pattern %02d", t.Name, u.session.Project.Pattern+1))
}

// pastePattern replaces the selected track's steps and step size; its sound
// state for the pattern stays.
func (u *UI) pastePattern() {
	t := u.selectedTrack()
	if u.patternClip == nil || t == nil {
		return
	}
	id, clip := t.ID, u.patternClip
	u.edit("Paste pattern", func(p *groove.Project) error {
		target := p.Track(id)
		pattern := &target.Patterns[p.Pattern]
		pattern.Resolution = clip.Resolution
		pattern.Steps = make([]groove.Step, len(clip.Steps))
		for i, s := range clip.Steps {
			pattern.Steps[i] = cloneStep(s)
			keepLocks(target, &pattern.Steps[i])
		}
		return nil
	})
}

// nudgeVelocity changes the selected step's velocity in tenths.
func (u *UI) nudgeVelocity(delta float64) {
	t := u.selectedTrack()
	if t == nil {
		return
	}
	u.editStep(t.ID, "Velocity", func(s *groove.Step) {
		s.Velocity = math.Round(min(1, max(.1, s.Velocity+delta))*100) / 100
	})
	if s := u.currentStep(t.ID); s != nil {
		u.status.SetText(fmt.Sprintf("Step %02d velocity %.2f", u.selectedStep+1, s.Velocity))
	}
}

// cycleEditorTab moves between Steps, Piano roll, Song, Mixer and Perform.
func (u *UI) cycleEditorTab(delta int) {
	n := len(u.editorTabs.Items)
	u.editorTabs.SelectIndex((u.editorTabs.SelectedIndex() + delta + n) % n)
}
