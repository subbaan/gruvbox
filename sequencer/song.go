package sequencer

// SongStep represents a single step in a song arrangement
type SongStep struct {
	PatternID int // Which pattern to play (0-15 for 16 patterns)
	Repeats   int // How many times to repeat this pattern
}

// Song represents a complete song arrangement
type Song struct {
	steps          []SongStep
	currentStep    int  // Current position in song
	currentRepeat  int  // Current repeat count for current step
	loopEnabled    bool // Whether to loop the song
	songMode       bool // Whether song mode is active (vs single pattern mode)
}

// NewSong creates a new empty song
func NewSong() *Song {
	return &Song{
		steps:       make([]SongStep, 0, 64), // Allocate space for 64 song steps
		currentStep: 0,
		currentRepeat: 0,
		loopEnabled: true,
		songMode:    false,
	}
}

// AddStep adds a pattern to the song
func (s *Song) AddStep(patternID int, repeats int) {
	if repeats < 1 {
		repeats = 1
	}
	s.steps = append(s.steps, SongStep{
		PatternID: patternID,
		Repeats:   repeats,
	})
}

// InsertStep inserts a pattern at a specific position
func (s *Song) InsertStep(index int, patternID int, repeats int) {
	if index < 0 || index > len(s.steps) {
		return
	}
	if repeats < 1 {
		repeats = 1
	}

	step := SongStep{PatternID: patternID, Repeats: repeats}
	s.steps = append(s.steps[:index], append([]SongStep{step}, s.steps[index:]...)...)
}

// RemoveStep removes a step from the song
func (s *Song) RemoveStep(index int) {
	if index < 0 || index >= len(s.steps) {
		return
	}
	s.steps = append(s.steps[:index], s.steps[index+1:]...)

	// Adjust current step if needed
	if s.currentStep >= len(s.steps) && len(s.steps) > 0 {
		s.currentStep = len(s.steps) - 1
	}
}

// GetStep returns the song step at the given index
func (s *Song) GetStep(index int) *SongStep {
	if index < 0 || index >= len(s.steps) {
		return nil
	}
	return &s.steps[index]
}

// SetStep updates a song step
func (s *Song) SetStep(index int, patternID int, repeats int) {
	if index < 0 || index >= len(s.steps) {
		return
	}
	if repeats < 1 {
		repeats = 1
	}
	s.steps[index].PatternID = patternID
	s.steps[index].Repeats = repeats
}

// GetStepCount returns the number of steps in the song
func (s *Song) GetStepCount() int {
	return len(s.steps)
}

// GetCurrentStep returns the current song step index
func (s *Song) GetCurrentStep() int {
	return s.currentStep
}

// GetCurrentPatternID returns the pattern ID that should be playing
func (s *Song) GetCurrentPatternID() int {
	if !s.songMode || len(s.steps) == 0 {
		return 0 // Default to pattern 0
	}
	if s.currentStep >= 0 && s.currentStep < len(s.steps) {
		return s.steps[s.currentStep].PatternID
	}
	return 0
}

// OnPatternComplete should be called when the current pattern finishes playing
// Returns true if the song has ended (when not looping)
func (s *Song) OnPatternComplete() bool {
	if !s.songMode || len(s.steps) == 0 {
		return false
	}

	// Increment repeat counter
	s.currentRepeat++

	// Check if we've completed all repeats for this step
	if s.currentRepeat >= s.steps[s.currentStep].Repeats {
		s.currentRepeat = 0
		s.currentStep++

		// Check if song is complete
		if s.currentStep >= len(s.steps) {
			if s.loopEnabled {
				s.currentStep = 0 // Loop back to start
				return false
			} else {
				return true // Song ended
			}
		}
	}

	return false
}

// Reset resets the song to the beginning
func (s *Song) Reset() {
	s.currentStep = 0
	s.currentRepeat = 0
}

// SetSongMode enables or disables song mode
func (s *Song) SetSongMode(enabled bool) {
	s.songMode = enabled
	if enabled {
		s.Reset()
	}
}

// IsSongMode returns whether song mode is active
func (s *Song) IsSongMode() bool {
	return s.songMode
}

// SetLoopEnabled sets whether the song should loop
func (s *Song) SetLoopEnabled(enabled bool) {
	s.loopEnabled = enabled
}

// IsLoopEnabled returns whether the song loops
func (s *Song) IsLoopEnabled() bool {
	return s.loopEnabled
}

// Clear clears all steps from the song
func (s *Song) Clear() {
	s.steps = s.steps[:0]
	s.Reset()
}

// DuplicateStep duplicates a step at the given index
func (s *Song) DuplicateStep(index int) {
	if index < 0 || index >= len(s.steps) {
		return
	}
	step := s.steps[index]
	s.InsertStep(index+1, step.PatternID, step.Repeats)
}

// MoveStep moves a step from one position to another
func (s *Song) MoveStep(fromIndex, toIndex int) {
	if fromIndex < 0 || fromIndex >= len(s.steps) || toIndex < 0 || toIndex >= len(s.steps) {
		return
	}
	if fromIndex == toIndex {
		return
	}

	step := s.steps[fromIndex]
	s.RemoveStep(fromIndex)
	if toIndex > fromIndex {
		toIndex-- // Adjust for removal
	}
	s.InsertStep(toIndex, step.PatternID, step.Repeats)
}
