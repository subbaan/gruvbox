package drums

import (
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/util"
)

// HiHatLFOTarget represents what parameter the LFO modulates
type HiHatLFOTarget int

const (
	HiHatLFOTargetNone HiHatLFOTarget = iota
	HiHatLFOTargetTone     // Modulate tone/brightness (0.0-1.0)
	HiHatLFOTargetNoiseMix // Modulate noise amount (0.0-1.0)
	HiHatLFOTargetDecay    // Modulate decay time (0.01-1.0s)
)

// HiHat is an analog-modeled hi-hat synthesizer
// Can be configured for closed or open hi-hat sounds
type HiHat struct {
	// Six non-harmonic square wave oscillators for metallic timbre
	phases [6]float32
	freqs  [6]float32

	// Noise component (for extra metallic character)
	noiseGen    *dsp.NoiseGenerator
	noiseFilter *dsp.BiquadFilter

	// Envelope
	env *synth.DecayEnvelope

	// Parameters
	decay    float32 // Decay time (closed: ~30ms, open: ~200ms)
	tone     float32 // Tone control (affects filter cutoff)
	noiseMix float32 // Noise amount

	// LFO modulation
	lfo1        *dsp.LFO
	lfo2        *dsp.LFO
	lfo1Target  HiHatLFOTarget
	lfo1Enabled bool
	lfo2Target  HiHatLFOTarget
	lfo2Enabled bool

	// Internal
	sampleRate float32
	active     bool
}

// NewHiHatClosed creates a new closed hi-hat
func NewHiHatClosed(sampleRate float32) *HiHat {
	return newHiHat(sampleRate, 0.04) // 40ms decay
}

// NewHiHatOpen creates a new open hi-hat
func NewHiHatOpen(sampleRate float32) *HiHat {
	return newHiHat(sampleRate, 0.25) // 250ms decay
}

// newHiHat creates a new hi-hat with specified decay
func newHiHat(sampleRate float32, decay float32) *HiHat {
	hihat := &HiHat{
		// Non-harmonic ratios for metallic sound
		freqs: [6]float32{
			325.0,
			543.0,
			811.0,
			1217.0,
			1829.0,
			2744.0,
		},
		noiseGen:    dsp.NewNoiseGenerator(),
		noiseFilter: dsp.NewBiquadFilter(sampleRate),
		env:         synth.NewDecayEnvelope(sampleRate),
		decay:       decay,
		tone:        0.5,
		noiseMix:    0.3,
		sampleRate:  sampleRate,
		active:      false,
	}

	// Configure high-pass filter for noise (keep only high frequencies)
	hihat.noiseFilter.SetHighPass(8000.0, 0.707)
	hihat.env.SetDecayTime(hihat.decay)

	// Initialize LFOs
	hihat.lfo1 = dsp.NewLFO(sampleRate)
	hihat.lfo2 = dsp.NewLFO(sampleRate)
	hihat.lfo1Target = HiHatLFOTargetNone
	hihat.lfo1Enabled = false
	hihat.lfo2Target = HiHatLFOTargetNone
	hihat.lfo2Enabled = false

	// Configure LFO1 defaults
	hihat.lfo1.SetWaveform(dsp.LFOSine)
	hihat.lfo1.SetDepth(0.5)
	hihat.lfo1.SetRange(0.3, 0.7) // Default tone range
	hihat.lfo1.SetRetrigger(false)

	// Configure LFO2 defaults
	hihat.lfo2.SetWaveform(dsp.LFOTriangle)
	hihat.lfo2.SetDepth(0.3)
	hihat.lfo2.SetRange(0.2, 0.6) // Default noise mix range
	hihat.lfo2.SetRetrigger(false)

	return hihat
}

// SetDecay sets the decay time in seconds
func (h *HiHat) SetDecay(decay float32) {
	h.decay = util.Clamp(decay, 0.01, 1.0)
	h.env.SetDecayTime(h.decay)
}

// SetTone sets the tone control (0.0 = dark, 1.0 = bright)
func (h *HiHat) SetTone(tone float32) {
	h.tone = util.Clamp(tone, 0.0, 1.0)
}

