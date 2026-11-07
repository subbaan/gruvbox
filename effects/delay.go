package effects

import (
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/util"
)

// Delay is a delay/echo effect with feedback
type Delay struct {
	BaseEffect

	// Delay line (circular buffer)
	buffer     []float32
	writePos   int
	bufferSize int

	// Parameters
	delayTime float32 // Delay time in seconds
	feedback  float32 // Feedback amount (0.0 to 0.95)

	// Feedback filter (tone control in feedback path)
	feedbackFilter *dsp.BiquadFilter

	// Internal
	sampleRate   float32
	delaySamples int
	wetBuffer    []float32
}

// NewDelay creates a new delay effect
// maxDelayTime is the maximum delay time in seconds (e.g., 1.0 for 1 second max)
func NewDelay(sampleRate float32, bufferSize int, maxDelayTime float32) *Delay {
	delayBufferSize := int(sampleRate * maxDelayTime)

	d := &Delay{
		BaseEffect:     NewBaseEffect(),
		buffer:         make([]float32, delayBufferSize),
		writePos:       0,
		bufferSize:     delayBufferSize,
		delayTime:      0.25, // 250ms default
		feedback:       0.4,  // 40% feedback
		feedbackFilter: dsp.NewBiquadFilter(sampleRate),
		sampleRate:     sampleRate,
		wetBuffer:      make([]float32, bufferSize),
	}

	// Configure feedback filter (lowpass to simulate tape delay)
	d.feedbackFilter.SetLowPass(4000.0, 0.707)

	d.updateDelaySamples()

	return d
}

// SetDelayTime sets the delay time in seconds
func (d *Delay) SetDelayTime(time float32) {
	maxTime := float32(d.bufferSize) / d.sampleRate
	d.delayTime = util.Clamp(time, 0.001, maxTime)
	d.updateDelaySamples()
}

// SetFeedback sets the feedback amount (0.0 to 0.95)
func (d *Delay) SetFeedback(feedback float32) {
	d.feedback = util.Clamp(feedback, 0.0, 0.95)
}

// updateDelaySamples calculates the delay in samples
func (d *Delay) updateDelaySamples() {
	d.delaySamples = int(d.delayTime * d.sampleRate)
	if d.delaySamples < 1 {
		d.delaySamples = 1
	}
	if d.delaySamples >= d.bufferSize {
		d.delaySamples = d.bufferSize - 1
	}
}

// Process processes the audio buffer
func (d *Delay) Process(buffer []float32) {
	if !d.enabled {
		return
	}

	// Process each sample
	for i := range buffer {
		// Calculate read position (behind write position by delayTime)
		readPos := d.writePos - d.delaySamples
		if readPos < 0 {
			readPos += d.bufferSize
		}

		// Read delayed sample
		delayedSample := d.buffer[readPos]

		// Apply feedback filter
		filteredFeedback := d.feedbackFilter.Process(delayedSample)

		// Write to delay line (input + filtered feedback)
		d.buffer[d.writePos] = buffer[i] + filteredFeedback*d.feedback

		// Store wet signal
		d.wetBuffer[i] = delayedSample

		// Advance write position
		d.writePos++
		if d.writePos >= d.bufferSize {
			d.writePos = 0
		}
	}

	// Mix dry and wet
	d.ProcessMix(buffer, d.wetBuffer)
}

// Reset clears the delay line
func (d *Delay) Reset() {
	for i := range d.buffer {
		d.buffer[i] = 0.0
	}
	d.writePos = 0
	d.feedbackFilter.Reset()
}

// Getter methods for UI

// GetDelayTime returns the delay time in seconds
func (d *Delay) GetDelayTime() float32 {
	return d.delayTime
}

// GetFeedback returns the feedback amount
func (d *Delay) GetFeedback() float32 {
	return d.feedback
}
