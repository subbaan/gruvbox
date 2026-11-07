package desktop

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
)

func phraseSelect(c generatorControls, first string) *widget.Select {
	for _, s := range c.selects {
		if len(s.Options) > 0 && s.Options[0] == first {
			return s
		}
	}
	return nil
}

func TestPhraseDraftComponentsModesApplyUndoAndScope(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("phrase")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	w.Resize(fyne.NewSize(1320, 780))
	if w.Content().MinSize().Width > 1320 {
		t.Fatal("generator action deck widened the main window", w.Content().MinSize())
	}
	before := u.session.Project.Clone()
	u.phraseGenerator()
	panel := u.drawerTitled("Phrase generator")
	if panel == nil {
		t.Fatal("phrase generator did not open in a drawer")
	}
	u.phraseGenerator()
	if u.drawerTitled("Phrase generator") != panel {
		t.Fatal("duplicate phrase window")
	}
	c := findGeneratorControls(panel)
	length, mode := phraseSelect(c, "1 bar / 16 steps"), phraseSelect(c, "Whole phrase")
	root, register, scale := phraseSelect(c, "C"), phraseSelect(c, "Octave 1"), phraseSelect(c, "Minor pentatonic")
	if length == nil || mode == nil || root == nil || register == nil || scale == nil || !c.preview.notes {
		t.Fatal("missing phrase controls or note preview")
	}
	length.SetSelectedIndex(2)
	root.SetSelected("F#")
	register.SetSelected("Octave 3")
	scale.SetSelected("Dorian")
	c.sliders["Notes per bar"].SetValue(10)
	c.sliders["Bar variation"].SetValue(0)
	if !reflect.DeepEqual(u.session.Project, before) || u.liveSession != nil || len(c.preview.pattern.Steps) != 64 {
		t.Fatal("draft changed the project, opened a gesture or failed to resize")
	}
	draft := c.preview.pattern
	u.refresh()
	if !reflect.DeepEqual(c.preview.pattern, draft) {
		t.Fatal("ordinary refresh rerolled the draft")
	}
	test.Tap(c.buttons["Pitches"])
	for i, step := range draft.Steps {
		if step.Active != c.preview.pattern.Steps[i].Active {
			t.Fatal("pitch reroll lost rhythm")
		}
	}
	draft = c.preview.pattern
	test.Tap(c.buttons["Feel"])
	for i, step := range draft.Steps {
		if step.Active != c.preview.pattern.Steps[i].Active || step.Note != c.preview.pattern.Steps[i].Note {
			t.Fatal("feel reroll lost melody")
		}
	}
	draft = c.preview.pattern
	u.player.SetPlaying(true)
	test.Tap(c.buttons["Apply phrase"])
	if !reflect.DeepEqual(u.selectedTrack().Patterns[0], draft) || !reflect.DeepEqual(c.preview.pattern, draft) || !u.player.Playing() {
		t.Fatal("Apply did not retain exact preview during playback")
	}
	u.undo()
	if !reflect.DeepEqual(u.session.Project, before) {
		t.Fatal("phrase did not undo once")
	}
	u.redo()
	mode.SetSelected("Feel only")
	if !root.Disabled() || !c.buttons["Pitches"].Disabled() || !c.buttons["Rhythm"].Disabled() || c.buttons["Feel"].Disabled() {
		t.Fatal("feel-only controls misleadingly allow pitch/rhythm rerolls")
	}
	for i, step := range draft.Steps {
		if step.Note != c.preview.pattern.Steps[i].Note || step.Active != c.preview.pattern.Steps[i].Active {
			t.Fatal("feel-only replaced applied melody")
		}
	}
	mode.SetSelected("Pitches only")
	for i, step := range draft.Steps {
		got := c.preview.pattern.Steps[i]
		got.Note = step.Note
		if !reflect.DeepEqual(got, step) {
			t.Fatal("pitch-only replaced applied articulation or rhythm")
		}
	}
	mode.SetSelected("Whole phrase")
	length.SetSelectedIndex(0)
	u.page = 3
	test.Tap(c.buttons["Apply phrase"])
	if u.page != 0 || len(u.selectedTrack().Patterns[0].Steps) != 16 || !c.sliders["Bar variation"].Disabled() {
		t.Fatal("shorter phrase left the editor beyond its end")
	}
	u.edit("Switch pattern", func(p *groove.Project) error { p.Pattern = 1; return nil })
	if u.drawerTitled("Phrase generator") != nil {
		t.Fatal("pattern switch left stale phrase window")
	}
	afterSwitch := u.session.Project.Clone()
	test.Tap(c.buttons["Apply phrase"])
	if !reflect.DeepEqual(u.session.Project, afterSwitch) {
		t.Fatal("closed phrase window edited another pattern")
	}
}

func TestPhraseOnlyPitchedTracksAndRemovedTarget(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("scope")
	u := newUI(a, w, theme.Library{Dir: t.TempDir()})
	w.SetContent(u.content())
	u.selected = u.session.Project.Tracks[2].ID
	u.phraseGenerator()
	if u.drawerTitled("Phrase generator") != nil {
		t.Fatal("unpitched drum opened melodic generator")
	}
	for _, kind := range []groove.Kind{groove.Dual, groove.Tom, groove.Sampler} {
		track := groove.NewTrack(kind, 0)
		u.session.Project.Tracks = append(u.session.Project.Tracks, track)
		u.selected = track.ID
		u.refresh()
		u.phraseGenerator()
		panel := u.drawerTitled("Phrase generator")
		if panel == nil {
			t.Fatal("pitched instrument cannot generate phrases", kind)
		}
		c := findGeneratorControls(panel)
		test.Tap(c.buttons["Apply phrase"])
		if u.session.Project.Validate() != nil {
			t.Fatal("generated invalid pitched pattern")
		}
		if kind == groove.Tom || kind == groove.Sampler {
			for _, step := range u.selectedTrack().Patterns[0].Steps {
				if step.Slide {
					t.Fatal("generated unsupported slide")
				}
			}
		}
		panel.Close()
	}
	u.phraseGenerator()
	u.removeTrack()
	if u.drawerTitled("Phrase generator") != nil {
		t.Fatal("removed target left generator window")
	}
}
