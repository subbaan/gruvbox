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
	data, err := json.MarshalIndent(p, "", "  ")
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
