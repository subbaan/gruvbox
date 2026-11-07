package groove

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
)

type EffectKind string

const (
	Distortion EffectKind = "distortion"
	Filter     EffectKind = "filter"
	Delay      EffectKind = "delay"
	Reverb     EffectKind = "reverb"
	Chorus     EffectKind = "chorus"
	BitCrusher EffectKind = "bitcrusher"
	// Pedals (alpha.35).
	Overdrive      EffectKind = "overdrive"
	Fuzz           EffectKind = "fuzz"
	EnvelopeFilter EffectKind = "envfilter"
	Phaser         EffectKind = "phaser"
	Flanger        EffectKind = "flanger"
	TapeEcho       EffectKind = "tape"
	Compressor     EffectKind = "compressor"
)

// EffectKinds is the order of the Add menu: drive pedals first, then filters,
// modulation, time and dynamics.
var EffectKinds = []EffectKind{Overdrive, Fuzz, Distortion, EnvelopeFilter, Filter, Phaser, Flanger, Chorus, TapeEcho, Delay, Reverb, BitCrusher, Compressor}

// TempoSynced kinds have time/sync/division controls that follow the BPM.
func (k EffectKind) TempoSynced() bool { return k == Delay || k == TapeEcho }

// EffectChoices names the options of a stepped selector parameter, or nil
// if id is a continuous control.
func EffectChoices(kind EffectKind, id string) []string {
	switch {
	case kind == Filter && id == "mode":
		return []string{"Low-pass", "High-pass", "Band-pass"}
	case kind == EnvelopeFilter && id == "shape":
		return []string{"Low-pass", "Band-pass"}
	case kind == EnvelopeFilter && id == "direction":
		return []string{"Sweep up", "Sweep down"}
	case kind == Phaser && id == "stages":
		return []string{"4 stages", "6 stages", "8 stages", "12 stages"}
	}
	return nil
}

// PhaserStages maps the stages selector to a stage count.
var PhaserStages = []int{4, 6, 8, 12}

func (k EffectKind) Name() string {
	switch k {
	case Distortion:
		return "Distortion"
	case Filter:
		return "Filter"
	case Delay:
		return "Delay"
	case Reverb:
		return "Reverb"
	case Chorus:
		return "Chorus"
	case BitCrusher:
		return "Bit crusher"
	case Overdrive:
		return "Overdrive"
	case Fuzz:
		return "Fuzz"
	case EnvelopeFilter:
		return "Envelope filter"
	case Phaser:
		return "Phaser"
	case Flanger:
		return "Flanger"
	case TapeEcho:
		return "Tape echo"
	case Compressor:
		return "Compressor"
	}
	return string(k)
}

type Effect struct {
	LFOs       [2]LFO             `json:"lfos,omitempty"`
	ID         string             `json:"id"`
	Kind       EffectKind         `json:"kind"`
	Enabled    bool               `json:"enabled"`
	Parameters map[string]float64 `json:"parameters"`
}

