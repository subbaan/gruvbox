package groove

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
)

// Styles share the phrase pipeline; a new recipe only needs rhythm priorities,
// motif size and articulation weights, without changing the desktop controls.
type PhraseStyle string

const (
	PhraseHypnotic PhraseStyle = "Hypnotic"
	PhraseAcid     PhraseStyle = "Acid"
	PhraseRolling  PhraseStyle = "Rolling"
)

type phraseRecipe struct {
	style                  PhraseStyle
	rhythm                 [16]int
	motif                  int
	pitchVariation, slides float64
}

var phraseRecipes = []phraseRecipe{
	{PhraseHypnotic, [16]int{0, 6, 10, 3, 14, 8, 12, 2, 7, 11, 15, 4, 5, 9, 13, 1}, 3, .3, .2},
	{PhraseAcid, [16]int{0, 3, 6, 10, 14, 7, 12, 2, 9, 15, 4, 11, 5, 13, 8, 1}, 4, .7, .65},
	{PhraseRolling, [16]int{0, 2, 6, 8, 10, 14, 3, 7, 11, 15, 4, 12, 5, 9, 13, 1}, 4, .5, .35},
}

func PhraseStyles() []string {
	styles := make([]string, len(phraseRecipes))
	for i, recipe := range phraseRecipes {
		styles[i] = string(recipe.style)
	}
	return styles
}

type PhraseScale string

const (
	PhraseMinorPentatonic PhraseScale = "Minor pentatonic"
	PhraseMinor           PhraseScale = "Natural minor"
	PhraseDorian          PhraseScale = "Dorian"
	PhrasePhrygian        PhraseScale = "Phrygian"
	PhraseMajorPentatonic PhraseScale = "Major pentatonic"
	PhraseChromatic       PhraseScale = "Chromatic"
)

var phraseScales = []struct {
	name  PhraseScale
	notes []int
}{
	{PhraseMinorPentatonic, []int{0, 3, 5, 7, 10, 12}},
	{PhraseMinor, []int{0, 2, 3, 5, 7, 8, 10, 12}},
	{PhraseDorian, []int{0, 2, 3, 5, 7, 9, 10, 12}},
	{PhrasePhrygian, []int{0, 1, 3, 5, 7, 8, 10, 12}},
	{PhraseMajorPentatonic, []int{0, 2, 4, 7, 9, 12}},
	{PhraseChromatic, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}},
}

func PhraseScales() []string {
	scales := make([]string, len(phraseScales))
	for i, scale := range phraseScales {
		scales[i] = string(scale.name)
	}
	return scales
}

type PhraseMode string

const (
	PhraseAll     PhraseMode = "Whole phrase"
	PhrasePitches PhraseMode = "Pitches only"
	PhraseFeel    PhraseMode = "Feel only"
)

type PhraseOptions struct {
	Bars, Root                              int // 4/4, sixteenth notes; Root is MIDI.
	Style                                   PhraseStyle
	Scale                                   PhraseScale
	Mode                                    PhraseMode
	Density, Movement, Variation, Feel      float64
	RhythmSeed, PitchSeed, ArticulationSeed uint64
}

func phraseRandom(seed, stream uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^stream))
}

