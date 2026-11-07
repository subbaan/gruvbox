package effects

import (
	"github.com/subbaan/gruvbox/util"
)

// Reverb is a Freeverb-style reverb effect
type Reverb struct {
	BaseEffect

	// Comb filters (parallel)
	combBuffers [8][]float32
	combIndices [8]int
	combLengths [8]int

	// Allpass filters (series)
	allpassBuffers [4][]float32
	allpassIndices [4]int
	allpassLengths [4]int

	// Parameters
	roomSize float32 // Room size (0.0 to 1.0)
	damping  float32 // High frequency damping (0.0 to 1.0)

	// Internal state
	combFeedback [8]float32
	combDamp     [8]float32
	sampleRate   float32
	wetBuffer    []float32
}

// NewReverb creates a new reverb effect
func NewReverb(sampleRate float32, bufferSize int) *Reverb {
	r := &Reverb{
		BaseEffect: NewBaseEffect(),
		roomSize:   0.5,
		damping:    0.5,
		sampleRate: sampleRate,
		wetBuffer:  make([]float32, bufferSize),
	}

	// Comb filter delays (in samples) - tuned for 44.1kHz
	// These are prime numbers to avoid modal resonances
	combDelays := [8]int{1116, 1188, 1277, 1356, 1422, 1491, 1557, 1617}

	// Scale for sample rate
	scale := sampleRate / 44100.0
	for i := 0; i < 8; i++ {
		r.combLengths[i] = int(float32(combDelays[i]) * scale)
		r.combBuffers[i] = make([]float32, r.combLengths[i])
		r.combIndices[i] = 0
	}

	// Allpass filter delays
	allpassDelays := [4]int{556, 441, 341, 225}
	for i := 0; i < 4; i++ {
		r.allpassLengths[i] = int(float32(allpassDelays[i]) * scale)
		r.allpassBuffers[i] = make([]float32, r.allpassLengths[i])
		r.allpassIndices[i] = 0
	}

	r.updateParameters()

	return r
}

// SetRoomSize sets the room size (0.0 = small, 1.0 = large)
func (r *Reverb) SetRoomSize(size float32) {
	r.roomSize = util.Clamp(size, 0.0, 1.0)
	r.updateParameters()
}

// SetDamping sets the high frequency damping (0.0 = bright, 1.0 = dark)
func (r *Reverb) SetDamping(damping float32) {
	r.damping = util.Clamp(damping, 0.0, 1.0)
	r.updateParameters()
}

// updateParameters updates internal parameters based on room size and damping
func (r *Reverb) updateParameters() {
	// Calculate comb filter feedback based on room size
	feedback := 0.84 + r.roomSize*0.14 // 0.84 to 0.98

	for i := range r.combFeedback {
		r.combFeedback[i] = feedback
		r.combDamp[i] = r.damping
	}
}

// Process processes the audio buffer
func (r *Reverb) Process(buffer []float32) {
	if !r.enabled {
		return
	}

	for i := range buffer {
		input := buffer[i]

		// Process through parallel comb filters
		combOutput := float32(0.0)
		for c := 0; c < 8; c++ {
			// Read from buffer
			output := r.combBuffers[c][r.combIndices[c]]

			// Simple damping (lowpass in feedback path)
			dampedOutput := output*(1.0-r.combDamp[c]) + r.combDamp[c]*output*0.5

			// Write input + feedback to buffer
			r.combBuffers[c][r.combIndices[c]] = input + dampedOutput*r.combFeedback[c]

			// Accumulate output
			combOutput += output

			// Advance index
			r.combIndices[c]++
			if r.combIndices[c] >= r.combLengths[c] {
				r.combIndices[c] = 0
			}
		}

		// Average comb outputs
		combOutput /= 8.0

		// Process through series allpass filters
		allpassOutput := combOutput
		for a := 0; a < 4; a++ {
			// Read from buffer
			bufferOut := r.allpassBuffers[a][r.allpassIndices[a]]

			// Allpass formula: out = -in + buffer
			output := -allpassOutput + bufferOut

			// Write to buffer: in + buffer * 0.5
			r.allpassBuffers[a][r.allpassIndices[a]] = allpassOutput + bufferOut*0.5

			allpassOutput = output

			// Advance index
			r.allpassIndices[a]++
			if r.allpassIndices[a] >= r.allpassLengths[a] {
				r.allpassIndices[a] = 0
			}
		}

		r.wetBuffer[i] = allpassOutput * 0.25 // Scale down
	}

	// Mix dry and wet
	r.ProcessMix(buffer, r.wetBuffer)
}

// Reset clears all delay lines
func (r *Reverb) Reset() {
	for i := range r.combBuffers {
		for j := range r.combBuffers[i] {
			r.combBuffers[i][j] = 0.0
		}
		r.combIndices[i] = 0
	}

	for i := range r.allpassBuffers {
		for j := range r.allpassBuffers[i] {
			r.allpassBuffers[i][j] = 0.0
		}
		r.allpassIndices[i] = 0
	}
}

// Getter methods for UI

// GetRoomSize returns the room size
func (r *Reverb) GetRoomSize() float32 {
	return r.roomSize
}

// GetDamping returns the damping amount
func (r *Reverb) GetDamping() float32 {
	return r.damping
}
