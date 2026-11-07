package drums

import (
	"math"

	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/util"
)

// TomLFOTarget represents what parameter the LFO modulates
type TomLFOTarget int

const (
	TomLFOTargetNone       TomLFOTarget = iota
	TomLFOTargetPitch                   // Modulate base pitch (40-300 Hz)
	TomLFOTargetDecay                   // Modulate decay time (0.1-2.0s)
	TomLFOTargetTune                    // Modulate tuning (-12 to +12 semitones)
	TomLFOTargetPitchDecay              // Modulate pitch envelope depth (0.0-1.0)
	TomLFOTargetTone                    // Modulate tone/noise balance (0.0-1.0)
	TomLFOTargetAttack                  // Modulate attack character (0.0-1.0)
)

// TomDrum is an analog-modeled tom drum synthesizer
// Combines pitched sine wave with noise for realistic tom sound
type TomDrum struct {
	// Oscillator for pitched body
	phase    float32
	phaseInc float32

	// Noise generator for attack
	noiseGen *dsp.NoiseGenerator
	filter   *dsp.BiquadFilter

	// Envelopes
	pitchEnv *synth.DecayEnvelope // For pitch sweep
	ampEnv   *synth.DecayEnvelope // For amplitude
	noiseEnv *synth.DecayEnvelope // For noise component

	// Parameters
	pitch      float32 // Base pitch (40-300 Hz typical)
	decay      float32 // Decay time (0.1 to 2.0 seconds)
	tune       float32 // Pitch tuning (-12 to +12 semitones)
	pitchDecay float32 // Pitch envelope depth (0.0 to 1.0)
	tone       float32 // Balance between tone and noise (0.0 to 1.0)
	attack     float32 // Attack character (0.0 to 1.0)

	// LFO modulation
	lfo1        *dsp.LFO
	lfo2        *dsp.LFO
	lfo1Target  TomLFOTarget
	lfo1Enabled bool
	lfo2Target  TomLFOTarget
	lfo2Enabled bool

	// Internal
	noteRatio  float32
	sampleRate float32
	active     bool
	startPitch float32 // Initial pitch for sweep
	endPitch   float32 // Final pitch after sweep
}

// NewTomDrum creates a new tom drum synthesizer
func NewTomDrum(sampleRate float32) *TomDrum {
	tom := &TomDrum{
		noiseGen:   dsp.NewNoiseGenerator(),
		filter:     dsp.NewBiquadFilter(sampleRate),
		pitchEnv:   synth.NewDecayEnvelope(sampleRate),
		ampEnv:     synth.NewDecayEnvelope(sampleRate),
		noiseEnv:   synth.NewDecayEnvelope(sampleRate),
		noteRatio:  1,
		pitch:      100.0, // Mid tom default (around 100Hz)
		decay:      0.5,   // 500ms default
		tune:       0.0,   // No detuning
		pitchDecay: 0.5,   // Moderate pitch sweep
		tone:       0.7,   // More tone than noise
		attack:     0.5,   // Moderate attack
		sampleRate: sampleRate,
		active:     false,
		phase:      0.0,
	}

	// Configure filter for noise (band-pass for punchy attack)
	tom.filter.SetBandPass(300.0, 1.5)

	// Set envelope times
	tom.ampEnv.SetDecayTime(tom.decay)
	tom.pitchEnv.SetDecayTime(0.05) // Quick pitch sweep
	tom.noiseEnv.SetDecayTime(0.02) // Very short noise burst

	// Initialize LFOs
	tom.lfo1 = dsp.NewLFO(sampleRate)
	tom.lfo2 = dsp.NewLFO(sampleRate)
	tom.lfo1Target = TomLFOTargetNone
	tom.lfo1Enabled = false
	tom.lfo2Target = TomLFOTargetNone
	tom.lfo2Enabled = false

	// Configure LFO1 defaults
	tom.lfo1.SetWaveform(dsp.LFOSine)
	tom.lfo1.SetDepth(0.5)
	tom.lfo1.SetRange(80.0, 200.0) // Default pitch range
	tom.lfo1.SetRetrigger(false)

	// Configure LFO2 defaults
	tom.lfo2.SetWaveform(dsp.LFOTriangle)
	tom.lfo2.SetDepth(0.3)
	tom.lfo2.SetRange(0.3, 0.9) // Default tone range
	tom.lfo2.SetRetrigger(false)

	return tom
}

// SetPitch sets the base pitch in Hz (40-300)
func (t *TomDrum) SetPitch(hz float32) {
	t.pitch = util.Clamp(hz, 40.0, 300.0)
}

