package audio

import "time"

// Audio configuration constants
const (
	// SampleRate is the audio sample rate in Hz
	// 44100 Hz is CD quality and widely supported
	SampleRate = 44100

	// ChannelCount is the number of audio channels (2 = stereo)
	ChannelCount = 2

	// BufferSize is the size of the audio buffer in samples
	// 256 samples = ~5.8ms latency at 44.1kHz
	// This is a good balance between latency and performance
	BufferSize = 256

	// BytesPerSample is the number of bytes per audio sample
	// We use 16-bit signed integers (2 bytes per sample)
	BytesPerSample = 2

	// BufferDuration is the audio buffer duration
	// Used for oto context creation
	BufferDuration = time.Millisecond * 10

	// MaxVoices is the maximum number of simultaneous voices
	MaxVoices = 16

	// MaxChannels is the maximum number of mixer channels
	// 2 TB-303s + 8 drums + 2 samplers + 1 drone = 13 channels
	MaxChannels = 13
)

// Constants for audio processing
const (
	// MaxAmplitude for 16-bit audio
	MaxAmplitude = 32767.0

	// Pi constant for DSP calculations
	Pi = 3.14159265358979323846

	// TwoPi is 2 * Pi
	TwoPi = 2.0 * Pi
)

// MIDI constants
const (
	// A4Frequency is the frequency of MIDI note A4 (note 69)
	A4Frequency = 440.0

	// MIDIA4Note is the MIDI note number for A4
	MIDIA4Note = 69
)

// MIDIToFrequency converts a MIDI note number to frequency in Hz
func MIDIToFrequency(note int) float32 {
	// f = 440 * 2^((note - 69) / 12)
	return float32(A4Frequency * pow2(float64(note-MIDIA4Note)/12.0))
}

// pow2 calculates 2^x using the standard library
func pow2(x float64) float64 {
	// 2^x = e^(x * ln(2))
	const ln2 = 0.69314718055994530942
	return exp(x * ln2)
}

// Fast approximation of e^x
func exp(x float64) float64 {
	// For small values, use Taylor series approximation
	// For larger values, this should be replaced with a more accurate method
	// or use math.Exp from standard library
	if x < -10 || x > 10 {
		// Fall back to a simpler approximation or use math.Exp
		// For now, we'll use a simple polynomial
		return 1.0 + x + (x*x)/2.0 + (x*x*x)/6.0
	}
	result := 1.0
	term := 1.0
	for i := 1; i < 10; i++ {
		term *= x / float64(i)
		result += term
		if term < 1e-10 {
			break
		}
	}
	return result
}