// SetNoiseMix sets the noise amount (0.0 to 1.0)
func (h *HiHat) SetNoiseMix(mix float32) {
	h.noiseMix = util.Clamp(mix, 0.0, 1.0)
}

// Trigger starts the hi-hat
func (h *HiHat) Trigger(velocity float32) {
	// Reset all oscillator phases to random values for varied timbre
	for i := range h.phases {
		h.phases[i] = float32(i) * 0.123 // Pseudo-random offsets
	}

	// Trigger envelope
	h.env.Trigger()
	h.env.SetValue(velocity)

	h.active = true
}

// Process generates a buffer of audio samples
func (h *HiHat) Process(output []float32) {
	if !h.active && !h.env.IsActive() {
		// Silent - clear buffer
		for i := range output {
			output[i] = 0.0
		}
		return
	}

	for i := range output {
		// Process LFOs
		var lfo1Value, lfo2Value float32
		if h.lfo1Enabled {
			lfo1Value = h.lfo1.Process()
		}
		if h.lfo2Enabled {
			lfo2Value = h.lfo2.Process()
		}

		// Initialize modulated parameters with base values
		modTone := h.tone
		modNoiseMix := h.noiseMix

		// Apply LFO1 modulation
		if h.lfo1Enabled {
			switch h.lfo1Target {
			case HiHatLFOTargetTone:
				modTone = lfo1Value
			case HiHatLFOTargetNoiseMix:
				modNoiseMix = lfo1Value
			// Note: Decay modulation not implemented (requires envelope changes)
			}
		}

		// Apply LFO2 modulation (can override LFO1 if targeting same parameter)
		if h.lfo2Enabled {
			switch h.lfo2Target {
			case HiHatLFOTargetTone:
				modTone = lfo2Value
			case HiHatLFOTargetNoiseMix:
				modNoiseMix = lfo2Value
			// Note: Decay modulation not implemented (requires envelope changes)
			}
		}

		// Get envelope value
		envValue := h.env.Process()

		// Generate six square waves at non-harmonic frequencies
		metallic := float32(0.0)
		for j := 0; j < 6; j++ {
			// Square wave (with varying amplitudes for each oscillator)
			amplitude := 1.0 / float32(j+1) // Higher frequencies quieter
			if h.phases[j] < 0.5 {
				metallic += amplitude
			} else {
				metallic -= amplitude
			}

			// Advance phase
			h.phases[j] += h.freqs[j] / h.sampleRate
			if h.phases[j] >= 1.0 {
				h.phases[j] -= 1.0
			}
		}
		metallic /= 6.0 // Normalize

		// Apply tone control (using modulated tone value)
		metallic *= (0.3 + modTone*0.7)

		// Generate noise component
		noise := h.noiseGen.NextWhite()
		noise = h.noiseFilter.Process(noise)

		// Mix metallic sound with noise (using modulated noise mix)
		sample := metallic*(1.0-modNoiseMix) + noise*modNoiseMix

		// Apply envelope
		output[i] = sample * envValue * 0.4 // Scale to prevent clipping
	}

	// Check if still active
	if !h.env.IsActive() {
		h.active = false
	}
}

// IsActive returns true if the hi-hat is producing sound
func (h *HiHat) IsActive() bool {
	return h.env.IsActive()
}

// Reset resets the hi-hat state
func (h *HiHat) Reset() {
	for i := range h.phases {
		h.phases[i] = 0.0
	}
	h.env.Reset()
	h.noiseFilter.Reset()
	h.active = false
}

// Getter methods for UI

// GetDecay returns the decay time
func (h *HiHat) GetDecay() float32 {
	return h.decay
}

// GetTone returns the tone control value
func (h *HiHat) GetTone() float32 {
	return h.tone
}

// GetNoiseMix returns the noise mix amount
func (h *HiHat) GetNoiseMix() float32 {
	return h.noiseMix
}

