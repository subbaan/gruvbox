package groove

import (
	"fmt"
	"math/rand/v2"
)

// A Pedalboard is an effect-chain archetype: pedals in order with ranges
// for the settings that matter, so a randomised board still sounds like its
// idea (a wah into a light drive, a fuzz into tape, and so on). Names and
// settings are original.
type Pedalboard struct {
	Name   string
	Drums  bool // suited to drum tracks rather than basslines
	pedals []fxChance
}

// fixed is a span with one value, for selectors and divisions.
func fixed(v float64) span { return span{v, v} }

var Pedalboards = []Pedalboard{
	{Name: "Squelch wah", pedals: []fxChance{
		{EnvelopeFilter, 1, map[string]span{"shape": fixed(1), "sensitivity": {.55, .8}, "cutoff": {.15, .3}, "range": {.7, .95}, "resonance": {.6, .85}, "decay": {60, 160}}},
		{Overdrive, .7, map[string]span{"drive": {.2, .4}, "tone": {.5, .7}, "level": {.45, .55}}},
		{Delay, .5, map[string]span{"division": fixed(.125), "feedback": {.2, .35}, "mix": {.1, .18}}},
	}},
	{Name: "Green drive", pedals: []fxChance{
		{Overdrive, 1, map[string]span{"drive": {.55, .8}, "tone": {.45, .65}, "level": {.4, .5}}},
		{TapeEcho, .8, map[string]span{"division": fixed(.1875), "feedback": {.3, .45}, "wow": {.15, .35}, "mix": {.15, .25}}},
	}},
	{Name: "Fuzz acid", pedals: []fxChance{
		{Fuzz, 1, map[string]span{"fuzz": {.55, .9}, "tone": {.3, .6}, "level": {.38, .48}}},
		{Reverb, .5, map[string]span{"room": {.2, .45}, "damping": {.5, .8}, "mix": {.06, .12}}},
	}},
	{Name: "Phased acid", pedals: []fxChance{
		{Overdrive, .7, map[string]span{"drive": {.3, .5}, "tone": {.5, .7}, "level": {.45, .55}}},
		{Phaser, 1, map[string]span{"rate": {.08, .35}, "depth": {.6, .9}, "feedback": {.45, .75}, "stages": {1, 2}, "mix": {.45, .55}}},
		{Delay, .5, map[string]span{"division": fixed(.1875), "feedback": {.25, .45}, "mix": {.12, .2}}},
	}},
	{Name: "Jet stream", pedals: []fxChance{
		{Distortion, .5, map[string]span{"drive": {1.5, 3.5}, "tone": {.5, .75}, "mix": {.25, .45}}},
		{Flanger, 1, map[string]span{"rate": {.05, .2}, "depth": {.6, .9}, "manual": {1, 4}, "feedback": {.6, .85}, "mix": {.45, .55}}},
	}},
	{Name: "Dub tape", pedals: []fxChance{
		{Compressor, .8, map[string]span{"threshold": {-24, -16}, "ratio": {3, 5}, "attack": {5, 15}, "release": {100, 250}, "makeup": {4, 8}}},
		{TapeEcho, 1, map[string]span{"division": fixed(.1875), "feedback": {.55, .75}, "wow": {.35, .65}, "tone": {.25, .45}, "drive": {.3, .55}, "mix": {.28, .4}}},
		{Reverb, .7, map[string]span{"room": {.5, .8}, "damping": {.4, .7}, "mix": {.1, .2}}},
	}},
	{Name: "Pump and grind", pedals: []fxChance{
		{Compressor, 1, map[string]span{"threshold": {-32, -24}, "ratio": {8, 14}, "attack": {2, 8}, "release": {60, 150}, "makeup": {3, 6}}},
		{Distortion, 1, map[string]span{"drive": {3, 6}, "tone": {.4, .65}, "mix": {.35, .6}}},
	}},
	{Name: "Falling wah", pedals: []fxChance{
		{EnvelopeFilter, 1, map[string]span{"direction": fixed(1), "sensitivity": {.5, .8}, "cutoff": {.2, .35}, "range": {.6, .9}, "resonance": {.5, .8}, "decay": {150, 400}}},
		{Chorus, .6, map[string]span{"rate": {.2, .6}, "depth": {.3, .6}, "mix": {.15, .3}}},
	}},
	{Name: "Worn cassette", pedals: []fxChance{
		{BitCrusher, .7, map[string]span{"bits": {8, 12}, "rate": {1.5, 3}, "mix": {.2, .4}}},
		{Filter, 1, map[string]span{"mode": fixed(0), "cutoff": {1800, 4500}, "resonance": {.1, .3}, "mix": {1, 1}}},
		{TapeEcho, 1, map[string]span{"division": fixed(.125), "feedback": {.25, .4}, "wow": {.6, 1}, "tone": {.2, .4}, "mix": {.15, .25}}},
	}},
	{Name: "Glue", pedals: []fxChance{
		{Compressor, 1, map[string]span{"threshold": {-20, -12}, "ratio": {2, 4}, "attack": {10, 30}, "release": {120, 300}, "makeup": {3, 6}}},
		{Chorus, .4, map[string]span{"rate": {.15, .4}, "depth": {.2, .4}, "mix": {.1, .2}}},
	}},
	{Name: "Crunch kit", Drums: true, pedals: []fxChance{
		{Compressor, 1, map[string]span{"threshold": {-28, -18}, "ratio": {4, 8}, "attack": {5, 20}, "release": {60, 150}, "makeup": {5, 9}}},
		{Overdrive, 1, map[string]span{"drive": {.3, .6}, "tone": {.4, .7}, "level": {.45, .55}, "mix": {.3, .6}}},
	}},
	{Name: "Tape kit", Drums: true, pedals: []fxChance{
		{TapeEcho, 1, map[string]span{"division": fixed(.1875), "feedback": {.2, .4}, "wow": {.2, .5}, "mix": {.1, .2}}},
	}},
	{Name: "Swirl", Drums: true, pedals: []fxChance{
		{Phaser, .5, map[string]span{"rate": {.1, .5}, "depth": {.5, .9}, "feedback": {.3, .6}, "stages": {0, 3}, "mix": {.4, .5}}},
		{Flanger, .5, map[string]span{"rate": {.05, .3}, "depth": {.5, .9}, "manual": {.5, 3}, "feedback": {.4, .8}, "mix": {.35, .5}}},
	}},
	{Name: "Small room", Drums: true, pedals: []fxChance{
		{Reverb, 1, map[string]span{"room": {.25, .55}, "damping": {.4, .7}, "mix": {.08, .18}}},
	}},
	{Name: "Crushed", Drums: true, pedals: []fxChance{
		{BitCrusher, 1, map[string]span{"bits": {6, 10}, "rate": {2, 6}, "mix": {.2, .45}}},
	}},
}