// SetDecay sets the decay time in seconds
func (t *TomDrum) SetDecay(decay float32) {
	t.decay = util.Clamp(decay, 0.1, 2.0)
	t.ampEnv.SetDecayTime(t.decay)
}

// SetTune sets the pitch tuning in semitones (-12 to +12)
func (t *TomDrum) SetTune(semitones float32) {
	t.tune = util.Clamp(semitones, -12.0, 12.0)
}

// SetPitchDecay sets the pitch envelope depth (0.0 to 1.0)
func (t *TomDrum) SetPitchDecay(depth float32) {
	t.pitchDecay = util.Clamp(depth, 0.0, 1.0)
}

// SetTone sets the tone/noise balance (0.0 to 1.0)
func (t *TomDrum) SetTone(tone float32) {
	t.tone = util.Clamp(tone, 0.0, 1.0)
}

// SetAttack sets the attack character (0.0 to 1.0)
func (t *TomDrum) SetAttack(attack float32) {
	t.attack = util.Clamp(attack, 0.0, 1.0)
}

// Trigger starts the tom drum
func (t *TomDrum) Trigger(velocity float32) {
	// Calculate tuned pitch
	tuneFactor := util.Exp2(t.tune / 12.0)
	basePitch := util.Clamp(t.pitch*tuneFactor*t.noteRatio, 20, t.sampleRate/6)

	// Set pitch sweep range
	t.endPitch = basePitch
	t.startPitch = basePitch * (1.0 + t.pitchDecay*2.0) // Sweep down from higher pitch

	// Trigger envelopes
	t.pitchEnv.Trigger()
	t.pitchEnv.SetValue(1.0)

	t.ampEnv.Trigger()
	t.ampEnv.SetValue(velocity)

	t.noiseEnv.Trigger()
	t.noiseEnv.SetValue(velocity * t.attack)

	// Reset phase
	t.phase = 0.0

	t.active = true
}

// Process generates a buffer of audio samples
func (t *TomDrum) Process(output []float32) {
	// Process LFOs (always run to keep them in sync)
	var lfo1Value, lfo2Value float32
	if t.lfo1Enabled {
		lfo1Value = t.lfo1.Process()
	}
	if t.lfo2Enabled {
		lfo2Value = t.lfo2.Process()
	}

	if !t.active && !t.ampEnv.IsActive() {
		// Silent - clear buffer
		for i := range output {
			output[i] = 0.0
		}
		return
	}

	// Initialize modulated parameters with base values
	modPitch := t.pitch
	modDecay := t.decay
	modTune := t.tune
	modPitchDecay := t.pitchDecay
	modTone := t.tone
	modAttack := t.attack

	// Apply LFO1 modulation
	if t.lfo1Enabled {
		switch t.lfo1Target {
		case TomLFOTargetPitch:
			modPitch = lfo1Value
		case TomLFOTargetDecay:
			modDecay = lfo1Value
		case TomLFOTargetTune:
			modTune = lfo1Value
		case TomLFOTargetPitchDecay:
			modPitchDecay = lfo1Value
		case TomLFOTargetTone:
			modTone = lfo1Value
		case TomLFOTargetAttack:
			modAttack = lfo1Value
		}
	}

	// Apply LFO2 modulation
	if t.lfo2Enabled {
		switch t.lfo2Target {
		case TomLFOTargetPitch:
			modPitch = lfo2Value
		case TomLFOTargetDecay:
			modDecay = lfo2Value
		case TomLFOTargetTune:
			modTune = lfo2Value
		case TomLFOTargetPitchDecay:
			modPitchDecay = lfo2Value
		case TomLFOTargetTone:
			modTone = lfo2Value
		case TomLFOTargetAttack:
			modAttack = lfo2Value
		}
	}

	// Update envelope decay time with modulated decay
	t.ampEnv.SetDecayTime(modDecay)

	// Update filter frequency based on modulated pitch
	filterFreq := modPitch * 3.0
	t.filter.SetBandPass(filterFreq, 1.5)

	// Calculate tuned pitch with modulated tune
	tuneFactor := util.Exp2(modTune / 12.0)
	basePitch := util.Clamp(modPitch*tuneFactor*t.noteRatio, 20, t.sampleRate/6)

	// Set pitch sweep range with modulated pitchDecay
	endPitch := basePitch
	startPitch := basePitch * (1.0 + modPitchDecay*2.0)

	for i := range output {
		// Process envelopes
		pitchEnvVal := t.pitchEnv.Process()
		ampEnvVal := t.ampEnv.Process()
		noiseEnvVal := t.noiseEnv.Process()

		// Calculate current pitch with envelope (using modulated values)
		currentPitch := endPitch + (startPitch-endPitch)*pitchEnvVal
		t.phaseInc = currentPitch / t.sampleRate

		// Generate tonal component (sine wave)
		toneSample := float32(math.Sin(float64(t.phase*2.0*math.Pi))) * ampEnvVal

		// Advance phase
		t.phase += t.phaseInc
		if t.phase >= 1.0 {
			t.phase -= 1.0
		}

		// Generate noise component
		noise := t.noiseGen.NextWhite()
		noiseSample := t.filter.Process(noise) * noiseEnvVal

		// Mix tone and noise (using modulated tone)
		sample := toneSample*modTone + noiseSample*(1.0-modTone)

		// Apply soft saturation for warmth (modulated attack affects saturation)
		sample = util.FastTanh(sample * (1.0 + modAttack*0.5))

		output[i] = sample * 0.7 // Scale to prevent clipping
	}

	// Check if still active
	if !t.ampEnv.IsActive() {
		t.active = false
	}
}

