package dsp

import (
	"math"
)

// FilterType defines the type of biquad filter
type FilterType int

const (
	FilterLowPass FilterType = iota
	FilterHighPass
	FilterBandPass
	FilterNotch
	FilterPeak
	FilterLowShelf
	FilterHighShelf
)

// BiquadFilter is a 2-pole IIR filter
type BiquadFilter struct {
	// Filter coefficients
	b0, b1, b2 float32 // Feedforward
	a1, a2     float32 // Feedback (a0 is normalized to 1)

	// State variables
	x1, x2 float32 // Input history
	y1, y2 float32 // Output history

	// Filter parameters
	sampleRate float32
	frequency  float32
	q          float32
	gain       float32 // For peak/shelf filters (in dB)
	filterType FilterType
}

// NewBiquadFilter creates a new biquad filter
func NewBiquadFilter(sampleRate float32) *BiquadFilter {
	return &BiquadFilter{
		sampleRate: sampleRate,
		frequency:  1000.0,
		q:          0.707, // Butterworth Q
		gain:       0.0,
		filterType: FilterLowPass,
		b0:         1.0,
	}
}

// SetLowPass configures the filter as a low-pass
func (f *BiquadFilter) SetLowPass(frequency, q float32) {
	f.filterType = FilterLowPass
	f.frequency = frequency
	f.q = q
	f.calculateCoefficients()
}

// SetHighPass configures the filter as a high-pass
func (f *BiquadFilter) SetHighPass(frequency, q float32) {
	f.filterType = FilterHighPass
	f.frequency = frequency
	f.q = q
	f.calculateCoefficients()
}

// SetBandPass configures the filter as a band-pass
func (f *BiquadFilter) SetBandPass(frequency, q float32) {
	f.filterType = FilterBandPass
	f.frequency = frequency
	f.q = q
	f.calculateCoefficients()
}

// Reset clears the filter state
func (f *BiquadFilter) Reset() {
	f.x1 = 0
	f.x2 = 0
	f.y1 = 0
	f.y2 = 0
}

// calculateCoefficients computes the biquad coefficients
func (f *BiquadFilter) calculateCoefficients() {
	// Clamp frequency to Nyquist
	freq := f.frequency
	nyquist := f.sampleRate * 0.5
	if freq >= nyquist {
		freq = nyquist * 0.99
	}

	// Calculate intermediate values
	omega := 2.0 * math.Pi * float64(freq) / float64(f.sampleRate)
	sinOmega := float32(math.Sin(omega))
	cosOmega := float32(math.Cos(omega))
	alpha := sinOmega / (2.0 * f.q)

	switch f.filterType {
	case FilterLowPass:
		f.b0 = (1.0 - cosOmega) / 2.0
		f.b1 = 1.0 - cosOmega
		f.b2 = (1.0 - cosOmega) / 2.0
		a0 := 1.0 + alpha
		f.a1 = -2.0 * cosOmega / a0
		f.a2 = (1.0 - alpha) / a0
		f.b0 /= a0
		f.b1 /= a0
		f.b2 /= a0

	case FilterHighPass:
		f.b0 = (1.0 + cosOmega) / 2.0
		f.b1 = -(1.0 + cosOmega)
		f.b2 = (1.0 + cosOmega) / 2.0
		a0 := 1.0 + alpha
		f.a1 = -2.0 * cosOmega / a0
		f.a2 = (1.0 - alpha) / a0
		f.b0 /= a0
		f.b1 /= a0
		f.b2 /= a0

	case FilterBandPass:
		f.b0 = alpha
		f.b1 = 0.0
		f.b2 = -alpha
		a0 := 1.0 + alpha
		f.a1 = -2.0 * cosOmega / a0
		f.a2 = (1.0 - alpha) / a0
		f.b0 /= a0
		f.b1 /= a0
		f.b2 /= a0
	}
}

// Process processes a single sample
func (f *BiquadFilter) Process(input float32) float32 {
	// Direct Form 1 implementation
	output := f.b0*input + f.b1*f.x1 + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2

	// Update state
	f.x2 = f.x1
	f.x1 = input
	f.y2 = f.y1
	f.y1 = output

	return output
}

// ProcessBuffer processes a buffer of samples
func (f *BiquadFilter) ProcessBuffer(buffer []float32) {
	for i := range buffer {
		buffer[i] = f.Process(buffer[i])
	}
}
