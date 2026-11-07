package groove

import (
	"fmt"
	"math/rand/v2"
)

type LFO struct {
	Enabled   bool    `json:"enabled"`
	Target    int     `json:"target"`
	Waveform  int     `json:"waveform"`
	Rate      float64 `json:"rate_hz"`
	Depth     float64 `json:"depth"`
	Sync      int     `json:"sync"` // 0 Hz, 1 whole-note, 2 steps
	Division  float64 `json:"division"`
	Steps     int     `json:"steps"`
	Retrigger bool    `json:"retrigger"`
	Phase     float64 `json:"phase"`
}

func DefaultLFOs() [2]LFO {
	return [2]LFO{{Target: 1, Rate: 2, Depth: .5, Sync: 1, Division: .25, Steps: 16}, {Target: 2, Waveform: 1, Rate: 1, Depth: .3, Sync: 1, Division: 1, Steps: 32}}
}
func LFOTargets(kind Kind) []string {
	switch kind {
	case Dual:
		return []string{"None", "Cutoff", "Resonance", "Pitch", "Pulse width", "Oscillator 2 detune", "Oscillator 2 level", "Filter drive", "Volume"}
	case Acid:
		return []string{"None", "Cutoff", "Resonance", "Pitch", "Envelope", "Pulse width", "Volume"}
	case Kick:
		return []string{"None", "Pitch", "Decay", "Pitch decay", "Drive"}
	case Snare:
		return []string{"None", "Tone mix", "Snap", "Noise decay", "Tone decay"}
	case Clap:
		return []string{"None", "Tune", "Decay", "Tone", "Drive"}
	case Rimshot:
		return []string{"None", "Pitch", "Decay", "Tone mix", "Snap"}
	case Tom:
		return []string{"None", "Pitch", "Decay", "Tune", "Pitch sweep", "Tone mix", "Attack"}
	case Crash:
		return []string{"None", "Decay", "Tone", "Metallic"}
	case ClosedHat, OpenHat:
		return []string{"None", "Tone", "Noise mix", "Decay"}
	}
	return nil
}
func (l LFO) validate(kind Kind) error {
	if len(LFOTargets(kind)) == 0 {
		if l.Enabled {
			return fmt.Errorf("LFO unsupported for %s", kind)
		}
		return nil
	}
	return l.validateTargets(len(LFOTargets(kind)))
}
func (l LFO) validateTargets(targets int) error {
	if l.Target < 0 || l.Target >= targets || l.Waveform < 0 || l.Waveform > 5 ||
		!inRange(l.Rate, .01, 100) || !inRange(l.Depth, 0, 1) || l.Sync < 0 || l.Sync > 2 ||
		!inRange(l.Division, 1./64, 4) || l.Steps < 1 || l.Steps > 128 || !inRange(l.Phase, 0, 1) {
		return fmt.Errorf("invalid LFO settings")
	}
	return nil
}

// archetypeChance is how often Randomise sound starts from one of the
// kind's bank sounds rather than drawing every control independently.
func archetypeChance(kind Kind) float64 {
	switch kind {
	case Acid:
		return .85
	case Dual:
		return .75
	case Sampler:
		return 0
	}
	return .7
}

// randomiseKeeps lists controls Randomise sound leaves as they are: tuning
// (so a line never changes key) and, on the Dual synth, its interval and the
// amp envelope shape that note lengths are written for.
func randomiseKeeps(kind Kind) []string {
	if kind == Dual {
		return []string{"tune", "osc2_tune", "amp_attack", "amp_sustain", "amp_release"}
	}
	if kind == Tom || kind == Kick || kind == Snare || kind == Clap || kind == Acid {
		return []string{"tune"}
	}
	return nil
}

// RandomiseSound gives the track a new sound; see RandomiseSoundFrom.
func RandomiseSound(t *Track) string { return RandomiseSoundFrom(t, newRand()) }

