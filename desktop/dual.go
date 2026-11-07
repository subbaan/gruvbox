package desktop

import (
	"fmt"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) dualSoundPanel(t *groove.Track) fyne.CanvasObject {
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
	for _, p := range groove.Parameters(groove.Dual) {
		definitions[p.ID] = p
	}
	value := func(key string) float64 { return u.session.Project.Track(id).Parameters[key] }
	section := func(ids ...string) fyne.CanvasObject {
		box := container.NewVBox()
		for _, key := range ids {
			key, p := key, definitions[key]
			commit := func(value float64) {
				u.edit("Set "+p.Name, func(project *groove.Project) error { project.Track(id).Parameters[p.ID] = value; return nil })
			}
			var options []string
			switch key {
			case "osc1_wave", "osc2_wave":
				options = []string{"Saw", "Pulse", "Triangle", "Sine"}
			case "slope":
				options = []string{"12 dB low-pass", "24 dB low-pass"}
			case "legato":
				options = []string{"Retrigger envelopes on slide", "Hold envelopes on slide"}
			}
			if len(options) > 0 {
				box.Add(widget.NewLabel(p.Name))
				box.Add(u.boundSelect(options, func() int { return int(value(key)) }, func(i int) { commit(float64(i)) }))
			} else if strings.HasSuffix(key, "_attack") || strings.HasSuffix(key, "_decay") || strings.HasSuffix(key, "_release") {
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
	accordion := widget.NewAccordion(
		widget.NewAccordionItem("Filter", section("cutoff", "resonance", "drive", "env_amount", "slope", "keytrack")),
		widget.NewAccordionItem("Oscillators", section("osc1_wave", "osc2_wave", "osc2_level", "osc2_tune", "detune", "pulse", "sub", "noise", "tune")),
		widget.NewAccordionItem("Amplitude envelope", section("amp_attack", "amp_decay", "amp_sustain", "amp_release")),
		widget.NewAccordionItem("Filter envelope", section("filter_attack", "filter_decay", "filter_sustain", "filter_release")),
		widget.NewAccordionItem("Glide / accent", section("glide", "legato", "accent")),
	)
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
	help := u.withHelp(widget.NewLabel("Mono voice"), "Mono voice · slide notes glide; hold mode keeps envelopes running. Randomise keeps tuning and attack/sustain shape. Use note length for sustained sounds.")
	return container.NewVBox(help, accordion)
}
