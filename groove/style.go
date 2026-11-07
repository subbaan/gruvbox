package groove

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
)

// A groove style is a recipe for a whole starting groove: tempo, feel, key,
// how each part is written and how it sounds. Values vary within ranges so
// two grooves in the same style still differ.

type span [2]float64

func (s span) pick(r *rand.Rand) float64 { return s[0] + r.Float64()*(s[1]-s[0]) }

type fxChance struct {
	kind   EffectKind
	chance float64
	params map[string]span
}

// partRecipe describes one track: generator actions (melodic) or drum
// families, sound ranges and pedal chances. An empty actions list leaves the
// part silent.
type partRecipe struct {
	actions []string
	sound   map[string]span
	square  float64 // 303: chance of the square wave
	lfo     float64 // chance of a slow, tempo-synced cutoff sweep
	octave  int     // transpose generated notes (counter lines sit higher)
	fx      []fxChance
	board   float64  // chance of a whole pedalboard instead of the fx list
	boards  []string // pedalboards to choose from
	patch   float64  // chance of a bank sound (voice, LFO and pedals) instead of all the above
	patches []string // bank sounds (FactorySounds names) to choose from
	hold    bool     // notes ring on to the next one (pads)
	level   float64  // track level, if set
}

type GrooveStyle struct {
	Name         string
	bars         []int // pattern lengths in bars to pick from (repeats weight them)
	bpm, swing   span
	scales       []PhraseScale
	lead, second partRecipe
	drums        map[Kind]partRecipe
	extras       []extraPart
	layout       layoutRecipe
	dual         []partRecipe
}

// A groove's layout (alpha.47) is drawn from the whole pool of track kinds:
// a 303 lead, Kick and Closed Hat always; a snare, a clap or both for the
// backbeat; a second 303 or a Dual synth; two to four of Open Hat, Rimshot,
// Tom and Crash; and one or two silent tracks with a sound ready, for Jam or
// you to bring in.
type layoutRecipe struct {
	snare, clap, both float64          // backbeat weights
	dual              float64          // chance the second voice is a Dual synth
	optional          map[Kind]float64 // weights for Open Hat, Rimshot, Tom, Crash
}

// optionalKinds are the drums a layout picks two to four of.
var optionalKinds = []Kind{OpenHat, Rimshot, Tom, Crash}

// extraPart is the style's recipe and level for a drum outside the original
// six-track layout.
type extraPart struct {
	kind   Kind
	level  float64
	recipe partRecipe
}

// extras builds a style's Clap, Rimshot, Tom and Crash parts from the
// families each may use.
func extras(clap, rim, tom, crash []string) []extraPart {
	return []extraPart{
		{Clap, .6, partRecipe{patch: .7, patches: []string{"Dry hands", "Warehouse clap", "Tight clap", "Hall clap", "Dirty clap", "Dub clap"}, actions: clap}},
		{Rimshot, .55, partRecipe{patch: .7, patches: []string{"Wood knock", "Sharp rim", "Clave", "Block", "Echo rim"}, actions: rim}},
		{Tom, .6, partRecipe{patch: .7, patches: []string{"Low tom", "Tiny conga", "Floor tom", "Electro tom", "Bongo", "Pitched echoes"}, actions: tom}},
		{Crash, .45, partRecipe{patch: .7, patches: []string{"Short splash", "Dark crash", "Trash crash"}, actions: crash}},
	}
}

// extra finds the style's recipe for kind among its extra parts.
func (s GrooveStyle) extra(kind Kind) (extraPart, bool) {
	for _, x := range s.extras {
		if x.kind == kind {
			return x, true
		}
	}
	return extraPart{}, false
}

// Dual synth parts. "Layer" doubles the 303 lead an octave down; hold lets
// notes ring to the next one, for pads.
var (
	dualCounter = partRecipe{level: .36, patch: .6, patches: []string{"Pulse pluck", "Detuned lead"}, actions: []string{"Sparse"}, octave: 12,
		sound: map[string]span{"cutoff": {.35, .6}, "resonance": {.25, .55}, "env_amount": {.3, .6}, "amp_decay": {.12, .3}, "amp_sustain": {.1, .4}},
		fx:    []fxChance{delayDotted, room}}
	dualStab = partRecipe{level: .36, patch: .6, patches: []string{"Pulse pluck", "Detuned lead"}, actions: []string{"Busy", "Sparse"}, octave: 12,
		sound: map[string]span{"cutoff": {.4, .65}, "resonance": {.3, .6}, "env_amount": {.4, .7}, "amp_decay": {.06, .15}, "amp_sustain": {0, .1}, "amp_release": {.03, .08}},
		fx:    []fxChance{chorus, delayEighth}}
	dualPad = partRecipe{level: .34, patch: .6, patches: []string{"Slow pad", "Dub interval"}, actions: []string{"Sparse"}, octave: 12, hold: true,
		sound: map[string]span{"cutoff": {.25, .45}, "resonance": {.15, .4}, "amp_attack": {.05, .3}, "amp_sustain": {.5, .8}, "amp_release": {.3, .8}, "env_amount": {.1, .3}},
		fx:    []fxChance{{Delay, .9, map[string]span{"division": {.1875, .1875}, "feedback": {.45, .65}, "mix": {.2, .35}}}, chorus, room}}
	dualLayer = partRecipe{level: .28, patch: .5, patches: []string{"Rounded mono", "Rubber bass"}, actions: []string{"Layer"},
		sound: map[string]span{"cutoff": {.2, .35}, "resonance": {.1, .3}, "detune": {8, 16}, "osc2_level": {.35, .6}, "sub": {.3, .55}, "amp_decay": {.15, .35}}}
)

