package drums

import (
	"math"

	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/util"
)

// KickLFOTarget represents what parameter the LFO modulates
type KickLFOTarget int

const (
	KickLFOTargetNone KickLFOTarget = iota
	KickLFOTargetPitch      // Modulate baseFreq (30-100Hz)
	KickLFOTargetDecay      // Modulate decay time (0.05-2.0s)
	KickLFOTargetPitchDecay // Modulate pitch envelope decay (0.001-0.1s)
	KickLFOTargetDrive      // Modulate drive amount (0.0-1.0)
)

// KickDrum is an analog-modeled kick drum synthesizer
type KickDrum struct {
	// Oscillator
	phase      float32
	baseFreq   float32 // Base frequency (40-80Hz)
	pitchStart float32 // Pitch envelope start frequency
	pitchDecay float32 // Pitch envelope decay time

	// Envelopes
	ampEnv   *synth.DecayEnvelope
	pitchEnv *synth.DecayEnvelope

	// Attack click
	clickAmount float32
	noiseGen    *dsp.NoiseGenerator

	// Parameters
	tune    float32 // -12 to +12 semitones
	decay   float32 // 0.1 to 1.0 seconds
	drive   float32 // 0.0 to 1.0
	clickMix float32 // 0.0 to 1.0

	// LFO modulation
	lfo1        *dsp.LFO
	lfo2        *dsp.LFO
	lfo1Target  KickLFOTarget
	lfo1Enabled bool
	lfo2Target  KickLFOTarget
	lfo2Enabled bool

	// Internal
	sampleRate float32
	active     bool
}

// NewKickDrum creates a new kick drum synthesizer
func NewKickDrum(sampleRate float32) *KickDrum {
	kick := &KickDrum{
		baseFreq:    50.0,  // 50Hz default
		pitchStart:  400.0, // Start at 400Hz
		pitchDecay:  0.01,  // 10ms pitch envelope
		ampEnv:      synth.NewDecayEnvelope(sampleRate),
		pitchEnv:    synth.NewDecayEnvelope(sampleRate),
		noiseGen:    dsp.NewNoiseGenerator(),
		tune:        0.0,
		decay:       0.3,   // 300ms default
		drive:       0.3,   // Moderate drive
		clickMix:    0.05,  // 5% click
		sampleRate:  sampleRate,
		active:      false,
	}

	kick.ampEnv.SetDecayTime(kick.decay)
	kick.pitchEnv.SetDecayTime(kick.pitchDecay)

	// Initialize LFOs
	kick.lfo1 = dsp.NewLFO(sampleRate)
	kick.lfo2 = dsp.NewLFO(sampleRate)
	kick.lfo1Target = KickLFOTargetNone
	kick.lfo1Enabled = false
	kick.lfo2Target = KickLFOTargetNone
	kick.lfo2Enabled = false

	// Configure LFO1 defaults
	kick.lfo1.SetWaveform(dsp.LFOSine)
	kick.lfo1.SetDepth(0.5)
	kick.lfo1.SetRange(50.0, 70.0) // Default pitch range
	kick.lfo1.SetRetrigger(false)

	// Configure LFO2 defaults
	kick.lfo2.SetWaveform(dsp.LFOTriangle)
	kick.lfo2.SetDepth(0.3)
	kick.lfo2.SetRange(0.2, 0.6) // Default decay range
	kick.lfo2.SetRetrigger(false)

	return kick
}

// SetPitch sets the base pitch in Hz (40-80Hz typical)
func (k *KickDrum) SetPitch(frequency float32) {
	k.baseFreq = util.Clamp(frequency, 30.0, 100.0)
}

// SetTune sets the tuning in semitones (-12 to +12)
func (k *KickDrum) SetTune(semitones float32) {
	k.tune = util.Clamp(semitones, -12.0, 12.0)
}

// SetDecay sets the amplitude decay time in seconds
func (k *KickDrum) SetDecay(decay float32) {
	k.decay = util.Clamp(decay, 0.05, 2.0)
	k.ampEnv.SetDecayTime(k.decay)
}

// SetPitchDecay sets the pitch envelope decay time in seconds
func (k *KickDrum) SetPitchDecay(decay float32) {
	k.pitchDecay = util.Clamp(decay, 0.001, 0.1)
	k.pitchEnv.SetDecayTime(k.pitchDecay)
}

// SetDrive sets the drive/saturation amount (0.0 to 1.0)
func (k *KickDrum) SetDrive(drive float32) {
	k.drive = util.Clamp(drive, 0.0, 1.0)
}

// SetClickAmount sets the attack click amount (0.0 to 1.0)
func (k *KickDrum) SetClickAmount(amount float32) {
	k.clickMix = util.Clamp(amount, 0.0, 1.0)
}

