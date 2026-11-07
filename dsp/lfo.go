package dsp

import (
	"math"
	"math/rand"
)

// LFOWaveform represents the LFO waveform type
type LFOWaveform int

const (
	LFOSine LFOWaveform = iota
	LFOTriangle
	LFOSquare
	LFOSawUp
	LFOSawDown
	LFORandom // Sample & Hold
)

// LFOSyncMode represents how the LFO syncs to tempo
type LFOSyncMode int

const (
	SyncModeTime  LFOSyncMode = iota // Musical time divisions (1/4, 1/8, etc.)
	SyncModeSteps                    // Sequencer steps (completes cycle over N steps)
)

// LFO is a low-frequency oscillator for modulation
type LFO struct {
	// Parameters
	waveform   LFOWaveform
	rate       float32 // Rate in Hz (or tempo-synced division)
	depth      float32 // Modulation depth (0.0 to 1.0)
	phase      float32 // Current phase (0.0 to 1.0)
	phaseReset float32 // Initial phase offset (0.0 to 1.0)

	// Range control
	min float32 // Minimum output value
	max float32 // Maximum output value

	// Tempo sync
	tempoSync     bool
	syncMode      LFOSyncMode // Time-based or step-based sync
	syncRate      float32     // Tempo division (e.g., 0.25 = 1/4 note, 0.5 = 1/2 note)
	stepsPerCycle int         // For step-locked mode (e.g., 32 steps per cycle)

	// Retrigger
	retrigger bool // If true, reset phase on each trigger

	// Internal state
	sampleRate    float32
	phaseInc      float32 // Phase increment per sample
	currentValue  float32 // Current LFO output
	randomValue   float32 // For sample & hold
	randomCounter int     // Sample counter for S&H
	randomRate    int     // Samples between S&H updates
}

// NewLFO creates a new LFO
func NewLFO(sampleRate float32) *LFO {
	lfo := &LFO{
		waveform:      LFOSine,
		rate:          2.0, // 2 Hz default
		depth:         0.5,
		phase:         0.0,
		phaseReset:    0.0,
		min:           0.0,
		max:           1.0,
		tempoSync:     true,          // Default to tempo sync ON
		syncMode:      SyncModeSteps, // Default to step-based sync
		syncRate:      0.25,          // 1/4 note
		stepsPerCycle: 16,            // Default 16 steps per cycle
		retrigger:     false,
		sampleRate:    sampleRate,
		currentValue:  0.0,
		randomValue:   0.0,
		randomCounter: 0,
		randomRate:    int(sampleRate / 10.0), // 10 Hz default for S&H
	}
	lfo.updatePhaseInc()
	return lfo
}

// SetWaveform sets the LFO waveform
func (lfo *LFO) SetWaveform(waveform LFOWaveform) {
	lfo.waveform = waveform
}

// SetRate sets the LFO rate in Hz
func (lfo *LFO) SetRate(rate float32) {
	if rate < 0.01 {
		rate = 0.01
	}
	if rate > 100.0 {
		rate = 100.0
	}
	lfo.rate = rate
	lfo.updatePhaseInc()
}

// SetDepth sets the modulation depth (0.0 to 1.0)
func (lfo *LFO) SetDepth(depth float32) {
	if depth < 0.0 {
		depth = 0.0
	}
	if depth > 1.0 {
		depth = 1.0
	}
	lfo.depth = depth
}

// SetRange sets the min/max output range
func (lfo *LFO) SetRange(min, max float32) {
	lfo.min = min
	lfo.max = max
}

// SetPhaseReset sets the initial phase offset (0.0 to 1.0)
func (lfo *LFO) SetPhaseReset(phase float32) {
	lfo.phaseReset = phase
}

// SetRetrigger enables/disables phase retrigger
func (lfo *LFO) SetRetrigger(retrigger bool) {
	lfo.retrigger = retrigger
}

// SetTempoSync enables/disables tempo sync
func (lfo *LFO) SetTempoSync(sync bool) {
	lfo.tempoSync = sync
}

// SetSyncRate sets the tempo sync division (e.g., 0.25 = 1/4 note)
func (lfo *LFO) SetSyncRate(rate float32) {
	lfo.syncRate = rate
}

// UpdateTempoRate updates the LFO rate based on BPM (call when BPM changes)
func (lfo *LFO) UpdateTempoRate(bpm float32) {
	if lfo.tempoSync {
		beatsPerSecond := bpm / 60.0

		if lfo.syncMode == SyncModeTime {
			// Musical time divisions (FIXED CALCULATION)
			// syncRate represents fraction of a whole note (4 beats)
			// e.g., 0.25 = quarter note (1 beat), 0.03125 = 1/32 note (1/8 beat)
			beatsPerNote := lfo.syncRate * 4.0
			lfo.rate = beatsPerSecond / beatsPerNote
		} else {
			// Step-locked mode: complete one cycle over N sequencer steps
			// Assuming 16th note steps (4 steps per beat)
			stepsPerSecond := beatsPerSecond * 4.0
			if lfo.stepsPerCycle > 0 {
				lfo.rate = stepsPerSecond / float32(lfo.stepsPerCycle)
			} else {
				lfo.rate = stepsPerSecond / 16.0 // Default to 16 steps
			}
		}

		lfo.updatePhaseInc()
	}
}

