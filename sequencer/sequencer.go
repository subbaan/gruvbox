package sequencer

import (
	"math/rand"
	"time"
)

// NoteEvent represents a note trigger event from the sequencer
type NoteEvent struct {
	Instrument InstrumentType
	Note       int
	Velocity   float32
	Accent     bool
	Slide      bool
	Step       int                    // Which step triggered this
	ParamLocks map[string]float32     // Parameter locks from the step
}

// Sequencer is the main sequencer engine
type Sequencer struct {
	// Clock for timing
	clock *Clock

	// Tracks for each instrument
	tracks map[InstrumentType]*Track

	// Song arrangement
	song *Song

	// Pattern bank - stores multiple patterns per instrument
	// Each instrument can have up to 16 patterns (0-15)
	patternBank map[InstrumentType][]*Pattern

	// Current pattern ID being edited (0-15)
	currentPatternID int

	// Event buffer for note events generated during processing
	// This is cleared each time Process() is called
	events []NoteEvent

	// Pattern wrap detection
	lastStep int // Previous step number to detect wraps

	// Loop counts for step conditions (1-based, starts at 1)
	loopCounts map[InstrumentType]int

	// Track the last pattern ID loaded in song mode (to detect pattern changes)
	lastSongPatternID int

	// Flag to trigger step 0 immediately on first Process() after Start()
	// This ensures instant playback response when pressing play
	triggerFirstStep bool

	// RNG for probability-based steps (audio thread only)
	rng *rand.Rand

	// Sample rate
	sampleRate int

	// Step trigger tracking (for LFO step sync)
	stepsTriggeredCount int // Number of steps triggered in last Process()
}

// NewSequencer creates a new sequencer
func NewSequencer(sampleRate int) *Sequencer {
	seq := &Sequencer{
		clock:             NewClock(sampleRate),
		tracks:            make(map[InstrumentType]*Track),
		song:              NewSong(),
		patternBank:       make(map[InstrumentType][]*Pattern),
		currentPatternID:  0,
		events:            make([]NoteEvent, 0, 16),
		loopCounts:        make(map[InstrumentType]int),
		lastSongPatternID: -1, // -1 indicates no pattern loaded yet
		rng:               rand.New(rand.NewSource(time.Now().UnixNano())),
		sampleRate:        sampleRate,
	}

	// Initialize default tracks
	instruments := []InstrumentType{
		InstrumentTB303_1, InstrumentTB303_2,
		InstrumentKick, InstrumentSnare,
		InstrumentHihatClosed, InstrumentHihatOpen,
		InstrumentClap,
		InstrumentRimshot, InstrumentCrash, InstrumentTom,
		InstrumentSampler1, InstrumentSampler2,
		InstrumentDrone,
	}
	mixerChannels := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}

	for i, instrument := range instruments {
		seq.tracks[instrument] = NewTrack(instrument, mixerChannels[i])

		// Initialize pattern bank with 16 empty patterns per instrument
		seq.patternBank[instrument] = make([]*Pattern, 16)
		for j := 0; j < 16; j++ {
			seq.patternBank[instrument][j] = NewPattern16()
		}

		// Point track to pattern 0 by default
		seq.tracks[instrument].Pattern = seq.patternBank[instrument][0]

		// Initialize loop count to 1 (start on first loop)
		seq.loopCounts[instrument] = 1
	}

	// Set default pattern length
	seq.SetPatternLength(16)

	return seq
}

// GetTrack returns a track for the specified instrument
func (s *Sequencer) GetTrack(instrument InstrumentType) *Track {
	return s.tracks[instrument]
}

// GetSong returns the song arrangement
func (s *Sequencer) GetSong() *Song {
	return s.song
}

// SetBPM sets the tempo
func (s *Sequencer) SetBPM(bpm float32) {
	s.clock.SetBPM(bpm)
}

// GetBPM returns the current tempo
func (s *Sequencer) GetBPM() float32 {
	return s.clock.GetBPM()
}

// SetSwing sets the swing amount
func (s *Sequencer) SetSwing(swing float32) {
	s.clock.SetSwing(swing)
}

// GetSwing returns the current swing amount
func (s *Sequencer) GetSwing() float32 {
	return s.clock.swing
}

// Start starts playback
func (s *Sequencer) Start() {
	// Reset track positions to ensure we start from step 0
	for _, track := range s.tracks {
		track.ResetStep()
	}

	s.clock.Start()

	// Set flag to trigger step 0 immediately on first Process() call
	// This eliminates the ~250ms delay before first note
	s.triggerFirstStep = true

	// If in song mode, load the first pattern
	if s.song.IsSongMode() {
		patternID := s.song.GetCurrentPatternID()
		if patternID >= 0 {
			s.loadPatternForPlayback(patternID)
		}
	}
}

