package effects

import (
	"math"

	"github.com/subbaan/gruvbox/util"
)

// FilterMode defines the filter type
type FilterMode int

const (
	FilterModeLowPass FilterMode = iota
	FilterModeHighPass
	FilterModeBandPass
)

// MultimodeFilter implements a state-variable filter with multiple modes
// Uses topology-preserving transform (TPT) for stability at high resonance
type MultimodeFilter struct {
	BaseEffect

	// Parameters
	cutoff     float32    // Cutoff frequency in Hz (20.0 to 20000.0)
	resonance  float32    // Resonance amount (0.0 to 0.95)
	filterMode FilterMode // LP, HP, or BP

	// State-variable filter state (mono - process each sample)
	ic1eq float32 // Integrator state 1
	ic2eq float32 // Integrator state 2

	// Pre-computed coefficients
	g          float32 // Frequency coefficient
	k          float32 // Resonance coefficient
	a1, a2, a3 float32 // SVF matrix coefficients

	// Internal buffers
	wetBuffer  []float32 // For dry/wet mixing
	sampleRate float32   // For coefficient calculation
}

// NewMultimodeFilter creates a new multimode filter effect
func NewMultimodeFilter(sampleRate float32, bufferSize int) *MultimodeFilter {
	mf := &MultimodeFilter{
		BaseEffect: NewBaseEffect(),
		cutoff:     2000.0, // Default to 2kHz
		resonance:  0.3,    // Mild resonance
		filterMode: FilterModeLowPass,
		ic1eq:      0.0,
		ic2eq:      0.0,
		sampleRate: sampleRate,
	}

	// Pre-allocate wet buffer
	mf.wetBuffer = make([]float32, bufferSize)

	// Calculate initial coefficients
	mf.updateCoefficients()

	return mf
}

// updateCoefficients recalculates filter coefficients
func (mf *MultimodeFilter) updateCoefficients() {
	// Clamp cutoff to safe range (avoid Nyquist issues)
	cutoffClamped := util.Clamp(mf.cutoff, 20.0, mf.sampleRate/2.0-100.0)

	// Calculate angular frequency
	w := 2.0 * math.Pi * float64(cutoffClamped) / float64(mf.sampleRate)

	// Pre-warped frequency (TPT)
	mf.g = float32(math.Tan(w / 2.0))

	// Damping (inversely proportional to resonance)
	mf.k = 2.0 - 2.0*mf.resonance

	// Matrix coefficients for TPT SVF
	mf.a1 = 1.0 / (1.0 + mf.g*(mf.g+mf.k))
	mf.a2 = mf.g * mf.a1
	mf.a3 = mf.g * mf.a2
}

// Process applies filtering to the buffer
func (mf *MultimodeFilter) Process(buffer []float32) {
	if !mf.enabled {
		return
	}

	// Copy to wet buffer for dry/wet mixing
	copy(mf.wetBuffer, buffer)

	// Process each sample
	for i := range mf.wetBuffer {
		input := mf.wetBuffer[i]

		// State-variable filter (TPT topology)
		v3 := input - mf.ic2eq
		v1 := mf.a1*mf.ic1eq + mf.a2*v3
		v2 := mf.ic2eq + mf.a2*mf.ic1eq + mf.a3*v3

		// Update integrator states
		mf.ic1eq = 2.0*v1 - mf.ic1eq
		mf.ic2eq = 2.0*v2 - mf.ic2eq

		// Select output based on mode
		var output float32
		switch mf.filterMode {
		case FilterModeLowPass:
			output = v2 // Low-pass output
		case FilterModeHighPass:
			output = input - mf.k*v1 - v2 // High-pass output
		case FilterModeBandPass:
			output = v1 // Band-pass output
		}

		mf.wetBuffer[i] = output
	}

	// Apply dry/wet mix
	mf.ProcessMix(buffer, mf.wetBuffer)
}

// Reset clears the effect state
func (mf *MultimodeFilter) Reset() {
	// Clear integrator states
	mf.ic1eq = 0.0
	mf.ic2eq = 0.0

	// Clear wet buffer
	for i := range mf.wetBuffer {
		mf.wetBuffer[i] = 0.0
	}
}

// SetCutoff sets the filter cutoff frequency in Hz
func (mf *MultimodeFilter) SetCutoff(cutoff float32) {
	mf.cutoff = util.Clamp(cutoff, 20.0, 20000.0)
	mf.updateCoefficients()
}

// GetCutoff returns the current cutoff frequency
func (mf *MultimodeFilter) GetCutoff() float32 {
	return mf.cutoff
}

// SetResonance sets the filter resonance (0.0 to 0.95)
func (mf *MultimodeFilter) SetResonance(resonance float32) {
	mf.resonance = util.Clamp(resonance, 0.0, 0.95)
	mf.updateCoefficients()
}

// GetResonance returns the current resonance
func (mf *MultimodeFilter) GetResonance() float32 {
	return mf.resonance
}

// SetFilterMode sets the filter mode (LP, HP, BP)
func (mf *MultimodeFilter) SetFilterMode(mode FilterMode) {
	mf.filterMode = mode
}

// GetFilterMode returns the current filter mode
func (mf *MultimodeFilter) GetFilterMode() FilterMode {
	return mf.filterMode
}