var delayEighth = fxChance{Delay, .6, map[string]span{"division": {.125, .125}, "feedback": {.2, .4}, "mix": {.12, .22}}}
var delayDotted = fxChance{Delay, .8, map[string]span{"division": {.1875, .1875}, "feedback": {.35, .55}, "mix": {.18, .3}}}
var dirt = fxChance{Distortion, .5, map[string]span{"drive": {2.5, 7}, "tone": {.4, .75}, "mix": {.2, .45}}}
var room = fxChance{Reverb, .5, map[string]span{"room": {.3, .7}, "damping": {.4, .7}, "mix": {.08, .18}}}
var chorus = fxChance{Chorus, .5, map[string]span{"rate": {.2, .8}, "depth": {.3, .6}, "mix": {.15, .3}}}
var crush = fxChance{BitCrusher, .35, map[string]span{"bits": {7, 11}, "rate": {1.5, 4}, "mix": {.12, .3}}}

var GrooveStyles = []GrooveStyle{
	{
		Name: "Acid", bars: []int{1, 2, 2, 4}, bpm: span{124, 136}, swing: span{0, .08},
		scales: []PhraseScale{PhraseMinorPentatonic, PhraseMinor, PhrasePhrygian},
		lead: partRecipe{patch: .5, patches: []string{"Night squelch", "Chirp", "Fizz acid", "Screamer", "Phase squeal", "Resonant acid", "Laser tag"}, actions: []string{"Acid", "Busy"}, square: .25, lfo: .2,
			sound: map[string]span{"cutoff": {.35, .6}, "resonance": {.72, .92}, "env": {.6, 1}, "decay": {.2, .55}, "accent": {.6, .9}, "sub": {.1, .35}, "sweep": {.4, 1}, "drive": {0, .35}},
			fx:    []fxChance{dirt, delayEighth}, board: .65, boards: []string{"Squelch wah", "Green drive", "Fuzz acid", "Phased acid"}},
		second: partRecipe{patch: .4, patches: []string{"Ring bell", "Needle lead", "Staccato stab", "Glass acid"}, actions: []string{"Sparse", ""}, octave: 12, square: .5,
			sound: map[string]span{"cutoff": {.45, .7}, "resonance": {.5, .8}, "env": {.4, .8}, "decay": {.15, .35}, "sub": {0, .15}},
			fx:    []fxChance{delayDotted}, board: .3, boards: []string{"Phased acid", "Jet stream"}},
		drums: map[Kind]partRecipe{
			Kick:      {patch: .4, patches: []string{"Tight punch", "Driven floor", "Crunch box"}, actions: []string{"Four", "Four push", "Four skip"}, sound: map[string]span{"decay": {.3, .5}, "drive": {.3, .6}}},
			Snare:     {patch: .35, patches: []string{"Dry snap", "Electro snare", "Tight crack"}, actions: []string{"Backbeat", "Backbeat ghosts"}},
			ClosedHat: {patch: .35, patches: []string{"Tight tick", "Glass tick"}, actions: []string{"8ths", "16ths", "Offbeat 8ths"}},
			OpenHat:   {patch: .35, patches: []string{"Bright lift", "Short open"}, actions: []string{"Offbeat", "Lift"}},
		},
		extras: extras([]string{"Backbeat", "Stacked"}, []string{"Syncopated", "Sparse"}, []string{"Call", "Sparse"}, []string{"Downbeat"}),
		layout: layoutRecipe{snare: .6, clap: .15, both: .25, dual: .3, optional: map[Kind]float64{OpenHat: .9, Rimshot: .3, Tom: .25, Crash: .5}},
		dual:   []partRecipe{dualCounter, dualStab},
	},
	{
		Name: "Minimal", bars: []int{1, 2, 4, 4}, bpm: span{120, 128}, swing: span{.04, .16},
		scales: []PhraseScale{PhraseMinor, PhraseDorian, PhraseMinorPentatonic},
		lead: partRecipe{patch: .5, patches: []string{"Rubber worm", "Soft sub", "Wah walker", "Tremble", "Rounded sub", "Hollow pipe", "Hollow band"}, actions: []string{"Sparse", "Acid"}, lfo: .5,
			sound: map[string]span{"cutoff": {.25, .45}, "resonance": {.55, .78}, "env": {.3, .6}, "decay": {.15, .35}, "accent": {.4, .7}, "sub": {.2, .5}, "gate": {.35, .7}, "sweep": {0, .4}},
			fx:    []fxChance{delayEighth, room}, board: .45, boards: []string{"Falling wah", "Glue", "Dub tape", "Squelch wah"}},
		second: partRecipe{patch: .4, patches: []string{"Hollow pipe", "Glass acid", "PWM drift"}, actions: []string{"", "Sparse"}, octave: 12,
			sound: map[string]span{"cutoff": {.3, .5}, "resonance": {.4, .65}, "env": {.3, .5}, "decay": {.1, .25}},
			fx:    []fxChance{delayDotted, room}, board: .25, boards: []string{"Jet stream", "Dub tape"}},
		drums: map[Kind]partRecipe{
			Kick:      {patch: .4, patches: []string{"Soft pillow", "Tight punch", "Deep thump"}, actions: []string{"Four", "Four skip"}, sound: map[string]span{"decay": {.2, .35}, "click": {.1, .3}, "drive": {.1, .3}}},
			Snare:     {patch: .35, patches: []string{"Dry snap", "Tight crack"}, actions: []string{"", "Late", "Backbeat"}, sound: map[string]span{"decay": {.06, .12}, "snap": {.6, .9}}},
			ClosedHat: {patch: .4, patches: []string{"Glass tick", "Tight tick", "Phased hat"}, actions: []string{"Sparse ticks", "Offbeat 8ths", "Shuffle"}, sound: map[string]span{"decay": {.02, .05}}},
			OpenHat:   {patch: .35, patches: []string{"Short open", "Bright lift"}, actions: []string{"Offbeat", "Two lifts", ""}},
		},
		extras: extras([]string{"Halftime", "Offbeat flam", "Backbeat"}, []string{"Syncopated", "Clave", "Sparse", "Rolling"}, []string{"Sparse", "Call"}, []string{"Downbeat", "Sparse"}),
		layout: layoutRecipe{snare: .4, clap: .35, both: .25, dual: .35, optional: map[Kind]float64{OpenHat: .6, Rimshot: .8, Tom: .3, Crash: .3}},
		dual:   []partRecipe{dualCounter, dualPad},
	},
	{
		Name: "Electro", bars: []int{1, 1, 2, 4}, bpm: span{118, 132}, swing: span{0, .04},
		scales: []PhraseScale{PhraseMinor, PhrasePhrygian, PhraseMinorPentatonic},
		lead: partRecipe{patch: .5, patches: []string{"Staccato stab", "Grit pump", "Dry square", "Cassette bass", "Laser tag"}, actions: []string{"Busy", "Acid"}, square: .7,
			sound: map[string]span{"cutoff": {.35, .55}, "resonance": {.4, .65}, "env": {.4, .7}, "decay": {.1, .25}, "accent": {.5, .8}, "sub": {.2, .45}, "gate": {.25, .55}, "drive": {.2, .6}, "pulse": {.25, .45}},
			fx:    []fxChance{crush, chorus}, board: .55, boards: []string{"Worn cassette", "Pump and grind", "Jet stream", "Fuzz acid"}},
		second: partRecipe{patch: .4, patches: []string{"Needle lead", "Ring bell", "Staccato stab", "Paper cut"}, actions: []string{"Sparse", ""}, octave: 12, square: .5,
			sound: map[string]span{"cutoff": {.5, .7}, "resonance": {.3, .55}, "decay": {.1, .2}, "gate": {.3, .5}},
			fx:    []fxChance{delayEighth}, board: .3, boards: []string{"Worn cassette", "Phased acid"}},
		drums: map[Kind]partRecipe{
			Kick:      {patch: .45, patches: []string{"Electro zap", "Crunch box", "Driven floor"}, actions: []string{"Electro", "Electro double", "Broken"}, sound: map[string]span{"decay": {.35, .7}, "pitch_decay": {.02, .05}, "drive": {.4, .8}}, board: .3, boards: []string{"Crunch kit"}},
			Snare:     {patch: .4, patches: []string{"Electro snare", "Tight crack", "Lo-fi snare"}, actions: []string{"Electro", "Backbeat"}, sound: map[string]span{"tone": {.4, .7}, "snap": {.7, .95}}},
			ClosedHat: {patch: .35, patches: []string{"Tight tick", "Glass tick", "Dusty hat"}, actions: []string{"16ths", "Gallop", "8ths"}, sound: map[string]span{"decay": {.02, .06}, "tone": {.7, .95}}},
			OpenHat:   {patch: .35, patches: []string{"Short open", "Jet open"}, actions: []string{"Lift", "Two lifts", ""}},
		},
		extras: extras([]string{"Backbeat", "Stacked", "Offbeat flam"}, []string{"Clave", "Syncopated"}, []string{"Call", "Toms"}, []string{"Downbeat"}),
		layout: layoutRecipe{snare: .35, clap: .3, both: .35, dual: .5, optional: map[Kind]float64{OpenHat: .5, Rimshot: .5, Tom: .6, Crash: .3}},
		dual:   []partRecipe{dualStab, dualStab, dualLayer},
	},
	{
		Name: "Dub", bars: []int{2, 4, 4}, bpm: span{112, 124}, swing: span{.06, .2},
		scales: []PhraseScale{PhraseMinor, PhraseDorian},
		lead: partRecipe{patch: .55, patches: []string{"Dub pressure", "Soft sub", "Swamp", "Falling sigh"}, actions: []string{"Sparse"}, lfo: .3,
			sound: map[string]span{"cutoff": {.18, .35}, "resonance": {.2, .5}, "env": {.2, .45}, "decay": {.4, .8}, "accent": {.3, .6}, "sub": {.45, .75}},
			fx:    []fxChance{delayDotted, room}, board: .6, boards: []string{"Dub tape", "Glue", "Falling wah"}},
		second: partRecipe{patch: .4, patches: []string{"Ring bell", "Glass acid", "Hollow pipe", "Breath"}, actions: []string{"Sparse"}, octave: 24, square: .4,
			sound: map[string]span{"cutoff": {.35, .55}, "resonance": {.45, .7}, "env": {.3, .6}, "decay": {.15, .3}, "sub": {0, .1}},
			fx:    []fxChance{{Delay, 1, map[string]span{"division": {.1875, .1875}, "feedback": {.5, .7}, "mix": {.25, .4}}}, chorus}, board: .5, boards: []string{"Dub tape", "Jet stream"}},
		drums: map[Kind]partRecipe{
			Kick:      {patch: .45, patches: []string{"Dub boom", "Long boom", "Deep thump"}, actions: []string{"Dub", "Four", "Halftime"}, sound: map[string]span{"decay": {.45, .8}, "tune": {-4, 0}, "click": {0, .1}}},
			Snare:     {patch: .4, patches: []string{"Big room", "Fat body", "Dusty backbeat"}, actions: []string{"Halftime", "Late"}, sound: map[string]span{"decay": {.15, .3}, "tone": {.4, .7}}, board: .6, boards: []string{"Tape kit", "Small room"}},
			ClosedHat: {patch: .35, patches: []string{"Soft shuffle", "Swish", "Dusty hat"}, actions: []string{"Shuffle", "Offbeat 8ths"}, sound: map[string]span{"decay": {.04, .1}, "tone": {.4, .7}}},
			OpenHat:   {patch: .4, patches: []string{"Dark wash", "Loose wash"}, actions: []string{"Offbeat", "Sparse"}, sound: map[string]span{"decay": {.3, .6}}},
		},
		extras: extras([]string{"Halftime", "Offbeat flam"}, []string{"Sparse", "Rolling", "Clave"}, []string{"Sparse", "Call"}, []string{"Downbeat", "Sparse"}),
		layout: layoutRecipe{snare: .5, clap: .3, both: .2, dual: .5, optional: map[Kind]float64{OpenHat: .7, Rimshot: .8, Tom: .4, Crash: .3}},
		dual:   []partRecipe{dualPad, dualPad, dualCounter},
	},
	{
		Name: "Hypnotic", bars: []int{1, 2, 4, 4}, bpm: span{128, 140}, swing: span{0, .06},
		scales: []PhraseScale{PhrasePhrygian, PhraseMinorPentatonic, PhraseMinor},
		lead: partRecipe{patch: .55, patches: []string{"Night squelch", "Wobble", "Phase squeal", "Jet acid", "Slow motion"}, actions: []string{"Acid", "Busy"}, lfo: .8,
			sound: map[string]span{"cutoff": {.3, .5}, "resonance": {.65, .88}, "env": {.5, .8}, "decay": {.25, .5}, "accent": {.5, .8}, "sub": {.15, .4}, "sweep": {.5, 1}, "gate": {.6, 1}},
			fx:    []fxChance{delayEighth, chorus}, board: .6, boards: []string{"Phased acid", "Squelch wah", "Jet stream", "Green drive"}},
		second: partRecipe{patch: .4, patches: []string{"Tremble", "PWM drift", "Glass acid"}, actions: []string{"Sparse", "Acid"}, octave: 12, lfo: .5,
			sound: map[string]span{"cutoff": {.4, .6}, "resonance": {.55, .8}, "env": {.4, .7}, "decay": {.15, .35}},
			fx:    []fxChance{delayDotted, room}, board: .35, boards: []string{"Dub tape", "Phased acid"}},
		drums: map[Kind]partRecipe{
			Kick:      {patch: .4, patches: []string{"Driven floor", "Tight punch", "Crunch box"}, actions: []string{"Four", "Rolling"}, sound: map[string]span{"decay": {.3, .45}, "drive": {.4, .75}}},
			Snare:     {patch: .3, patches: []string{"Dry snap", "Big room"}, actions: []string{"", "Backbeat"}},
			ClosedHat: {patch: .4, patches: []string{"Phased hat", "Tight tick"}, actions: []string{"Rolling", "16ths", "Offbeat 8ths"}, board: .4, boards: []string{"Swirl"}},
			OpenHat:   {patch: .4, patches: []string{"Jet open", "Long sizzle"}, actions: []string{"Offbeat"}},
		},
		extras: extras([]string{"Backbeat", "Halftime", "Stacked"}, []string{"Rolling", "Syncopated"}, []string{"Toms", "Sparse"}, []string{"Downbeat"}),
		layout: layoutRecipe{snare: .45, clap: .25, both: .3, dual: .35, optional: map[Kind]float64{OpenHat: .9, Rimshot: .5, Tom: .3, Crash: .45}},
		dual:   []partRecipe{dualCounter, dualPad, dualLayer},
	},
	{
		Name: "Broken", bars: []int{2, 2, 4}, bpm: span{126, 138}, swing: span{.08, .22},
		scales: []PhraseScale{PhraseDorian, PhraseMinorPentatonic, PhraseMajorPentatonic},
		lead: partRecipe{patch: .5, patches: []string{"Molten", "Fizz acid", "Rubber worm", "Wah walker", "Grit pump"}, actions: []string{"Acid", "Sparse"}, square: .4,
			sound: map[string]span{"cutoff": {.3, .6}, "resonance": {.5, .85}, "env": {.4, .9}, "decay": {.15, .45}, "accent": {.5, .9}, "sub": {.1, .4}, "drive": {.2, .7}, "gate": {.4, .8}},
			fx:    []fxChance{dirt, room}, board: .55, boards: []string{"Fuzz acid", "Green drive", "Falling wah", "Pump and grind"}},
		second: partRecipe{patch: .4, patches: []string{"Staccato stab", "Cassette bass", "Needle lead", "Half and half"}, actions: []string{"Sparse", ""}, octave: 12,
			sound: map[string]span{"cutoff": {.4, .65}, "resonance": {.4, .7}, "decay": {.1, .3}},
			fx:    []fxChance{delayEighth}, board: .3, boards: []string{"Worn cassette", "Jet stream"}},
		drums: map[Kind]partRecipe{
			Kick:      {patch: .4, patches: []string{"Deep thump", "Long boom", "Tight punch"}, actions: []string{"Broken", "Electro", "Four skip"}, sound: map[string]span{"decay": {.25, .45}}},
			Snare:     {patch: .4, patches: []string{"Fat body", "Lo-fi snare", "Wire rattle"}, actions: []string{"Broken", "Backbeat ghosts"}},
			ClosedHat: {patch: .35, patches: []string{"Dusty hat", "Swish", "Soft shuffle"}, actions: []string{"Shuffle", "Gallop", "Sparse ticks"}},
			OpenHat:   {patch: .35, patches: []string{"Short open", "Dark wash"}, actions: []string{"Two lifts", "Lift"}},
		},
		extras: extras([]string{"Offbeat flam", "Stacked", "Backbeat"}, []string{"Syncopated", "Clave", "Rolling"}, []string{"Call", "Toms", "Sparse"}, []string{"Downbeat", "Sparse"}),
		layout: layoutRecipe{snare: .4, clap: .25, both: .35, dual: .45, optional: map[Kind]float64{OpenHat: .6, Rimshot: .7, Tom: .5, Crash: .3}},
		dual:   []partRecipe{dualLayer, dualLayer, dualStab},
	},
}

