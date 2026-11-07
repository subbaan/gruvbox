package drums

import (
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/util"
)

// ClapLFOTarget represents what parameter the LFO modulates
type ClapLFOTarget int

const (
	ClapLFOTargetNone  ClapLFOTarget = iota
	ClapLFOTargetTune                // Modulate filter frequency tuning (-12 to +12 semitones)
	ClapLFOTargetDecay               // Modulate decay time (0.01 to 0.3s)
	ClapLFOTargetTone                // Modulate filter brightness (0.0 to 1.0)
	ClapLFOTargetDrive               // Modulate saturation/drive (0.0 to 1.0)
)

// ClapDrum is an analog-modeled clap synthesizer
// Simulates hand claps using filtered noise with multiple attack layers
type ClapDrum struct {
	// Noise generator and filters
	noiseGen *dsp.NoiseGenerator
	filter1  *dsp.BiquadFilter // Main clap filter
	filter2  *dsp.BiquadFilter // Secondary filter for brightness

	// Envelopes for multi-layer attack
	env1 *synth.DecayEnvelope // First slap
	env2 *synth.DecayEnvelope // Second slap (slightly delayed)
	env3 *synth.DecayEnvelope // Third slap (slightly delayed)

	// Delay counter for staggered attack
	delaySamples1 int // Delay for env2
	delaySamples2 int // Delay for env3
	delayCount    int // Current delay counter

	// Parameters
	tune  float32 // Filter frequency adjustment (-12 to +12 semitones)
	decay float32 // Decay time (0.01 to 0.3 seconds)
	tone  float32 // Filter brightness (0.0 to 1.0)
	drive float32 // Saturation/drive (0.0 to 1.0)

	// LFO modulation
	lfo1        *dsp.LFO
	lfo2        *dsp.LFO
	lfo1Target  ClapLFOTarget
	lfo1Enabled bool
	lfo2Target  ClapLFOTarget
	lfo2Enabled bool

	// Internal
	sampleRate float32
	active     bool
	velocity   float32
}

// NewClapDrum creates a new clap drum synthesizer
func NewClapDrum(sampleRate float32) *ClapDrum {
	clap := &ClapDrum{
		noiseGen:      dsp.NewNoiseGenerator(),
		filter1:       dsp.NewBiquadFilter(sampleRate),
		filter2:       dsp.NewBiquadFilter(sampleRate),
		env1:          synth.NewDecayEnvelope(sampleRate),
		env2:          synth.NewDecayEnvelope(sampleRate),
		env3:          synth.NewDecayEnvelope(sampleRate),
		delaySamples1: int(sampleRate * 0.005), // 5ms delay
		delaySamples2: int(sampleRate * 0.010), // 10ms delay
		tune:          0.0,
		decay:         0.08, // 80ms default
		tone:          0.6,  // Mid-bright
		drive:         0.3,  // Moderate drive
		sampleRate:    sampleRate,
		active:        false,
	}

	// Configure filters (bandpass around 1.2kHz for clap sound)
	clap.filter1.SetBandPass(1200.0, 2.0) // Main clap frequency
	clap.filter2.SetHighPass(800.0, 0.7)  // Brightness/crispness

	// Set decay times
	clap.env1.SetDecayTime(clap.decay)
	clap.env2.SetDecayTime(clap.decay * 1.2) // Slightly longer
	clap.env3.SetDecayTime(clap.decay * 1.4) // Even longer

	// Initialize LFOs
	clap.lfo1 = dsp.NewLFO(sampleRate)
	clap.lfo2 = dsp.NewLFO(sampleRate)
	clap.lfo1Target = ClapLFOTargetNone
	clap.lfo1Enabled = false
	clap.lfo2Target = ClapLFOTargetNone
	clap.lfo2Enabled = false

	// Configure LFO1 defaults
	clap.lfo1.SetWaveform(dsp.LFOSine)
	clap.lfo1.SetDepth(0.5)
	clap.lfo1.SetRange(0.3, 0.9) // Default tone range
	clap.lfo1.SetRetrigger(false)

	// Configure LFO2 defaults
	clap.lfo2.SetWaveform(dsp.LFOTriangle)
	clap.lfo2.SetDepth(0.3)
	clap.lfo2.SetRange(0.05, 0.15) // Default decay range
	clap.lfo2.SetRetrigger(false)

	return clap
}

