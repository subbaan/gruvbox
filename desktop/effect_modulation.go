package desktop

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func (u *UI) effectModulation(trackID, effectID string, position int) {
	track := u.session.Project.Track(trackID)
	if track == nil {
		return
	}
	effect := track.Effect(effectID)
	if effect == nil {
		return
	}
	title := fmt.Sprintf("FX %d / %s / %s modulation", position+1, track.Name, effect.Kind.Name())
	if u.effectModulationRefresh == nil {
		u.effectModulationRefresh = map[string]func(){}
	}
	w, _ := u.openDrawer(title, func() { delete(u.effectModulationRefresh, title) })
	session := u.session
	var rebuild func()
	rebuild = func() {
		if u.session != session {
			w.Close()
			return
		}
		track := session.Project.Track(trackID)
		if track == nil || track.Effect(effectID) == nil {
			w.Close()
			return
		}
		effect := track.Effect(effectID)
		settings := func() [2]groove.LFO {
			settings := effect.LFOs
			if t := u.session.Project.Track(trackID); t != nil && t.Effect(effectID) != nil {
				settings = t.Effect(effectID).LFOs
			}
			for i := range settings {
				if settings[i] == (groove.LFO{}) {
					settings[i] = groove.DefaultLFOs()[i]
				}
			}
			return settings
		}
		controls := u.lfoControls(settings, groove.EffectLFOTargets(effect.Kind), func(index int, label string, fn func(*groove.LFO)) {
			if u.session != session || u.session.Project.Track(trackID) == nil || u.session.Project.Track(trackID).Effect(effectID) == nil {
				w.Close()
				return
			}
			u.edit("FX LFO "+label, func(p *groove.Project) error {
				lfo := &p.Track(trackID).Effect(effectID).LFOs[index]
				if *lfo == (groove.LFO{}) {
					*lfo = groove.DefaultLFOs()[index]
				}
				fn(lfo)
				return nil
			})

		})
		help := widget.NewLabel("LFOs move around the current knob or step-lock value. Depth limits the excursion; two LFOs targeting the same control add together. FX modulation also works on sampler tracks.")
		help.Wrapping = fyne.TextWrapWord
		w.SetContent(container.NewPadded(container.NewVScroll(container.NewVBox(help, controls))))
	}
	u.effectModulationRefresh[title] = rebuild
	rebuild()
}