// Trigger starts the kick drum
func (k *KickDrum) Trigger(velocity float32) {
	// Reset phase
	k.phase = 0.0

	// Trigger envelopes
	k.ampEnv.Trigger()
	k.ampEnv.SetValue(velocity)
	k.pitchEnv.Trigger()

	k.active = true
}

// Process generates a buffer of audio samples
func (k *KickDrum) Process(output []float32) {
	// Process LFOs (always run to keep them in sync)
	var lfo1Value, lfo2Value float32
	if k.lfo1Enabled {
		lfo1Value = k.lfo1.Process()
	}
	if k.lfo2Enabled {
		lfo2Value = k.lfo2.Process()
	}

	if !k.active && !k.ampEnv.IsActive() {
		// Silent - clear buffer
		for i := range output {
			output[i] = 0.0
		}
		return
	}

	// Apply tuning to base frequency
	tunedFreq := k.baseFreq * float32(math.Pow(2.0, float64(k.tune/12.0)))

	for i := range output {

		// Initialize modulated parameters with base values
		modBaseFreq := tunedFreq
		modDrive := k.drive

		// Apply LFO1 modulation
		if k.lfo1Enabled {
			switch k.lfo1Target {
			case KickLFOTargetPitch:
				modBaseFreq = lfo1Value
			case KickLFOTargetDrive:
				modDrive = lfo1Value
			// Note: Decay modulation not implemented (requires envelope changes)
			}
		}

		// Apply LFO2 modulation (can override LFO1 if targeting same parameter)
		if k.lfo2Enabled {
			switch k.lfo2Target {
			case KickLFOTargetPitch:
				modBaseFreq = lfo2Value
			case KickLFOTargetDrive:
				modDrive = lfo2Value
			// Note: Decay modulation not implemented (requires envelope changes)
			}
		}

		// Get envelope values
		ampEnvValue := k.ampEnv.Process()
		pitchEnvValue := k.pitchEnv.Process()

		// Pitch modulation: start frequency sweeps down to base frequency
		freq := modBaseFreq + (k.pitchStart-modBaseFreq)*pitchEnvValue

		// Generate sine wave
		sine := float32(math.Sin(float64(k.phase) * 2.0 * math.Pi))

		// Apply drive/saturation (using modulated drive value)
		if modDrive > 0.0 {
			driveAmount := 1.0 + modDrive*5.0 // 1x to 6x
			sine = util.FastTanh(sine * driveAmount)
		}

		// Add attack click (short noise burst)
		click := float32(0.0)
		if k.clickMix > 0.0 && pitchEnvValue > 0.01 {
			click = k.noiseGen.NextWhite() * k.clickMix * pitchEnvValue * 0.5
		}

		// Combine sine + click
		sample := sine + click

		// Apply amplitude envelope
		output[i] = sample * ampEnvValue * 0.7 // Scale to prevent clipping

		// Advance phase
		phaseIncrement := freq / k.sampleRate
		k.phase += phaseIncrement
		if k.phase >= 1.0 {
			k.phase -= 1.0
		}
	}

	// Check if still active
	if !k.ampEnv.IsActive() {
		k.active = false
	}
}

// IsActive returns true if the kick is producing sound
func (k *KickDrum) IsActive() bool {
	return k.ampEnv.IsActive()
}

// Reset resets the kick drum state
func (k *KickDrum) Reset() {
	k.phase = 0.0
	k.ampEnv.Reset()
	k.pitchEnv.Reset()
	k.active = false
}

// Getter methods for UI

// GetTune returns the tuning in semitones
func (k *KickDrum) GetTune() float32 {
	return k.tune
}

// GetDecay returns the decay time in seconds
func (k *KickDrum) GetDecay() float32 {
	return k.decay
}

// GetPitchDecay returns the pitch envelope decay time
func (k *KickDrum) GetPitchDecay() float32 {
	return k.pitchDecay
}

// GetDrive returns the drive amount
func (k *KickDrum) GetDrive() float32 {
	return k.drive
}

// GetClickAmount returns the click amount
func (k *KickDrum) GetClickAmount() float32 {
	return k.clickMix
}

// GetPitch returns the base pitch in Hz
func (k *KickDrum) GetPitch() float32 {
	return k.baseFreq
}

// GetCurrentPitch returns the current LFO-modulated pitch value
func (k *KickDrum) GetCurrentPitch() float32 {
	tunedFreq := k.baseFreq * float32(math.Pow(2.0, float64(k.tune/12.0)))

	// Check if either LFO is modulating pitch
	if k.lfo1Enabled && k.lfo1Target == KickLFOTargetPitch {
		return util.Clamp(k.lfo1.GetValue(), 30.0, 100.0)
	}
	if k.lfo2Enabled && k.lfo2Target == KickLFOTargetPitch {
		return util.Clamp(k.lfo2.GetValue(), 30.0, 100.0)
	}

	return tunedFreq
}

