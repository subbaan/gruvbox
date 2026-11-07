package groove

import "fmt"

// TransformNotes moves or copies a monophonic phrase atomically. Refuse collisions
// and out-of-range pitches rather than silently destroying neighbouring notes.
func TransformNotes(pattern *Pattern, indices []int, offset, semitones int, copyNotes bool) error {
	selected := map[int]bool{}
	for _, index := range indices {
		if index < 0 || index >= len(pattern.Steps) || !pattern.Steps[index].Active {
			return fmt.Errorf("select active notes")
		}
		selected[index] = true
	}
	if len(selected) == 0 {
		return nil
	}
	for index := range selected {
		step := pattern.Steps[index]
		target := index + offset
		if target < 0 || target >= len(pattern.Steps) || step.Note+semitones < 0 || step.Note+semitones > 127 {
			return fmt.Errorf("phrase would leave the pattern or note range")
		}
		if target != index && (copyNotes || !selected[target]) && (pattern.Steps[target].Active || len(pattern.Steps[target].Locks) > 0) {
			return fmt.Errorf("phrase would overwrite a note or parameter locks")
		}
		if copyNotes && target == index {
			return fmt.Errorf("choose space for the copied phrase")
		}
	}
	original := clonePattern(*pattern)
	if !copyNotes {
		for index := range selected {
			pattern.Steps[index] = DefaultStep()
		}
	}
	for index := range selected {
		step := original.Steps[index]
		step.Note += semitones
		pattern.Steps[index+offset] = step
	}
	return nil
}
