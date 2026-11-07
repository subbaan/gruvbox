package drums

import (
	"math"

	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/util"
)

// RimshotLFOTarget represents what parameter the LFO modulates
type RimshotLFOTarget int

const (
	RimshotLFOTargetNone RimshotLFOTarget = iota
	RimshotLFOTargetPitch // Modulate oscillator pitch (200-800 Hz)
	RimshotLFOTargetDecay // Modulate decay time (0.01-0.15s)
	RimshotLFOTargetTone  // Modulate tone/noise balance (0.0-1.0)
	RimshotLFOTargetSnap  // Modulate attack sharpness (0.0-1.0)
)

// RimshotDrum is an analog-modeled rimshot synthesizer (909-style)
// Combines pitched oscillator with short noise burst for classic "tick" sound
type RimshotDrum struct {
	// Oscillator for tonal component
	phase       float32
	phaseInc    float32

	// Noise generator for click
	noiseGen *dsp.NoiseGenerator
	filter   *dsp.BiquadFilter

	// Envelopes
	toneEnv  *synth.DecayEnvelope // For pitched component
	noiseEnv *synth.DecayEnvelope // For noise component

	// Parameters
	pitch float32 // Base pitch (200-800 Hz typical)
	decay float32 // Decay time (0.01 to 0.15 seconds)
	tone  float32 // Balance between tone and noise (0.0 = all noise, 1.0 = all tone)
	snap  float32 // Sharpness of attack (0.0 to 1.0)

	// LFO modulation
	lfo1        *dsp.LFO
	lfo2        *dsp.LFO
	lfo1Target  RimshotLFOTarget
	lfo1Enabled bool
	lfo2Target  RimshotLFOTarget
	lfo2Enabled bool

	// Internal
	sampleRate float32
	active     bool
}

// NewRimshotDrum creates a new rimshot drum synthesizer
func NewRimshotDrum(sampleRate float32) *RimshotDrum {
	rim := &RimshotDrum{
		noiseGen:   dsp.NewNoiseGenerator(),
		filter:     dsp.NewBiquadFilter(sampleRate),
		toneEnv:    synth.NewDecayEnvelope(sampleRate),
		noiseEnv:   synth.NewDecayEnvelope(sampleRate),
		pitch:      550.0, // Around 550Hz for 909-style
		decay:      0.04,  // 40ms default - very short
		tone:       0.6,   // Balanced tone/noise
		snap:       0.8,   // Sharp attack
		sampleRate: sampleRate,
		active:     false,
		phase:      0.0,
	}

	// Calculate phase increment for pitch
	rim.phaseInc = rim.pitch / sampleRate

	// Configure filter for noise (high-pass for crisp click)
	rim.filter.SetHighPass(2000.0, 0.7)

	// Set envelope times
	rim.toneEnv.SetDecayTime(rim.decay)
	rim.noiseEnv.SetDecayTime(rim.decay * 0.5) // Noise decays faster

	// Initialize LFOs
	rim.lfo1 = dsp.NewLFO(sampleRate)
	rim.lfo2 = dsp.NewLFO(sampleRate)
	rim.lfo1Target = RimshotLFOTargetNone
	rim.lfo1Enabled = false
	rim.lfo2Target = RimshotLFOTargetNone
	rim.lfo2Enabled = false

	// Configure LFO1 defaults
	rim.lfo1.SetWaveform(dsp.LFOSine)
	rim.lfo1.SetDepth(0.5)
	rim.lfo1.SetRange(400.0, 700.0) // Default pitch range
	rim.lfo1.SetRetrigger(false)

	// Configure LFO2 defaults
	rim.lfo2.SetWaveform(dsp.LFOTriangle)
	rim.lfo2.SetDepth(0.3)
	rim.lfo2.SetRange(0.3, 0.9) // Default tone range
	rim.lfo2.SetRetrigger(false)

	return rim
}

// SetPitch sets the oscillator pitch in Hz (200-800)
func (r *RimshotDrum) SetPitch(hz float32) {
	r.pitch = util.Clamp(hz, 200.0, 800.0)
}

// SetDecay sets the decay time in seconds
func (r *RimshotDrum) SetDecay(decay float32) {
	r.decay = util.Clamp(decay, 0.01, 0.15)
	r.toneEnv.SetDecayTime(r.decay)
	r.noiseEnv.SetDecayTime(r.decay * 0.5)
}

// SetTone sets the tone/noise balance (0.0 to 1.0)
func (r *RimshotDrum) SetTone(tone float32) {
	r.tone = util.Clamp(tone, 0.0, 1.0)
}

// SetSnap sets the attack sharpness (0.0 to 1.0)
func (r *RimshotDrum) SetSnap(snap float32) {
	r.snap = util.Clamp(snap, 0.0, 1.0)
}

