package sampler

import (
	"math"
)

// Voice plays back a sample (one-shot)
type Voice struct {
	sample   *Sample
	playing  bool
	position float64 // Current playback position (fractional for resampling)
	volume   float32
}

// NewVoice creates a new sampler voice
func NewVoice() *Voice {
	return &Voice{
		sample:   nil,
		playing:  false,
		position: 0,
		volume:   1.0,
	}
}

// SetSample sets the sample to play
func (v *Voice) SetSample(sample *Sample) {
	v.sample = sample
}

// Trigger starts playing the sample from the beginning
func (v *Voice) Trigger(velocity float32) {
	if v.sample == nil || len(v.sample.Data) == 0 {
		return
	}

	v.playing = true
	v.position = 0
	v.volume = velocity
}

// Stop stops playback
func (v *Voice) Stop() {
	v.playing = false
}

// IsPlaying returns whether the voice is currently playing
func (v *Voice) IsPlaying() bool {
	return v.playing
}

// GetPosition returns the current playback position (in samples/frames)
// This is relative to the trimmed data, not the original sample
func (v *Voice) GetPosition() float64 {
	return v.position
}

// Process generates audio for the given number of samples
// Returns stereo output (left, right)
func (v *Voice) Process(numSamples int, sampleRate int) ([]float32, []float32) {
	left := make([]float32, numSamples)
	right := make([]float32, numSamples)

	if !v.playing || v.sample == nil {
		return left, right
	}

	trimmedData := v.sample.GetTrimmedData()
	trimmedLength := len(trimmedData)

	if v.sample.Channels == 1 {
		trimmedLength = len(trimmedData)
	} else {
		trimmedLength = len(trimmedData) / 2
	}

	// Calculate playback rate (resample if sample rate differs)
	playbackRate := float64(v.sample.SampleRate) / float64(sampleRate)

	for i := 0; i < numSamples; i++ {
		if v.position >= float64(trimmedLength) {
			// Sample finished
			v.playing = false
			break
		}

		// Linear interpolation for resampling
		pos := int(v.position)
		frac := float32(v.position - float64(pos))

		var sample1L, sample1R, sample2L, sample2R float32

		if v.sample.Channels == 1 {
			// Mono sample
			if pos < len(trimmedData) {
				sample1L = trimmedData[pos]
				sample1R = sample1L
			}
			if pos+1 < len(trimmedData) {
				sample2L = trimmedData[pos+1]
				sample2R = sample2L
			}
		} else {
			// Stereo sample
			if pos*2 < len(trimmedData) {
				sample1L = trimmedData[pos*2]
				sample1R = trimmedData[pos*2+1]
			}
			if (pos+1)*2+1 < len(trimmedData) {
				sample2L = trimmedData[(pos+1)*2]
				sample2R = trimmedData[(pos+1)*2+1]
			}
		}

		// Linear interpolation
		left[i] = (sample1L*(1.0-frac) + sample2L*frac) * v.volume
		right[i] = (sample1R*(1.0-frac) + sample2R*frac) * v.volume

		// Advance position
		v.position += playbackRate
	}

	return left, right
}

// SetVolume sets the playback volume (0.0 to 1.0)
func (v *Voice) SetVolume(volume float32) {
	v.volume = clamp(volume, 0.0, 1.0)
}

func clamp(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Sampler manages multiple sampler voices (for polyphony if needed)
type Sampler struct {
	voices     []*Voice
	maxVoices  int
	nextVoice  int
	currentSample *Sample
}

// NewSampler creates a new sampler with the specified number of voices
func NewSampler(maxVoices int) *Sampler {
	voices := make([]*Voice, maxVoices)
	for i := range voices {
		voices[i] = NewVoice()
	}

	return &Sampler{
		voices:    voices,
		maxVoices: maxVoices,
		nextVoice: 0,
		currentSample: nil,
	}
}

// LoadSample loads a sample and assigns it to all voices
func (s *Sampler) LoadSample(sample *Sample) {
	s.currentSample = sample
	for _, voice := range s.voices {
		voice.SetSample(sample)
	}
}

// GetSample returns the current sample
func (s *Sampler) GetSample() *Sample {
	return s.currentSample
}

// Trigger triggers the next available voice
func (s *Sampler) Trigger(velocity float32) {
	// Simple round-robin voice allocation
	s.voices[s.nextVoice].Trigger(velocity)
	s.nextVoice = (s.nextVoice + 1) % s.maxVoices
}

// Process generates audio from all voices
func (s *Sampler) Process(numSamples int, sampleRate int) ([]float32, []float32) {
	left := make([]float32, numSamples)
	right := make([]float32, numSamples)

	// Mix all voices
	for _, voice := range s.voices {
		if voice.IsPlaying() {
			voiceLeft, voiceRight := voice.Process(numSamples, sampleRate)
			for i := 0; i < numSamples; i++ {
				left[i] += voiceLeft[i]
				right[i] += voiceRight[i]
			}
		}
	}

	return left, right
}

// StopAll stops all voices
func (s *Sampler) StopAll() {
	for _, voice := range s.voices {
		voice.Stop()
	}
}

// SetVolume sets the volume for all voices
func (s *Sampler) SetVolume(volume float32) {
	for _, voice := range s.voices {
		voice.SetVolume(volume)
	}
}

// GetPlaybackPosition returns the current playback position in the original sample
// Returns -1 if no voice is currently playing
// The position is in samples (for mono) or frames (for stereo)
func (s *Sampler) GetPlaybackPosition() int {
	if s.currentSample == nil {
		return -1
	}

	// Find the first playing voice
	for _, voice := range s.voices {
		if voice.IsPlaying() {
			// The voice position is relative to the trimmed data
			// We need to convert it to the original sample position
			trimmedPos := int(voice.GetPosition())
			originalPos := s.currentSample.TrimStart + trimmedPos
			return originalPos
		}
	}

	return -1
}

func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

func abs(x float32) float32 {
	return float32(math.Abs(float64(x)))
}