// GrooveInfo summarises a generated groove for the status line.
func GrooveInfo(style string, p Project) string {
	info := fmt.Sprintf("%s groove · %.0f BPM", style, p.BPM)
	steps := 0
	for _, t := range p.Tracks {
		steps = max(steps, len(t.Patterns[p.Pattern].Steps))
	}
	if bars := steps / 16; bars > 1 {
		info += fmt.Sprintf(" · %d bars", bars)
	}
	if p.Key != nil {
		info += " · " + p.Key.Name()
	}
	return info
}

// NewGroove builds the usual six-track layout (303, 303, Kick, Snare,
// ClosedHat, OpenHat) from a random style and reports the style's name.
func NewGroove(r *rand.Rand) (Project, string) {
	style := GrooveStyles[r.IntN(len(GrooveStyles))]
	return NewGrooveInStyle(style, r), style.Name
}

// RandomGroove is NewGroove plus a status line naming the style, tempo, key
// and any bank sounds the 303s started on.
func RandomGroove(r *rand.Rand) (Project, string) {
	style := GrooveStyles[r.IntN(len(GrooveStyles))]
	p, sounds := newGroove(style, r)
	return p, grooveStatus(style, p, sounds)
}

// grooveStatus names the style, tempo, key and the bank sounds tracks
// started on.
func grooveStatus(style GrooveStyle, p Project, sounds []string) string {
	info := GrooveInfo(style.Name, p)
	kit := []string{}
	for i, name := range sounds {
		switch {
		case name == "":
		case p.Tracks[i].Kind == Acid || p.Tracks[i].Kind == Dual:
			info += fmt.Sprintf(" · %s: %s", p.Tracks[i].Name, name)
		default:
			kit = append(kit, name)
		}
	}
	if len(kit) > 0 {
		info += " · kit: " + strings.Join(kit, ", ")
	}
	return info
}

