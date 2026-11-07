package drums

import (
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/util"
)

// CrashLFOTarget represents what parameter the LFO modulates
type CrashLFOTarget int

const (
	CrashLFOTargetNone     CrashLFOTarget = iota
	CrashLFOTargetDecay                   // Modulate decay time (0.3-3.0s)
	CrashLFOTargetTone                    // Modulate brightness (0.0-1.0)
	CrashLFOTargetMetallic                // Modulate metallic character (0.0-1.0)
)

// CrashDrum is an analog-modeled crash cymbal synthesizer
// Uses filtered metallic noise with long decay for realistic crash sound
type CrashDrum struct {
	// Noise generator
	noiseGen *dsp.NoiseGenerator

	// Multiple filters for metallic character
	filterHP1 *dsp.BiquadFilter // High-pass to remove low rumble
	filterHP2 *dsp.BiquadFilter // Additional high-pass
	filterBP1 *dsp.BiquadFilter // Band-pass for metallic character 1
	filterBP2 *dsp.BiquadFilter // Band-pass for metallic character 2
	filterBP3 *dsp.BiquadFilter // Band-pass for metallic character 3

	// Envelope
	env *synth.DecayEnvelope

	// Parameters
	decay    float32 // Decay time (0.3 to 3.0 seconds)
	tone     float32 // Brightness (0.0 to 1.0)
	metallic float32 // Metallic character (0.0 to 1.0)

	// LFO modulation
	lfo1        *dsp.LFO
	lfo2        *dsp.LFO
	lfo1Target  CrashLFOTarget
	lfo1Enabled bool
	lfo2Target  CrashLFOTarget
	lfo2Enabled bool

	// Internal
	sampleRate float32
	active     bool
}

// NewCrashDrum creates a new crash cymbal synthesizer
func NewCrashDrum(sampleRate float32) *CrashDrum {
	crash := &CrashDrum{
		noiseGen:   dsp.NewNoiseGenerator(),
		filterHP1:  dsp.NewBiquadFilter(sampleRate),
		filterHP2:  dsp.NewBiquadFilter(sampleRate),
		filterBP1:  dsp.NewBiquadFilter(sampleRate),
		filterBP2:  dsp.NewBiquadFilter(sampleRate),
		filterBP3:  dsp.NewBiquadFilter(sampleRate),
		env:        synth.NewDecayEnvelope(sampleRate),
		decay:      1.0, // 1 second default
		tone:       0.6, // Mid-bright
		metallic:   0.7, // Fairly metallic
		sampleRate: sampleRate,
		active:     false,
	}

	// Configure filters for metallic crash sound
	crash.filterHP1.SetHighPass(3000.0, 0.7) // Remove low frequencies
	crash.filterHP2.SetHighPass(5000.0, 0.5) // Additional high-pass

	// Inharmonic frequencies for metallic character
	crash.filterBP1.SetBandPass(5400.0, 2.0)
	crash.filterBP2.SetBandPass(7300.0, 2.5)
	crash.filterBP3.SetBandPass(9100.0, 3.0)

	// Set envelope time
	crash.env.SetDecayTime(crash.decay)

	// Initialize LFOs
	crash.lfo1 = dsp.NewLFO(sampleRate)
	crash.lfo2 = dsp.NewLFO(sampleRate)
	crash.lfo1Target = CrashLFOTargetNone
	crash.lfo1Enabled = false
	crash.lfo2Target = CrashLFOTargetNone
	crash.lfo2Enabled = false

	// Configure LFO1 defaults
	crash.lfo1.SetWaveform(dsp.LFOSine)
	crash.lfo1.SetDepth(0.5)
	crash.lfo1.SetRange(0.5, 2.0) // Default decay range
	crash.lfo1.SetRetrigger(false)

	// Configure LFO2 defaults
	crash.lfo2.SetWaveform(dsp.LFOTriangle)
	crash.lfo2.SetDepth(0.3)
	crash.lfo2.SetRange(0.3, 0.9) // Default tone range
	crash.lfo2.SetRetrigger(false)

	return crash
}

// SetDecay sets the decay time in seconds
func (c *CrashDrum) SetDecay(decay float32) {
	c.decay = util.Clamp(decay, 0.3, 3.0)
	c.env.SetDecayTime(c.decay)
}

// SetTone sets the brightness (0.0 to 1.0)
func (c *CrashDrum) SetTone(tone float32) {
	c.tone = util.Clamp(tone, 0.0, 1.0)
}