// SetTune sets the filter frequency tuning in semitones (-12 to +12)
func (c *ClapDrum) SetTune(semitones float32) {
	c.tune = util.Clamp(semitones, -12.0, 12.0)
}

// SetDecay sets the decay time in seconds
func (c *ClapDrum) SetDecay(decay float32) {
	c.decay = util.Clamp(decay, 0.01, 0.3)
	c.env1.SetDecayTime(c.decay)
	c.env2.SetDecayTime(c.decay * 1.2)
	c.env3.SetDecayTime(c.decay * 1.4)
}

// SetTone sets the filter brightness (0.0 to 1.0)
func (c *ClapDrum) SetTone(tone float32) {
	c.tone = util.Clamp(tone, 0.0, 1.0)
}

// SetDrive sets the saturation/drive amount (0.0 to 1.0)
func (c *ClapDrum) SetDrive(drive float32) {
	c.drive = util.Clamp(drive, 0.0, 1.0)
}

// Trigger starts the clap drum
func (c *ClapDrum) Trigger(velocity float32) {
	c.velocity = util.Clamp(velocity, 0, 1)
	c.env2.Reset()
	c.env3.Reset()
	// Trigger first envelope immediately
	c.env1.Trigger()
	c.env1.SetValue(c.velocity)

	// Reset delay counter for staggered envelopes
	c.delayCount = 0

	c.active = true
}

// Process generates a buffer of audio samples
func (c *ClapDrum) Process(output []float32) {
	// Process LFOs (always run to keep them in sync)
	var lfo1Value, lfo2Value float32
	if c.lfo1Enabled {
		lfo1Value = c.lfo1.Process()
	}
	if c.lfo2Enabled {
		lfo2Value = c.lfo2.Process()
	}

	if !c.active && !c.env1.IsActive() {
		// Silent - clear buffer
		for i := range output {
			output[i] = 0.0
		}
		return
	}

	// Initialize modulated parameters with base values
	modTune := c.tune
	modDecay := c.decay
	modTone := c.tone
	modDrive := c.drive

	// Apply LFO1 modulation
	if c.lfo1Enabled {
		switch c.lfo1Target {
		case ClapLFOTargetTune:
			modTune = lfo1Value
		case ClapLFOTargetDecay:
			modDecay = lfo1Value
		case ClapLFOTargetTone:
			modTone = lfo1Value
		case ClapLFOTargetDrive:
			modDrive = lfo1Value
		}
	}

	// Apply LFO2 modulation
	if c.lfo2Enabled {
		switch c.lfo2Target {
		case ClapLFOTargetTune:
			modTune = lfo2Value
		case ClapLFOTargetDecay:
			modDecay = lfo2Value
		case ClapLFOTargetTone:
			modTone = lfo2Value
		case ClapLFOTargetDrive:
			modDrive = lfo2Value
		}
	}

	// Calculate filter frequency with tuning (using modulated tune)
	// Base frequency is 1200Hz, tune adjusts it
	tuneFactor := util.Exp2(modTune / 12.0)
	filterFreq := 1200.0 * tuneFactor

	// Update filter frequencies (using modulated tone)
	c.filter1.SetBandPass(filterFreq, 2.0+modTone*2.0) // Tone affects Q
	c.filter2.SetHighPass(filterFreq*0.67, 0.7)        // Brightness

	// Update envelope decay times with modulated decay
	c.env1.SetDecayTime(modDecay)
	c.env2.SetDecayTime(modDecay * 1.2)
	c.env3.SetDecayTime(modDecay * 1.4)

	for i := range output {
		// Handle delayed envelope triggers
		c.delayCount++
		if c.delayCount == c.delaySamples1 {
			// Trigger second slap
			c.env2.Trigger()
			c.env2.SetValue(0.8 * c.velocity) // Slightly quieter
		}
		if c.delayCount == c.delaySamples2 {
			// Trigger third slap
			c.env3.Trigger()
			c.env3.SetValue(0.6 * c.velocity) // Even quieter
		}

		// Process envelopes
		env1Val := c.env1.Process()
		env2Val := c.env2.Process()
		env3Val := c.env3.Process()

		// Generate white noise
		noise := c.noiseGen.NextWhite()

		// Filter the noise
		filtered := c.filter1.Process(noise)

		// Apply secondary filter for brightness (controlled by modulated tone)
		if modTone > 0.5 {
			brightMix := (modTone - 0.5) * 2.0 // 0 to 1 range
			bright := c.filter2.Process(filtered)
			filtered = filtered*(1.0-brightMix) + bright*brightMix
		}

		// Sum the three envelope layers for multi-slap effect
		envSum := env1Val + env2Val*0.7 + env3Val*0.5

		// Apply envelope to filtered noise
		sample := filtered * envSum

		// Apply drive/saturation (using modulated drive)
		if modDrive > 0.0 {
			// Soft clipping with drive amount
			driveAmount := 1.0 + modDrive*3.0
			sample = sample * driveAmount
			sample = util.FastTanh(sample)
		}

		output[i] = sample * 0.7 // Scale to prevent clipping
	}

	// Check if still active
	if !c.env1.IsActive() && !c.env2.IsActive() && !c.env3.IsActive() {
		c.active = false
	}
}