func NewGrooveInStyle(style GrooveStyle, r *rand.Rand) Project {
	p, _ := newGroove(style, r)
	return p
}

// layoutPart is one track of a new groove's layout.
type layoutPart struct {
	kind    Kind
	playing bool
}

// trackOrder is the order a new groove's tracks appear in.
var trackOrder = []Kind{Acid, Dual, Kick, Snare, Clap, ClosedHat, OpenHat, Rimshot, Tom, Crash}

// chooseLayout draws a layout from the style, in trackOrder.
func (s GrooveStyle) chooseLayout(r *rand.Rand, steps int) []layoutPart {
	l := s.layout
	parts := map[Kind]*layoutPart{}
	acids := []*layoutPart{{Acid, true}}
	add := func(kind Kind, playing bool) {
		if kind == Acid {
			acids = append(acids, &layoutPart{Acid, playing})
			return
		}
		parts[kind] = &layoutPart{kind, playing}
	}
	add(Kick, true)
	add(ClosedHat, true)
	second, other := Acid, Dual
	if r.Float64() < l.dual {
		second, other = Dual, Acid
	}
	add(second, true)
	switch x := r.Float64() * (l.snare + l.clap + l.both); {
	case x < l.snare:
		add(Snare, true)
	case x < l.snare+l.clap:
		add(Clap, true)
	default:
		add(Snare, true)
		add(Clap, true)
	}
	// Two to four optional drums, weighted, without repeats. A crash hits
	// once per loop, so it plays only in loops of two bars or more.
	weights := map[Kind]float64{}
	for _, kind := range optionalKinds {
		if kind != Crash || steps >= 32 {
			weights[kind] = l.optional[kind]
		}
	}
	for n := []int{2, 2, 2, 3, 3, 4}[r.IntN(6)]; n > 0; n-- {
		total := 0.
		for _, kind := range optionalKinds {
			total += weights[kind]
		}
		if total <= 0 {
			break
		}
		x := r.Float64() * total
		for _, kind := range optionalKinds {
			if weights[kind] > 0 && x < weights[kind] {
				add(kind, true)
				weights[kind] = 0
				break
			}
			x -= weights[kind]
		}
	}
	// One or two silent tracks from what is left, so Jam has something to
	// bring in.
	// The spare synth counts half, so silent drums are more usual.
	pool := map[Kind]float64{}
	for _, kind := range []Kind{Snare, Clap, OpenHat, Rimshot, Tom, Crash, other} {
		if _, used := parts[kind]; !used && !(kind == Acid && len(acids) > 1) {
			pool[kind] = map[bool]float64{true: .5, false: 1}[kind.synthPart()]
		}
	}
	for n := []int{1, 1, 2}[r.IntN(3)]; n > 0; n-- {
		total := 0.
		for _, w := range pool {
			total += w
		}
		if total == 0 {
			break
		}
		x := r.Float64() * total
		for _, kind := range []Kind{Snare, Clap, OpenHat, Rimshot, Tom, Crash, other} {
			if w := pool[kind]; w > 0 && x < w {
				add(kind, false)
				delete(pool, kind)
				break
			} else {
				x -= w
			}
		}
	}
	layout := []layoutPart{}
	for _, kind := range trackOrder {
		if kind == Acid {
			for _, a := range acids {
				layout = append(layout, *a)
			}
			continue
		}
		if part, ok := parts[kind]; ok {
			layout = append(layout, *part)
		}
	}
	return layout
}