// SetMetallic sets the metallic character (0.0 to 1.0)
func (c *CrashDrum) SetMetallic(metallic float32) {
	c.metallic = util.Clamp(metallic, 0.0, 1.0)
}

// Trigger starts the crash cymbal
func (c *CrashDrum) Trigger(velocity float32) {
	// Trigger envelope
	c.env.Trigger()
	c.env.SetValue(velocity)

	c.active = true
}

// Process generates a buffer of audio samples
func (c *CrashDrum) Process(output []float32) {
	// Process LFOs (always run to keep them in sync)
	var lfo1Value, lfo2Value float32
	if c.lfo1Enabled {
		lfo1Value = c.lfo1.Process()
	}
	if c.lfo2Enabled {
		lfo2Value = c.lfo2.Process()
	}

	if !c.active && !c.env.IsActive() {
		// Silent - clear buffer
		for i := range output {
			output[i] = 0.0
		}
		return
	}

	// Initialize modulated parameters with base values
	modDecay := c.decay
	modTone := c.tone
	modMetallic := c.metallic

	// Apply LFO1 modulation
	if c.lfo1Enabled {
		switch c.lfo1Target {
		case CrashLFOTargetDecay:
			modDecay = lfo1Value
		case CrashLFOTargetTone:
			modTone = lfo1Value
		case CrashLFOTargetMetallic:
			modMetallic = lfo1Value
		}
	}

	// Apply LFO2 modulation
	if c.lfo2Enabled {
		switch c.lfo2Target {
		case CrashLFOTargetDecay:
			modDecay = lfo2Value
		case CrashLFOTargetTone:
			modTone = lfo2Value
		case CrashLFOTargetMetallic:
			modMetallic = lfo2Value
		}
	}

	// Update envelope decay time with modulated decay
	c.env.SetDecayTime(modDecay)

	// Update filter frequencies based on modulated tone
	hpFreq1 := 3000.0 + modTone*2000.0 // 3-5kHz
	hpFreq2 := 5000.0 + modTone*3000.0 // 5-8kHz
	bp1Freq := 5400.0 + modTone*2000.0 // 5.4-7.4kHz
	bp2Freq := 7300.0 + modTone*2700.0 // 7.3-10kHz
	bp3Freq := 9100.0 + modTone*3400.0 // 9.1-12.5kHz

	c.filterHP1.SetHighPass(hpFreq1, 0.7)
	c.filterHP2.SetHighPass(hpFreq2, 0.5)
	c.filterBP1.SetBandPass(bp1Freq, 2.0+modMetallic*2.0)
	c.filterBP2.SetBandPass(bp2Freq, 2.5+modMetallic*2.0)
	c.filterBP3.SetBandPass(bp3Freq, 3.0+modMetallic*2.0)

	for i := range output {
		// Process envelope
		envVal := c.env.Process()

		// Generate white noise
		noise := c.noiseGen.NextWhite()

		// Apply high-pass filters
		sample := c.filterHP1.Process(noise)
		sample = c.filterHP2.Process(sample)

		// Apply band-pass filters for metallic character
		bp1 := c.filterBP1.Process(sample)
		bp2 := c.filterBP2.Process(sample)
		bp3 := c.filterBP3.Process(sample)

		// Mix filters based on metallic parameter
		metalSample := (bp1 + bp2 + bp3) / 3.0
		sample = sample*(1.0-modMetallic) + metalSample*modMetallic

		// Apply envelope
		sample = sample * envVal

		// Soft clipping to prevent harshness
		sample = util.FastTanh(sample * 1.2)

		output[i] = sample * 0.5 // Scale to prevent clipping
	}

	// Check if still active
	if !c.env.IsActive() {
		c.active = false
	}
}

// IsActive returns true if the crash is producing sound
func (c *CrashDrum) IsActive() bool {
	return c.env.IsActive()
}

// Reset resets the crash cymbal state
func (c *CrashDrum) Reset() {
	c.env.Reset()
	c.filterHP1.Reset()
	c.filterHP2.Reset()
	c.filterBP1.Reset()
	c.filterBP2.Reset()
	c.filterBP3.Reset()
	c.active = false
}

// Getter methods for UI

// GetDecay returns the decay time
func (c *CrashDrum) GetDecay() float32 {
	return c.decay
}

// GetTone returns the tone/brightness
func (c *CrashDrum) GetTone() float32 {
	return c.tone
}

// GetMetallic returns the metallic character
func (c *CrashDrum) GetMetallic() float32 {
	return c.metallic
}