func EffectParameters(k EffectKind) []Parameter {
	switch k {
	case Distortion:
		return []Parameter{{"drive", "Drive", 1, 20, 2, .1, true}, {"tone", "Tone", 0, 1, .7, .01, true}, {"mix", "Mix", 0, 1, .3, .01, true}}
	case Filter:
		return []Parameter{{"cutoff", "Cutoff (Hz)", 20, 20000, 4000, 10, true}, {"resonance", "Resonance", 0, .95, .2, .01, true}, {"mode", "Mode", 0, 2, 0, 1, false}, {"mix", "Mix", 0, 1, 1, .01, true}}
	case Delay:
		return []Parameter{{"time", "Time (seconds)", .001, 2, .25, .001, true}, {"sync", "Tempo sync", 0, 1, 1, 1, false}, {"division", "Tempo division", 1. / 64, 1, .125, 1. / 64, true}, {"feedback", "Feedback", 0, .85, .3, .01, true}, {"mix", "Mix", 0, 1, .25, .01, true}}
	case Reverb:
		return []Parameter{{"room", "Room size", 0, 1, .5, .01, true}, {"damping", "Damping", 0, 1, .5, .01, true}, {"mix", "Mix", 0, 1, .2, .01, true}}
	case Chorus:
		return []Parameter{{"rate", "Rate (Hz)", .05, 8, .6, .05, true}, {"depth", "Depth", 0, 1, .5, .01, true}, {"delay_ms", "Delay (ms)", 5, 35, 18, .1, true}, {"feedback", "Feedback", 0, .7, 0, .01, true}, {"mix", "Mix", 0, 1, .25, .01, true}}
	case BitCrusher:
		return []Parameter{{"bits", "Bit depth", 1, 16, 8, .1, true}, {"rate", "Rate reduction", 1, 32, 4, .1, true}, {"mix", "Mix", 0, 1, .25, .01, true}}
	case Overdrive:
		return []Parameter{{"drive", "Drive", 0, 1, .5, .01, true}, {"tone", "Tone", 0, 1, .55, .01, true}, {"level", "Level", 0, 1, .5, .01, true}, {"mix", "Mix", 0, 1, 1, .01, true}}
	case Fuzz:
		return []Parameter{{"fuzz", "Fuzz", 0, 1, .6, .01, true}, {"tone", "Tone", 0, 1, .5, .01, true}, {"level", "Level", 0, 1, .5, .01, true}, {"mix", "Mix", 0, 1, 1, .01, true}}
	case EnvelopeFilter:
		return []Parameter{{"sensitivity", "Sensitivity", 0, 1, .6, .01, true}, {"cutoff", "Base", 0, 1, .25, .01, true}, {"range", "Range", 0, 1, .7, .01, true}, {"resonance", "Resonance", 0, .95, .6, .01, true}, {"decay", "Decay (ms)", 10, 1000, 120, 1, true}, {"shape", "Shape", 0, 1, 0, 1, false}, {"direction", "Direction", 0, 1, 0, 1, false}, {"mix", "Mix", 0, 1, 1, .01, true}}
	case Phaser:
		return []Parameter{{"rate", "Rate (Hz)", .02, 8, .4, .01, true}, {"depth", "Depth", 0, 1, .7, .01, true}, {"feedback", "Feedback", 0, .9, .4, .01, true}, {"stages", "Stages", 0, 3, 1, 1, false}, {"mix", "Mix", 0, 1, .5, .01, true}}
	case Flanger:
		return []Parameter{{"rate", "Rate (Hz)", .02, 8, .25, .01, true}, {"depth", "Depth", 0, 1, .7, .01, true}, {"manual", "Manual (ms)", .3, 10, 2.5, .05, true}, {"feedback", "Feedback", 0, .92, .5, .01, true}, {"mix", "Mix", 0, 1, .5, .01, true}}
	case TapeEcho:
		return []Parameter{{"time", "Time (seconds)", .01, 2, .35, .001, true}, {"sync", "Tempo sync", 0, 1, 1, 1, false}, {"division", "Tempo division", 1. / 64, 1, .1875, 1. / 64, true}, {"feedback", "Feedback", 0, .95, .45, .01, true}, {"wow", "Wow & flutter", 0, 1, .3, .01, true}, {"tone", "Tone", 0, 1, .5, .01, true}, {"drive", "Saturation", 0, 1, .3, .01, true}, {"mix", "Mix", 0, 1, .3, .01, true}}
	case Compressor:
		return []Parameter{{"threshold", "Threshold (dB)", -60, 0, -18, .5, true}, {"ratio", "Ratio", 1, 20, 4, .1, true}, {"attack", "Attack (ms)", .1, 200, 10, .1, true}, {"release", "Release (ms)", 5, 2000, 150, 1, true}, {"makeup", "Makeup (dB)", 0, 24, 6, .1, true}, {"mix", "Mix", 0, 1, 1, .01, true}}
	}
	return nil
}
func NewEffect(kind EffectKind) Effect {
	e := Effect{LFOs: DefaultLFOs(), ID: newID(), Kind: kind, Enabled: true, Parameters: map[string]float64{}}
	for _, p := range EffectParameters(kind) {
		e.Parameters[p.ID] = p.Default
	}
	return e
}
func EffectLockID(effectID, parameter string) string { return "fx." + effectID + "." + parameter }
func (t *Track) Effect(id string) *Effect {
	for i := range t.Effects {
		if t.Effects[i].ID == id {
			return &t.Effects[i]
		}
	}
	return nil
}

// LockParameters gives instrument and effect controls one validated namespace.
func LockParameters(t Track) []Parameter {
	params := Parameters(t.Kind)
	for i, e := range t.Effects {
		for _, p := range EffectParameters(e.Kind) {
			p.ID = EffectLockID(e.ID, p.ID)
			p.Name = fmt.Sprintf("FX %d · %s · %s", i+1, e.Kind.Name(), p.Name)
			params = append(params, p)
		}
	}
	return params
}
func (t Track) ParameterValue(id string) float64 {
	if value, ok := t.Parameters[id]; ok {
		return value
	}
	for _, e := range t.Effects {
		for p, value := range e.Parameters {
			if id == EffectLockID(e.ID, p) {
				return value
			}
		}
	}
	return 0
}
func (e Effect) validate() error {
	defs := EffectParameters(e.Kind)
	if e.ID == "" || strings.Contains(e.ID, ".") || len(defs) == 0 {
		return fmt.Errorf("invalid effect %q", e.Kind)
	}
	for _, lfo := range e.LFOs {
		if lfo == (LFO{}) {
			continue
		}
		if err := lfo.validateTargets(len(EffectLFOTargets(e.Kind))); err != nil {
			return fmt.Errorf("%s: %w", e.Kind, err)
		}
	}
	for _, p := range defs {
		v, ok := e.Parameters[p.ID]
		if !ok || !inRange(v, p.Min, p.Max) {
			return fmt.Errorf("invalid %s %s", e.Kind, p.ID)
		}
	}
	for _, p := range defs {
		if v := e.Parameters[p.ID]; (p.ID == "sync" || EffectChoices(e.Kind, p.ID) != nil) && v != math.Trunc(v) {
			return fmt.Errorf("invalid effect selector %s", p.ID)
		}
	}
	return nil
}