// Trigger starts the rimshot drum
func (r *RimshotDrum) Trigger(velocity float32) {
	// Trigger envelopes
	r.toneEnv.Trigger()
	r.toneEnv.SetValue(velocity)
	r.noiseEnv.Trigger()
	r.noiseEnv.SetValue(velocity)

	// Reset oscillator phase for consistent attack
	r.phase = 0.0
	r.phaseInc = r.pitch / r.sampleRate

	r.active = true
}

// Process generates a buffer of audio samples
func (r *RimshotDrum) Process(output []float32) {
	// Process LFOs (always run to keep them in sync)
	var lfo1Value, lfo2Value float32
	if r.lfo1Enabled {
		lfo1Value = r.lfo1.Process()
	}
	if r.lfo2Enabled {
		lfo2Value = r.lfo2.Process()
	}

	if !r.active && !r.toneEnv.IsActive() {
		// Silent - clear buffer
		for i := range output {
			output[i] = 0.0
		}
		return
	}

	// Initialize modulated parameters with base values
	modPitch := r.pitch
	modDecay := r.decay
	modTone := r.tone
	modSnap := r.snap

	// Apply LFO1 modulation
	if r.lfo1Enabled {
		switch r.lfo1Target {
		case RimshotLFOTargetPitch:
			modPitch = lfo1Value
		case RimshotLFOTargetDecay:
			modDecay = lfo1Value
		case RimshotLFOTargetTone:
			modTone = lfo1Value
		case RimshotLFOTargetSnap:
			modSnap = lfo1Value
		}
	}

	// Apply LFO2 modulation
	if r.lfo2Enabled {
		switch r.lfo2Target {
		case RimshotLFOTargetPitch:
			modPitch = lfo2Value
		case RimshotLFOTargetDecay:
			modDecay = lfo2Value
		case RimshotLFOTargetTone:
			modTone = lfo2Value
		case RimshotLFOTargetSnap:
			modSnap = lfo2Value
		}
	}

	// Update envelope decay times with modulated decay
	r.toneEnv.SetDecayTime(modDecay)
	r.noiseEnv.SetDecayTime(modDecay * 0.5)

	// Update phase increment with modulated pitch
	modPhaseInc := modPitch / r.sampleRate

	for i := range output {
		// Process envelopes
		toneEnvVal := r.toneEnv.Process()
		noiseEnvVal := r.noiseEnv.Process()

		// Generate tonal component (sine wave)
		toneSample := float32(math.Sin(float64(r.phase*2.0*math.Pi))) * toneEnvVal

		// Advance phase with modulated phase increment
		r.phase += modPhaseInc
		if r.phase >= 1.0 {
			r.phase -= 1.0
		}

		// Generate noise component
		noise := r.noiseGen.NextWhite()
		noiseSample := r.filter.Process(noise) * noiseEnvVal

		// Mix tone and noise based on modulated tone parameter
		sample := toneSample*modTone + noiseSample*(1.0-modTone)

		// Apply modulated snap (additional shaping)
		if modSnap > 0.0 {
			// Soft saturation for sharper attack
			sample = sample * (1.0 + modSnap)
			sample = util.FastTanh(sample)
		}

		output[i] = sample * 0.8 // Scale to prevent clipping
	}

	// Check if still active
	if !r.toneEnv.IsActive() && !r.noiseEnv.IsActive() {
		r.active = false
	}
}

// IsActive returns true if the rimshot is producing sound
func (r *RimshotDrum) IsActive() bool {
	return r.toneEnv.IsActive() || r.noiseEnv.IsActive()
}

// Reset resets the rimshot drum state
func (r *RimshotDrum) Reset() {
	r.toneEnv.Reset()
	r.noiseEnv.Reset()
	r.filter.Reset()
	r.active = false
}

// Getter methods for UI

// GetPitch returns the pitch in Hz
func (r *RimshotDrum) GetPitch() float32 {
	return r.pitch
}

// GetDecay returns the decay time
func (r *RimshotDrum) GetDecay() float32 {
	return r.decay
}

// GetTone returns the tone/noise balance
func (r *RimshotDrum) GetTone() float32 {
	return r.tone
}

// GetSnap returns the snap amount
func (r *RimshotDrum) GetSnap() float32 {
	return r.snap
}

// GetCurrentPitch returns the current LFO-modulated pitch value
func (r *RimshotDrum) GetCurrentPitch() float32 {
	// Check if either LFO is modulating pitch
	if r.lfo1Enabled && r.lfo1Target == RimshotLFOTargetPitch {
		return util.Clamp(r.lfo1.GetValue(), 200.0, 800.0)
	}
	if r.lfo2Enabled && r.lfo2Target == RimshotLFOTargetPitch {
		return util.Clamp(r.lfo2.GetValue(), 200.0, 800.0)
	}

	return r.pitch
}