// recipeFor gives the style's recipe for a track: the lead for the first
// 303, the second part for later ones, a Dual part, or a drum's.
func (s GrooveStyle) recipeFor(kind Kind, acids int, r *rand.Rand) partRecipe {
	switch kind {
	case Acid:
		if acids == 0 {
			return s.lead
		}
		return s.second
	case Dual:
		return s.dual[r.IntN(len(s.dual))]
	}
	if x, ok := s.extra(kind); ok {
		return x.recipe
	}
	return s.drums[kind]
}

// newGroove builds a groove and returns, per track, the bank sound it
// started on ("" if none).
func newGroove(style GrooveStyle, r *rand.Rand) (Project, []string) {
	p := NewProject()
	p.Tracks = nil
	p.BPM = float64(int(style.bpm.pick(r)))
	p.Swing = float64(int(style.swing.pick(r)*100)) / 100
	key := RandomKey(r, style.scales)
	p.Key = &key
	steps := style.bars[r.IntN(len(style.bars))] * 16
	layout := style.chooseLayout(r, steps)
	sounds := make([]string, len(layout))
	acids, synths := 0, 0
	backbeats := 0
	for _, part := range layout {
		if part.kind == Snare || part.kind == Clap {
			backbeats++
		}
	}
	for i, part := range layout {
		// Synths take the first colours in order; drums keep one per family
		// (a tom with the kick, a crash with the open hat).
		colour := drumColours[part.kind]
		if part.kind.synthPart() {
			colour = synthColour(layout, synths)
			synths++
		}
		p.Tracks = append(p.Tracks, NewTrack(part.kind, colour))
		t := &p.Tracks[i]
		if part.kind == Acid {
			t.Name = fmt.Sprintf("303-%d", acids+1)
		}
		resetPatternSlot(t, 0, steps)
		recipe := style.recipeFor(part.kind, acids, r)
		if part.kind == Acid {
			acids++
		}
		if x, ok := style.extra(part.kind); ok {
			t.Level = x.level
			if part.kind == Clap && backbeats == 1 {
				t.Level = .7 // the only backbeat
			}
		}
		if recipe.level > 0 {
			t.Level = recipe.level
		}
		if !part.playing {
			recipe.actions = nil
		}
		jitterSound(t, r)
		sounds[i] = applyRecipe(&p, t, recipe, r, 0)
	}
	return p, sounds
}

