package dsp

import (
	"math"

	"github.com/subbaan/gruvbox/util"
)

// LadderFilter implements a 4-pole (24dB/oct) transistor ladder filter
// Based on the topology-preserving transform (TPT) method by Vadim Zavalishin
// This is the foundation for TB-303 diode ladder emulation
type LadderFilter struct {
	// State variables for 4 stages
	s1, s2, s3, s4 float32

	// Filter parameters
	cutoff     float32 // Normalized frequency 0.0 to 1.0
	resonance  float32 // 0.0 to 1.0 (TB-303 never reaches full resonance)
	sampleRate float32

	// Pre-calculated coefficients (TPT method)
	g  float32 // Filter coefficient
	g0 float32 // Input gain
	k  float32 // Resonance coefficient

	// Saturation/non-linearity
	saturation float32 // Amount of saturation (0.0 to 1.0)
}

// NewLadderFilter creates a new ladder filter
func NewLadderFilter(sampleRate float32) *LadderFilter {
	filter := &LadderFilter{
		sampleRate: sampleRate,
		cutoff:     0.5,
		resonance:  0.0,
		saturation: 0.3,
	}
	filter.updateCoefficients()
	return filter
}

// SetCutoff sets the filter cutoff frequency (0.0 to 1.0)
// 0.0 = ~20Hz, 1.0 = ~20kHz (or Nyquist)
func (f *LadderFilter) SetCutoff(cutoff float32) {
	f.cutoff = util.Clamp(cutoff, 0.0, 1.0)
	f.updateCoefficients()
}

// SetCutoffHz sets the filter cutoff frequency in Hz
func (f *LadderFilter) SetCutoffHz(hz float32) {
	// Clamp to reasonable range (20Hz to Nyquist)
	maxFreq := f.sampleRate * 0.45 // Stay below Nyquist
	hz = util.Clamp(hz, 20.0, maxFreq)

	// Convert to normalized frequency (logarithmic mapping)
	// TB-303 range: approximately 20Hz to 4kHz
	normalized := float32(math.Log(float64(hz/20.0)) / math.Log(200.0)) // 20Hz to 4kHz
	f.SetCutoff(util.Clamp(normalized, 0.0, 1.0))
}

// SetResonance sets the filter resonance (0.0 to 1.0)
// TB-303 characteristic: never reaches full self-oscillation (~0.95 max)
func (f *LadderFilter) SetResonance(resonance float32) {
	f.resonance = util.Clamp(resonance, 0.0, 0.98)
	f.updateCoefficients()
}

// SetSaturation sets the amount of non-linear saturation
func (f *LadderFilter) SetSaturation(saturation float32) {
	f.saturation = util.Clamp(saturation, 0.0, 1.0)
}

// updateCoefficients calculates filter coefficients from cutoff and resonance
func (f *LadderFilter) updateCoefficients() {
	// Convert normalized cutoff to Hz (20Hz to 4kHz for TB-303)
	cutoffHz := 20.0 * float32(math.Pow(200.0, float64(f.cutoff)))

	// Pre-warp the cutoff frequency (bilinear transform)
	// g = tan(π * fc / fs)
	wc := cutoffHz / f.sampleRate
	g := float32(math.Tan(math.Pi * float64(wc)))

	// Limit g to prevent instability
	if g > 1.3 {
		g = 1.3
	}

	f.g = g
	f.g0 = g / (1.0 + g)

	// Resonance to feedback coefficient
	// k controls how much of the output is fed back to the input
	f.k = f.resonance * 4.0
}

// Reset resets the filter state
func (f *LadderFilter) Reset() {
	f.s1 = 0.0
	f.s2 = 0.0
	f.s3 = 0.0
	f.s4 = 0.0
}

