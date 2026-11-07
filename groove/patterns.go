package groove

import "fmt"

func clonePattern(pattern Pattern) Pattern {
	result := pattern
	result.State = cloneTrackState(pattern.State)
	result.Steps = append([]Step(nil), pattern.Steps...)
	for i, step := range pattern.Steps {
		result.Steps[i].Locks = cloneMap(step.Locks)
	}
	return result
}

// CopyPattern copies notes and an independent stored control snapshot.
// An empty track ID copies the whole groove; a track ID copies only that track.
func CopyPattern(p *Project, source, target int, trackID string) error {
	if source < 0 || target < 0 || source >= PatternCount || target >= PatternCount || source == target {
		return fmt.Errorf("choose a different pattern slot")
	}
	if trackID != "" && p.Track(trackID) == nil {
		return fmt.Errorf("track no longer exists")
	}
	for i := range p.Tracks {
		t := &p.Tracks[i]
		if trackID == "" || trackID == t.ID {
			t.Patterns[target] = clonePattern(t.Patterns[source])
		}
	}
	return nil
}
func NextEmptyPattern(p Project, after int) int {
	for offset := 1; offset < PatternCount; offset++ {
		if slot := (after + offset) % PatternCount; PatternEmpty(p, slot) {
			return slot
		}
	}
	return -1
}

// PatternEmpty reports whether no track has notes, locks or stored state in slot.
func PatternEmpty(p Project, slot int) bool {
	for _, t := range p.Tracks {
		if t.Patterns[slot].State != nil {
			return false
		}
		for _, s := range t.Patterns[slot].Steps {
			if s.Active || len(s.Locks) > 0 {
				return false
			}
		}
	}
	return true
}
