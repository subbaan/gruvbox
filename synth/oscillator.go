package synth

import (
	"math"

	"github.com/subbaan/gruvbox/util"
)

// Waveform types
type Waveform int

const (
	WaveformSaw Waveform = iota
	WaveformSquare
	WaveformTriangle
	WaveformSine
)

// Oscillator generates band-limited waveforms using polyBLEP anti-aliasing
type Oscillator struct {
	waveform   Waveform
	phase      float32 // 0.0 to 1.0
	frequency  float32 // Hz
	sampleRate float32

	// For pulse width modulation (square wave)
	pulseWidth float32 // 0.0 to 1.0 (default 0.5)

	// Integrator for saw->triangle conversion
	integrator float32
}

// NewOscillator creates a new oscillator
func NewOscillator(waveform Waveform, sampleRate float32) *Oscillator {
	return &Oscillator{
		waveform:   waveform,
		phase:      0.0,
		frequency:  440.0,
		sampleRate: sampleRate,
		pulseWidth: 0.5,
		integrator: 0.0,
	}
}

// SetWaveform sets the oscillator waveform
func (osc *Oscillator) SetWaveform(waveform Waveform) {
	osc.waveform = waveform
}

// SetFrequency sets the oscillator frequency in Hz
func (osc *Oscillator) SetFrequency(freq float32) {
	osc.frequency = freq
}

// SetPulseWidth sets the pulse width for square wave (0.0 to 1.0)
func (osc *Oscillator) SetPulseWidth(pw float32) {
	if pw < 0.01 {
		pw = 0.01
	}
	if pw > 0.99 {
		pw = 0.99
	}
	osc.pulseWidth = pw
}

// GetPulseWidth returns the current pulse width setting
func (osc *Oscillator) GetPulseWidth() float32 {
	return osc.pulseWidth
}

// Reset resets the oscillator phase to 0
func (osc *Oscillator) Reset() {
	osc.phase = 0.0
	osc.integrator = 0.0
}

// Process generates one sample and advances the phase
func (osc *Oscillator) Process() float32 {
	var output float32

	switch osc.waveform {
	case WaveformSaw:
		output = osc.generateSaw()
	case WaveformSquare:
		output = osc.generateSquare()
	case WaveformTriangle:
		output = osc.generateTriangle()
	case WaveformSine:
		output = osc.generateSine()
	default:
		output = 0.0
	}

	// Advance phase
	osc.phase += osc.frequency / osc.sampleRate
	if osc.phase >= 1.0 {
		osc.phase -= 1.0
	}

	return output
}

// ProcessBlend generates saw and square from the same phase and mixes in
// amount (0–1) of the wave that is not selected, then advances the phase.
// amount 0 is identical to Process for saw or square.
func (osc *Oscillator) ProcessBlend(amount float32) float32 {
	saw, square := osc.generateSaw(), osc.generateSquare()
	main, other := saw, square
	if osc.waveform == WaveformSquare {
		main, other = square, saw
	}
	osc.phase += osc.frequency / osc.sampleRate
	if osc.phase >= 1.0 {
		osc.phase -= 1.0
	}
	return main + (other-main)*amount*.5
}

// generateSaw generates a band-limited sawtooth wave using polyBLEP
func (osc *Oscillator) generateSaw() float32 {
	// Naive sawtooth
	value := 2.0*osc.phase - 1.0

	// Apply polyBLEP at discontinuities to reduce aliasing
	value -= osc.polyBLEP(osc.phase)

	return value
}

// generateSquare generates a band-limited square wave using polyBLEP
func (osc *Oscillator) generateSquare() float32 {
	var value float32

	// Naive square wave
	if osc.phase < osc.pulseWidth {
		value = 1.0
	} else {
		value = -1.0
	}

	// Apply polyBLEP at rising edge (phase = 0)
	value += osc.polyBLEP(osc.phase)

	// Apply polyBLEP at falling edge (phase = pulseWidth)
	value -= osc.polyBLEP(osc.phase - osc.pulseWidth + 1.0)

	return value
}

// generateTriangle generates a triangle wave by integrating a square wave
func (osc *Oscillator) generateTriangle() float32 {
	// Generate square wave
	var square float32
	if osc.phase < 0.5 {
		square = 1.0
	} else {
		square = -1.0
	}

	// Integrate with leaky integrator
	osc.integrator += (square - osc.integrator) * (osc.frequency / osc.sampleRate) * 4.0

	return osc.integrator
}

// generateSine generates a pure sine wave (no anti-aliasing needed)
func (osc *Oscillator) generateSine() float32 {
	return float32(math.Sin(float64(osc.phase) * 2.0 * math.Pi))
}

// polyBLEP implements the Polynomial Band-Limited Step function
// This reduces aliasing at discontinuities
func (osc *Oscillator) polyBLEP(t float32) float32 {
	// Ensure t is in [0, 1]
	for t >= 1.0 {
		t -= 1.0
	}
	for t < 0.0 {
		t += 1.0
	}

	dt := osc.frequency / osc.sampleRate

	// 0 <= t < dt
	if t < dt {
		t /= dt
		return t + t - t*t - 1.0
	}

	// -dt < t < 0 (wrapped around 1.0)
	if t > 1.0-dt {
		t = (t - 1.0) / dt
		return t*t + t + t + 1.0
	}

	// No discontinuity
	return 0.0
}

// ProcessBuffer fills a buffer with oscillator output
func (osc *Oscillator) ProcessBuffer(buffer []float32) {
	for i := range buffer {
		buffer[i] = osc.Process()
	}
}

// TB303Oscillator is a specialized oscillator for TB-303 emulation
type TB303Oscillator struct {
	*Oscillator

	// TB-303 has a distinctive characteristic where the square wave
	// has a slightly different timbre than a perfect 50% duty cycle
	squarePhaseShift float32
}

// NewTB303Oscillator creates an oscillator optimized for TB-303
func NewTB303Oscillator(sampleRate float32) *TB303Oscillator {
	osc := &TB303Oscillator{
		Oscillator:       NewOscillator(WaveformSaw, sampleRate),
		squarePhaseShift: 0.0,
	}
	return osc
}

// SetNote sets the oscillator frequency from a MIDI note number
func (osc *Oscillator) SetNote(note int) {
	osc.frequency = util.MIDIToFrequency(note)
}

// SetNoteTuned sets the oscillator frequency from a MIDI note with fine tuning
// detune is in semitones (-12.0 to +12.0)
func (osc *Oscillator) SetNoteTuned(note int, detune float32) {
	baseFreq := util.MIDIToFrequency(note)
	// detune: semitones to frequency ratio
	// ratio = 2^(detune/12)
	ratio := float32(math.Pow(2.0, float64(detune)/12.0))
	osc.frequency = baseFreq * ratio
}
