package drums

import (
	"math"

	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/util"
)

// SnareLFOTarget represents what parameter the LFO modulates
type SnareLFOTarget int

const (
	SnareLFOTargetNone SnareLFOTarget = iota
	SnareLFOTargetToneMix    // Modulate tone/noise balance (0.0-1.0)
	SnareLFOTargetSnap       // Modulate attack sharpness (0.0-1.0)
	SnareLFOTargetNoiseDecay // Modulate noise decay (0.05-0.5s)
	SnareLFOTargetToneDecay  // Modulate tone decay (0.01-0.3s)
)

// SnareDrum is an analog-modeled snare drum synthesizer
type SnareDrum struct {
	// Tonal component (two oscillators)
	phase1 float32
	phase2 float32
	freq1  float32 // ~180Hz
	freq2  float32 // ~330Hz

	// Noise component
	noiseGen    *dsp.NoiseGenerator
	noiseFilter *dsp.BiquadFilter

	// Envelopes
	toneEnv  *synth.DecayEnvelope
	noiseEnv *synth.DecayEnvelope

	// Parameters
	tune      float32 // -12 to +12 semitones
	snap      float32 // Attack sharpness (0.0 to 1.0)
	toneDecay float32 // Tone decay time
	noiseDecay float32 // Noise decay time
	toneMix   float32 // Tone/noise balance (0.0 = all noise, 1.0 = all tone)

	// LFO modulation
	lfo1        *dsp.LFO
	lfo2        *dsp.LFO
	lfo1Target  SnareLFOTarget
	lfo1Enabled bool
	lfo2Target  SnareLFOTarget
	lfo2Enabled bool

	// Internal
	sampleRate float32
	active     bool
}

// NewSnareDrum creates a new snare drum synthesizer
func NewSnareDrum(sampleRate float32) *SnareDrum {
	snare := &SnareDrum{
		freq1:       180.0,
		freq2:       330.0,
		noiseGen:    dsp.NewNoiseGenerator(),
		noiseFilter: dsp.NewBiquadFilter(sampleRate),
		toneEnv:     synth.NewDecayEnvelope(sampleRate),
		noiseEnv:    synth.NewDecayEnvelope(sampleRate),
		tune:        0.0,
		snap:        0.5,
		toneDecay:   0.06,  // 60ms
		noiseDecay:  0.15,  // 150ms
		toneMix:     0.4,   // 40% tone, 60% noise
		sampleRate:  sampleRate,
		active:      false,
	}

	// Configure noise filter (bandpass around 3kHz)
	snare.noiseFilter.SetBandPass(3000.0, 1.5)

	snare.toneEnv.SetDecayTime(snare.toneDecay)
	snare.noiseEnv.SetDecayTime(snare.noiseDecay)

	// Initialize LFOs
	snare.lfo1 = dsp.NewLFO(sampleRate)
	snare.lfo2 = dsp.NewLFO(sampleRate)
	snare.lfo1Target = SnareLFOTargetNone
	snare.lfo1Enabled = false
	snare.lfo2Target = SnareLFOTargetNone
	snare.lfo2Enabled = false

	// Configure LFO1 defaults
	snare.lfo1.SetWaveform(dsp.LFOSine)
	snare.lfo1.SetDepth(0.5)
	snare.lfo1.SetRange(0.2, 0.8) // Default tone mix range
	snare.lfo1.SetRetrigger(false)

	// Configure LFO2 defaults
	snare.lfo2.SetWaveform(dsp.LFOTriangle)
	snare.lfo2.SetDepth(0.3)
	snare.lfo2.SetRange(0.3, 0.7) // Default snap range
	snare.lfo2.SetRetrigger(false)

	return snare
}

// SetTune sets the tuning in semitones (-12 to +12)
func (s *SnareDrum) SetTune(semitones float32) {
	s.tune = util.Clamp(semitones, -12.0, 12.0)
}

// SetSnap sets the attack sharpness (0.0 to 1.0)
func (s *SnareDrum) SetSnap(snap float32) {
	s.snap = util.Clamp(snap, 0.0, 1.0)
}

// SetToneDecay sets the tone decay time in seconds
func (s *SnareDrum) SetToneDecay(decay float32) {
	s.toneDecay = util.Clamp(decay, 0.01, 0.3)
	s.toneEnv.SetDecayTime(s.toneDecay)
}

// SetNoiseDecay sets the noise decay time in seconds
func (s *SnareDrum) SetNoiseDecay(decay float32) {
	s.noiseDecay = util.Clamp(decay, 0.05, 0.5)
	s.noiseEnv.SetDecayTime(s.noiseDecay)
}

// SetToneMix sets the tone/noise balance (0.0 = all noise, 1.0 = all tone)
func (s *SnareDrum) SetToneMix(mix float32) {
	s.toneMix = util.Clamp(mix, 0.0, 1.0)
}