// GetCurrentDrive returns the current LFO-modulated drive value
func (k *KickDrum) GetCurrentDrive() float32 {
	// Check if either LFO is modulating drive
	if k.lfo1Enabled && k.lfo1Target == KickLFOTargetDrive {
		return util.Clamp(k.lfo1.GetValue(), 0.0, 1.0)
	}
	if k.lfo2Enabled && k.lfo2Target == KickLFOTargetDrive {
		return util.Clamp(k.lfo2.GetValue(), 0.0, 1.0)
	}

	return k.drive
}

// GetCurrentDecay returns the current LFO-modulated decay value
func (k *KickDrum) GetCurrentDecay() float32 {
	// Check if either LFO is modulating decay
	if k.lfo1Enabled && k.lfo1Target == KickLFOTargetDecay {
		return util.Clamp(k.lfo1.GetValue(), 0.05, 2.0)
	}
	if k.lfo2Enabled && k.lfo2Target == KickLFOTargetDecay {
		return util.Clamp(k.lfo2.GetValue(), 0.05, 2.0)
	}

	return k.decay
}

// GetCurrentPitchDecay returns the current LFO-modulated pitch decay value
func (k *KickDrum) GetCurrentPitchDecay() float32 {
	// Check if either LFO is modulating pitch decay
	if k.lfo1Enabled && k.lfo1Target == KickLFOTargetPitchDecay {
		return util.Clamp(k.lfo1.GetValue(), 0.001, 0.1)
	}
	if k.lfo2Enabled && k.lfo2Target == KickLFOTargetPitchDecay {
		return util.Clamp(k.lfo2.GetValue(), 0.001, 0.1)
	}

	return k.pitchDecay
}

// LFO control methods

// GetLFO1 returns the LFO1 instance
func (k *KickDrum) GetLFO1() *dsp.LFO {
	return k.lfo1
}

// GetLFO2 returns the LFO2 instance
func (k *KickDrum) GetLFO2() *dsp.LFO {
	return k.lfo2
}

// SetLFO1Enabled enables or disables LFO1
func (k *KickDrum) SetLFO1Enabled(enabled bool) {
	k.lfo1Enabled = enabled
}

// GetLFO1Enabled returns whether LFO1 is enabled
func (k *KickDrum) GetLFO1Enabled() bool {
	return k.lfo1Enabled
}

// SetLFO2Enabled enables or disables LFO2
func (k *KickDrum) SetLFO2Enabled(enabled bool) {
	k.lfo2Enabled = enabled
}

// GetLFO2Enabled returns whether LFO2 is enabled
func (k *KickDrum) GetLFO2Enabled() bool {
	return k.lfo2Enabled
}

// SetLFO1Target sets the LFO1 modulation target and updates range
func (k *KickDrum) SetLFO1Target(target KickLFOTarget) {
	k.lfo1Target = target
	// Update LFO range based on target
	switch target {
	case KickLFOTargetPitch:
		k.lfo1.SetRange(30.0, 100.0) // Full frequency range
	case KickLFOTargetDecay:
		k.lfo1.SetRange(0.05, 2.0) // Full decay range
	case KickLFOTargetPitchDecay:
		k.lfo1.SetRange(0.001, 0.1) // Pitch env range
	case KickLFOTargetDrive:
		k.lfo1.SetRange(0.0, 1.0) // Drive range
	}
}

// GetLFO1Target returns the LFO1 modulation target
func (k *KickDrum) GetLFO1Target() KickLFOTarget {
	return k.lfo1Target
}

// SetLFO2Target sets the LFO2 modulation target and updates range
func (k *KickDrum) SetLFO2Target(target KickLFOTarget) {
	k.lfo2Target = target
	// Update LFO range based on target
	switch target {
	case KickLFOTargetPitch:
		k.lfo2.SetRange(30.0, 100.0)
	case KickLFOTargetDecay:
		k.lfo2.SetRange(0.05, 2.0)
	case KickLFOTargetPitchDecay:
		k.lfo2.SetRange(0.001, 0.1)
	case KickLFOTargetDrive:
		k.lfo2.SetRange(0.0, 1.0)
	}
}

// GetLFO2Target returns the LFO2 modulation target
func (k *KickDrum) GetLFO2Target() KickLFOTarget {
	return k.lfo2Target
}

// UpdateLFOTempo updates the LFO rates based on BPM for tempo sync
func (k *KickDrum) UpdateLFOTempo(bpm float32) {
	k.lfo1.UpdateTempoRate(bpm)
	k.lfo2.UpdateTempoRate(bpm)
}