// GetCurrentDecay returns the current LFO-modulated decay value
func (c *CrashDrum) GetCurrentDecay() float32 {
	// Check if either LFO is modulating decay
	if c.lfo1Enabled && c.lfo1Target == CrashLFOTargetDecay {
		return util.Clamp(c.lfo1.GetValue(), 0.3, 3.0)
	}
	if c.lfo2Enabled && c.lfo2Target == CrashLFOTargetDecay {
		return util.Clamp(c.lfo2.GetValue(), 0.3, 3.0)
	}

	return c.decay
}

// GetCurrentTone returns the current LFO-modulated tone value
func (c *CrashDrum) GetCurrentTone() float32 {
	// Check if either LFO is modulating tone
	if c.lfo1Enabled && c.lfo1Target == CrashLFOTargetTone {
		return util.Clamp(c.lfo1.GetValue(), 0.0, 1.0)
	}
	if c.lfo2Enabled && c.lfo2Target == CrashLFOTargetTone {
		return util.Clamp(c.lfo2.GetValue(), 0.0, 1.0)
	}

	return c.tone
}

// GetCurrentMetallic returns the current LFO-modulated metallic value
func (c *CrashDrum) GetCurrentMetallic() float32 {
	// Check if either LFO is modulating metallic
	if c.lfo1Enabled && c.lfo1Target == CrashLFOTargetMetallic {
		return util.Clamp(c.lfo1.GetValue(), 0.0, 1.0)
	}
	if c.lfo2Enabled && c.lfo2Target == CrashLFOTargetMetallic {
		return util.Clamp(c.lfo2.GetValue(), 0.0, 1.0)
	}

	return c.metallic
}

// LFO1 Methods

// SetLFO1Enabled enables/disables LFO1
func (c *CrashDrum) SetLFO1Enabled(enabled bool) {
	c.lfo1Enabled = enabled
}

// GetLFO1Enabled returns whether LFO1 is enabled
func (c *CrashDrum) GetLFO1Enabled() bool {
	return c.lfo1Enabled
}

// SetLFO1Target sets what parameter LFO1 modulates
func (c *CrashDrum) SetLFO1Target(target CrashLFOTarget) {
	c.lfo1Target = target
	// Update LFO range based on target
	switch target {
	case CrashLFOTargetDecay:
		c.lfo1.SetRange(0.3, 3.0) // 0.3-3.0s
	case CrashLFOTargetTone:
		c.lfo1.SetRange(0.0, 1.0) // 0-100%
	case CrashLFOTargetMetallic:
		c.lfo1.SetRange(0.0, 1.0) // 0-100%
	}
}

// GetLFO1Target returns what parameter LFO1 modulates
func (c *CrashDrum) GetLFO1Target() CrashLFOTarget {
	return c.lfo1Target
}

// GetLFO1 returns the LFO1 instance
func (c *CrashDrum) GetLFO1() *dsp.LFO {
	return c.lfo1
}

// LFO2 Methods

// SetLFO2Enabled enables/disables LFO2
func (c *CrashDrum) SetLFO2Enabled(enabled bool) {
	c.lfo2Enabled = enabled
}

// GetLFO2Enabled returns whether LFO2 is enabled
func (c *CrashDrum) GetLFO2Enabled() bool {
	return c.lfo2Enabled
}

// SetLFO2Target sets what parameter LFO2 modulates
func (c *CrashDrum) SetLFO2Target(target CrashLFOTarget) {
	c.lfo2Target = target
	// Update LFO range based on target
	switch target {
	case CrashLFOTargetDecay:
		c.lfo2.SetRange(0.3, 3.0) // 0.3-3.0s
	case CrashLFOTargetTone:
		c.lfo2.SetRange(0.0, 1.0) // 0-100%
	case CrashLFOTargetMetallic:
		c.lfo2.SetRange(0.0, 1.0) // 0-100%
	}
}

// GetLFO2Target returns what parameter LFO2 modulates
func (c *CrashDrum) GetLFO2Target() CrashLFOTarget {
	return c.lfo2Target
}

// GetLFO2 returns the LFO2 instance
func (c *CrashDrum) GetLFO2() *dsp.LFO {
	return c.lfo2
}

// UpdateLFOTempo updates the LFO rates based on BPM for tempo sync
func (c *CrashDrum) UpdateLFOTempo(bpm float32) {
	c.lfo1.UpdateTempoRate(bpm)
	c.lfo2.UpdateTempoRate(bpm)
}