// Trigger starts the snare drum
func (s *SnareDrum) Trigger(velocity float32) {
	// Reset phases
	s.phase1 = 0.0
	s.phase2 = 0.0

	// Trigger envelopes with snap affecting attack
	s.toneEnv.Trigger()
	s.toneEnv.SetValue(velocity * (1.0 + s.snap*0.5))
	s.noiseEnv.Trigger()
	s.noiseEnv.SetValue(velocity * (1.0 + s.snap*0.3))

	s.active = true
}

// Process generates a buffer of audio samples
func (s *SnareDrum) Process(output []float32) {
	// Process LFOs (always run to keep them in sync)
	var lfo1Value, lfo2Value float32
	if s.lfo1Enabled {
		lfo1Value = s.lfo1.Process()
	}
	if s.lfo2Enabled {
		lfo2Value = s.lfo2.Process()
	}

	if !s.active && !s.noiseEnv.IsActive() {
		// Silent - clear buffer
		for i := range output {
			output[i] = 0.0
		}
		return
	}

	// Apply tuning
	tuneFactor := float32(math.Pow(2.0, float64(s.tune/12.0)))
	freq1 := s.freq1 * tuneFactor
	freq2 := s.freq2 * tuneFactor

	for i := range output {

		// Initialize modulated parameters with base values
		modToneMix := s.toneMix

		// Apply LFO1 modulation
		if s.lfo1Enabled {
			switch s.lfo1Target {
			case SnareLFOTargetToneMix:
				modToneMix = lfo1Value
			// Note: Snap and decay modulation not implemented (requires envelope changes)
			}
		}

		// Apply LFO2 modulation (can override LFO1 if targeting same parameter)
		if s.lfo2Enabled {
			switch s.lfo2Target {
			case SnareLFOTargetToneMix:
				modToneMix = lfo2Value
			// Note: Snap and decay modulation not implemented (requires envelope changes)
			}
		}

		// Get envelope values
		toneEnvValue := s.toneEnv.Process()
		noiseEnvValue := s.noiseEnv.Process()

		// Generate tonal component (two sine waves)
		sine1 := float32(math.Sin(float64(s.phase1) * 2.0 * math.Pi))
		sine2 := float32(math.Sin(float64(s.phase2) * 2.0 * math.Pi))
		tone := (sine1 + sine2) * 0.5 * toneEnvValue

		// Generate noise component
		noise := s.noiseGen.NextWhite()
		noise = s.noiseFilter.Process(noise)
		noise = noise * noiseEnvValue

		// Mix tone and noise (using modulated tone mix)
		sample := tone*modToneMix + noise*(1.0-modToneMix)

		output[i] = sample * 0.6 // Scale to prevent clipping

		// Advance phases
		s.phase1 += freq1 / s.sampleRate
		s.phase2 += freq2 / s.sampleRate
		if s.phase1 >= 1.0 {
			s.phase1 -= 1.0
		}
		if s.phase2 >= 1.0 {
			s.phase2 -= 1.0
		}
	}

	// Check if still active
	if !s.noiseEnv.IsActive() {
		s.active = false
	}
}

// IsActive returns true if the snare is producing sound
func (s *SnareDrum) IsActive() bool {
	return s.noiseEnv.IsActive()
}

// Reset resets the snare drum state
func (s *SnareDrum) Reset() {
	s.phase1 = 0.0
	s.phase2 = 0.0
	s.toneEnv.Reset()
	s.noiseEnv.Reset()
	s.noiseFilter.Reset()
	s.active = false
}

// Getter methods for UI

// GetTune returns the tuning in semitones
func (s *SnareDrum) GetTune() float32 {
	return s.tune
}

// GetSnap returns the snap amount
func (s *SnareDrum) GetSnap() float32 {
	return s.snap
}

// GetToneDecay returns the tone decay time
func (s *SnareDrum) GetToneDecay() float32 {
	return s.toneDecay
}

// GetNoiseDecay returns the noise decay time
func (s *SnareDrum) GetNoiseDecay() float32 {
	return s.noiseDecay
}

// GetToneMix returns the tone/noise balance
func (s *SnareDrum) GetToneMix() float32 {
	return s.toneMix
}

// GetCurrentToneMix returns the current LFO-modulated tone mix value
func (s *SnareDrum) GetCurrentToneMix() float32 {
	// Check if either LFO is modulating tone mix
	if s.lfo1Enabled && s.lfo1Target == SnareLFOTargetToneMix {
		return util.Clamp(s.lfo1.GetValue(), 0.0, 1.0)
	}
	if s.lfo2Enabled && s.lfo2Target == SnareLFOTargetToneMix {
		return util.Clamp(s.lfo2.GetValue(), 0.0, 1.0)
	}

	return s.toneMix
}

