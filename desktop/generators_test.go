package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
	"reflect"
	"strings"
	"testing"
)

type generatorControls struct {
	buttons map[string]*widget.Button
	sliders map[string]*widget.Slider
	selects []*widget.Select
	preview *generatorPreview
}

func findGeneratorControls(w interface{ Content() fyne.CanvasObject }) generatorControls {
	c := generatorControls{buttons: map[string]*widget.Button{}, sliders: map[string]*widget.Slider{}}
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *fyne.Container:
			if len(o.Objects) == 2 {
				if label, ok := o.Objects[0].(*widget.Label); ok {
					if slider, ok := o.Objects[1].(*widget.Slider); ok {
						c.sliders[strings.Split(label.Text, "  ")[0]] = slider
					}
				}
			}
			for _, child := range o.Objects {
				walk(child)
			}
		case *container.Scroll:
			walk(o.Content)
		case *widget.Button:
			c.buttons[o.Text] = o
		case *widget.Select:
			c.selects = append(c.selects, o)
		case *generatorPreview:
			c.preview = o
		}
	}
	walk(w.Content())
	return c
}

func TestEuclideanDraftApplyUndoAndStaleScope(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("generators")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	before := u.session.Project.Clone()
	u.euclideanGenerator()
	panel := u.drawerTitled("Euclidean rhythm")
	if panel == nil {
		t.Fatal("Euclidean generator did not open in a drawer")
	}
	u.euclideanGenerator()
	if u.drawerTitled("Euclidean rhythm") != panel {
		t.Fatal("duplicate generator window")
	}
	c := findGeneratorControls(panel)
	c.sliders["Pulses"].SetValue(7)
	c.sliders["Rotation"].SetValue(3)
	if !reflect.DeepEqual(u.session.Project, before) || u.liveSession != nil {
		t.Fatal("draft controls changed the project or opened a gesture")
	}
	draft := c.preview.pattern
	u.player.SetPlaying(true)
	test.Tap(c.buttons["Apply rhythm"])
	if !reflect.DeepEqual(u.selectedTrack().Patterns[0], draft) || !u.player.Playing() {
		t.Fatal("Apply did not use the exact preview while playing")
	}
	if !reflect.DeepEqual(c.preview.pattern, draft) {
		t.Fatal("Apply replaced the preview with another take")
	}
	u.undo()
	if !reflect.DeepEqual(u.session.Project, before) {
		t.Fatal("Euclidean apply did not undo once")
	}
	c.sliders["Cycle steps"].SetValue(1)
	if !c.sliders["Rotation"].Disabled() || c.sliders["Pulses"].Value > 1 || len(c.preview.pattern.Steps) != 1 {
		t.Fatal("one-step cycle did not clamp controls")
	}
	c.sliders["Cycle steps"].SetValue(64)
	if len(c.preview.pattern.Steps) != 64 {
		t.Fatal("full cycle preview missing")
	}
	u.session = groove.NewSession(groove.NewProject())
	u.selected = u.session.Project.Tracks[0].ID
	u.refresh()
	if u.drawerTitled("Euclidean rhythm") != nil || len(u.generatorRefresh) != 0 {
		t.Fatal("replacement project left a generator window")
	}
	current := u.session.Project.Clone()
	test.Tap(c.buttons["Apply rhythm"])
	if !reflect.DeepEqual(u.session.Project, current) {
		t.Fatal("stale generator callback edited the new project")
	}
}

func TestLockDraftFXRerollApplyUndoAndRemovedEffect(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("locks")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	u.addEffect(groove.Chorus)
	id := u.selectedTrack().Effects[0].ID
	before := u.session.Project.Clone()
	u.lockGenerator()
	panel := u.drawerTitled("Parameter-lock generator")
	c := findGeneratorControls(panel)
	var group, parameter, shape *widget.Select
	for _, s := range c.selects {
		switch {
		case len(s.Options) > 0 && s.Options[0] == "Instrument":
			group = s
		case len(s.Options) > 0 && s.Options[0] == string(groove.LockRandom):
			shape = s
		default:
			parameter = s
		}
	}
	if group == nil || parameter == nil || shape == nil {
		t.Fatal("generator selectors missing")
	}
	group.SetSelected("FX 1 / Chorus")
	parameter.SetSelected("Mix")
	shape.SetSelected(string(groove.LockRandom))
	draft := c.preview.pattern
	u.refresh()
	if !reflect.DeepEqual(c.preview.pattern, draft) {
		t.Fatal("ordinary refresh rerolled the random draft")
	}
	test.Tap(c.buttons["Reroll"])
	if reflect.DeepEqual(c.preview.pattern, draft) {
		t.Fatal("Reroll did not change the preview")
	}
	draft = c.preview.pattern
	if !reflect.DeepEqual(u.session.Project, before) {
		t.Fatal("random previews edited the project")
	}
	test.Tap(c.buttons["Apply locks"])
	if !reflect.DeepEqual(u.selectedTrack().Patterns[0], draft) || !reflect.DeepEqual(c.preview.pattern, draft) {
		t.Fatal("Apply did not retain the exact random preview")
	}
	key := groove.EffectLockID(id, "mix")
	for _, step := range u.selectedTrack().Patterns[0].Steps {
		if step.Active {
			if _, ok := step.Locks[key]; !ok {
				t.Fatal("FX locks were not applied")
			}
		}
	}
	u.undo()
	if !reflect.DeepEqual(u.session.Project, before) {
		t.Fatal("lock generation undo failed")
	}
	u.edit("Remove FX", func(p *groove.Project) error { groove.RemoveEffect(p.Track(u.selected), id); return nil })
	if u.drawerTitled("Parameter-lock generator") != nil {
		t.Fatal("removed effect left a stale generator")
	}
	afterRemoval := u.session.Project.Clone()
	test.Tap(c.buttons["Apply locks"])
	if !reflect.DeepEqual(u.session.Project, afterRemoval) {
		t.Fatal("closed FX generator modified the project")
	}
	u.lockGenerator()
	u.edit("Change pattern", func(p *groove.Project) error { p.Pattern = 1; return nil })
	if u.drawerTitled("Parameter-lock generator") != nil {
		t.Fatal("pattern switch left a stale generator")
	}
}
