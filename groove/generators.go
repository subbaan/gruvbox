package groove

import (
	"fmt"
	"math"
	"math/rand/v2"
)

type EuclideanOptions struct {
	Steps, Pulses, Rotation int
	Velocity                float64
	Layer                   bool
}

// EuclideanPattern changes the rhythm mask while retaining note settings and
// locks. Integer spacing avoids rounding errors for arbitrary 1–64 step cycles.
func EuclideanPattern(source Pattern, options EuclideanOptions) (Pattern, error) {
	if options.Steps < 1 || options.Steps > 64 || options.Pulses < 0 || options.Pulses > options.Steps || options.Rotation < 0 || options.Rotation >= options.Steps || !inRange(options.Velocity, 0, 1) || len(source.Steps) == 0 {
		return Pattern{}, fmt.Errorf("choose 1–64 steps, a pulse count within the cycle and a valid rotation/velocity")
	}
	result := clonePattern(source)
	for len(result.Steps) < options.Steps {
		result.Steps = append(result.Steps, DefaultStep())
	}
	result.Steps = result.Steps[:options.Steps]
	var hits [64]bool
	for i := 0; i < options.Steps; i++ {
		if (i*options.Pulses)%options.Steps < options.Pulses {
			hits[(i+options.Rotation)%options.Steps] = true
		}
	}
	for i := range result.Steps {
		step := &result.Steps[i]
		if hits[i] && !step.Active {
			step.Velocity = options.Velocity
		}
		step.Active = hits[i] || (options.Layer && step.Active)
	}
	return result, nil
}

type LockShape string

const (
	LockRandom        LockShape = "Random chaos"
	LockHumaniseMild  LockShape = "Humanise (mild)"
	LockHumaniseHeavy LockShape = "Humanise (heavy)"
	LockHumaniseWild  LockShape = "Humanise (wild)"
	LockLadder        LockShape = "Step ladder"
	LockSine          LockShape = "Smooth wave"
	LockUp            LockShape = "Sweep up"
	LockDown          LockShape = "Sweep down"
	LockPulse         LockShape = "Filter pulse"
	LockAccent        LockShape = "Accent dance"
	LockDecay         LockShape = "Decay fade"
	LockBuild         LockShape = "Build up"
	LockBreak         LockShape = "Break down"
	LockShuffle       LockShape = "Hi-hat shuffle"
	LockPunch         LockShape = "Kick punch"
	LockClear         LockShape = "Clear locks"
)

var LockShapes = []LockShape{LockRandom, LockHumaniseMild, LockHumaniseHeavy, LockHumaniseWild, LockLadder, LockSine, LockUp, LockDown, LockPulse, LockAccent, LockDecay, LockBuild, LockBreak, LockShuffle, LockPunch, LockClear}

func (shape LockShape) Random() bool {
	return shape == LockRandom || shape == LockHumaniseMild || shape == LockHumaniseHeavy || shape == LockHumaniseWild
}
func (shape LockShape) Description() string {
	switch shape {
	case LockRandom:
		return "Random values between Low and High. Reroll explores another take."
	case LockHumaniseMild:
		return "Vary existing locks or the current knob by ±5% of the chosen range."
	case LockHumaniseHeavy:
		return "Vary existing locks or the current knob by ±20% of the chosen range."
	case LockHumaniseWild:
		return "Vary existing locks or the current knob by ±35% of the chosen range."
	case LockLadder:
		return "An eight-step staircase: low, quarter, half, three-quarter, high, then down."
	case LockSine:
		return "One sine-wave cycle over the step range."
	case LockUp:
		return "Rise from Low to High over the step range."
	case LockDown:
		return "Fall from High to Low over the step range."
	case LockPulse:
		return "Alternate high and low values on successive steps."
	case LockAccent:
		return "Emphasise every fourth step using the selected parameter."
	case LockDecay:
		return "Fall from High to 20% of the chosen range."
	case LockBuild:
		return "Rise from 20% of the chosen range to High."
	case LockBreak:
		return "A curved fall from High to Low."
	case LockShuffle:
		return "Alternate longer and shorter values within the chosen range."
	case LockPunch:
		return "Strongest every sixteenth step, medium every fourth, softer between."
	case LockClear:
		return "Remove only this parameter’s locks from eligible steps in the selected range."
	}
	return ""
}