// GetCurrentDecay returns the current LFO-modulated decay value
func (r *RimshotDrum) GetCurrentDecay() float32 {
	// Check if either LFO is modulating decay
	if r.lfo1Enabled && r.lfo1Target == RimshotLFOTargetDecay {
		return util.Clamp(r.lfo1.GetValue(), 0.01, 0.15)
	}
	if r.lfo2Enabled && r.lfo2Target == RimshotLFOTargetDecay {
		return util.Clamp(r.lfo2.GetValue(), 0.01, 0.15)
	}

	return r.decay
}

// GetCurrentTone returns the current LFO-modulated tone value
func (r *RimshotDrum) GetCurrentTone() float32 {
	// Check if either LFO is modulating tone
	if r.lfo1Enabled && r.lfo1Target == RimshotLFOTargetTone {
		return util.Clamp(r.lfo1.GetValue(), 0.0, 1.0)
	}
	if r.lfo2Enabled && r.lfo2Target == RimshotLFOTargetTone {
		return util.Clamp(r.lfo2.GetValue(), 0.0, 1.0)
	}

	return r.tone
}

// GetCurrentSnap returns the current LFO-modulated snap value
func (r *RimshotDrum) GetCurrentSnap() float32 {
	// Check if either LFO is modulating snap
	if r.lfo1Enabled && r.lfo1Target == RimshotLFOTargetSnap {
		return util.Clamp(r.lfo1.GetValue(), 0.0, 1.0)
	}
	if r.lfo2Enabled && r.lfo2Target == RimshotLFOTargetSnap {
		return util.Clamp(r.lfo2.GetValue(), 0.0, 1.0)
	}

	return r.snap
}

// LFO1 Methods

// SetLFO1Enabled enables/disables LFO1
func (r *RimshotDrum) SetLFO1Enabled(enabled bool) {
	r.lfo1Enabled = enabled
}

// GetLFO1Enabled returns whether LFO1 is enabled
func (r *RimshotDrum) GetLFO1Enabled() bool {
	return r.lfo1Enabled
}

// SetLFO1Target sets what parameter LFO1 modulates
func (r *RimshotDrum) SetLFO1Target(target RimshotLFOTarget) {
	r.lfo1Target = target
	// Update LFO range based on target
	switch target {
	case RimshotLFOTargetPitch:
		r.lfo1.SetRange(200.0, 800.0) // Full pitch range
	case RimshotLFOTargetDecay:
		r.lfo1.SetRange(0.01, 0.15) // 10-150ms
	case RimshotLFOTargetTone:
		r.lfo1.SetRange(0.0, 1.0) // 0-100%
	case RimshotLFOTargetSnap:
		r.lfo1.SetRange(0.0, 1.0) // 0-100%
	}
}

// GetLFO1Target returns what parameter LFO1 modulates
func (r *RimshotDrum) GetLFO1Target() RimshotLFOTarget {
	return r.lfo1Target
}

// GetLFO1 returns the LFO1 instance
func (r *RimshotDrum) GetLFO1() *dsp.LFO {
	return r.lfo1
}

// LFO2 Methods

// SetLFO2Enabled enables/disables LFO2
func (r *RimshotDrum) SetLFO2Enabled(enabled bool) {
	r.lfo2Enabled = enabled
}

// GetLFO2Enabled returns whether LFO2 is enabled
func (r *RimshotDrum) GetLFO2Enabled() bool {
	return r.lfo2Enabled
}

// SetLFO2Target sets what parameter LFO2 modulates
func (r *RimshotDrum) SetLFO2Target(target RimshotLFOTarget) {
	r.lfo2Target = target
	// Update LFO range based on target
	switch target {
	case RimshotLFOTargetPitch:
		r.lfo2.SetRange(200.0, 800.0) // Full pitch range
	case RimshotLFOTargetDecay:
		r.lfo2.SetRange(0.01, 0.15) // 10-150ms
	case RimshotLFOTargetTone:
		r.lfo2.SetRange(0.0, 1.0) // 0-100%
	case RimshotLFOTargetSnap:
		r.lfo2.SetRange(0.0, 1.0) // 0-100%
	}
}

// GetLFO2Target returns what parameter LFO2 modulates
func (r *RimshotDrum) GetLFO2Target() RimshotLFOTarget {
	return r.lfo2Target
}

// GetLFO2 returns the LFO2 instance
func (r *RimshotDrum) GetLFO2() *dsp.LFO {
	return r.lfo2
}

// UpdateLFOTempo updates the LFO rates based on BPM for tempo sync
func (r *RimshotDrum) UpdateLFOTempo(bpm float32) {
	r.lfo1.UpdateTempoRate(bpm)
	r.lfo2.UpdateTempoRate(bpm)
}