// PhrasePattern is a pure draft with independent random streams for rhythm,
// pitch and feel. Bar one establishes the motif; later bars develop its second
// half. Variation zero repeats it exactly. Stored locks remain at their positions.
func PhrasePattern(source Pattern, kind Kind, options PhraseOptions) (Pattern, error) {
	if !kind.Capabilities().Notes {
		return Pattern{}, fmt.Errorf("choose a pitched instrument: 303, Dual synth, Tom or Sampler")
	}
	if options.Bars != 1 && options.Bars != 2 && options.Bars != 4 {
		return Pattern{}, fmt.Errorf("choose 1, 2 or 4 bars")
	}
	if len(source.Steps) < 1 || len(source.Steps) > 64 || options.Root < 0 || options.Root > 115 || !inRange(options.Density, .125, 1) || !inRange(options.Movement, 0, 1) || !inRange(options.Variation, 0, 1) || !inRange(options.Feel, 0, 1) {
		return Pattern{}, fmt.Errorf("invalid phrase settings")
	}
	if options.Mode != PhraseAll && options.Mode != PhrasePitches && options.Mode != PhraseFeel {
		return Pattern{}, fmt.Errorf("choose whole phrase, pitches only or feel only")
	}
	var recipe *phraseRecipe
	for i := range phraseRecipes {
		if phraseRecipes[i].style == options.Style {
			recipe = &phraseRecipes[i]
		}
	}
	var scale []int
	for _, s := range phraseScales {
		if s.name == options.Scale {
			scale = s.notes
		}
	}
	if recipe == nil || scale == nil {
		return Pattern{}, fmt.Errorf("unknown phrase style or scale")
	}
	result := clonePattern(source)
	count := options.Bars * 16
	for len(result.Steps) < count {
		step := DefaultStep()
		if options.Mode != PhraseAll {
			step = source.Steps[len(result.Steps)%len(source.Steps)]
			step.Locks = nil
		}
		result.Steps = append(result.Steps, step)
	}
	result.Steps, result.Resolution = result.Steps[:count], 16
	if options.Mode == PhraseAll {
		phraseRhythm(&result, *recipe, options)
	}
	if options.Mode != PhraseFeel {
		phrasePitches(&result, *recipe, scale, options)
	}
	if options.Mode != PhrasePitches {
		phraseArticulation(&result, kind, *recipe, options)
	}
	return result, nil
}

func phraseRhythm(pattern *Pattern, recipe phraseRecipe, options PhraseOptions) {
	rng := phraseRandom(options.RhythmSeed, 0xd1b54a32d192ed03)
	var weights [16]float64
	for rank, index := range recipe.rhythm {
		weights[index] = float64(rank) + rng.Float64()*5
	}
	weights[0] = -1 // A stable downbeat is the motif's rhythmic anchor.
	indices := make([]int, 16)
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool { return weights[indices[i]] < weights[indices[j]] })
	var motif [16]bool
	for _, index := range indices[:int(math.Round(options.Density*16))] {
		motif[index] = true
	}
	for bar := 0; bar < options.Bars; bar++ {
		hits := motif
		// Change at most two hits in the second half, retaining density and the
		// first half. In four bars, bar two mostly repeats; bar three develops.
		strength := phraseDevelopment(bar, options.Bars) * options.Variation
		for attempt := 0; attempt < 2; attempt++ {
			roll := rng.Float64()
			from, to := 8+rng.IntN(8), 8+rng.IntN(8)
			if roll < strength && hits[from] && !hits[to] {
				hits[from], hits[to] = false, true
			}
		}
		for i, active := range hits {
			step := &pattern.Steps[bar*16+i]
			locks := step.Locks
			*step = DefaultStep()
			step.Locks, step.Active = locks, active
		}
	}
}

func phraseDevelopment(bar, bars int) float64 {
	if bar == 0 {
		return 0
	}
	if bars == 4 && bar == 1 {
		return .2
	}
	if bars == 4 && bar == 2 {
		return .65
	}
	return 1
}

