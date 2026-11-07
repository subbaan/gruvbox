package effects

import (
	"math"

	"github.com/subbaan/gruvbox/util"
)

// BitCrusher implements a digital lo-fi effect
// Combines bit depth reduction (quantization) and sample rate reduction (decimation)
type BitCrusher struct {
	BaseEffect

	// Parameters
	bitDepth  float32 // Bit depth (1.0 to 16.0 bits)
	crushRate float32 // Sample rate reduction factor (1.0 to 32.0)

	// Quantization
	quantStep float32 // Calculated quantization step size

	// Sample rate reduction state
	holdSample  float32 // Currently held sample
	holdCounter float32 // Counter for sample-and-hold (fractional)

	// Internal buffers
	wetBuffer []float32 // For dry/wet mixing
}

// NewBitCrusher creates a new bit crusher effect
func NewBitCrusher(sampleRate float32, bufferSize int) *BitCrusher {
	bc := &BitCrusher{
		BaseEffect:  NewBaseEffect(),
		bitDepth:    8.0, // 8-bit classic lo-fi
		crushRate:   4.0, // Reduce sample rate by 4x
		holdSample:  0.0,
		holdCounter: 0.0,
	}

	// Pre-allocate wet buffer
	bc.wetBuffer = make([]float32, bufferSize)

	// Calculate initial quantization step
	bc.updateQuantStep()

	return bc
}

// updateQuantStep recalculates the quantization step size
func (bc *BitCrusher) updateQuantStep() {
	// Quantization step for range [-1.0, 1.0]
	// Number of levels = 2^bitDepth
	numLevels := math.Pow(2.0, float64(bc.bitDepth))
	bc.quantStep = float32(2.0 / numLevels)
}

// Process applies bit crushing to the buffer
func (bc *BitCrusher) Process(buffer []float32) {
	if !bc.enabled {
		return
	}

	// Copy to wet buffer for dry/wet mixing
	copy(bc.wetBuffer, buffer)

	// Process each sample
	for i := range bc.wetBuffer {
		input := bc.wetBuffer[i]

		// Sample rate reduction (decimation with sample-and-hold)
		bc.holdCounter += 1.0
		if bc.holdCounter >= bc.crushRate {
			bc.holdCounter -= bc.crushRate
			if bc.holdCounter >= bc.crushRate {
				bc.holdCounter = float32(math.Mod(float64(bc.holdCounter), float64(bc.crushRate)))
			}
			bc.holdSample = input // Update held sample
		}
		sample := bc.holdSample // Use held sample

		// Bit depth reduction (quantization)
		// Quantize to discrete levels
		if bc.quantStep > 0.0 {
			// Divide by step, floor to integer level, multiply back
			level := float32(math.Floor(float64(sample / bc.quantStep)))
			sample = level * bc.quantStep
		}

		// Clamp to valid range [-1.0, 1.0]
		sample = util.Clamp(sample, -1.0, 1.0)

		bc.wetBuffer[i] = sample
	}

	// Apply dry/wet mix
	bc.ProcessMix(buffer, bc.wetBuffer)
}

// Reset clears the effect state
func (bc *BitCrusher) Reset() {
	// Clear state
	bc.holdSample = 0.0
	bc.holdCounter = 0.0

	// Clear wet buffer
	for i := range bc.wetBuffer {
		bc.wetBuffer[i] = 0.0
	}
}

// SetBitDepth sets the bit depth (1.0 to 16.0 bits)
func (bc *BitCrusher) SetBitDepth(bitDepth float32) {
	bc.bitDepth = util.Clamp(bitDepth, 1.0, 16.0)
	bc.updateQuantStep()
}

// GetBitDepth returns the current bit depth
func (bc *BitCrusher) GetBitDepth() float32 {
	return bc.bitDepth
}

// SetCrushRate sets the sample rate reduction factor (1.0 to 32.0)
func (bc *BitCrusher) SetCrushRate(crushRate float32) {
	bc.crushRate = util.Clamp(crushRate, 1.0, 32.0)
}

// GetCrushRate returns the current crush rate
func (bc *BitCrusher) GetCrushRate() float32 {
	return bc.crushRate
}
