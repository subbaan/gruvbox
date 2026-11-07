package synth

import (
	"math/rand"
)

// ArpPattern defines the arpeggiator pattern type
type ArpPattern int

const (
	ArpPatternUp ArpPattern = iota
	ArpPatternDown
	ArpPatternUpDown
	ArpPatternDownUp
	ArpPatternRandom
	ArpPatternAsPlayed
)

// ArpRate defines the arpeggiator rate (subdivision)
type ArpRate int

const (
	ArpRate1_32 ArpRate = iota
	ArpRate1_16T
	ArpRate1_16
	ArpRate1_8T
	ArpRate1_8
	ArpRate1_4
)

// Arpeggiator generates arpeggiated note sequences from chord input
type Arpeggiator struct {
	// Parameters
	enabled      bool       // Is arpeggiator enabled
	pattern      ArpPattern // Arpeggiator pattern
	rate         ArpRate    // Arpeggiator rate (subdivision)
	gateLength   float32    // Gate length (0.0-1.0)
	octaveRange  int        // Number of octaves to span (1-4)
	swing        float32    // Swing amount (0.0-1.0)
	latchMode    bool       // Latch mode (hold notes after release)

	// Input notes
	inputNotes []int // MIDI note numbers from current step
	heldNotes  []int // Notes currently held (for latch mode)

	// Generated sequence
	sequence     []int // Generated arp sequence
	sequencePos  int   // Current position in sequence
	lastStepNote int   // Last step that was triggered

	// Timing
	stepCounter   int // Counts samples since last arp step
	samplesPerDiv int // Samples per subdivision (calculated from BPM)
}

// NewArpeggiator creates a new arpeggiator
func NewArpeggiator() *Arpeggiator {
	return &Arpeggiator{
		enabled:     false,
		pattern:     ArpPatternUp,
		rate:        ArpRate1_16,
		gateLength:  0.75,
		octaveRange: 1,
		swing:       0.0,
		latchMode:   false,
		inputNotes:  make([]int, 0, 4),
		heldNotes:   make([]int, 0, 4),
		sequence:    make([]int, 0, 32),
		sequencePos: 0,
	}
}

// SetEnabled enables or disables the arpeggiator
func (a *Arpeggiator) SetEnabled(enabled bool) {
	a.enabled = enabled
}

// GetEnabled returns whether the arpeggiator is enabled
func (a *Arpeggiator) GetEnabled() bool {
	return a.enabled
}

// SetPattern sets the arpeggiator pattern
func (a *Arpeggiator) SetPattern(pattern ArpPattern) {
	a.pattern = pattern
	a.regenerateSequence()
}

// GetPattern returns the current pattern
func (a *Arpeggiator) GetPattern() ArpPattern {
	return a.pattern
}

// SetRate sets the arpeggiator rate
func (a *Arpeggiator) SetRate(rate ArpRate) {
	a.rate = rate
}

// GetRate returns the current rate
func (a *Arpeggiator) GetRate() ArpRate {
	return a.rate
}

// SetGateLength sets the gate length (0.0-1.0)
func (a *Arpeggiator) SetGateLength(length float32) {
	if length < 0.1 {
		length = 0.1
	} else if length > 1.0 {
		length = 1.0
	}
	a.gateLength = length
}

// GetGateLength returns the current gate length
func (a *Arpeggiator) GetGateLength() float32 {
	return a.gateLength
}

// SetOctaveRange sets the octave range (1-4)
func (a *Arpeggiator) SetOctaveRange(octaves int) {
	if octaves < 1 {
		octaves = 1
	} else if octaves > 4 {
		octaves = 4
	}
	a.octaveRange = octaves
	a.regenerateSequence()
}

// GetOctaveRange returns the current octave range
func (a *Arpeggiator) GetOctaveRange() int {
	return a.octaveRange
}

// SetSwing sets the swing amount (0.0-1.0)
func (a *Arpeggiator) SetSwing(swing float32) {
	if swing < 0.0 {
		swing = 0.0
	} else if swing > 1.0 {
		swing = 1.0
	}
	a.swing = swing
}

// GetSwing returns the current swing amount
func (a *Arpeggiator) GetSwing() float32 {
	return a.swing
}

// SetLatchMode enables or disables latch mode
func (a *Arpeggiator) SetLatchMode(enabled bool) {
	a.latchMode = enabled
}

// GetLatchMode returns whether latch mode is enabled
func (a *Arpeggiator) GetLatchMode() bool {
	return a.latchMode
}

// SetInputNotes sets the input notes for arpeggiation
func (a *Arpeggiator) SetInputNotes(notes []int) {
	a.inputNotes = make([]int, len(notes))
	copy(a.inputNotes, notes)

	// Update held notes if latch mode is on
	if a.latchMode && len(notes) > 0 {
		a.heldNotes = make([]int, len(notes))
		copy(a.heldNotes, notes)
	}

	a.regenerateSequence()
	a.sequencePos = 0
}

