package groove

import (
	"fmt"
	"math"
	"reflect"
)

// TrackState deliberately excludes identity and sequence data. Samples are
// immutable; all mutable sound/FX maps are copied at history/store boundaries.
type TrackState struct {
	Mute            bool               `json:"mute"`
	Solo            bool               `json:"solo"`
	Level           float64            `json:"level"`
	Pan             float64            `json:"pan"`
	Parameters      map[string]float64 `json:"parameters"`
	LFOs            [2]LFO             `json:"lfos"`
	Effects         []Effect           `json:"effects,omitempty"`
	EffectsBypassed bool               `json:"effects_bypassed,omitempty"`
	Sample          *Sample            `json:"sample,omitempty"`
}

func CaptureTrackState(t Track) *TrackState {
	return cloneTrackState(&TrackState{Mute: t.Mute, Solo: t.Solo, Level: t.Level, Pan: t.Pan,
		Parameters: t.Parameters, LFOs: t.LFOs, Effects: t.Effects, EffectsBypassed: t.EffectsBypassed, Sample: t.Sample})
}

func cloneTrackState(state *TrackState) *TrackState {
	if state == nil {
		return nil
	}
	copy := *state
	copy.Parameters = cloneMap(state.Parameters)
	copy.Effects = cloneEffects(state.Effects)
	return &copy
}

func cloneEffects(effects []Effect) []Effect {
	if len(effects) == 0 {
		return nil
	}
	copy := append([]Effect{}, effects...)
	for i := range copy {
		copy[i].Parameters = cloneMap(copy[i].Parameters)
	}
	return copy
}

// withTrackState shares immutable controls when preparing audio on the control
// thread. Editable recall passes it a deep copy instead.
func withTrackState(t Track, state *TrackState) Track {
	if state != nil {
		t.Mute, t.Solo, t.Level, t.Pan = state.Mute, state.Solo, state.Level, state.Pan
		t.Parameters, t.LFOs, t.Effects, t.EffectsBypassed, t.Sample = state.Parameters, state.LFOs, state.Effects, state.EffectsBypassed, state.Sample
	}
	return t
}

func patternTarget(p *Project, slot int, trackID string) error {
	if slot < 0 || slot >= PatternCount {
		return fmt.Errorf("invalid pattern slot")
	}
	if trackID != "" && p.Track(trackID) == nil {
		return fmt.Errorf("track no longer exists")
	}
	return nil
}

// StorePatternState captures all tracks, or one explicitly selected track.
// Notes remain live pattern data; this action stores only the control snapshot.
func StorePatternState(p *Project, slot int, trackID string) error {
	if err := patternTarget(p, slot, trackID); err != nil {
		return err
	}
	for i := range p.Tracks {
		t := &p.Tracks[i]
		if trackID == "" || t.ID == trackID {
			t.Patterns[slot].State = CaptureTrackState(*t)
			prunePatternFXLocks(&t.Patterns[slot], t.Effects)
		}
	}
	return nil
}

func RecallPatternState(p *Project, slot int, trackID string) error {
	if err := patternTarget(p, slot, trackID); err != nil {
		return err
	}
	for i := range p.Tracks {
		t := &p.Tracks[i]
		if (trackID == "" || t.ID == trackID) && t.Patterns[slot].State != nil {
			*t = withTrackState(*t, cloneTrackState(t.Patterns[slot].State))
			for j := range t.Patterns {
				effects := t.Effects
				if state := t.Patterns[j].State; state != nil {
					effects = append(cloneEffects(effects), state.Effects...)
				}
				prunePatternFXLocks(&t.Patterns[j], effects)
			}
		}
	}
	return nil
}

func SelectPattern(p *Project, slot int) error {
	if err := RecallPatternState(p, slot, ""); err != nil {
		return err
	}
	p.Pattern = slot
	return nil
}

func ClearPatternState(p *Project, slot int) error {
	if err := patternTarget(p, slot, ""); err != nil {
		return err
	}
	for i := range p.Tracks {
		t := &p.Tracks[i]
		t.Patterns[slot].State = nil
		prunePatternFXLocks(&t.Patterns[slot], t.Effects)
	}
	return nil
}

func PatternStateStatus(p Project, slot int) (stored, edited int) {
	for _, t := range p.Tracks {
		if state := t.Patterns[slot].State; state != nil {
			stored++
			if !reflect.DeepEqual(state, CaptureTrackState(t)) {
				edited++
			}
		}
	}
	return
}

func sameLiveControls(a, b Track) bool {
	return a.ID == b.ID && a.Kind == b.Kind && a.Mute == b.Mute && a.Solo == b.Solo && a.Level == b.Level && a.Pan == b.Pan && a.LFOs == b.LFOs && a.EffectsBypassed == b.EffectsBypassed && a.Sample == b.Sample && reflect.DeepEqual(a.Parameters, b.Parameters) && reflect.DeepEqual(a.Effects, b.Effects)
}

func prunePatternFXLocks(pattern *Pattern, effects []Effect) {
	allowed := map[string]bool{}
	for _, effect := range effects {
		for _, parameter := range EffectParameters(effect.Kind) {
			allowed[EffectLockID(effect.ID, parameter.ID)] = true
		}
	}
	for i := range pattern.Steps {
		for id := range pattern.Steps[i].Locks {
			if len(id) > 3 && id[:3] == "fx." && !allowed[id] {
				delete(pattern.Steps[i].Locks, id)
			}
		}
	}
}

func validateTrackControls(t Track) error {
	if len(Parameters(t.Kind)) == 0 {
		return fmt.Errorf("unsupported instrument %q", t.Kind)
	}
	for _, lfo := range t.LFOs {
		if err := lfo.validate(t.Kind); err != nil {
			return err
		}
	}
	if t.Sample != nil && (t.Kind != Sampler || t.Sample.Frames() == 0) {
		return fmt.Errorf("invalid sample on %s", t.Name)
	}
	if !inRange(t.Level, 0, 1) || !inRange(t.Pan, -1, 1) {
		return fmt.Errorf("invalid mixer controls on %s", t.Name)
	}
	for _, param := range Parameters(t.Kind) {
		value, ok := t.Parameters[param.ID]
		if !ok || !inRange(value, param.Min, param.Max) || (ParameterChoices(t.Kind, param.ID) != nil && value != math.Trunc(value)) {
			return fmt.Errorf("invalid %s parameter on %s", param.ID, t.Name)
		}
	}
	if t.Kind == Sampler {
		if t.Parameters["start"] >= t.Parameters["end"] {
			return fmt.Errorf("sample trim start must precede end")
		}
		for _, id := range []string{"mode", "reverse", "normalize"} {
			if v := t.Parameters[id]; v != math.Trunc(v) {
				return fmt.Errorf("invalid sampler playback mode")
			}
		}
	}
	if t.Kind == Dual {
		for _, id := range []string{"osc1_wave", "osc2_wave", "slope", "legato"} {
			if v := t.Parameters[id]; v != math.Trunc(v) {
				return fmt.Errorf("invalid Dual synth selector %s", id)
			}
		}
	}
	ids := map[string]bool{}
	for _, effect := range t.Effects {
		if ids[effect.ID] {
			return fmt.Errorf("duplicate effect ID")
		}
		ids[effect.ID] = true
		if err := effect.validate(); err != nil {
			return err
		}
	}
	return nil
}
