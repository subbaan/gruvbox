package util

import "math"

// Clamp restricts a value to the range [min, max]
func Clamp(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Lerp performs linear interpolation between a and b
func Lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

// FastTanh is a fast approximation of tanh for soft clipping
// Good for audio applications where speed matters
func FastTanh(x float32) float32 {
	if x < -3.0 {
		return -1.0
	}
	if x > 3.0 {
		return 1.0
	}

	// Polynomial approximation: tanh(x) ≈ x * (27 + x²) / (27 + 9x²)
	x2 := x * x
	return x * (27.0 + x2) / (27.0 + 9.0*x2)
}

// FastTanh2 is an even faster approximation using a simple rational function
func FastTanh2(x float32) float32 {
	// tanh(x) ≈ x / (1 + |x|)
	return x / (1.0 + abs(x))
}

// abs returns the absolute value of x
func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// Sine generates a sine wave value for the given phase (0.0 to 1.0)
func Sine(phase float32) float32 {
	return float32(math.Sin(float64(phase) * 2.0 * math.Pi))
}

// FastSine approximates sin(2π·phase) for phase in [0, 1) without calling
// math.Sin: a parabola refined once (error under 0.1%).
func FastSine(phase float32) float32 {
	u := 2*phase - 1 // sin(2πp) = -sin(πu)
	a := u
	if a < 0 {
		a = -a
	}
	y := 4*u - 4*u*a
	ay := y
	if ay < 0 {
		ay = -ay
	}
	y = .225*(y*ay-y) + y
	return -y
}

// Exp2 calculates 2^x
func Exp2(x float32) float32 {
	return float32(math.Exp2(float64(x)))
}

// Log2 calculates log2(x)
func Log2(x float32) float32 {
	return float32(math.Log2(float64(x)))
}

// DbToLinear converts decibels to linear amplitude
func DbToLinear(db float32) float32 {
	return float32(math.Pow(10.0, float64(db)/20.0))
}

// LinearToDb converts linear amplitude to decibels
func LinearToDb(linear float32) float32 {
	if linear <= 0.0 {
		return -96.0 // Effective silence
	}
	return float32(20.0 * math.Log10(float64(linear)))
}

// MIDIToFrequency converts a MIDI note number to frequency in Hz
func MIDIToFrequency(note int) float32 {
	// f = 440 * 2^((note - 69) / 12)
	// A4 (MIDI note 69) = 440 Hz
	return 440.0 * float32(math.Pow(2.0, float64(note-69)/12.0))
}
