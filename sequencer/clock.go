package sequencer

import "github.com/subbaan/gruvbox/util"

// Clock provides sample-accurate timing for the sequencer
type Clock struct {
	// Tempo in beats per minute
	bpm float32

	// Sample rate
	sampleRate int

	// Current position in samples since last step
	samplePosition int64

	// Samples per step (calculated from BPM and resolution)
	samplesPerStep int64

	// Current step number (0-based)
	currentStep int

	// Pattern length (number of steps before wrapping)
	patternLength int

	// Transport state
	playing bool

	// Swing amount (0.0 to 1.0)
	// 0.0 = straight, 0.5 = triplet feel, 1.0 = maximum swing
	swing float32

	// Internal: swing delay for even steps (in samples)
	swingDelay int64

	// Reusable buffer for triggered steps (avoids per-call allocs)
	triggeredSteps []int
}

// NewClock creates a new sequencer clock
func NewClock(sampleRate int) *Clock {
	c := &Clock{
		bpm:            120.0,
		sampleRate:     sampleRate,
		samplePosition: 0,
		currentStep:    0,
		patternLength:  16,
		playing:        false,
		swing:          0.0,
		swingDelay:     0,
		triggeredSteps: make([]int, 0, 4),
	}
	c.updateTiming()
	return c
}

// SetBPM sets the tempo in beats per minute
func (c *Clock) SetBPM(bpm float32) {
	c.bpm = util.Clamp(bpm, 40.0, 300.0)
	c.updateTiming()
}

// GetBPM returns the current BPM
func (c *Clock) GetBPM() float32 {
	return c.bpm
}

// SetSwing sets the swing amount (0.0 to 1.0)
func (c *Clock) SetSwing(swing float32) {
	c.swing = util.Clamp(swing, 0.0, 1.0)
	c.updateTiming()
}

// SetPatternLength sets the number of steps before wrapping
func (c *Clock) SetPatternLength(length int) {
	if length < 1 {
		length = 1
	}
	if length > 64 {
		length = 64
	}
	c.patternLength = length

	// Wrap current step if needed
	if c.currentStep >= c.patternLength {
		c.currentStep = c.currentStep % c.patternLength
	}
}

// updateTiming calculates samples per step based on BPM
func (c *Clock) updateTiming() {
	// Calculate samples per 16th note
	// BPM is quarter notes per minute
	// So: samplesPerQuarterNote = (60 * sampleRate) / BPM
	//     samplesPerSixteenthNote = samplesPerQuarterNote / 4
	samplesPerQuarter := (60.0 * float32(c.sampleRate)) / c.bpm
	c.samplesPerStep = int64(samplesPerQuarter / 4.0)

	// Calculate swing delay (applied to even steps)
	// Swing delays even steps and shortens odd steps
	// At 50% swing, even steps are delayed by 1/3 of step duration
	c.swingDelay = int64(float32(c.samplesPerStep) * c.swing * 0.33)
}

// Start starts the clock
func (c *Clock) Start() {
	c.playing = true
}

// Stop stops the clock
func (c *Clock) Stop() {
	c.playing = false
}

// Reset resets the clock to step 0
func (c *Clock) Reset() {
	c.samplePosition = 0
	c.currentStep = 0
}

// IsPlaying returns true if the clock is running
func (c *Clock) IsPlaying() bool {
	return c.playing
}

// GetCurrentStep returns the current step number
func (c *Clock) GetCurrentStep() int {
	return c.currentStep
}

// Tick advances the clock by one sample
// Returns true if a new step was reached
func (c *Clock) Tick() bool {
	if !c.playing {
		return false
	}

	c.samplePosition++

	// Calculate target position for this step
	// Apply swing to even steps (0, 2, 4, etc.)
	targetSamples := c.samplesPerStep
	if c.swing > 0.0 && c.currentStep%2 == 1 {
		targetSamples += c.swingDelay
	} else if c.swing > 0.0 && c.currentStep%2 == 0 && c.currentStep > 0 {
		// Odd steps are shortened to compensate
		targetSamples -= c.swingDelay
	}

	// Check if we've reached the next step
	if c.samplePosition >= targetSamples {
		c.samplePosition = 0
		c.currentStep++

		// Wrap around at pattern length
		if c.currentStep >= c.patternLength {
			c.currentStep = 0
		}

		return true
	}

	return false
}

// TickBuffer advances the clock by multiple samples
// Returns a slice of step numbers that were triggered
// This is more efficient than calling Tick() in a loop
func (c *Clock) TickBuffer(numSamples int) []int {
	if !c.playing {
		return nil
	}

	c.triggeredSteps = c.triggeredSteps[:0]

	for i := 0; i < numSamples; i++ {
		// Store current step before ticking
		stepBeforeTick := c.currentStep
		if c.Tick() {
			// Trigger the step that just completed (before the increment)
			c.triggeredSteps = append(c.triggeredSteps, stepBeforeTick)
		}
	}

	return c.triggeredSteps
}

// GetProgress returns the progress through the current step (0.0 to 1.0)
func (c *Clock) GetProgress() float32 {
	if c.samplesPerStep == 0 {
		return 0.0
	}
	return float32(c.samplePosition) / float32(c.samplesPerStep)
}

// SetStepDirect sets the current step directly (for pattern changes, etc.)
func (c *Clock) SetStepDirect(step int) {
	if step >= 0 && step < c.patternLength {
		c.currentStep = step
		c.samplePosition = 0
	}
}

// GetSamplesPerStep returns the current samples per step
func (c *Clock) GetSamplesPerStep() int64 {
	return c.samplesPerStep
}

// SyncToStep synchronizes the clock to a specific step
// Useful for pattern changes or manual control
func (c *Clock) SyncToStep(step int) {
	c.currentStep = step % c.patternLength
	c.samplePosition = 0
}