// IsActive returns true if the tom is producing sound
func (t *TomDrum) IsActive() bool {
	return t.ampEnv.IsActive()
}

// Reset resets the tom drum state
func (t *TomDrum) Reset() {
	t.pitchEnv.Reset()
	t.ampEnv.Reset()
	t.noiseEnv.Reset()
	t.filter.Reset()
	t.active = false
}

// Getter methods for UI

// GetPitch returns the pitch in Hz
func (t *TomDrum) GetPitch() float32 {
	return t.pitch
}

// GetDecay returns the decay time
func (t *TomDrum) GetDecay() float32 {
	return t.decay
}

// GetTune returns the tuning in semitones
func (t *TomDrum) GetTune() float32 {
	return t.tune
}

// GetPitchDecay returns the pitch envelope depth
func (t *TomDrum) GetPitchDecay() float32 {
	return t.pitchDecay
}

// GetTone returns the tone/noise balance
func (t *TomDrum) GetTone() float32 {
	return t.tone
}

// GetAttack returns the attack character
func (t *TomDrum) GetAttack() float32 {
	return t.attack
}

// GetCurrentPitch returns the current LFO-modulated pitch value
func (t *TomDrum) GetCurrentPitch() float32 {
	// Check if either LFO is modulating pitch
	if t.lfo1Enabled && t.lfo1Target == TomLFOTargetPitch {
		return util.Clamp(t.lfo1.GetValue(), 40.0, 300.0)
	}
	if t.lfo2Enabled && t.lfo2Target == TomLFOTargetPitch {
		return util.Clamp(t.lfo2.GetValue(), 40.0, 300.0)
	}

	return t.pitch
}

// GetCurrentDecay returns the current LFO-modulated decay value
func (t *TomDrum) GetCurrentDecay() float32 {
	// Check if either LFO is modulating decay
	if t.lfo1Enabled && t.lfo1Target == TomLFOTargetDecay {
		return util.Clamp(t.lfo1.GetValue(), 0.1, 2.0)
	}
	if t.lfo2Enabled && t.lfo2Target == TomLFOTargetDecay {
		return util.Clamp(t.lfo2.GetValue(), 0.1, 2.0)
	}

	return t.decay
}

// GetCurrentTune returns the current LFO-modulated tune value
func (t *TomDrum) GetCurrentTune() float32 {
	// Check if either LFO is modulating tune
	if t.lfo1Enabled && t.lfo1Target == TomLFOTargetTune {
		return util.Clamp(t.lfo1.GetValue(), -12.0, 12.0)
	}
	if t.lfo2Enabled && t.lfo2Target == TomLFOTargetTune {
		return util.Clamp(t.lfo2.GetValue(), -12.0, 12.0)
	}

	return t.tune
}

// GetCurrentPitchDecay returns the current LFO-modulated pitch decay value
func (t *TomDrum) GetCurrentPitchDecay() float32 {
	// Check if either LFO is modulating pitch decay
	if t.lfo1Enabled && t.lfo1Target == TomLFOTargetPitchDecay {
		return util.Clamp(t.lfo1.GetValue(), 0.0, 1.0)
	}
	if t.lfo2Enabled && t.lfo2Target == TomLFOTargetPitchDecay {
		return util.Clamp(t.lfo2.GetValue(), 0.0, 1.0)
	}

	return t.pitchDecay
}

// GetCurrentTone returns the current LFO-modulated tone value
func (t *TomDrum) GetCurrentTone() float32 {
	// Check if either LFO is modulating tone
	if t.lfo1Enabled && t.lfo1Target == TomLFOTargetTone {
		return util.Clamp(t.lfo1.GetValue(), 0.0, 1.0)
	}
	if t.lfo2Enabled && t.lfo2Target == TomLFOTargetTone {
		return util.Clamp(t.lfo2.GetValue(), 0.0, 1.0)
	}

	return t.tone
}

