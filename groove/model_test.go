package groove

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHistoryPreservesNestedStateAndRejectsInvalidCommand(t *testing.T) {
	p := NewProject()
	id := p.Tracks[0].ID
	s := NewSession(p)
	if err := s.Do("Lock", func(p *Project) error {
		p.Track(id).Patterns[0].Steps[0].Locks = map[string]float64{"cutoff": .7}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	locked := s.Project.Clone()
	if err := s.Do("Edit lock", func(p *Project) error { p.Track(id).Patterns[0].Steps[0].Locks["cutoff"] = .2; return nil }); err != nil {
		t.Fatal(err)
	}
	if !s.Undo() || !reflect.DeepEqual(s.Project, locked) {
		t.Fatal("undo lost nested lock state")
	}
	if !s.Redo() || s.Project.Track(id).Patterns[0].Steps[0].Locks["cutoff"] != .2 {
		t.Fatal("redo failed")
	}
	before := s.Project.Clone()
	if err := s.Do("Invalid tempo", func(p *Project) error { p.BPM = -1; return nil }); err == nil {
		t.Fatal("invalid command accepted")
	}
	if !reflect.DeepEqual(before, s.Project) {
		t.Fatal("invalid command changed project")
	}
	s.MarkSaved()
	if s.Dirty() {
		t.Fatal("saved project is dirty")
	}
	if !s.Undo() || !s.Dirty() {
		t.Fatal("undo after save must mark project dirty")
	}
}
func TestDynamicTracksAndProjectRoundTrip(t *testing.T) {
	p := NewProject()
	for i := 0; i < 20; i++ {
		p.Tracks = append(p.Tracks, NewTrack(Acid, i))
	}
	Generate(&p.Tracks[0], 0, "Acid")
	p.Tracks[0].Patterns[0].Steps[0].Locks = map[string]float64{"cutoff": .42}
	path := filepath.Join(t.TempDir(), "test.gruvbox3")
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	loaded, err := Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, loaded) {
		t.Fatal("dynamic project changed during save/load")
	}
	if _, err := Decode(bytes.NewBufferString(`{"version":"2.16.9","sampler1":{}}`)); err == nil {
		t.Fatal("legacy project silently accepted")
	}
	before, _ := os.ReadFile(path)
	p.BPM = -1
	if err := Save(path, p); err == nil {
		t.Fatal("invalid save accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid save replaced valid file")
	}
}
func TestGenerationUndoAndVariation(t *testing.T) {
	s := NewSession(NewProject())
	id := s.Project.Tracks[2].ID
	before := s.Project.Clone()
	if err := s.Do("4 Floor", func(p *Project) error { Generate(p.Track(id), 0, "4 Floor"); return nil }); err != nil {
		t.Fatal(err)
	}
	for i, step := range s.Project.Track(id).Patterns[0].Steps {
		if step.Active != (i%4 == 0) {
			t.Fatal("four-floor rhythm is wrong")
		}
	}
	if !s.Undo() || !reflect.DeepEqual(s.Project, before) {
		t.Fatal("generator cannot be undone")
	}
	s.Redo()
	original := s.Project.Clone()
	Generate(s.Project.Track(id), 0, "Variation")
	if !reflect.DeepEqual(original.Track(id).Patterns[0], s.Project.Track(id).Patterns[0]) {
		t.Fatal("variation altered source pattern")
	}
	data, _ := json.Marshal(s.Project)
	if _, err := Decode(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}