// PedalboardNamed finds a board by name.
func PedalboardNamed(name string) (Pedalboard, bool) {
	for _, b := range Pedalboards {
		if b.Name == name {
			return b, true
		}
	}
	return Pedalboard{}, false
}

// suitsTrack reports whether a board is meant for this kind of track.
func (b Pedalboard) suitsTrack(kind Kind) bool {
	drums := kind != Acid && kind != Dual && kind != Sampler
	return b.Drums == drums || kind == Sampler
}

// RandomPedalboard picks a board suited to the track's kind.
func RandomPedalboard(kind Kind, r *rand.Rand) Pedalboard {
	suited := []Pedalboard{}
	for _, b := range Pedalboards {
		if b.suitsTrack(kind) {
			suited = append(suited, b)
		}
	}
	return suited[r.IntN(len(suited))]
}

// ApplyPedalboard replaces the track's chain with the board's pedals. Each
// pedal is included by its chance (at least one always is) and its listed
// settings are drawn from their ranges; other settings keep their defaults.
func ApplyPedalboard(t *Track, b Pedalboard, r *rand.Rand) {
	for len(t.Effects) > 0 {
		RemoveEffect(t, t.Effects[0].ID)
	}
	t.EffectsBypassed = false
	for _, pedal := range b.pedals {
		if r.Float64() >= pedal.chance {
			continue
		}
		t.Effects = append(t.Effects, pedalEffect(pedal, r))
	}
	if len(t.Effects) == 0 {
		t.Effects = append(t.Effects, pedalEffect(b.pedals[0], r))
	}
}

// pedalEffect makes an enabled effect from a pedal recipe, rounding stepped
// controls to whole values.
func pedalEffect(pedal fxChance, r *rand.Rand) Effect {
	e := NewEffect(pedal.kind)
	for _, param := range EffectParameters(pedal.kind) {
		s, ok := pedal.params[param.ID]
		if !ok {
			continue
		}
		value := s.pick(r)
		if param.Increment >= 1 {
			value = float64(int(value + .5))
		}
		e.Parameters[param.ID] = max(param.Min, min(param.Max, value))
	}
	if _, ok := pedal.params["division"]; ok {
		e.Parameters["sync"] = 1
	}
	return e
}

// PedalboardInfo describes a board for the status line.
func PedalboardInfo(b Pedalboard, t Track) string {
	names := ""
	for i, e := range t.Effects {
		if i > 0 {
			names += " → "
		}
		names += e.Kind.Name()
	}
	return fmt.Sprintf("%s: %s", b.Name, names)
}
