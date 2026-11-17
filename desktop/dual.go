package desktop

import (
	"fmt"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// dualSoundPanel lays out a mono synth's controls (Dual synth or SH-101) in
// sections that keep their open state per track.
func (u *UI) dualSoundPanel(t *groove.Track) fyne.CanvasObject {
	kind := t.Kind
	id := t.ID
	if u.dualSections == nil {
		u.dualSections = map[string][]bool{}
	}
	if u.dualAccordion != nil {
		open := make([]bool, len(u.dualAccordion.Items))
		for i, item := range u.dualAccordion.Items {
			open[i] = item.Open
		}
		u.dualSections[u.dualAccordionTrack] = open
	}
	definitions := map[string]groove.Parameter{}
	for _, p := range groove.Parameters(kind) {
		definitions[p.ID] = p
	}
	value := func(key string) float64 { return u.session.Project.Track(id).Parameters[key] }
	section := func(ids ...string) fyne.CanvasObject {
		box := container.NewVBox()
		for _, key := range ids {
			p, known := definitions[key]
			if !known {
				continue
			}
			commit := func(value float64) {
				u.edit("Set "+p.Name, func(project *groove.Project) error { project.Track(id).Parameters[p.ID] = value; return nil })
			}
			options := groove.ParameterChoices(kind, key)
			if len(options) > 0 {
				box.Add(widget.NewLabel(p.Name))
				box.Add(u.boundSelect(options, func() int { return int(value(key)) }, func(i int) { commit(float64(i)) }))
			} else if groove.TimeParameter(kind, key) {
				// Logarithmic time controls leave useful room for both millisecond
				// transients and slow envelopes; actual seconds remain in the model.
				toSlider := func(seconds float64) float64 { return math.Log(seconds/p.Min) / math.Log(p.Max/p.Min) }
				fromSlider := func(position float64) float64 { return p.Min * math.Pow(p.Max/p.Min, position) }
				box.Add(u.instrumentParam(id, p, toSlider, fromSlider, func(seconds float64) string { return fmt.Sprintf("%.3f", seconds) }))
			} else {
				box.Add(u.instrumentParam(id, p, nil, nil, nil))
			}
		}
		return box
	}
	var accordion *widget.Accordion
	about := "Dual synth · two oscillators into a 12/24 dB filter with its own envelope. Interval stacks oscillator 2 (a fifth, an octave) so each note is a chord of two. Sync restarts oscillator 2 with oscillator 1: raise its interval, or Osc 2 sweep (the filter envelope bending its pitch up to two octaves), for the tearing sync sound. FM amount bends oscillator 1's phase with oscillator 2 (sines give bells); ring mod multiplies them (metallic). Unison adds two detuned copies of oscillator 1. Slide notes glide; hold mode keeps envelopes running. Randomise keeps tuning, the interval and the attack/sustain shape."
	if kind == groove.Dual {
		accordion = widget.NewAccordion(
			widget.NewAccordionItem("Filter", section("cutoff", "resonance", "drive", "env_amount", "slope", "keytrack")),
			widget.NewAccordionItem("Oscillators", section("osc1_wave", "osc1_level", "unison", "osc2_wave", "osc2_level", "osc2_tune", "detune", "pulse", "sub", "noise", "tune")),
			widget.NewAccordionItem("Sync, FM and ring mod", section("sync", "osc2_env", "fm", "ring")),
			widget.NewAccordionItem("Amplitude envelope", section("amp_attack", "amp_decay", "amp_sustain", "amp_release")),
			widget.NewAccordionItem("Filter envelope", section("filter_attack", "filter_decay", "filter_sustain", "filter_release")),
			widget.NewAccordionItem("Glide / accent", section("glide", "legato", "accent")),
		)
	} else {
		// The SH-101's front panel, left to right.
		accordion = widget.NewAccordion(
			widget.NewAccordionItem("VCF", section("cutoff", "resonance", "env_amount", "filter_lfo", "keytrack")),
			widget.NewAccordionItem("VCO and mixer", section("range", "tune", "saw", "pulse", "pw", "pwm_source", "sub", "sub_mode", "noise", "vco_lfo")),
			widget.NewAccordionItem("Envelope and VCA", section("attack", "decay", "sustain", "release", "env_trigger", "vca_mode")),
			widget.NewAccordionItem("LFO", section("lfo_rate", "lfo_wave")),
			widget.NewAccordionItem("Portamento / accent", section("glide", "glide_mode", "accent")),
		)
		about = "SH-101 · one VCO mixing saw, pulse, a locked sub-oscillator and noise into a self-oscillating 24 dB filter, with one envelope for filter and amp. Pulse width mod: the slider sets the width (Manual) or how far the LFO or envelope sweeps it. Envelope trigger: Gate holds through slides, Gate + trigger restarts on every note, LFO repeats it while a note is held. Portamento Auto glides only on slide steps. Randomise keeps range, tuning and note shape."
	}
	accordion.MultiOpen = true
	if open, exists := u.dualSections[id]; exists {
		for i, yes := range open {
			if yes {
				accordion.Open(i)
			}
		}
	} else {
		accordion.Open(0)
	}
	u.dualAccordion, u.dualAccordionTrack = accordion, id
	help := u.withHelp(widget.NewLabel("Mono voice"), about)
	return container.NewVBox(help, accordion)
}
