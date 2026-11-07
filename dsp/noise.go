package dsp

import (
	"math"
)

// NoiseGenerator generates various types of noise
type NoiseGenerator struct {
	// Linear congruential generator state
	seed uint32
}

// NewNoiseGenerator creates a new noise generator
func NewNoiseGenerator() *NoiseGenerator {
	return &NoiseGenerator{
		seed: 1, // Non-zero seed
	}
}

// SetSeed sets the random seed
func (n *NoiseGenerator) SetSeed(seed uint32) {
	if seed == 0 {
		seed = 1
	}
	n.seed = seed
}

// NextWhite generates the next white noise sample
// Returns a value in the range [-1.0, 1.0]
func (n *NoiseGenerator) NextWhite() float32 {
	// Linear congruential generator
	// Using parameters from Numerical Recipes
	n.seed = n.seed*1664525 + 1013904223

	// Convert to float in range [-1.0, 1.0]
	return float32(int32(n.seed))/float32(math.MaxInt32)
}

// ProcessWhite fills a buffer with white noise
func (n *NoiseGenerator) ProcessWhite(output []float32) {
	for i := range output {
		output[i] = n.NextWhite()
	}
}