var drumColours = map[Kind]int{Kick: 2, Snare: 3, ClosedHat: 4, OpenHat: 5, Clap: 6, Rimshot: 7, Tom: 2, Crash: 5}

// synthColour gives the nth synth of a layout its colour: the first two
// take 0 and 1, a third the first colour no drum in the layout uses.
func synthColour(layout []layoutPart, n int) int {
	if n < 2 {
		return n
	}
	used := map[int]bool{}
	for _, part := range layout {
		if !part.kind.synthPart() {
			used[drumColours[part.kind]] = true
		}
	}
	for _, c := range []int{6, 7, 3, 5, 4, 2} {
		if !used[c] {
			return c
		}
	}
	return n % 8
}

// playChance is how likely Randomise all is to give kind a part, given the
// tracks already in the project: optional drums by their layout weight, a
// clap or snare alongside the other by the style's backbeat weights.
func (s GrooveStyle) playChance(p Project, kind Kind) float64 {
	has := func(k Kind) bool {
		for _, t := range p.Tracks {
			if t.Kind == k {
				return true
			}
		}
		return false
	}
	l := s.layout
	total := max(l.snare+l.clap+l.both, 1e-9)
	switch kind {
	case Snare:
		if has(Clap) {
			return (l.snare + l.both) / total
		}
		return 1
	case Clap:
		if has(Snare) {
			return (l.clap + l.both) / total
		}
		return 1
	}
	if w, ok := l.optional[kind]; ok {
		return min(1, w)
	}
	return 1
}

// jitterSound nudges every continuous parameter a little around its default
// so even unstyled parts differ between grooves.
func jitterSound(t *Track, r *rand.Rand) { jitterSoundBy(t, r, .06) }

// jitterSoundBy moves every continuous lockable control by up to ±share of
// its range.
func jitterSoundBy(t *Track, r *rand.Rand, share float64) {
	for _, param := range Parameters(t.Kind) {
		if param.Increment >= 1 || !param.Lockable {
			continue
		}
		// Controls parked at an "off" end (drive 0, gate 1) stay off.
		if v := t.Parameters[param.ID]; v == param.Default && (v == param.Min || v == param.Max) {
			continue
		}
		width := (param.Max - param.Min) * share
		t.Parameters[param.ID] = min(param.Max, max(param.Min, t.Parameters[param.ID]+(r.Float64()*2-1)*width))
	}
}

// applyRecipe returns the bank sound the part started on, if any.
func applyRecipe(p *Project, t *Track, recipe partRecipe, r *rand.Rand, slot int) string {
	sound := applyRecipePatch(t, recipe, r)
	if sound == "" {
		applyRecipeSound(t, recipe, r)
	}
	writeRecipeNotes(p, t, recipe, r, slot)
	return sound
}