// GetCurrentSnap returns the current LFO-modulated snap value
func (s *SnareDrum) GetCurrentSnap() float32 {
	// Check if either LFO is modulating snap
	if s.lfo1Enabled && s.lfo1Target == SnareLFOTargetSnap {
		return util.Clamp(s.lfo1.GetValue(), 0.0, 1.0)
	}
	if s.lfo2Enabled && s.lfo2Target == SnareLFOTargetSnap {
		return util.Clamp(s.lfo2.GetValue(), 0.0, 1.0)
	}

	return s.snap
}

// GetCurrentNoiseDecay returns the current LFO-modulated noise decay value
func (s *SnareDrum) GetCurrentNoiseDecay() float32 {
	// Check if either LFO is modulating noise decay
	if s.lfo1Enabled && s.lfo1Target == SnareLFOTargetNoiseDecay {
		return util.Clamp(s.lfo1.GetValue(), 0.05, 0.5)
	}
	if s.lfo2Enabled && s.lfo2Target == SnareLFOTargetNoiseDecay {
		return util.Clamp(s.lfo2.GetValue(), 0.05, 0.5)
	}

	return s.noiseDecay
}

// GetCurrentToneDecay returns the current LFO-modulated tone decay value
func (s *SnareDrum) GetCurrentToneDecay() float32 {
	// Check if either LFO is modulating tone decay
	if s.lfo1Enabled && s.lfo1Target == SnareLFOTargetToneDecay {
		return util.Clamp(s.lfo1.GetValue(), 0.01, 0.3)
	}
	if s.lfo2Enabled && s.lfo2Target == SnareLFOTargetToneDecay {
		return util.Clamp(s.lfo2.GetValue(), 0.01, 0.3)
	}

	return s.toneDecay
}

// LFO control methods

// GetLFO1 returns the LFO1 instance
func (s *SnareDrum) GetLFO1() *dsp.LFO {
	return s.lfo1
}

// GetLFO2 returns the LFO2 instance
func (s *SnareDrum) GetLFO2() *dsp.LFO {
	return s.lfo2
}

// SetLFO1Enabled enables or disables LFO1
func (s *SnareDrum) SetLFO1Enabled(enabled bool) {
	s.lfo1Enabled = enabled
}

// GetLFO1Enabled returns whether LFO1 is enabled
func (s *SnareDrum) GetLFO1Enabled() bool {
	return s.lfo1Enabled
}

// SetLFO2Enabled enables or disables LFO2
func (s *SnareDrum) SetLFO2Enabled(enabled bool) {
	s.lfo2Enabled = enabled
}

// GetLFO2Enabled returns whether LFO2 is enabled
func (s *SnareDrum) GetLFO2Enabled() bool {
	return s.lfo2Enabled
}

// SetLFO1Target sets the LFO1 modulation target and updates range
func (s *SnareDrum) SetLFO1Target(target SnareLFOTarget) {
	s.lfo1Target = target
	// Update LFO range based on target
	switch target {
	case SnareLFOTargetToneMix:
		s.lfo1.SetRange(0.0, 1.0) // Full tone/noise range
	case SnareLFOTargetSnap:
		s.lfo1.SetRange(0.0, 1.0) // Full snap range
	case SnareLFOTargetNoiseDecay:
		s.lfo1.SetRange(0.05, 0.5) // Noise decay range
	case SnareLFOTargetToneDecay:
		s.lfo1.SetRange(0.01, 0.3) // Tone decay range
	}
}

// GetLFO1Target returns the LFO1 modulation target
func (s *SnareDrum) GetLFO1Target() SnareLFOTarget {
	return s.lfo1Target
}

// SetLFO2Target sets the LFO2 modulation target and updates range
func (s *SnareDrum) SetLFO2Target(target SnareLFOTarget) {
	s.lfo2Target = target
	// Update LFO range based on target
	switch target {
	case SnareLFOTargetToneMix:
		s.lfo2.SetRange(0.0, 1.0)
	case SnareLFOTargetSnap:
		s.lfo2.SetRange(0.0, 1.0)
	case SnareLFOTargetNoiseDecay:
		s.lfo2.SetRange(0.05, 0.5)
	case SnareLFOTargetToneDecay:
		s.lfo2.SetRange(0.01, 0.3)
	}
}

// GetLFO2Target returns the LFO2 modulation target
func (s *SnareDrum) GetLFO2Target() SnareLFOTarget {
	return s.lfo2Target
}

// UpdateLFOTempo updates the LFO rates based on BPM for tempo sync
func (s *SnareDrum) UpdateLFOTempo(bpm float32) {
	s.lfo1.UpdateTempoRate(bpm)
	s.lfo2.UpdateTempoRate(bpm)
}