// Removal cleans locks referencing the removed unit across all pattern slots.
func RemoveEffect(t *Track, id string) {
	e := t.Effect(id)
	if e == nil {
		return
	}
	for i := range t.Patterns {
		if state := t.Patterns[i].State; state != nil {
			stored := false
			for _, saved := range state.Effects {
				stored = stored || saved.ID == id
			}
			if stored {
				continue // Its stored chain still owns these locks.
			}
		}
		for j := range t.Patterns[i].Steps {
			for _, p := range EffectParameters(e.Kind) {
				delete(t.Patterns[i].Steps[j].Locks, EffectLockID(id, p.ID))
			}
		}
	}
	for i := range t.Effects {
		if t.Effects[i].ID == id {
			t.Effects = append(t.Effects[:i], t.Effects[i+1:]...)
			break
		}
	}
}
func RandomiseEffects(t *Track) {
	if len(t.Effects) == 0 {
		// An empty chain gets a whole pedalboard; settings that belong
		// together then move together instead of every knob going random.
		ApplyPedalboard(t, RandomPedalboard(t.Kind, newRand()), newRand())
		return
	}
	enabled := false
	for i := range t.Effects {
		e := &t.Effects[i]
		e.Enabled = rand.Float64() < .65
		enabled = enabled || e.Enabled
		set := func(id string, min, max float64) { e.Parameters[id] = min + rand.Float64()*(max-min) }
		switch e.Kind {
		case Distortion:
			set("drive", 1, 8)
			set("tone", .3, .9)
			set("mix", .1, .5)
		case Filter:
			e.Parameters["cutoff"] = math.Exp(math.Log(200) + rand.Float64()*math.Log(40))
			set("resonance", 0, .65)
			set("mix", .3, .8)
			e.Parameters["mode"] = float64(rand.IntN(3))
		case Delay:
			set("time", .06, .6)
			set("feedback", .1, .5)
			set("mix", .1, .35)
			e.Parameters["sync"] = 1
			e.Parameters["division"] = []float64{.0625, .125, .1875, .25, .375, .5}[rand.IntN(6)]
		case Chorus:
			set("rate", .15, 2)
			set("depth", .2, .8)
			set("delay_ms", 10, 28)
			set("feedback", 0, .25)
			set("mix", .1, .4)
		case BitCrusher:
			set("bits", 5, 12)
			set("rate", 1, 8)
			set("mix", .1, .4)
		case Reverb:
			set("room", .2, .8)
			set("damping", .2, .8)
			set("mix", .1, .35)
		case Overdrive:
			set("drive", .2, .85)
			set("tone", .35, .75)
			set("level", .4, .55)
		case Fuzz:
			set("fuzz", .4, .95)
			set("tone", .2, .75)
			set("level", .35, .5)
		case EnvelopeFilter:
			set("sensitivity", .4, .85)
			set("cutoff", .1, .4)
			set("range", .5, .95)
			set("resonance", .35, .85)
			set("decay", 50, 400)
			e.Parameters["shape"] = float64(rand.IntN(2))
			e.Parameters["direction"] = float64(rand.IntN(4) / 3) // mostly up
		case Phaser:
			set("rate", .05, 1.2)
			set("depth", .4, .95)
			set("feedback", .2, .75)
			set("mix", .4, .55)
			e.Parameters["stages"] = float64(rand.IntN(len(PhaserStages)))
		case Flanger:
			set("rate", .04, .6)
			set("depth", .4, .95)
			set("manual", .6, 5)
			set("feedback", .3, .85)
			set("mix", .35, .55)
		case TapeEcho:
			set("feedback", .2, .7)
			set("wow", .1, .7)
			set("tone", .2, .7)
			set("drive", .1, .6)
			set("mix", .12, .35)
			e.Parameters["sync"] = 1
			e.Parameters["division"] = []float64{.125, .1875, .25, .375}[rand.IntN(4)]
		case Compressor:
			set("threshold", -32, -12)
			set("ratio", 2, 12)
			set("attack", 2, 30)
			set("release", 60, 300)
			set("makeup", 3, 10)
		}
		for n := range e.LFOs {
			if e.LFOs[n].Enabled {
				e.LFOs[n].Waveform = rand.IntN(6)
				e.LFOs[n].Depth = .1 + rand.Float64()*.35
				e.LFOs[n].Rate = .1 + rand.Float64()*3
			}
		}
	}
	if !enabled {
		t.Effects[0].Enabled = true
	}
}

func EffectModulationParameters(kind EffectKind) []Parameter {
	params := []Parameter{}
	for _, p := range EffectParameters(kind) {
		if p.Lockable {
			params = append(params, p)
		}
	}
	return params
}
func EffectLFOTargets(kind EffectKind) []string {
	targets := []string{"None"}
	for _, p := range EffectModulationParameters(kind) {
		targets = append(targets, p.Name)
	}
	return targets
}
