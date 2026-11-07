package sequencer

// InstrumentType identifies which synth/drum this track controls
type InstrumentType int

const (
	InstrumentTB303_1 InstrumentType = iota
	InstrumentTB303_2
	InstrumentKick
	InstrumentSnare
	InstrumentHihatClosed
	InstrumentHihatOpen
	InstrumentClap
	InstrumentSampler1
	InstrumentSampler2
	InstrumentDrone  // Drone synthesizer
	InstrumentRimshot
	InstrumentCrash
	InstrumentTom
)

// String returns the name of the instrument
func (i InstrumentType) String() string {
	names := []string{
		"TB-303 #1",
		"TB-303 #2",
		"Kick",
		"Snare",
		"Hi-Hat (Closed)",
		"Hi-Hat (Open)",
		"Clap",
		"Sampler #1",
		"Sampler #2",
		"Drone",
		"Rimshot",
		"Crash",
		"Tom",
	}
	if int(i) < len(names) {
		return names[i]
	}
	return "Unknown"
}

// Track represents a sequencer track for a specific instrument
type Track struct {
	// Instrument this track controls
	Instrument InstrumentType

	// Pattern for this track
	Pattern *Pattern

	// Mute state
	Mute bool

	// Solo state
	Solo bool

	// Channel number in the mixer (for routing)
	MixerChannel int

	// Transpose offset in semitones (only applies to TB-303 tracks)
	Transpose int

	// Current step position for this track (for polymeter support)
	currentStep int

	// Last triggered step (for UI display to match audio playback)
	// This is the step that most recently generated audio
	lastTriggeredStep int
}

// NewTrack creates a new track for the specified instrument
func NewTrack(instrument InstrumentType, mixerChannel int) *Track {
	return &Track{
		Instrument:        instrument,
		Pattern:           NewPattern16(),
		Mute:              false,
		Solo:              false,
		MixerChannel:      mixerChannel,
		Transpose:         0,
		currentStep:       0,
		lastTriggeredStep: -1, // -1 indicates no step triggered yet
	}
}

// IsActive returns true if this track should play
// (not muted, or soloed when solo is active)
func (t *Track) IsActive(anySolo bool) bool {
	if t.Mute {
		return false
	}
	if anySolo && !t.Solo {
		return false
	}
	return true
}

// GetStep returns the step at the given index
func (t *Track) GetStep(index int) *Step {
	return t.Pattern.GetStep(index)
}

// SetPattern sets a new pattern for this track
func (t *Track) SetPattern(pattern *Pattern) {
	t.Pattern = pattern
}

// Clone creates a copy of the track
func (t *Track) Clone() *Track {
	return &Track{
		Instrument:        t.Instrument,
		Pattern:           t.Pattern.Clone(),
		Mute:              t.Mute,
		Solo:              t.Solo,
		MixerChannel:      t.MixerChannel,
		Transpose:         t.Transpose,
		currentStep:       t.currentStep,
		lastTriggeredStep: t.lastTriggeredStep,
	}
}

// GetCurrentStep returns the current step position for this track
func (t *Track) GetCurrentStep() int {
	return t.currentStep
}

// GetLastTriggeredStep returns the last step that generated audio
// This is used by the UI to display which step is currently being heard
// Returns -1 if no step has been triggered yet
func (t *Track) GetLastTriggeredStep() int {
	return t.lastTriggeredStep
}

// SetLastTriggeredStep updates the last triggered step
// This should be called when a step generates audio
func (t *Track) SetLastTriggeredStep(step int) {
	t.lastTriggeredStep = step
}

// AdvanceStep moves to the next step, wrapping at pattern length
// Returns true if the pattern wrapped (looped back to start)
func (t *Track) AdvanceStep() bool {
	t.currentStep++
	if t.currentStep >= t.Pattern.Length {
		t.currentStep = 0
		return true // Pattern wrapped
	}
	return false
}

// ResetStep resets the step position to the beginning
func (t *Track) ResetStep() {
	t.currentStep = 0
	t.lastTriggeredStep = -1 // Reset to "no step triggered"
}