// IsActive returns true if the clap is producing sound
func (c *ClapDrum) IsActive() bool {
	return c.env1.IsActive() || c.env2.IsActive() || c.env3.IsActive()
}

// Reset resets the clap drum state
func (c *ClapDrum) Reset() {
	c.env1.Reset()
	c.env2.Reset()
	c.env3.Reset()
	c.filter1.Reset()
	c.filter2.Reset()
	c.delayCount = 0
	c.active = false
}

// Getter methods for UI

// GetTune returns the tuning in semitones
func (c *ClapDrum) GetTune() float32 {
	return c.tune
}

// GetDecay returns the decay time
func (c *ClapDrum) GetDecay() float32 {
	return c.decay
}

// GetTone returns the tone/brightness amount
func (c *ClapDrum) GetTone() float32 {
	return c.tone
}

// GetDrive returns the drive amount
func (c *ClapDrum) GetDrive() float32 {
	return c.drive
}

// GetCurrentTune returns the current LFO-modulated tune value
func (c *ClapDrum) GetCurrentTune() float32 {
	// Check if either LFO is modulating tune
	if c.lfo1Enabled && c.lfo1Target == ClapLFOTargetTune {
		return util.Clamp(c.lfo1.GetValue(), -12.0, 12.0)
	}
	if c.lfo2Enabled && c.lfo2Target == ClapLFOTargetTune {
		return util.Clamp(c.lfo2.GetValue(), -12.0, 12.0)
	}

	return c.tune
}

// GetCurrentDecay returns the current LFO-modulated decay value
func (c *ClapDrum) GetCurrentDecay() float32 {
	// Check if either LFO is modulating decay
	if c.lfo1Enabled && c.lfo1Target == ClapLFOTargetDecay {
		return util.Clamp(c.lfo1.GetValue(), 0.01, 0.3)
	}
	if c.lfo2Enabled && c.lfo2Target == ClapLFOTargetDecay {
		return util.Clamp(c.lfo2.GetValue(), 0.01, 0.3)
	}

	return c.decay
}