// GetCurrentTone returns the current LFO-modulated tone value
func (h *HiHat) GetCurrentTone() float32 {
	// Check if either LFO is modulating tone
	if h.lfo1Enabled && h.lfo1Target == HiHatLFOTargetTone {
		return util.Clamp(h.lfo1.GetValue(), 0.0, 1.0)
	}
	if h.lfo2Enabled && h.lfo2Target == HiHatLFOTargetTone {
		return util.Clamp(h.lfo2.GetValue(), 0.0, 1.0)
	}

	return h.tone
}

// GetCurrentNoiseMix returns the current LFO-modulated noise mix value
func (h *HiHat) GetCurrentNoiseMix() float32 {
	// Check if either LFO is modulating noise mix
	if h.lfo1Enabled && h.lfo1Target == HiHatLFOTargetNoiseMix {
		return util.Clamp(h.lfo1.GetValue(), 0.0, 1.0)
	}
	if h.lfo2Enabled && h.lfo2Target == HiHatLFOTargetNoiseMix {
		return util.Clamp(h.lfo2.GetValue(), 0.0, 1.0)
	}

	return h.noiseMix
}

// GetCurrentDecay returns the current LFO-modulated decay value
func (h *HiHat) GetCurrentDecay() float32 {
	// Check if either LFO is modulating decay
	if h.lfo1Enabled && h.lfo1Target == HiHatLFOTargetDecay {
		return util.Clamp(h.lfo1.GetValue(), 0.01, 1.0)
	}
	if h.lfo2Enabled && h.lfo2Target == HiHatLFOTargetDecay {
		return util.Clamp(h.lfo2.GetValue(), 0.01, 1.0)
	}

	return h.decay
}

// LFO control methods

// GetLFO1 returns the LFO1 instance
func (h *HiHat) GetLFO1() *dsp.LFO {
	return h.lfo1
}

// GetLFO2 returns the LFO2 instance
func (h *HiHat) GetLFO2() *dsp.LFO {
	return h.lfo2
}

// SetLFO1Enabled enables or disables LFO1
func (h *HiHat) SetLFO1Enabled(enabled bool) {
	h.lfo1Enabled = enabled
}

// GetLFO1Enabled returns whether LFO1 is enabled
func (h *HiHat) GetLFO1Enabled() bool {
	return h.lfo1Enabled
}

// SetLFO2Enabled enables or disables LFO2
func (h *HiHat) SetLFO2Enabled(enabled bool) {
	h.lfo2Enabled = enabled
}

// GetLFO2Enabled returns whether LFO2 is enabled
func (h *HiHat) GetLFO2Enabled() bool {
	return h.lfo2Enabled
}

// SetLFO1Target sets the LFO1 modulation target and updates range
func (h *HiHat) SetLFO1Target(target HiHatLFOTarget) {
	h.lfo1Target = target
	// Update LFO range based on target
	switch target {
	case HiHatLFOTargetTone:
		h.lfo1.SetRange(0.0, 1.0) // Full tone range
	case HiHatLFOTargetNoiseMix:
		h.lfo1.SetRange(0.0, 1.0) // Full noise mix range
	case HiHatLFOTargetDecay:
		h.lfo1.SetRange(0.01, 1.0) // Full decay range
	}
}

// GetLFO1Target returns the LFO1 modulation target
func (h *HiHat) GetLFO1Target() HiHatLFOTarget {
	return h.lfo1Target
}

// SetLFO2Target sets the LFO2 modulation target and updates range
func (h *HiHat) SetLFO2Target(target HiHatLFOTarget) {
	h.lfo2Target = target
	// Update LFO range based on target
	switch target {
	case HiHatLFOTargetTone:
		h.lfo2.SetRange(0.0, 1.0)
	case HiHatLFOTargetNoiseMix:
		h.lfo2.SetRange(0.0, 1.0)
	case HiHatLFOTargetDecay:
		h.lfo2.SetRange(0.01, 1.0)
	}
}

// GetLFO2Target returns the LFO2 modulation target
func (h *HiHat) GetLFO2Target() HiHatLFOTarget {
	return h.lfo2Target
}

// UpdateLFOTempo updates the LFO rates based on BPM for tempo sync
func (h *HiHat) UpdateLFOTempo(bpm float32) {
	h.lfo1.UpdateTempoRate(bpm)
	h.lfo2.UpdateTempoRate(bpm)
}