// Trigger resets the LFO phase (if retrigger is enabled)
func (lfo *LFO) Trigger() {
	if lfo.retrigger {
		lfo.phase = lfo.phaseReset
	}
}

// Process advances the LFO by one sample and returns the current value
func (lfo *LFO) Process() float32 {
	// Generate waveform
	var raw float32

	switch lfo.waveform {
	case LFOSine:
		raw = float32(math.Sin(float64(lfo.phase * 2.0 * math.Pi)))

	case LFOTriangle:
		if lfo.phase < 0.5 {
			raw = lfo.phase*4.0 - 1.0 // -1 to 1
		} else {
			raw = 3.0 - lfo.phase*4.0 // 1 to -1
		}

	case LFOSquare:
		if lfo.phase < 0.5 {
			raw = 1.0
		} else {
			raw = -1.0
		}

	case LFOSawUp:
		raw = lfo.phase*2.0 - 1.0 // -1 to 1

	case LFOSawDown:
		raw = 1.0 - lfo.phase*2.0 // 1 to -1

	case LFORandom:
		// Sample & Hold - update at random rate
		lfo.randomCounter++
		if lfo.randomCounter >= lfo.randomRate {
			lfo.randomValue = rand.Float32()*2.0 - 1.0 // -1 to 1
			lfo.randomCounter = 0
		}
		raw = lfo.randomValue
	}

	// Convert from -1..1 to 0..1
	normalized := (raw + 1.0) * 0.5

	// Apply depth and range
	lfo.currentValue = lfo.min + (lfo.max-lfo.min)*normalized*lfo.depth

	// Advance phase only in time mode; step mode advances via StepTrigger.
	if lfo.syncMode == SyncModeTime {
		lfo.phase += lfo.phaseInc
		if lfo.phase >= 1.0 {
			lfo.phase -= 1.0
		}
	}

	return lfo.currentValue
}

// StepTrigger advances the LFO by one sequencer step (for step-locked mode)
func (lfo *LFO) StepTrigger() {
	if lfo.syncMode == SyncModeSteps && lfo.stepsPerCycle > 0 {
		// Advance by one step's worth of phase
		stepPhase := 1.0 / float32(lfo.stepsPerCycle)
		lfo.phase += stepPhase
		if lfo.phase >= 1.0 {
			lfo.phase -= 1.0
		}
	}
}

// GetValue returns the current LFO value without advancing
func (lfo *LFO) GetValue() float32 {
	return lfo.currentValue
}

// Reset resets the LFO to initial state
func (lfo *LFO) Reset() {
	lfo.phase = lfo.phaseReset
	lfo.currentValue = 0.0
	lfo.randomValue = 0.0
	lfo.randomCounter = 0
}

// updatePhaseInc calculates the phase increment per sample
func (lfo *LFO) updatePhaseInc() {
	lfo.phaseInc = lfo.rate / lfo.sampleRate
	lfo.randomRate = max(1, int(lfo.sampleRate/lfo.rate))
}

// Getter methods for UI

// GetWaveform returns the current waveform
func (lfo *LFO) GetWaveform() LFOWaveform {
	return lfo.waveform
}

// GetRate returns the LFO rate in Hz
func (lfo *LFO) GetRate() float32 {
	return lfo.rate
}

// GetDepth returns the modulation depth
func (lfo *LFO) GetDepth() float32 {
	return lfo.depth
}

// GetMin returns the minimum output value
func (lfo *LFO) GetMin() float32 {
	return lfo.min
}

// GetMax returns the maximum output value
func (lfo *LFO) GetMax() float32 {
	return lfo.max
}

// GetTempoSync returns whether tempo sync is enabled
func (lfo *LFO) GetTempoSync() bool {
	return lfo.tempoSync
}

// GetSyncRate returns the tempo sync division
func (lfo *LFO) GetSyncRate() float32 {
	return lfo.syncRate
}

// GetRetrigger returns whether retrigger is enabled
func (lfo *LFO) GetRetrigger() bool {
	return lfo.retrigger
}

// SetSyncMode sets the sync mode (time-based or step-based)
func (lfo *LFO) SetSyncMode(mode LFOSyncMode) {
	lfo.syncMode = mode
	if mode == SyncModeSteps {
		// Steps mode is inherently sequencer-synced.
		lfo.tempoSync = true
	}
}

// GetSyncMode returns the current sync mode
func (lfo *LFO) GetSyncMode() LFOSyncMode {
	return lfo.syncMode
}

// SetStepsPerCycle sets the number of steps for a full LFO cycle (step-locked mode)
func (lfo *LFO) SetStepsPerCycle(steps int) {
	if steps < 1 {
		steps = 1
	}
	if steps > 256 {
		steps = 256
	}
	lfo.stepsPerCycle = steps
}

// GetStepsPerCycle returns the number of steps per cycle
func (lfo *LFO) GetStepsPerCycle() int {
	return lfo.stepsPerCycle
}