// applyRecipePatch sometimes gives the part a sound from the bank, slightly
// varied. It returns the sound's name, or "".
func applyRecipePatch(t *Track, recipe partRecipe, r *rand.Rand) string {
	if len(recipe.patches) == 0 || r.Float64() >= recipe.patch {
		return ""
	}
	preset, ok := FactorySoundNamed(t.Kind, recipe.patches[r.IntN(len(recipe.patches))])
	if !ok {
		return ""
	}
	tune := t.Parameters["tune"]
	if ApplySound(t, preset, true) != nil {
		return ""
	}
	jitterSound(t, r)
	// Pitched parts keep their tuning so they stay in key; a drum takes its
	// sound's tuning, which is part of its character.
	if t.Kind == Acid || t.Kind == Dual || t.Kind == Tom {
		t.Parameters["tune"] = tune
	}
	return preset.Name
}

func applyRecipeSound(t *Track, recipe partRecipe, r *rand.Rand) {
	for _, id := range sortedKeys(recipe.sound) {
		s := recipe.sound[id]
		value := s.pick(r)
		for _, param := range Parameters(t.Kind) {
			if param.ID == id && param.Increment >= 1 {
				value = float64(int(value + .5))
			}
		}
		t.Parameters[id] = value
	}
	if t.Kind == Acid && r.Float64() < recipe.square {
		t.Parameters["square"] = 1
	}
	if recipe.lfo > 0 && r.Float64() < recipe.lfo {
		lfo := DefaultLFOs()[0]
		lfo.Enabled, lfo.Target, lfo.Sync = true, 1, 1
		lfo.Division = []float64{1, 2, 4}[r.IntN(3)] // bars, up to the 4-bar limit
		lfo.Depth = .08 + r.Float64()*.14
		lfo.Waveform = r.IntN(2)
		t.LFOs[0] = lfo
	}
	fx := recipe.fx
	if len(recipe.boards) > 0 && r.Float64() < recipe.board {
		if b, ok := PedalboardNamed(recipe.boards[r.IntN(len(recipe.boards))]); ok {
			ApplyPedalboard(t, b, r)
			fx = nil
		}
	}
	for _, chance := range fx {
		if r.Float64() >= chance.chance {
			continue
		}
		e := NewEffect(chance.kind)
		for _, id := range sortedKeys(chance.params) {
			s := chance.params[id]
			e.Parameters[id] = s.pick(r)
		}
		t.Effects = append(t.Effects, e)
	}
}

func writeRecipeNotes(p *Project, t *Track, recipe partRecipe, r *rand.Rand, slot int) {
	if len(recipe.actions) == 0 {
		return
	}
	action := recipe.actions[r.IntN(len(recipe.actions))]
	if action == "" {
		return
	}
	if action == "Layer" {
		layerLead(p, t, slot)
		return
	}
	if t.Kind.Capabilities().Notes && t.Kind != Tom {
		generate(t, slot, action, p.Key, r)
		if recipe.octave != 0 {
			for i := range t.Patterns[slot].Steps {
				s := &t.Patterns[slot].Steps[i]
				s.Note = min(127, s.Note+recipe.octave)
			}
		}
		if recipe.hold {
			holdNotes(t.Patterns[slot].Steps)
		}
		return
	}
	DrumPattern(t.Patterns[slot].Steps, t.Kind, action, r)
	shapeDrumPart(t, slot, p.Key)
}

// layerLead doubles the first other synth that plays in slot an octave down
// (never below C1), so a Dual layer thickens the 303 lead. It reports whether
// there was a line to double.
func layerLead(p *Project, t *Track, slot int) bool {
	for _, lead := range p.Tracks {
		if lead.ID == t.ID || !lead.Kind.synthPart() || !hasNotes(lead.Patterns[slot]) {
			continue
		}
		steps := t.Patterns[slot].Steps
		for i := range steps {
			s := DefaultStep()
			if src := lead.Patterns[slot].Steps; len(src) > 0 {
				s = src[i%len(src)]
			}
			s.Note, s.Accent, s.Locks = max(24, s.Note-12), false, nil
			steps[i] = s
		}
		return true
	}
	return false
}

// holdNotes lets each note ring until the next one, up to a bar.
func holdNotes(steps []Step) {
	for i := range steps {
		if !steps[i].Active {
			continue
		}
		gap := 1
		for gap < 16 && i+gap < len(steps) && !steps[i+gap].Active {
			gap++
		}
		steps[i].Length = gap
	}
}

// shapeDrumPart finishes a drum family pattern: a crash hits once per loop,
// on its first hit, and toms fall through the key within each bar so a call
// sounds like a run down the kit.
func shapeDrumPart(t *Track, slot int, key *Key) {
	steps := t.Patterns[slot].Steps
	switch t.Kind {
	case Crash:
		first := true
		for i := range steps {
			steps[i].Active = steps[i].Active && first
			first = first && !steps[i].Active
		}
	case Tom:
		if key == nil {
			return
		}
		notes := []int{}
		for _, degree := range scaleNotes(key.Scale) {
			notes = append(notes, foldNote(36+key.Root+degree, 36, 48))
		}
		slices.Sort(notes)
		for bar := 0; bar*16 < len(steps); bar++ {
			hits := []int{}
			for i := bar * 16; i < min(len(steps), bar*16+16); i++ {
				if steps[i].Active {
					hits = append(hits, i)
				}
			}
			for k, i := range hits {
				n := len(notes) - 1
				if len(hits) > 1 {
					n = (len(hits) - 1 - k) * (len(notes) - 1) / (len(hits) - 1)
				}
				steps[i].Note = notes[n]
			}
		}
	}
}