// GetInputNotes returns the current input notes
func (a *Arpeggiator) GetInputNotes() []int {
	return a.inputNotes
}

// GetSequence returns the generated arp sequence
func (a *Arpeggiator) GetSequence() []int {
	return a.sequence
}

// GetSequencePos returns the current position in the sequence
func (a *Arpeggiator) GetSequencePos() int {
	return a.sequencePos
}

// Reset resets the arpeggiator state
func (a *Arpeggiator) Reset() {
	a.sequencePos = 0
	a.stepCounter = 0
	if !a.latchMode {
		a.heldNotes = make([]int, 0, 4)
	}
}

// regenerateSequence generates the arp sequence based on current settings
func (a *Arpeggiator) regenerateSequence() {
	// Use held notes if latch mode, otherwise use input notes
	notes := a.inputNotes
	if a.latchMode && len(a.heldNotes) > 0 {
		notes = a.heldNotes
	}

	if len(notes) == 0 {
		a.sequence = make([]int, 0)
		return
	}

	// Create base sequence from notes
	var baseSeq []int
	switch a.pattern {
	case ArpPatternUp:
		baseSeq = sortedAscending(notes)
	case ArpPatternDown:
		baseSeq = sortedDescending(notes)
	case ArpPatternUpDown:
		asc := sortedAscending(notes)
		baseSeq = append(asc, sortedDescending(notes)[1:]...) // Avoid repeating top note
	case ArpPatternDownUp:
		desc := sortedDescending(notes)
		baseSeq = append(desc, sortedAscending(notes)[1:]...) // Avoid repeating bottom note
	case ArpPatternRandom:
		baseSeq = shuffled(notes)
	case ArpPatternAsPlayed:
		baseSeq = notes // Keep original order
	}

	// Extend across octave range
	a.sequence = make([]int, 0, len(baseSeq)*a.octaveRange)
	for oct := 0; oct < a.octaveRange; oct++ {
		for _, note := range baseSeq {
			a.sequence = append(a.sequence, note+(oct*12))
		}
	}
}

// Update advances the arpeggiator clock and returns a note event
// Returns: note number, noteOn (true=trigger, false=off), changed (true if state changed)
func (a *Arpeggiator) Update(sampleRate float32, bpm float32) (note int, noteOn bool, changed bool) {
	if !a.enabled || len(a.sequence) == 0 {
		return -1, false, false
	}

	// Ensure sequence position is within bounds (in case sequence was regenerated)
	if a.sequencePos >= len(a.sequence) {
		a.sequencePos = a.sequencePos % len(a.sequence)
	}

	// Calculate samples per subdivision based on BPM and rate
	beatsPerSecond := bpm / 60.0
	samplesPerBeat := sampleRate / beatsPerSecond

	var subdivision float32
	switch a.rate {
	case ArpRate1_32:
		subdivision = 0.125 // 1/32 note = 1/8 of a beat
	case ArpRate1_16T:
		subdivision = 1.0 / 6.0 // 1/16 triplet
	case ArpRate1_16:
		subdivision = 0.25 // 1/16 note = 1/4 of a beat
	case ArpRate1_8T:
		subdivision = 1.0 / 3.0 // 1/8 triplet
	case ArpRate1_8:
		subdivision = 0.5 // 1/8 note = 1/2 of a beat
	case ArpRate1_4:
		subdivision = 1.0 // 1/4 note = 1 beat
	}

	samplesPerDiv := int(samplesPerBeat * subdivision)

	// Apply swing (delay every other step)
	if a.sequencePos%2 == 1 && a.swing > 0 {
		samplesPerDiv += int(float32(samplesPerDiv) * a.swing * 0.5)
	}

	// Increment step counter
	a.stepCounter++

	// Check if we should advance to next note
	if a.stepCounter >= samplesPerDiv {
		a.stepCounter = 0
		note = a.sequence[a.sequencePos]
		a.sequencePos = (a.sequencePos + 1) % len(a.sequence)
		return note, true, true
	}

	// Check if we should end current note (gate off)
	gateOffSamples := int(float32(samplesPerDiv) * a.gateLength)
	if a.stepCounter == gateOffSamples {
		return -1, false, true
	}

	return -1, false, false
}

// GetNextNote returns the next note in the arp sequence
// Returns -1 if no note should play
func (a *Arpeggiator) GetNextNote() int {
	if !a.enabled || len(a.sequence) == 0 {
		return -1
	}

	note := a.sequence[a.sequencePos]
	a.sequencePos = (a.sequencePos + 1) % len(a.sequence)
	return note
}

// Helper functions

func sortedAscending(notes []int) []int {
	result := make([]int, len(notes))
	copy(result, notes)
	// Simple bubble sort (fine for small arrays)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func sortedDescending(notes []int) []int {
	result := sortedAscending(notes)
	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func shuffled(notes []int) []int {
	result := make([]int, len(notes))
	copy(result, notes)
	// Fisher-Yates shuffle
	for i := len(result) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		result[i], result[j] = result[j], result[i]
	}
	return result
}