// Process processes one sample through the filter
func (f *LadderFilter) Process(input float32) float32 {
	// Zero-delay feedback using topology-preserving transform

	// Calculate feedback
	feedback := f.k * f.s4

	// Input with feedback compensation
	u := (input - feedback) / (1.0 + f.k*f.g0*f.g0*f.g0*f.g0)

	// Apply soft saturation to input (non-linearity)
	if f.saturation > 0.0 {
		u = (1.0-f.saturation)*u + f.saturation*util.FastTanh(u)
	}

	// Process through 4 one-pole filters (cascaded)
	// Stage 1
	v1 := (u - f.s1) * f.g0
	y1 := v1 + f.s1
	f.s1 = y1 + v1

	// Stage 2
	v2 := (y1 - f.s2) * f.g0
	y2 := v2 + f.s2
	f.s2 = y2 + v2

	// Stage 3
	v3 := (y2 - f.s3) * f.g0
	y3 := v3 + f.s3
	f.s3 = y3 + v3

	// Stage 4
	v4 := (y3 - f.s4) * f.g0
	y4 := v4 + f.s4
	f.s4 = y4 + v4

	// Output with slight saturation
	output := y4
	if f.saturation > 0.0 {
		output = (1.0-f.saturation*0.5)*output + (f.saturation*0.5)*util.FastTanh(output*2.0)
	}

	return output
}

// DiodeLadderFilter is a specialized ladder filter that emulates the TB-303's
// diode ladder topology with mismatched capacitor values
type DiodeLadderFilter struct {
	*LadderFilter

	// TB-303 specific: C18 capacitor is half the value of others
	// This creates a 3-pole-like characteristic and prevents self-oscillation
	stage2Scalar float32

	// Mode picks the output by mixing the stage outputs (pole mixing):
	// 0 low-pass (the classic output), 1 band-pass, 2 high-pass.
	Mode int
}

// NewDiodeLadderFilter creates a TB-303 style diode ladder filter
func NewDiodeLadderFilter(sampleRate float32) *DiodeLadderFilter {
	return &DiodeLadderFilter{
		LadderFilter: NewLadderFilter(sampleRate),
		stage2Scalar: 0.5, // Half capacitance = 2x frequency for stage 2
	}
}

// Process processes one sample through the diode ladder filter
func (f *DiodeLadderFilter) Process(input float32) float32 {
	// Zero-delay feedback
	feedback := f.k * f.s4

	// Input with feedback compensation
	// The diode ladder has slightly different compensation
	u := (input - feedback) / (1.0 + f.k*f.g0*f.g0*f.g0*f.g0*0.85)

	// Stronger input saturation (diode clipping characteristic)
	u = util.FastTanh(u * 1.2)

	// Stage 1
	v1 := (u - f.s1) * f.g0
	y1 := v1 + f.s1
	f.s1 = y1 + v1

	// Stage 2 - DIFFERENT! (half capacitance = 2x cutoff)
	// This is the TB-303 characteristic
	g0_stage2 := f.g0 * f.stage2Scalar
	v2 := (y1 - f.s2) * g0_stage2
	y2 := v2 + f.s2
	f.s2 = y2 + v2

	// Apply slight saturation between stages (transistor non-linearity)
	y2 = util.FastTanh2(y2 * 1.1)

	// Stage 3
	v3 := (y2 - f.s3) * f.g0
	y3 := v3 + f.s3
	f.s3 = y3 + v3

	// Stage 4
	v4 := (y3 - f.s4) * f.g0
	y4 := v4 + f.s4
	f.s4 = y4 + v4

	// Output saturation (diode clipping). Band- and high-pass are 12 dB
	// pole mixes of the input and the first two stages.
	out := y4
	switch f.Mode {
	case 1:
		out = 2*y1 - 2*y2
	case 2:
		out = u - 2*y1 + y2
	}
	output := util.FastTanh2(out * 1.3)

	return output
}

// ProcessBuffer processes a buffer of samples
func (f *LadderFilter) ProcessBuffer(input, output []float32) {
	for i := range input {
		output[i] = f.Process(input[i])
	}
}

// GetCutoffHz returns the current cutoff frequency in Hz
func (f *LadderFilter) GetCutoffHz() float32 {
	return 20.0 * float32(math.Pow(200.0, float64(f.cutoff)))
}