// GetCurrentAttack returns the current LFO-modulated attack value
func (t *TomDrum) GetCurrentAttack() float32 {
	// Check if either LFO is modulating attack
	if t.lfo1Enabled && t.lfo1Target == TomLFOTargetAttack {
		return util.Clamp(t.lfo1.GetValue(), 0.0, 1.0)
	}
	if t.lfo2Enabled && t.lfo2Target == TomLFOTargetAttack {
		return util.Clamp(t.lfo2.GetValue(), 0.0, 1.0)
	}

	return t.attack
}

// LFO1 Methods

// SetLFO1Enabled enables/disables LFO1
func (t *TomDrum) SetLFO1Enabled(enabled bool) {
	t.lfo1Enabled = enabled
}

// GetLFO1Enabled returns whether LFO1 is enabled
func (t *TomDrum) GetLFO1Enabled() bool {
	return t.lfo1Enabled
}

// SetLFO1Target sets what parameter LFO1 modulates
func (t *TomDrum) SetLFO1Target(target TomLFOTarget) {
	t.lfo1Target = target
	// Update LFO range based on target
	switch target {
	case TomLFOTargetPitch:
		t.lfo1.SetRange(40.0, 300.0) // Full pitch range
	case TomLFOTargetDecay:
		t.lfo1.SetRange(0.1, 2.0) // 0.1-2.0s
	case TomLFOTargetTune:
		t.lfo1.SetRange(-12.0, 12.0) // ±12 semitones
	case TomLFOTargetPitchDecay:
		t.lfo1.SetRange(0.0, 1.0) // 0-100%
	case TomLFOTargetTone:
		t.lfo1.SetRange(0.0, 1.0) // 0-100%
	case TomLFOTargetAttack:
		t.lfo1.SetRange(0.0, 1.0) // 0-100%
	}
}

// GetLFO1Target returns what parameter LFO1 modulates
func (t *TomDrum) GetLFO1Target() TomLFOTarget {
	return t.lfo1Target
}

// GetLFO1 returns the LFO1 instance
func (t *TomDrum) GetLFO1() *dsp.LFO {
	return t.lfo1
}

// LFO2 Methods

// SetLFO2Enabled enables/disables LFO2
func (t *TomDrum) SetLFO2Enabled(enabled bool) {
	t.lfo2Enabled = enabled
}

// GetLFO2Enabled returns whether LFO2 is enabled
func (t *TomDrum) GetLFO2Enabled() bool {
	return t.lfo2Enabled
}

// SetLFO2Target sets what parameter LFO2 modulates
func (t *TomDrum) SetLFO2Target(target TomLFOTarget) {
	t.lfo2Target = target
	// Update LFO range based on target
	switch target {
	case TomLFOTargetPitch:
		t.lfo2.SetRange(40.0, 300.0) // Full pitch range
	case TomLFOTargetDecay:
		t.lfo2.SetRange(0.1, 2.0) // 0.1-2.0s
	case TomLFOTargetTune:
		t.lfo2.SetRange(-12.0, 12.0) // ±12 semitones
	case TomLFOTargetPitchDecay:
		t.lfo2.SetRange(0.0, 1.0) // 0-100%
	case TomLFOTargetTone:
		t.lfo2.SetRange(0.0, 1.0) // 0-100%
	case TomLFOTargetAttack:
		t.lfo2.SetRange(0.0, 1.0) // 0-100%
	}
}

// GetLFO2Target returns what parameter LFO2 modulates
func (t *TomDrum) GetLFO2Target() TomLFOTarget {
	return t.lfo2Target
}

// GetLFO2 returns the LFO2 instance
func (t *TomDrum) GetLFO2() *dsp.LFO {
	return t.lfo2
}

// UpdateLFOTempo updates the LFO rates based on BPM for tempo sync
func (t *TomDrum) UpdateLFOTempo(bpm float32) {
	t.lfo1.UpdateTempoRate(bpm)
	t.lfo2.UpdateTempoRate(bpm)
}

// SetNote transposes the root pitch relative to C2. C2 preserves existing tom
// sounds; the ratio stays separate from pitch/tune controls and their LFOs.
func (t *TomDrum) SetNote(note int) {
	if note < 0 {
		note = 0
	}
	if note > 127 {
		note = 127
	}
	t.noteRatio = util.Exp2(float32(note-36) / 12)
}
