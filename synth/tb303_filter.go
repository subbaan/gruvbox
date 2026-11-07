package synth

import (
	"math"

	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/util"
)

// TB303Filter wraps the diode ladder filter with TB-303 specific behavior
type TB303Filter struct {
	filter *dsp.DiodeLadderFilter

	// TB-303 filter parameters
	cutoff     float32 // 0.0 to 1.0
	resonance  float32 // 0.0 to 1.0
	envMod     float32 // Envelope modulation amount (0.0 to 1.0)
	envDecay   float32 // Envelope decay time in seconds

	// Filter envelope (decay-only, like TB-303)
	filterEnv *DecayEnvelope

	// LFO modulation
	cutoffMod    float32 // LFO modulated cutoff (if set)
	resonanceMod float32 // LFO modulated resonance (if set)
	useLFOMod    bool    // Whether to use LFO modulation

	// Internal state
	sampleRate     float32
	currentCutoff  float32 // Actual cutoff with modulation applied
	accentAmount   float32 // Current accent level

	// Accent sweep: accents charge a "capacitor" whose smoothed output adds
	// to the cutoff, so a run of accents climbs higher each time.
	sweep, charge, wow, wowCoeff float32
	accentEnv, accentCoeff       float32 // slow per-accent pulse the charge rides on
}

// NewTB303Filter creates a new TB-303 filter
func NewTB303Filter(sampleRate float32) *TB303Filter {
	return &TB303Filter{
		filter:     dsp.NewDiodeLadderFilter(sampleRate),
		cutoff:     0.5,
		resonance:  0.4,
		envMod:     0.75,
		envDecay:   0.2,
		filterEnv:  NewDecayEnvelope(sampleRate),
		sampleRate: sampleRate,
		currentCutoff: 0.5,
		accentAmount: 0.0,
		wowCoeff:     1 - float32(math.Exp(-1/(0.03*float64(sampleRate)))), // ~30 ms smear
		accentCoeff:  float32(math.Exp(-1 / (0.35 * float64(sampleRate)))),
	}
}

// SetMode picks low-pass (0), band-pass (1) or high-pass (2).
func (f *TB303Filter) SetMode(mode int) { f.filter.Mode = max(0, min(2, mode)) }

// SetAccentSweep sets how strongly consecutive accents build the cutoff up
// (0 = off).
func (f *TB303Filter) SetAccentSweep(amount float32) {
	amount = util.Clamp(amount, 0, 1)
	if amount == 0 && f.sweep > 0 {
		f.filterEnv.SetDecayTime(f.envDecay) // undo any short accent decay
	}
	f.sweep = amount
}

// SetCutoff sets the base cutoff frequency (0.0 to 1.0)
func (f *TB303Filter) SetCutoff(cutoff float32) {
	f.cutoff = util.Clamp(cutoff, 0.0, 1.0)
}

// SetResonance sets the filter resonance (0.0 to 1.0)
func (f *TB303Filter) SetResonance(resonance float32) {
	f.resonance = util.Clamp(resonance, 0.0, 0.95) // TB-303 max
	f.filter.SetResonance(f.resonance)
}

// SetEnvMod sets the envelope modulation amount (0.0 to 1.0)
func (f *TB303Filter) SetEnvMod(envMod float32) {
	f.envMod = util.Clamp(envMod, 0.0, 1.0)
}

// SetEnvModModulation sets a temporary envelope modulation value (for LFO)
func (f *TB303Filter) SetEnvModModulation(envMod float32) {
	f.envMod = util.Clamp(envMod, 0.0, 1.0)
}

// SetDecay sets the filter envelope decay time in seconds
func (f *TB303Filter) SetDecay(decay float32) {
	f.envDecay = util.Clamp(decay, 0.05, 2.0)
	f.filterEnv.SetDecayTime(f.envDecay)
}