// RandomiseGroove rebuilds the selected pattern as a fresh groove, as if the
// app had just started: a new style sets tempo, swing, key and length (one,
// two or four bars), and every
// track gets a new sound, effects and notes in that pattern. Other patterns,
// the song, track layout and mixer stay. Tracks the style has no part for
// get a random sound and a suitable pattern; samplers keep their sample and
// sound and get a new pattern. It returns a status line.
func RandomiseGroove(p *Project, r *rand.Rand) string {
	return RandomiseGrooveParts(p, r, AllParts)
}

// GroovePart picks which tracks a randomise may change.
type GroovePart int

const (
	AllParts   GroovePart = iota
	DrumParts             // drums and samplers
	SynthParts            // 303s and dual synths
)

func (k Kind) synthPart() bool { return k == Acid || k == Dual }

// Rerolled reports whether a randomise of part changes t. Held tracks are
// never changed.
func (part GroovePart) Rerolled(t Track) bool {
	if t.Hold {
		return false
	}
	switch part {
	case DrumParts:
		return !t.Kind.synthPart()
	case SynthParts:
		return t.Kind.synthPart()
	}
	return true
}

// RandomiseGrooveParts rebuilds the selected pattern of the tracks part
// rerolls, in a random style. The other tracks keep their pattern and sound;
// when any are kept, so are the tempo and swing, and if a kept track plays
// notes the key stays too, so the new parts fit around them.
func RandomiseGrooveParts(p *Project, r *rand.Rand, part GroovePart) string {
	style := GrooveStyles[r.IntN(len(GrooveStyles))]
	bpm := float64(int(style.bpm.pick(r)))
	swing := float64(int(style.swing.pick(r)*100)) / 100
	key := RandomKey(r, style.scales)
	slot := p.Pattern
	kept, keptNotes, held := false, false, []string{}
	for _, t := range p.Tracks {
		if part.Rerolled(t) {
			continue
		}
		kept = true
		if t.Hold {
			held = append(held, t.Name)
		}
		if t.Kind.synthPart() {
			for _, s := range t.Patterns[slot].Steps {
				keptNotes = keptNotes || s.Active
			}
		}
	}
	if !kept {
		p.BPM, p.Swing = bpm, swing
	}
	if !keptNotes || p.Key == nil {
		p.Key = &key
	}
	sounds := make([]string, len(p.Tracks))
	steps := style.bars[r.IntN(len(style.bars))] * 16
	acid := 0
	layers := []int{} // Dual layers, redone once every lead is written
	for i := range p.Tracks {
		t := &p.Tracks[i]
		if !part.Rerolled(*t) {
			if t.Kind == Acid {
				acid++
			}
			continue
		}
		resetPatternSlot(t, slot, steps)
		if t.Kind == Sampler {
			generate(t, slot, []string{"4 Floor", "8ths", "Offbeat", "Sparse"}[r.IntN(4)], p.Key, r)
			continue
		}
		freshSound(t)
		recipe, ok := style.drums[t.Kind]
		if x, extra := style.extra(t.Kind); extra {
			recipe, ok = x.recipe, true
		}
		if t.Kind == Dual {
			recipe, ok = style.dual[r.IntN(len(style.dual))], true
		}
		if ok && !t.Kind.synthPart() && (r.Float64() >= style.playChance(*p, t.Kind) || t.Kind == Crash && steps < 32) {
			recipe.actions = nil
		}
		if t.Kind == Acid {
			recipe, ok = style.second, true
			if acid == 0 {
				recipe = style.lead
			}
			acid++
		}
		if ok {
			jitterSound(t, r)
			sounds[i] = applyRecipe(p, t, recipe, r, slot)
			if len(recipe.actions) == 1 && recipe.actions[0] == "Layer" {
				layers = append(layers, i)
			}
			continue
		}
		// No part in the style: a bank-based sound and a fitting pattern.
		sounds[i] = RandomiseSoundFrom(t, r)
		action := map[Kind]string{Dual: "Bassline", Tom: "Toms", Clap: "2 & 4", Rimshot: "Random", Crash: "Downbeat"}[t.Kind]
		if action == "" {
			action = "Random"
		}
		generate(t, slot, action, p.Key, r)
	}
	for _, i := range layers {
		resetPatternSlot(&p.Tracks[i], slot, steps)
		layerLead(p, &p.Tracks[i], slot)
	}
	info := grooveStatus(style, *p, sounds)
	if len(held) > 0 {
		info += " · held: " + strings.Join(held, ", ")
	}
	return info
}

// RerolledTracks counts the tracks a randomise of part would change.
func RerolledTracks(p Project, part GroovePart) int {
	n := 0
	for _, t := range p.Tracks {
		if part.Rerolled(t) {
			n++
		}
	}
	return n
}

// resetPatternSlot makes one pattern of a track an empty sixteenth-note
// pattern of the given length and forgets any sound stored with it.
func resetPatternSlot(t *Track, slot, steps int) {
	pattern := &t.Patterns[slot]
	pattern.Steps = make([]Step, steps)
	for i := range pattern.Steps {
		pattern.Steps[i] = DefaultStep()
	}
	pattern.Resolution = 16
	pattern.State = nil
}

// freshSound returns a track to its default sound, LFOs and an empty chain,
// as a new track starts.
func freshSound(t *Track) {
	for len(t.Effects) > 0 {
		RemoveEffect(t, t.Effects[0].ID)
	}
	t.EffectsBypassed = false
	t.LFOs = DefaultLFOs()
	t.Parameters = map[string]float64{}
	for _, param := range Parameters(t.Kind) {
		t.Parameters[param.ID] = param.Default
	}
}

// sortedKeys gives spans in a fixed order so seeded generation repeats
// (map iteration order is random).
func sortedKeys(m map[string]span) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