// Stop stops playback
func (s *Sequencer) Stop() {
	s.clock.Stop()
}

// Reset resets to step 0
func (s *Sequencer) Reset() {
	s.clock.Reset()
	s.lastStep = -1
	s.lastSongPatternID = -1 // Reset song pattern tracking

	// Reset loop counts to 1 (start fresh)
	for instrument := range s.loopCounts {
		s.loopCounts[instrument] = 1
	}

	// Reset all track positions
	for _, track := range s.tracks {
		track.ResetStep()
	}
}

// IsPlaying returns true if sequencer is playing
func (s *Sequencer) IsPlaying() bool {
	return s.clock.IsPlaying()
}

// GetCurrentStep returns the current step number
func (s *Sequencer) GetCurrentStep() int {
	return s.clock.GetCurrentStep()
}

// Process advances the sequencer by the specified number of samples
// Returns a slice of note events that were triggered
func (s *Sequencer) Process(numSamples int) []NoteEvent {
	// Clear event buffer
	s.events = s.events[:0]
	s.stepsTriggeredCount = 0

	// Check if we need to trigger step 0 immediately (first Process() after Start())
	// This eliminates the ~250ms delay before first note plays
	if s.triggerFirstStep {
		s.triggerFirstStep = false
		s.processStep(0)
		s.stepsTriggeredCount++
	}

	// Advance clock and get triggered steps
	triggeredSteps := s.clock.TickBuffer(numSamples)
	if len(triggeredSteps) > 0 {
		s.stepsTriggeredCount += len(triggeredSteps)
	}

	// Detect global pattern wrap for song mode
	// (Individual track wrapping is handled per-track in processStep)
	currentStep := s.clock.GetCurrentStep()
	if len(triggeredSteps) > 0 {
		// Check if global clock wrapped around
		if currentStep < s.lastStep || (s.lastStep == s.clock.patternLength-1 && currentStep == 0) {
			// Notify song if in song mode
			if s.song.IsSongMode() {
				songEnded := s.song.OnPatternComplete()
				if songEnded && !s.song.IsLoopEnabled() {
					// Song finished and not looping - stop playback
					s.Stop()
				} else {
					// Load the pattern for the current song step
					patternID := s.song.GetCurrentPatternID()
					if patternID >= 0 {
						s.loadPatternForPlayback(patternID)
					}
				}
			}
		}
		s.lastStep = currentStep
	}

	// Process each triggered step
	for _, stepNum := range triggeredSteps {
		s.processStep(stepNum)
	}

	return s.events
}

// processStep processes a single step across all tracks
// Each track maintains its own position and advances independently (polymeter)
func (s *Sequencer) processStep(stepNum int) {
	// Check if any tracks are soloed
	anySolo := false
	for _, track := range s.tracks {
		if track.Solo {
			anySolo = true
			break
		}
	}

	// Process each track at its own current step position
	for instrument, track := range s.tracks {
		// Get current step position for this track (polymeter support)
		currentStep := track.GetCurrentStep()

		// Update last triggered step for UI display (shows what's currently playing)
		track.SetLastTriggeredStep(currentStep)

		// Check if track should produce sound (not muted/soloed)
		if track.IsActive(anySolo) {
			// Get the step from the pattern
			step := track.GetStep(currentStep)
			if step != nil {
				// Get loop count for this instrument
				loopCount := s.loopCounts[instrument]

				// Check if step should play (considering probability and conditions)
				if step.ShouldPlay(s.rng.Float32(), loopCount) {
					// Create note event (apply transpose for TB-303 tracks)
					note := step.Note
					if instrument == InstrumentTB303_1 || instrument == InstrumentTB303_2 {
						note += track.Transpose
					}

					event := NoteEvent{
						Instrument: instrument,
						Note:       note,
						Velocity:   step.Velocity,
						Accent:     step.Accent,
						Slide:      step.Slide,
						Step:       currentStep,
						ParamLocks: step.ParamLocks, // Copy parameter locks
					}

					s.events = append(s.events, event)
				}
			}
		}

		// Advance this track's step position (always, even if muted)
		// This keeps muted tracks in sync when they're unmuted
		wrapped := track.AdvanceStep()
		if wrapped {
			// Track wrapped to beginning - increment its loop counter
			s.loopCounts[instrument]++
		}
	}
}

// SetPatternLength sets the pattern length for all tracks
func (s *Sequencer) SetPatternLength(length int) {
	s.clock.SetPatternLength(length)

	// Update all track patterns
	for _, track := range s.tracks {
		track.Pattern.SetLength(length)
	}
}

// GetPatternLength returns the current pattern length
func (s *Sequencer) GetPatternLength() int {
	return s.clock.patternLength
}