type LockOptions struct {
	Parameter   string
	Shape       LockShape
	First, Last int     // inclusive, zero based
	Low, High   float64 // native parameter units
	ActiveOnly  bool
	Seed        uint64
}

// LockPattern is a pure, seeded draft. Values use the actual instrument/effect
// range and knob increments, rather than the legacy assumption of 0–1 controls.
func LockPattern(track Track, slot int, options LockOptions) (Pattern, error) {
	if slot < 0 || slot >= len(track.Patterns) {
		return Pattern{}, fmt.Errorf("invalid pattern slot")
	}
	source := track.Patterns[slot]
	if options.First < 0 || options.Last < options.First || options.Last >= len(source.Steps) {
		return Pattern{}, fmt.Errorf("choose a step range inside the pattern")
	}
	var parameter Parameter
	found := false
	for _, p := range LockParameters(track) {
		if p.ID == options.Parameter && p.Lockable {
			parameter, found = p, true
			break
		}
	}
	if !found {
		return Pattern{}, fmt.Errorf("choose a lockable instrument or effect parameter")
	}
	if options.Shape.Description() == "" {
		return Pattern{}, fmt.Errorf("unknown lock generator")
	}
	if options.Shape != LockClear && (!inRange(options.Low, parameter.Min, parameter.Max) || !inRange(options.High, parameter.Min, parameter.Max) || options.Low > options.High) {
		return Pattern{}, fmt.Errorf("Low and High must lie within the parameter’s range")
	}
	result := clonePattern(source)
	rng := rand.New(rand.NewPCG(options.Seed, options.Seed^0x9e3779b97f4a7c15))
	length := options.Last - options.First + 1
	for i := options.First; i <= options.Last; i++ {
		step := &result.Steps[i]
		// Advance randomness on every step, so toggling Active only retains the
		// same values on notes shared by both previews.
		random := rng.Float64()
		if options.ActiveOnly && !step.Active {
			continue
		}
		if options.Shape == LockClear {
			delete(step.Locks, options.Parameter)
			continue
		}
		index := i - options.First
		progress := float64(index) / float64(max(1, length-1))
		value := 0.0
		switch options.Shape {
		case LockRandom:
			value = random
		case LockLadder:
			value = [...]float64{0, .25, .5, .75, 1, .75, .5, .25}[index%8]
		case LockSine:
			value = (math.Sin(2*math.Pi*float64(index)/float64(length)) + 1) * .5
		case LockUp:
			value = progress
		case LockDown:
			value = 1 - progress
		case LockPulse:
			if index%2 == 0 {
				value = .8
			} else {
				value = .2
			}
		case LockAccent:
			if index%4 == 0 {
				value = .9
			} else {
				value = .5
			}
		case LockDecay:
			value = 1 - progress*.8
		case LockBuild:
			value = .2 + progress*.8
		case LockBreak:
			value = (1 - progress) * (1 - progress)
		case LockShuffle:
			if index%2 == 0 {
				value = .7
			} else {
				value = .3
			}
		case LockPunch:
			if index%16 == 0 {
				value = 1
			} else if index%4 == 0 {
				value = .75
			} else {
				value = .5
			}
		}
		value = options.Low + value*(options.High-options.Low)
		if options.Shape == LockHumaniseMild || options.Shape == LockHumaniseHeavy || options.Shape == LockHumaniseWild {
			base, exists := step.Locks[options.Parameter]
			if !exists {
				base = track.ParameterValue(options.Parameter)
			}
			amount := .05
			if options.Shape == LockHumaniseHeavy {
				amount = .2
			} else if options.Shape == LockHumaniseWild {
				amount = .35
			}
			value = base + (random*2-1)*amount*(options.High-options.Low)
		}
		if parameter.Increment > 0 {
			value = parameter.Min + math.Round((value-parameter.Min)/parameter.Increment)*parameter.Increment
		}
		value = max(options.Low, min(options.High, value))
		if step.Locks == nil {
			step.Locks = map[string]float64{}
		}
		step.Locks[options.Parameter] = value
	}
	return result, nil
}
