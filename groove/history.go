package groove

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Groove history (alpha.45): each randomise result can be brought back, and
// favourites are kept on disk. See TRACK_BUILDER_PLAN.md.

// GrooveSnapshot is one pattern's groove: tempo, swing, key and each track's
// part in that pattern with its sound. Restoring matches parts to tracks by
// kind, in order, so a favourite works in any project with similar tracks.
type GrooveSnapshot struct {
	ID    string         `json:"id"`
	Label string         `json:"label"`
	Saved time.Time      `json:"saved"`
	BPM   float64        `json:"bpm"`
	Swing float64        `json:"swing"`
	Key   *Key           `json:"key,omitempty"`
	Parts []SnapshotPart `json:"parts"`
}

// SnapshotPart is one track's part. Samplers keep their own sample and
// settings on restore, so only their pattern is used.
type SnapshotPart struct {
	Kind            Kind               `json:"kind"`
	Name            string             `json:"name"`
	Pattern         Pattern            `json:"pattern"`
	Parameters      map[string]float64 `json:"parameters"`
	LFOs            [2]LFO             `json:"lfos"`
	Effects         []Effect           `json:"effects,omitempty"`
	EffectsBypassed bool               `json:"effects_bypassed,omitempty"`
}

// CaptureGroove snapshots the selected pattern.
func CaptureGroove(p Project, label string) GrooveSnapshot {
	s := GrooveSnapshot{ID: newID(), Label: label, Saved: time.Now(), BPM: p.BPM, Swing: p.Swing}
	if p.Key != nil {
		key := *p.Key
		s.Key = &key
	}
	for _, t := range p.Tracks {
		// The sound the pattern plays: its stored state if any.
		sound := withTrackState(t, t.Patterns[p.Pattern].State)
		pattern := clonePattern(t.Patterns[p.Pattern])
		pattern.State = nil
		s.Parts = append(s.Parts, SnapshotPart{Kind: t.Kind, Name: t.Name, Pattern: pattern,
			Parameters: cloneMap(sound.Parameters), LFOs: sound.LFOs, Effects: cloneEffects(sound.Effects), EffectsBypassed: sound.EffectsBypassed})
	}
	return s
}

// RestoreGroove writes a snapshot into the selected pattern. Held tracks stay
// as they are, and so do tempo and key when a held track keeps notes, as for
// Randomise all. Tracks with no matching part keep theirs. It reports how
// many parts found a track.
func RestoreGroove(p *Project, s GrooveSnapshot) (matched int, err error) {
	if len(s.Parts) == 0 {
		return 0, fmt.Errorf("this groove has no parts")
	}
	slot := p.Pattern
	keptNotes, kept := false, false
	for _, t := range p.Tracks {
		if t.Hold {
			kept = true
			keptNotes = keptNotes || t.Kind.synthPart() && hasNotes(t.Patterns[slot])
		}
	}
	if !kept {
		p.BPM, p.Swing = s.BPM, s.Swing
	}
	if s.Key != nil && !keptNotes {
		key := *s.Key
		p.Key = &key
	}
	used := map[int]bool{}
	for _, part := range s.Parts {
		for i := range p.Tracks {
			t := &p.Tracks[i]
			if used[i] || t.Kind != part.Kind {
				continue
			}
			used[i] = true
			matched++
			if t.Hold {
				break
			}
			pattern := clonePattern(part.Pattern)
			pattern.State = nil
			t.Patterns[slot] = pattern
			if t.Kind != Sampler {
				t.Parameters = cloneMap(part.Parameters)
				fillParameterDefaults(t.Kind, t.Parameters)
				t.LFOs, t.Effects, t.EffectsBypassed = part.LFOs, cloneEffects(part.Effects), part.EffectsBypassed
			}
			prunePatternFXLocks(&t.Patterns[slot], t.Effects)
			break
		}
	}
	if matched == 0 {
		return 0, fmt.Errorf("none of this groove's parts match a track here")
	}
	return matched, nil
}

// LoadFavourites reads saved favourite grooves; a missing file is no
// favourites.
func LoadFavourites(path string) ([]GrooveSnapshot, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []GrooveSnapshot
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("favourite grooves: %w", err)
	}
	return list, nil
}

// SaveFavourites writes the favourites atomically.
func SaveFavourites(path string, list []GrooveSnapshot) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".favourites-*.json")
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), path)
}