// GetCurrentTone returns the current LFO-modulated tone value
func (c *ClapDrum) GetCurrentTone() float32 {
	// Check if either LFO is modulating tone
	if c.lfo1Enabled && c.lfo1Target == ClapLFOTargetTone {
		return util.Clamp(c.lfo1.GetValue(), 0.0, 1.0)
	}
	if c.lfo2Enabled && c.lfo2Target == ClapLFOTargetTone {
		return util.Clamp(c.lfo2.GetValue(), 0.0, 1.0)
	}

	return c.tone
}

// GetCurrentDrive returns the current LFO-modulated drive value
func (c *ClapDrum) GetCurrentDrive() float32 {
	// Check if either LFO is modulating drive
	if c.lfo1Enabled && c.lfo1Target == ClapLFOTargetDrive {
		return util.Clamp(c.lfo1.GetValue(), 0.0, 1.0)
	}
	if c.lfo2Enabled && c.lfo2Target == ClapLFOTargetDrive {
		return util.Clamp(c.lfo2.GetValue(), 0.0, 1.0)
	}

	return c.drive
}

// LFO1 Methods

// SetLFO1Enabled enables/disables LFO1
func (c *ClapDrum) SetLFO1Enabled(enabled bool) {
	c.lfo1Enabled = enabled
}

// GetLFO1Enabled returns whether LFO1 is enabled
func (c *ClapDrum) GetLFO1Enabled() bool {
	return c.lfo1Enabled
}

// SetLFO1Target sets what parameter LFO1 modulates
func (c *ClapDrum) SetLFO1Target(target ClapLFOTarget) {
	c.lfo1Target = target
	// Update LFO range based on target
	switch target {
	case ClapLFOTargetTune:
		c.lfo1.SetRange(-6.0, 6.0) // ±6 semitones
	case ClapLFOTargetDecay:
		c.lfo1.SetRange(0.01, 0.2) // 10-200ms
	case ClapLFOTargetTone:
		c.lfo1.SetRange(0.2, 0.9) // 20-90%
	case ClapLFOTargetDrive:
		c.lfo1.SetRange(0.0, 0.8) // 0-80%
	}
}

// GetLFO1Target returns what parameter LFO1 modulates
func (c *ClapDrum) GetLFO1Target() ClapLFOTarget {
	return c.lfo1Target
}

// GetLFO1 returns the LFO1 instance
func (c *ClapDrum) GetLFO1() *dsp.LFO {
	return c.lfo1
}

// LFO2 Methods

// SetLFO2Enabled enables/disables LFO2
func (c *ClapDrum) SetLFO2Enabled(enabled bool) {
	c.lfo2Enabled = enabled
}

// GetLFO2Enabled returns whether LFO2 is enabled
func (c *ClapDrum) GetLFO2Enabled() bool {
	return c.lfo2Enabled
}

// SetLFO2Target sets what parameter LFO2 modulates
func (c *ClapDrum) SetLFO2Target(target ClapLFOTarget) {
	c.lfo2Target = target
	// Update LFO range based on target
	switch target {
	case ClapLFOTargetTune:
		c.lfo2.SetRange(-6.0, 6.0) // ±6 semitones
	case ClapLFOTargetDecay:
		c.lfo2.SetRange(0.01, 0.2) // 10-200ms
	case ClapLFOTargetTone:
		c.lfo2.SetRange(0.2, 0.9) // 20-90%
	case ClapLFOTargetDrive:
		c.lfo2.SetRange(0.0, 0.8) // 0-80%
	}
}

// GetLFO2Target returns what parameter LFO2 modulates
func (c *ClapDrum) GetLFO2Target() ClapLFOTarget {
	return c.lfo2Target
}

// GetLFO2 returns the LFO2 instance
func (c *ClapDrum) GetLFO2() *dsp.LFO {
	return c.lfo2
}

// UpdateLFOTempo updates the LFO rates based on BPM for tempo sync
func (c *ClapDrum) UpdateLFOTempo(bpm float32) {
	c.lfo1.UpdateTempoRate(bpm)
	c.lfo2.UpdateTempoRate(bpm)
}