// UpdateClockLength updates the clock to use the maximum pattern length across all tracks
// This enables polyrhythms where different tracks have different lengths
func (s *Sequencer) UpdateClockLength() {
	maxLength := 16
	for _, track := range s.tracks {
		if track.Pattern != nil && track.Pattern.Length > maxLength {
			maxLength = track.Pattern.Length
		}
	}
	s.clock.SetPatternLength(maxLength)
}

// ClearAllPatterns clears all patterns
func (s *Sequencer) ClearAllPatterns() {
	for _, track := range s.tracks {
		track.Pattern.ClearAll()
	}
}

// GetProgress returns the progress through the current step (0.0 to 1.0)
func (s *Sequencer) GetProgress() float32 {
	return s.clock.GetProgress()
}

// WereStepsTriggered returns true if any steps were triggered in the last Process() call
func (s *Sequencer) WereStepsTriggered() bool {
	return s.stepsTriggeredCount > 0
}

// GetTriggeredStepCount returns how many steps were triggered in the last Process() call
func (s *Sequencer) GetTriggeredStepCount() int {
	return s.stepsTriggeredCount
}

// GetCurrentPatternID returns the currently selected pattern ID for editing
func (s *Sequencer) GetCurrentPatternID() int {
	return s.currentPatternID
}

// GetSongCurrentPatternID returns the pattern ID currently being played in song mode
func (s *Sequencer) GetSongCurrentPatternID() int {
	return s.song.GetCurrentPatternID()
}

// IsSongMode returns whether song mode is active
func (s *Sequencer) IsSongMode() bool {
	return s.song.IsSongMode()
}

// SetCurrentPatternID switches to a different pattern for editing (0-15)
// This loads the pattern from the bank for all instruments
func (s *Sequencer) SetCurrentPatternID(patternID int) {
	if patternID < 0 || patternID >= 16 {
		return
	}

	s.currentPatternID = patternID

	// Switch all tracks to point to this pattern from the bank
	for instrument, track := range s.tracks {
		if patterns, exists := s.patternBank[instrument]; exists {
			track.Pattern = patterns[patternID]
		}
	}
}

// CopyCurrentPattern copies the current pattern to a specified slot (0-15)
func (s *Sequencer) CopyCurrentPattern(toPatternID int) {
	if toPatternID < 0 || toPatternID >= 16 {
		return
	}

	// Copy each instrument's current pattern to the target slot
	for instrument, track := range s.tracks {
		if patterns, exists := s.patternBank[instrument]; exists {
			patterns[toPatternID] = track.Pattern.Clone()
		}
	}
}

// GetPattern returns a specific pattern for an instrument from the bank
// Used by song mode to get the pattern for a specific pattern ID
func (s *Sequencer) GetPattern(instrument InstrumentType, patternID int) *Pattern {
	if patternID < 0 || patternID >= 16 {
		return nil
	}

	if patterns, exists := s.patternBank[instrument]; exists {
		return patterns[patternID]
	}

	return nil
}

// ClearPattern clears a specific pattern in the bank
func (s *Sequencer) ClearPattern(patternID int) {
	if patternID < 0 || patternID >= 16 {
		return
	}

	// Clear the pattern for all instruments
	for instrument := range s.patternBank {
		if patterns, exists := s.patternBank[instrument]; exists {
			patterns[patternID].ClearAll()
		}
	}
}

// loadPatternForPlayback loads a pattern from the bank for playback
// This is used during song mode to switch patterns without changing the edit pattern
func (s *Sequencer) loadPatternForPlayback(patternID int) {
	if patternID < 0 || patternID >= 16 {
		return
	}

	// Check if we're actually switching to a different pattern
	patternChanged := patternID != s.lastSongPatternID

	// Point all tracks to the specified pattern from the bank
	for instrument, track := range s.tracks {
		if patterns, exists := s.patternBank[instrument]; exists {
			track.Pattern = patterns[patternID]

			// Load mute/solo states from the new pattern
			if track.Pattern != nil && track.Pattern.SoundParams != nil {
				if mute, ok := track.Pattern.SoundParams["mute"]; ok {
					track.Mute = mute > 0.5
				} else {
					track.Mute = false
				}

				if solo, ok := track.Pattern.SoundParams["solo"]; ok {
					track.Solo = solo > 0.5
				} else {
					track.Solo = false
				}
			}

			// Reset track position to start from step 0
			track.ResetStep()
		}
	}

	// Update clock length to match the loaded pattern's length
	s.UpdateClockLength()

	// Only reset loop counts when switching to a different pattern
	// This allows "every Nth" conditions to work across pattern repeats in song mode
	if patternChanged {
		for instrument := range s.loopCounts {
			s.loopCounts[instrument] = 1
		}
	}

	// Track the current pattern ID for next comparison
	s.lastSongPatternID = patternID
}