// RandomiseSoundFrom usually picks an archetype from the kind's sound bank
// (its factory sounds) and varies it, so related settings stay together and
// results range across the whole bank instead of averaging to the middle of
// every knob. An empty FX chain also gets the archetype's pedals; an
// existing chain is left alone. Otherwise every control is drawn
// independently. It returns the archetype's name, or "" for an independent
// draw. The same r and starting track give the same result.
func RandomiseSoundFrom(t *Track, r *rand.Rand) string {
	bank := FactorySounds(t.Kind)
	if len(bank) >= 2 && r.Float64() < archetypeChance(t.Kind) {
		patch := bank[r.IntN(len(bank))]
		kept := map[string]float64{}
		for _, id := range randomiseKeeps(t.Kind) {
			kept[id] = t.Parameters[id]
		}
		t.Parameters = cloneMap(patch.Parameters)
		for i, l := range patch.LFOs {
			if l.Enabled {
				t.LFOs[i] = l
			}
		}
		jitterSoundBy(t, r, .1)
		for id, v := range kept {
			t.Parameters[id] = v
		}
		if len(t.Effects) == 0 {
			for _, e := range patch.Effects {
				e.ID, e.Parameters = newID(), cloneMap(e.Parameters)
				t.Effects = append(t.Effects, e)
			}
		}
		return patch.Name
	}
	set := func(id string, low, high float64) { t.Parameters[id] = low + r.Float64()*(high-low) }
	switch t.Kind {
	case Dual:
		t.Parameters["osc1_wave"], t.Parameters["osc2_wave"] = float64(r.IntN(4)), float64(r.IntN(4))
		set("osc2_level", 0, .7)
		set("detune", -15, 15)
		set("pulse", .2, .8)
		set("sub", .1, .5)
		set("noise", 0, .08)
		set("cutoff", .25, .7)
		set("resonance", .1, .75)
		set("drive", .05, .5)
		set("env_amount", .15, .7)
		set("amp_decay", .12, .6)
		set("amp_release", .06, .4)
		set("filter_decay", .08, .7)
	case Acid:
		set("cutoff", .3, .9)
		set("resonance", .2, .9)
		set("env", .4, 1)
		set("decay", .1, 1)
		set("accent", .3, .9)
		set("slide", .03, .18)
		t.Parameters["square"] = float64(r.IntN(2))
		set("sub", 0, .5)
		set("pulse", .2, .8)
		// The character controls are often left off, so plenty of
		// results still sound like the plain voice.
		t.Parameters["drive"], t.Parameters["sweep"], t.Parameters["gate"] = 0, 0, 1
		if r.Float64() < .45 {
			set("drive", .1, .75)
		}
		if r.Float64() < .55 {
			set("sweep", .2, 1)
		}
		if r.Float64() < .5 {
			set("gate", .25, .9)
		}
		t.Parameters["blend"], t.Parameters["noise"], t.Parameters["filter_mode"] = 0, 0, 0
		if r.Float64() < .25 {
			set("blend", .2, 1)
		}
		if r.Float64() < .15 {
			set("noise", .05, .5)
		}
		if r.Float64() < .2 {
			t.Parameters["filter_mode"] = float64(1 + r.IntN(2))
		}
	case Kick:
		set("tune", -6, 6)
		set("decay", .15, .5)
		set("pitch_decay", .005, .05)
		set("drive", .2, .8)
		set("click", 0, .3)
	case Snare:
		set("tune", -6, 6)
		set("tone_decay", .04, .12)
		set("decay", .08, .25)
		set("tone", .3, .9)
		set("snap", .3, .9)
	case ClosedHat, OpenHat:
		set("tone", .3, .95)
		set("noise", .2, .8)
		if t.Kind == ClosedHat {
			set("decay", .02, .12)
		} else {
			set("decay", .1, .6)
		}
	case Clap:
		set("tune", -6, 6)
		set("decay", .03, .15)
		set("tone", .35, .9)
		set("drive", 0, .65)
	case Rimshot:
		set("pitch", 300, 700)
		set("decay", .02, .08)
		set("tone", .25, .9)
		set("snap", .3, .95)
	case Tom:
		set("pitch", 55, 220)
		set("tune", -4, 4)
		set("decay", .15, 1.1)
		set("pitch_decay", .1, .8)
		set("tone", .4, .95)
		set("attack", .1, .8)
	case Crash:
		set("decay", .5, 2.2)
		set("tone", .3, .9)
		set("metallic", .25, .9)
	case Sampler:
		t.Parameters["pitch"] = float64([]int{-12, -7, -5, 0, 5, 7, 12}[r.IntN(7)])
	}
	for i := range t.LFOs {
		l := &t.LFOs[i]
		if !l.Enabled {
			continue
		}
		l.Waveform = r.IntN(6)
		l.Depth = .2 + r.Float64()*.65
		l.Rate = 1 + r.Float64()*8
		l.Division = []float64{1. / 16, 1. / 8, 1. / 4, 1. / 2, 1, 2}[r.IntN(6)]
		l.Steps = []int{4, 8, 16, 32, 64}[r.IntN(5)]
	}
	return ""
}

// TomBassTone tunes the root to C2 and removes the percussive sweep and noise.
// Notes, mixer settings, effects and parameter locks remain available.
func TomBassTone(t *Track) error {
	if t == nil || t.Kind != Tom {
		return fmt.Errorf("select a Tom track for bass tone")
	}
	t.Parameters["pitch"] = 65.40639132514966
	t.Parameters["tune"] = 0
	t.Parameters["pitch_decay"] = 0
	t.Parameters["tone"] = 1
	t.Parameters["attack"] = 0
	t.Parameters["decay"] = .4
	for i := range t.LFOs {
		t.LFOs[i].Enabled = false
	}
	return nil
}
