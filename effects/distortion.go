package effects

import (
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/util"
)

// Distortion is a waveshaping distortion/overdrive effect
type Distortion struct {
	BaseEffect

	// Parameters
	drive float32 // Pre-gain (1.0 to 20.0)
	tone  float32 // Post-distortion tone control (0.0 to 1.0)

	// Tone filter (simple lowpass to tame highs after distortion)
	toneFilter *dsp.BiquadFilter

	// Pre-allocated buffers
	wetBuffer  []float32
	sampleRate float32
}

// NewDistortion creates a new distortion effect
func NewDistortion(sampleRate float32, bufferSize int) *Distortion {
	d := &Distortion{
		BaseEffect: NewBaseEffect(),
		drive:      2.0,
		tone:       0.7,
		toneFilter: dsp.NewBiquadFilter(sampleRate),
		wetBuffer:  make([]float32, bufferSize),
		sampleRate: sampleRate,
	}

	// Configure tone filter
	d.updateToneFilter(sampleRate)

	return d
}

// SetDrive sets the drive amount (1.0 to 20.0)
func (d *Distortion) SetDrive(drive float32) {
	d.drive = util.Clamp(drive, 1.0, 20.0)
}

// SetTone sets the tone control (0.0 = dark, 1.0 = bright)
func (d *Distortion) SetTone(tone float32) {
	d.tone = util.Clamp(tone, 0.0, 1.0)
	d.updateToneFilter(d.sampleRate)
}

// updateToneFilter updates the tone filter cutoff based on tone setting
func (d *Distortion) updateToneFilter(sampleRate float32) {
	// Map tone 0.0-1.0 to cutoff 500Hz-8000Hz
	cutoff := 500.0 + d.tone*7500.0
	d.toneFilter.SetLowPass(cutoff, 0.707)
}

// Process processes the audio buffer
func (d *Distortion) Process(buffer []float32) {
	if !d.enabled {
		return
	}

	// Copy to wet buffer for dry/wet mixing
	copy(d.wetBuffer, buffer)

	// Apply distortion
	for i := range d.wetBuffer {
		// Apply drive (pre-gain)
		sample := d.wetBuffer[i] * d.drive

		// Soft clipping using polynomial approximation
		// This is smoother than hard clipping and sounds more musical
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		} else {
			// Polynomial soft clipping curve
			// f(x) = (3/2)x - (1/2)x³ for |x| < 1
			sample = 1.5*sample - 0.5*sample*sample*sample
		}

		d.wetBuffer[i] = sample
	}

	// Apply tone filter
	d.toneFilter.ProcessBuffer(d.wetBuffer)

	// Apply makeup gain (compensate for drive)
	makeupGain := 1.0 / (1.0 + d.drive*0.2)
	for i := range d.wetBuffer {
		d.wetBuffer[i] *= makeupGain
	}

	// Mix dry and wet
	d.ProcessMix(buffer, d.wetBuffer)
}

// Reset clears the effect state
func (d *Distortion) Reset() {
	d.toneFilter.Reset()
}

// Getter methods for UI

// GetDrive returns the drive amount
func (d *Distortion) GetDrive() float32 {
	return d.drive
}

// GetTone returns the tone control value
func (d *Distortion) GetTone() float32 {
	return d.tone
}