// Trigger triggers the filter envelope (called on note on)
func (f *TB303Filter) Trigger(accent bool) {
	f.filterEnv.Trigger()

	// Accent affects envelope depth and cutoff
	if accent {
		f.accentAmount = 0.5 // 50% boost
		f.charge = min(1, f.charge*.7+.4)
		f.accentEnv = 1
	} else {
		f.accentAmount = 0.0
		f.charge *= .5 // a plain note drains half the build-up
		f.accentEnv = 0
	}
	if f.sweep > 0 {
		// With sweep on, accented notes use a short fixed decay, as on
		// the original circuit; plain notes follow the Decay knob.
		decay := f.envDecay
		if accent {
			decay += (min(decay, .2) - decay) * f.sweep
		}
		f.filterEnv.SetDecayTime(decay)
	}
}

// Reset resets the filter state
func (f *TB303Filter) Reset() {
	f.filter.Reset()
	f.filterEnv.Reset()
	f.accentAmount = 0.0
	f.charge, f.wow, f.accentEnv = 0, 0, 0
}

// Process processes one sample through the TB-303 filter
func (f *TB303Filter) Process(input float32) float32 {
	// Update filter envelope
	envValue := f.filterEnv.Process()

	// Apply accent to envelope
	if f.accentAmount > 0.0 {
		envValue *= (1.0 + f.accentAmount)
		if envValue > 1.0 {
			envValue = 1.0
		}
	}

	// Determine base cutoff (from LFO or normal)
	baseCutoff := f.cutoff
	if f.useLFOMod {
		baseCutoff = f.cutoffMod
	}

	// Calculate modulated cutoff
	// TB-303 characteristic: exponential modulation
	modulation := envValue * f.envMod

	// Exponential scaling (more dramatic at higher modulation)
	// cutoff goes from base to base + modulation
	f.currentCutoff = baseCutoff + modulation * (1.0 - baseCutoff)

	if f.sweep > 0 {
		f.accentEnv *= f.accentCoeff
		f.wow += (f.charge*f.accentEnv - f.wow) * f.wowCoeff
		f.currentCutoff += f.wow * f.sweep * (1 - f.currentCutoff)
	}

	// Clamp to valid range
	f.currentCutoff = util.Clamp(f.currentCutoff, 0.0, 1.0)

	// Update filter cutoff
	f.filter.SetCutoff(f.currentCutoff)

	// Update resonance (from LFO or normal)
	if f.useLFOMod {
		f.filter.SetResonance(util.Clamp(f.resonanceMod, 0.0, 0.95))
	} else {
		f.filter.SetResonance(f.resonance)
	}

	// Process through filter
	return f.filter.Process(input)
}

// ProcessBuffer processes a buffer of samples
func (f *TB303Filter) ProcessBuffer(input, output []float32) {
	for i := range input {
		output[i] = f.Process(input[i])
	}
}

// GetCurrentCutoff returns the current cutoff with modulation applied
func (f *TB303Filter) GetCurrentCutoff() float32 {
	return f.currentCutoff
}

// GetEnvelopeValue returns the current filter envelope value
func (f *TB303Filter) GetEnvelopeValue() float32 {
	return f.filterEnv.GetValue()
}

// IsEnvelopeActive returns true if the filter envelope is active
func (f *TB303Filter) IsEnvelopeActive() bool {
	return f.filterEnv.IsActive()
}

// GetDecay returns the envelope decay time
func (f *TB303Filter) GetDecay() float32 {
	return f.envDecay
}

// SetCutoffModulation sets the LFO modulated cutoff value
func (f *TB303Filter) SetCutoffModulation(cutoff float32) {
	f.cutoffMod = cutoff
	f.useLFOMod = true
}

// SetResonanceModulation sets the LFO modulated resonance value
func (f *TB303Filter) SetResonanceModulation(resonance float32) {
	f.resonanceMod = resonance
	f.useLFOMod = true
}

// ClearLFOModulation disables LFO modulation
func (f *TB303Filter) ClearLFOModulation() {
	f.useLFOMod = false
}
