package sequencer

// StepCondition determines when a step should play during pattern repetition
type StepCondition int

const (
	ConditionAlways StepCondition = iota
	ConditionFirstLoop
	ConditionNotFirstLoop
	ConditionEveryNth  // Requires ConditionParameter
	ConditionOddLoops
	ConditionEvenLoops
)

// String returns the display name for a condition
func (c StepCondition) String() string {
	switch c {
	case ConditionAlways:
		return "Always"
	case ConditionFirstLoop:
		return "1st Loop"
	case ConditionNotFirstLoop:
		return "Not 1st"
	case ConditionEveryNth:
		return "Every Nth"
	case ConditionOddLoops:
		return "Odd Loops"
	case ConditionEvenLoops:
		return "Even Loops"
	default:
		return "Unknown"
	}
}

// Step represents a single step in a sequencer pattern
type Step struct {
	// Active indicates if this step triggers a note
	Active bool

	// Note is the MIDI note number (0-127)
	// Only used for melodic instruments like TB-303
	Note int

	// Velocity is the note velocity (0.0 to 1.0)
	Velocity float32

	// Accent flag - increases volume and filter modulation
	Accent bool

	// Slide flag - glides to this note from the previous note
	// TB-303 specific feature
	Slide bool

	// Probability for generative/random patterns (0.0 to 1.0)
	// 1.0 = always play, 0.5 = 50% chance, etc.
	Probability float32

	// Condition determines when this step plays during pattern repetition
	Condition StepCondition

	// ConditionParameter stores the N value for ConditionEveryNth
	ConditionParameter int

	// ParamLocks stores per-step parameter locks
	// Key is parameter name (e.g., "cutoff", "resonance")
	// Value is the locked parameter value (0.0-1.0 typically)
	ParamLocks map[string]float32
}

// NewStep creates a new inactive step
func NewStep() Step {
	return Step{
		Active:             false,
		Note:               36, // C2 default
		Velocity:           0.8,
		Accent:             false,
		Slide:              false,
		Probability:        1.0,
		Condition:          ConditionAlways,
		ConditionParameter: 0,
		ParamLocks:         make(map[string]float32),
	}
}

// NewStepActive creates a new active step with the given note
func NewStepActive(note int) Step {
	return Step{
		Active:             true,
		Note:               note,
		Velocity:           0.8,
		Accent:             false,
		Slide:              false,
		Probability:        1.0,
		Condition:          ConditionAlways,
		ConditionParameter: 0,
		ParamLocks:         make(map[string]float32),
	}
}

// NewStepDrum creates a drum step (active/inactive only)
func NewStepDrum(active bool) Step {
	return Step{
		Active:             active,
		Note:               0,
		Velocity:           0.8,
		Accent:             false,
		Slide:              false,
		Probability:        1.0,
		Condition:          ConditionAlways,
		ConditionParameter: 0,
		ParamLocks:         make(map[string]float32),
	}
}

// Clone creates a copy of the step
func (s Step) Clone() Step {
	// Deep copy ParamLocks map
	locks := make(map[string]float32)
	for k, v := range s.ParamLocks {
		locks[k] = v
	}

	return Step{
		Active:             s.Active,
		Note:               s.Note,
		Velocity:           s.Velocity,
		Accent:             s.Accent,
		Slide:              s.Slide,
		Probability:        s.Probability,
		Condition:          s.Condition,
		ConditionParameter: s.ConditionParameter,
		ParamLocks:         locks,
	}
}

// Clear resets the step to inactive
func (s *Step) Clear() {
	s.Active = false
	s.Accent = false
	s.Slide = false
	// Clear all parameter locks
	s.ParamLocks = make(map[string]float32)
}

// Toggle toggles the step on/off
func (s *Step) Toggle() {
	s.Active = !s.Active
}

// SetNote sets the note and activates the step
func (s *Step) SetNote(note int) {
	s.Note = note
	s.Active = true
}

// HasParamLocks returns true if this step has any parameter locks
func (s *Step) HasParamLocks() bool {
	return len(s.ParamLocks) > 0
}

// GetParamLock returns the locked value for a parameter, and whether it exists
func (s *Step) GetParamLock(paramName string) (float32, bool) {
	val, exists := s.ParamLocks[paramName]
	return val, exists
}

// SetParamLock locks a parameter to a specific value for this step
func (s *Step) SetParamLock(paramName string, value float32) {
	if s.ParamLocks == nil {
		s.ParamLocks = make(map[string]float32)
	}
	s.ParamLocks[paramName] = value
}

// ClearParamLock removes a parameter lock
func (s *Step) ClearParamLock(paramName string) {
	delete(s.ParamLocks, paramName)
}

// ClearAllParamLocks removes all parameter locks from this step
func (s *Step) ClearAllParamLocks() {
	s.ParamLocks = make(map[string]float32)
}

// ShouldPlay determines if this step should play based on probability and condition
// Pass a random value between 0.0 and 1.0, and the current loop count (1-based)
func (s *Step) ShouldPlay(random float32, loopCount int) bool {
	if !s.Active {
		return false
	}

	// Check probability first
	if random >= s.Probability {
		return false
	}

	// Check step condition
	switch s.Condition {
	case ConditionFirstLoop:
		return loopCount == 1
	case ConditionNotFirstLoop:
		return loopCount > 1
	case ConditionEveryNth:
		if s.ConditionParameter <= 0 {
			return true // Invalid parameter, play always
		}
		return loopCount%s.ConditionParameter == 0
	case ConditionOddLoops:
		return loopCount%2 == 1
	case ConditionEvenLoops:
		return loopCount%2 == 0
	case ConditionAlways:
		fallthrough
	default:
		return true
	}
}