func phrasePitches(pattern *Pattern, recipe phraseRecipe, scale []int, options PhraseOptions) {
	rng := phraseRandom(options.PitchSeed, 0x94d049bb133111eb)
	motif := make([]int, recipe.motif)
	for i := 1; i < len(motif); i++ {
		roll := rng.Float64()
		delta := 0
		if roll < options.Movement {
			delta = 1 + rng.IntN(2)
			if rng.IntN(2) == 0 {
				delta = -delta
			}
		}
		motif[i] = min(len(scale)-1, phraseAbs(motif[i-1]+delta))
		// Root/fifth/octave jumps become possible as movement increases.
		if rng.Float64() < options.Movement*.25 {
			motif[i] = []int{0, len(scale) - 1, nearestDegree(scale, 7)}[rng.IntN(3)]
		}
	}
	var base [16]int
	for i := range base {
		base[i] = motif[(i/2)%len(motif)]
	}
	for bar := 0; bar < options.Bars; bar++ {
		strength := phraseDevelopment(bar, options.Bars) * options.Variation * options.Movement * recipe.pitchVariation
		for i, degree := range base {
			roll, delta := rng.Float64(), 1
			if rng.IntN(2) == 0 {
				delta = -1
			}
			if i >= 8 && roll < strength {
				degree = max(0, min(len(scale)-1, degree+delta))
			}
			pattern.Steps[bar*16+i].Note = options.Root + scale[degree]
		}
	}
	// Resolve the final response back to the root. With zero variation all
	// bars remain identical, including the ending.
	if options.Bars > 1 && options.Variation > 0 {
		for i := len(pattern.Steps) - 1; i >= len(pattern.Steps)-8; i-- {
			if pattern.Steps[i].Active {
				pattern.Steps[i].Note = options.Root
				break
			}
		}
	}
}

func nearestDegree(scale []int, semitone int) int {
	best := 0
	for i, note := range scale {
		if phraseAbs(note-semitone) < phraseAbs(scale[best]-semitone) {
			best = i
		}
	}
	return best
}

func phraseAbs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func phraseArticulation(pattern *Pattern, kind Kind, recipe phraseRecipe, options PhraseOptions) {
	rng := phraseRandom(options.ArticulationSeed, 0x9e3779b97f4a7c15)
	var accents, slides, gates, velocities [16]float64
	var lengths [16]int
	for i := range accents {
		accents[i], slides[i], gates[i], velocities[i] = rng.Float64(), rng.Float64(), rng.Float64(), rng.Float64()
		lengths[i] = 2 + rng.IntN(3)
	}
	// First set gates and accents; then connect eligible incoming slides to
	// their preceding note, including the loop boundary.
	for i := range pattern.Steps {
		step := &pattern.Steps[i]
		if !step.Active {
			step.Accent, step.Slide, step.Length = false, false, 0
			continue
		}
		j := i % 16
		a, g, v := accents[j], gates[j], velocities[j]
		length := lengths[j]
		if j >= 8 && rng.Float64() < options.Variation*phraseDevelopment(i/16, options.Bars) {
			a, g, v = rng.Float64(), rng.Float64(), rng.Float64()
			length = 2 + rng.IntN(3)
		}
		focal := j%4 == 0 || j == recipe.rhythm[1] || j == recipe.rhythm[2]
		threshold := options.Feel * .25
		if focal {
			threshold = options.Feel * .75
		}
		step.Accent, step.Slide, step.Length = a < threshold, false, 0
		step.Velocity = .8 + (v-.5)*.24*options.Feel
		if focal {
			step.Velocity = min(1, step.Velocity+.12*options.Feel)
		}
		if kind.Capabilities().NoteLengths && g < options.Feel*.6 {
			gap := phraseNextGap(*pattern, i)
			step.Length = min(gap, length)
		}
	}
	for i := range pattern.Steps {
		step := &pattern.Steps[i]
		if !step.Active || !kind.Capabilities().Slide {
			continue
		}
		for gap := 1; gap <= 2; gap++ {
			previous := &pattern.Steps[(i-gap+len(pattern.Steps))%len(pattern.Steps)]
			if !previous.Active {
				continue
			}
			step.Slide = previous.Note != step.Note && previous.Probability == 1 && previous.Every <= 1 && slides[i%16] < options.Feel*recipe.slides
			if step.Slide {
				previous.Length = max(previous.GateSteps(), gap)
			}
			break
		}
	}
}

func phraseNextGap(pattern Pattern, index int) int {
	for gap := 1; gap < len(pattern.Steps); gap++ {
		if pattern.Steps[(index+gap)%len(pattern.Steps)].Active {
			return gap
		}
	}
	return len(pattern.Steps)
}
