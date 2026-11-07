package groove

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Decode explicitly rejects the legacy fixed-instrument format. Its importer is
// a later milestone, so an old project can never be silently opened as blank.
func Decode(reader io.Reader) (Project, error) {
	data, err := io.ReadAll(io.LimitReader(reader, 64*1024*1024+1))
	if err != nil {
		return Project{}, err
	}
	if len(data) > 64*1024*1024 {
		return Project{}, fmt.Errorf("project exceeds 64 MiB")
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	// Version 1 had 8 pattern slots; both versions may hold blank slots
	// written as {}. Either way every track ends up with PatternCount.
	if p.FormatVersion == 1 || p.FormatVersion == FormatVersion {
		p.FormatVersion = FormatVersion
		for i := range p.Tracks {
			t := &p.Tracks[i]
			for len(t.Patterns) < PatternCount {
				t.Patterns = append(t.Patterns, Pattern{})
			}
			for j := range t.Patterns {
				if t.Patterns[j].Steps == nil && t.Patterns[j].Resolution == 0 && t.Patterns[j].State == nil {
					t.Patterns[j] = NewPattern()
				}
			}
		}
	}
	// Alpha 1/2 projects omitted LFOs and these newly exposed sound controls.
	for i := range p.Tracks {
		t := &p.Tracks[i]
		if t.LFOs == ([2]LFO{}) {
			t.LFOs = DefaultLFOs()
		}
		// Controls added since the file was saved take their defaults,
		// which sound like the instrument did before they existed.
		fillParameterDefaults(t.Kind, t.Parameters)
		for j := range t.Patterns {
			if state := t.Patterns[j].State; state != nil {
				fillParameterDefaults(t.Kind, state.Parameters)
			}
		}
	}
	return p, p.Validate()
}

// fillParameterDefaults adds any missing instrument control at its default.
// A nil map stays nil so validation still rejects it.
func fillParameterDefaults(kind Kind, parameters map[string]float64) {
	if parameters == nil {
		return
	}
	for _, param := range Parameters(kind) {
		if _, ok := parameters[param.ID]; !ok {
			parameters[param.ID] = param.Default
		}
	}
}

func Save(path string, p Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(compact(p), "", "  ")
	if err != nil {
		return err
	}
	if len(data)+1 > 64*1024*1024 {
		return fmt.Errorf("project exceeds 64 MiB; use smaller samples")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".gruvbox-project-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// compact returns p with blank pattern slots emptied so they save as {}.
// It copies the track and pattern slices it changes; p is untouched.
func compact(p Project) Project {
	tracks := make([]Track, len(p.Tracks))
	for i, t := range p.Tracks {
		patterns := make([]Pattern, len(t.Patterns))
		for j, pattern := range t.Patterns {
			if !pattern.blank() {
				patterns[j] = pattern
			}
		}
		t.Patterns = patterns
		tracks[i] = t
	}
	p.Tracks = tracks
	return p
}
