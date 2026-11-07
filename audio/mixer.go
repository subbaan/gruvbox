package audio

import (
	"github.com/subbaan/gruvbox/effects"
	"github.com/subbaan/gruvbox/util"
)

// Channel represents a single mixer channel
type Channel struct {
	// Audio buffer for this channel (pre-allocated)
	Buffer []float32

	// Effect chain for this channel
	Effects *effects.Chain

	// Mix parameters (accessed from audio thread, updated via ring buffer)
	Level float32 // 0.0 to 1.0
	Pan   float32 // -1.0 (left) to 1.0 (right)
	Mute  bool
	Solo  bool

	// Metering (updated by audio thread, read by UI thread)
	PeakLevel float32
}

// Mixer handles multi-channel audio mixing
type Mixer struct {
	channels []Channel

	// Master output level
	masterLevel float32

	// Limiter state
	limiterGain float32

	// Pre-allocated buffers
	leftBuffer  []float32
	rightBuffer []float32

	bufferSize int
}

// NewMixer creates a new mixer with the specified number of channels
func NewMixer(numChannels, bufferSize int) *Mixer {
	m := &Mixer{
		channels:    make([]Channel, numChannels),
		masterLevel: 0.8, // Leave some headroom
		limiterGain: 1.0,
		leftBuffer:  make([]float32, bufferSize),
		rightBuffer: make([]float32, bufferSize),
		bufferSize:  bufferSize,
	}

	// Initialize channel buffers
	for i := range m.channels {
		m.channels[i].Buffer = make([]float32, bufferSize)
		m.channels[i].Effects = effects.NewChain()
		m.channels[i].Level = 0.75
		m.channels[i].Pan = 0.0
		m.channels[i].Mute = false
		m.channels[i].Solo = false
	}

	return m
}

// GetChannel returns a pointer to a specific channel
func (m *Mixer) GetChannel(index int) *Channel {
	if index >= 0 && index < len(m.channels) {
		return &m.channels[index]
	}
	return nil
}

// SetChannelLevel sets the level for a channel
func (m *Mixer) SetChannelLevel(index int, level float32) {
	if ch := m.GetChannel(index); ch != nil {
		ch.Level = util.Clamp(level, 0.0, 1.0)
	}
}

// SetChannelPan sets the pan for a channel
func (m *Mixer) SetChannelPan(index int, pan float32) {
	if ch := m.GetChannel(index); ch != nil {
		ch.Pan = util.Clamp(pan, -1.0, 1.0)
	}
}

// SetMasterLevel sets the master output level
func (m *Mixer) SetMasterLevel(level float32) {
	m.masterLevel = util.Clamp(level, 0.0, 1.0)
}

// GetMasterLevel returns the current master output level
func (m *Mixer) GetMasterLevel() float32 {
	return m.masterLevel
}

// Mix combines all channels into a stereo output buffer
// output should be stereo interleaved: [L, R, L, R, ...]
func (m *Mixer) Mix(output []float32) {
	// Clear output buffers
	for i := range m.leftBuffer {
		m.leftBuffer[i] = 0.0
		m.rightBuffer[i] = 0.0
	}

	// Check if any channels are soloed
	anySolo := false
	for i := range m.channels {
		if m.channels[i].Solo {
			anySolo = true
			break
		}
	}

	// Sum all channels
	for i := range m.channels {
		ch := &m.channels[i]

		// Skip muted channels
		if ch.Mute {
			continue
		}

		// Skip non-solo channels if any channel is soloed
		if anySolo && !ch.Solo {
			continue
		}

		// Calculate pan gains (constant power panning)
		// Left: cos(pan * π/4)
		// Right: sin(pan * π/4)
		panRad := (ch.Pan + 1.0) / 2.0 // Convert -1..1 to 0..1
		leftGain := util.Clamp(1.0-panRad, 0.0, 1.0)
		rightGain := util.Clamp(panRad, 0.0, 1.0)

		// Apply square root for constant power
		leftGain = float32(sqrt(float64(leftGain)))
		rightGain = float32(sqrt(float64(rightGain)))

		// Mix channel into left/right buffers
		maxPeak := float32(0.0)
		for j := 0; j < m.bufferSize; j++ {
			sample := ch.Buffer[j] * ch.Level

			// Track peak level
			if abs(sample) > maxPeak {
				maxPeak = abs(sample)
			}

			m.leftBuffer[j] += sample * leftGain
			m.rightBuffer[j] += sample * rightGain
		}

		// Update peak meter (with decay)
		ch.PeakLevel = ch.PeakLevel*0.95 + maxPeak*0.05
	}

	// Apply master level and limiter
	for i := 0; i < m.bufferSize; i++ {
		left := m.leftBuffer[i] * m.masterLevel
		right := m.rightBuffer[i] * m.masterLevel

		// Simple limiter (fast attack, slow release)
		peakLevel := max(abs(left), abs(right))
		if peakLevel > 0.95 {
			targetGain := 0.95 / peakLevel
			if targetGain < m.limiterGain {
				// Fast attack
				m.limiterGain = targetGain
			}
		}

		// Slow release
		if m.limiterGain < 1.0 {
			m.limiterGain += (1.0 - m.limiterGain) * 0.001
		}

		// Apply limiter gain
		left *= m.limiterGain
		right *= m.limiterGain

		// Soft clip (safety)
		left = util.FastTanh2(left)
		right = util.FastTanh2(right)

		// Write to interleaved output
		output[i*2] = left
		output[i*2+1] = right
	}
}

// ClearChannel clears a channel's buffer
func (m *Mixer) ClearChannel(index int) {
	if ch := m.GetChannel(index); ch != nil {
		for i := range ch.Buffer {
			ch.Buffer[i] = 0.0
		}
	}
}

// ClearAllChannels clears all channel buffers
func (m *Mixer) ClearAllChannels() {
	for i := range m.channels {
		for j := range m.channels[i].Buffer {
			m.channels[i].Buffer[j] = 0.0
		}
	}
}

// ProcessChannelEffects processes effects on all channels
func (m *Mixer) ProcessChannelEffects() {
	for i := range m.channels {
		if m.channels[i].Effects.IsEnabled() {
			m.channels[i].Effects.Process(m.channels[i].Buffer)
		}
	}
}

// Helper functions
func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton's method for square root
	z := x
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}
