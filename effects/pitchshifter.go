package effects

import (
	"math"

	"github.com/subbaan/gruvbox/util"
)

// PitchShifter implements a granular pitch shifting effect
// Uses overlapping grains with Hann windowing for smooth pitch transposition
type PitchShifter struct {
	BaseEffect

	// Parameters
	pitch      float32 // Pitch shift in semitones (-12.0 to +12.0)
	pitchRatio float32 // Playback rate ratio (calculated from pitch)

	// Granular synthesis buffers
	grainBuffer1 []float32 // First grain buffer
	grainBuffer2 []float32 // Second grain buffer
	inputBuffer  []float32 // Circular input buffer

	// Grain state
	grainSize   int     // Grain size in samples
	hopSize     int     // Hop between grains in samples
	writePos    int     // Write position in input buffer
	readPos1    float32 // Read position for grain 1 (fractional)
	readPos2    float32 // Read position for grain 2
	grainPhase1 int     // Current position in grain 1 output
	grainPhase2 int     // Current position in grain 2 output

	// Window for grain crossfading (pre-calculated)
	window []float32 // Hann window for smooth grain edges

	// Internal buffers
	wetBuffer  []float32 // For dry/wet mixing
	sampleRate float32   // For parameter calculations
}

// NewPitchShifter creates a new pitch shifter effect
func NewPitchShifter(sampleRate float32, bufferSize int) *PitchShifter {
	grainSize := 2048 // ~46ms at 44.1kHz
	hopSize := grainSize / 4

	ps := &PitchShifter{
		BaseEffect:  NewBaseEffect(),
		pitch:       0.0, // No pitch shift
		pitchRatio:  1.0, // Normal playback rate
		grainSize:   grainSize,
		hopSize:     hopSize,
		writePos:    0,
		readPos1:    0.0,
		readPos2:    float32(hopSize), // Offset second grain
		grainPhase1: 0,
		grainPhase2: hopSize,
		sampleRate:  sampleRate,
	}

	// Pre-allocate all buffers
	ps.grainBuffer1 = make([]float32, grainSize)
	ps.grainBuffer2 = make([]float32, grainSize)
	ps.inputBuffer = make([]float32, grainSize*4)
	ps.window = make([]float32, grainSize)
	ps.wetBuffer = make([]float32, bufferSize)

	// Pre-calculate Hann window
	for i := 0; i < grainSize; i++ {
		// Hann window: 0.5 * (1.0 - cos(2π * i / grainSize))
		ps.window[i] = 0.5 * (1.0 - float32(math.Cos(2.0*math.Pi*float64(i)/float64(grainSize))))
	}

	return ps
}

// Process applies pitch shifting to the buffer
func (ps *PitchShifter) Process(buffer []float32) {
	if !ps.enabled {
		return
	}

	// If pitch is zero, bypass with dry/wet mix
	if ps.pitch == 0.0 && ps.mix == 1.0 {
		return
	}

	// Copy to wet buffer for dry/wet mixing
	copy(ps.wetBuffer, buffer)

	inputBufferSize := len(ps.inputBuffer)

	for i := range ps.wetBuffer {
		// Write incoming sample to circular input buffer
		ps.inputBuffer[ps.writePos] = ps.wetBuffer[i]
		ps.writePos = (ps.writePos + 1) % inputBufferSize

		// Process grain 1
		grain1Sample := ps.processGrain(&ps.readPos1, &ps.grainPhase1)

		// Process grain 2
		grain2Sample := ps.processGrain(&ps.readPos2, &ps.grainPhase2)

		// Mix the two grains (Hann windows ensure smooth crossfade)
		ps.wetBuffer[i] = grain1Sample + grain2Sample
	}

	// Apply dry/wet mix
	ps.ProcessMix(buffer, ps.wetBuffer)
}

// processGrain processes a single grain and returns the output sample
func (ps *PitchShifter) processGrain(readPos *float32, grainPhase *int) float32 {
	inputBufferSize := float32(len(ps.inputBuffer))

	// Read sample from input buffer with linear interpolation
	readPosInt := int(*readPos) % len(ps.inputBuffer)
	readPosFrac := *readPos - float32(int(*readPos))

	// Get two adjacent samples for interpolation
	sample1 := ps.inputBuffer[readPosInt]
	sample2 := ps.inputBuffer[(readPosInt+1)%len(ps.inputBuffer)]

	// Linear interpolation
	interpolatedSample := util.Lerp(sample1, sample2, readPosFrac)

	// Apply Hann window
	windowedSample := interpolatedSample * ps.window[*grainPhase]

	// Advance read position by pitch ratio
	*readPos += ps.pitchRatio
	if *readPos >= inputBufferSize {
		*readPos -= inputBufferSize
	}

	// Advance grain phase
	*grainPhase++
	if *grainPhase >= ps.grainSize {
		// Grain finished, restart
		*grainPhase = 0

		// Reset read position to current write position minus grain size
		// This ensures we're always reading recent audio
		*readPos = float32((ps.writePos - ps.grainSize + len(ps.inputBuffer)) % len(ps.inputBuffer))
	}

	return windowedSample
}

// Reset clears the effect state
func (ps *PitchShifter) Reset() {
	// Clear all buffers
	for i := range ps.inputBuffer {
		ps.inputBuffer[i] = 0.0
	}
	for i := range ps.grainBuffer1 {
		ps.grainBuffer1[i] = 0.0
	}
	for i := range ps.grainBuffer2 {
		ps.grainBuffer2[i] = 0.0
	}
	for i := range ps.wetBuffer {
		ps.wetBuffer[i] = 0.0
	}

	// Reset positions
	ps.writePos = 0
	ps.readPos1 = 0.0
	ps.readPos2 = float32(ps.hopSize)
	ps.grainPhase1 = 0
	ps.grainPhase2 = ps.hopSize
}

// SetPitch sets the pitch shift amount in semitones (-12.0 to +12.0)
func (ps *PitchShifter) SetPitch(pitch float32) {
	ps.pitch = util.Clamp(pitch, -12.0, 12.0)
	// Convert semitones to playback ratio: 2^(pitch/12)
	ps.pitchRatio = util.Exp2(ps.pitch / 12.0)
}

// GetPitch returns the current pitch shift in semitones
func (ps *PitchShifter) GetPitch() float32 {
	return ps.pitch
}

// GetPitchRatio returns the calculated playback ratio
func (ps *PitchShifter) GetPitchRatio() float32 {
	return ps.pitchRatio
}
