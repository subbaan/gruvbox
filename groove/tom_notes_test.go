package groove

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestTomNotePlaybackAndSave(t *testing.T) {
	p := NewProject()
	p.Tracks = []Track{NewTrack(Tom, 0)}
	track := &p.Tracks[0]
	if err := TomBassTone(track); err != nil {
		t.Fatal(err)
	}
	track.Parameters["tune"] = 3
	track.Patterns[0].Steps[0].Active = true
	track.Patterns[0].Steps[0].Note = 48
	track.Patterns[0].Steps[0].Locks = map[string]float64{"pitch": 100}
	encoded, _ := json.Marshal(p)
	loaded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, loaded) {
		t.Fatal("pitched tom did not survive save")
	}
	reference := loaded.Clone()
	reference.Tracks[0].Patterns[0].Steps[0].Note = 36
	reference.Tracks[0].Patterns[0].Steps[0].Locks["pitch"] = 200
	a, b := NewPlayer(), NewPlayer()
	a.Publish(loaded)
	b.Publish(reference)
	a.SetPlaying(true)
	b.SetPlaying(true)
	outA, outB := make([]float32, BlockSize*2), make([]float32, BlockSize*2)
	energy := 0.
	for block := 0; block < 8; block++ {
		a.Render(outA)
		b.Render(outB)
		for i := range outA {
			energy += math.Abs(float64(outA[i]))
			if math.Abs(float64(outA[i]-outB[i])) > 1e-5 {
				t.Fatal("playback did not combine note, tune and pitch lock")
			}
		}
	}
	if energy == 0 {
		t.Fatal("pitched tom silent")
	}
}

func TestTomBassToneUndoAndGenerator(t *testing.T) {
	p := NewProject()
	p.Tracks = []Track{NewTrack(Tom, 0)}
	p.Tracks[0].LFOs[0].Enabled = true
	p.Tracks[0].Patterns[0].Steps[0].Note = 55
	before := p.Clone()
	session := NewSession(p)
	if err := session.Do("Bass tone", func(p *Project) error { return TomBassTone(&p.Tracks[0]) }); err != nil {
		t.Fatal(err)
	}
	got := session.Project.Tracks[0]
	if got.Parameters["pitch_decay"] != 0 || got.Parameters["tone"] != 1 || got.LFOs[0].Enabled || !reflect.DeepEqual(got.Patterns, before.Tracks[0].Patterns) {
		t.Fatal("bass tone did not preserve notes or remove sweep/noise")
	}
	session.Undo()
	if !reflect.DeepEqual(session.Project, before) {
		t.Fatal("bass tone undo failed")
	}
	if err := TomBassTone(nil); err == nil {
		t.Fatal("accepted missing tom")
	}
	// Generate repeatedly to check the full randomized note range.
	for n := 0; n < 20; n++ {
		Generate(&p.Tracks[0], 0, "Bassline")
		for _, step := range p.Tracks[0].Patterns[0].Steps {
			if step.Note < 36 || step.Note > 48 || step.Slide || len(step.Locks) > 0 {
				t.Fatalf("invalid bassline step: %+v", step)
			}
		}
	}
	if !Tom.Capabilities().Notes {
		t.Fatal("tom lacks piano roll capability")
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
}
