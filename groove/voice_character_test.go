package groove

import (
	"bytes"
	"context"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

var characterControls = []string{"pulse", "drive", "sweep", "gate"}

func TestAcidGateSamples(t *testing.T) {
	pattern := Pattern{Steps: make([]Step, 16), Resolution: 16}
	for i := range pattern.Steps {
		pattern.Steps[i] = DefaultStep()
	}
	step := 44100. * 60 / 120 * 4 / 16
	if n := acidGateSamples(1, 120, &pattern, 0); n != 0 {
		t.Fatalf("gate 1 should ring, got %d", n)
	}
	if n := acidGateSamples(.5, 120, &pattern, 0); n != int(.5*step) {
		t.Fatalf("half gate = %d, want %d", n, int(.5*step))
	}
	pattern.Steps[0].Length = 2
	if n := acidGateSamples(.5, 120, &pattern, 0); n != int(step) {
		t.Fatalf("two-step note at half gate = %d, want %d", n, int(step))
	}
	pattern.Steps[0].Length = 1
	pattern.Steps[1].Active, pattern.Steps[1].Slide = true, true
	if n := acidGateSamples(.5, 120, &pattern, 0); n != 0 {
		t.Fatalf("a note sliding into the next should be held, got %d", n)
	}
	pattern.Steps[0].Active, pattern.Steps[0].Slide = true, true
	if n := acidGateSamples(.5, 120, &pattern, 15); n != 0 {
		t.Fatal("the slide check should wrap to step 1")
	}
}

// Projects, stored pattern sounds and presets saved before a control existed
// load with its default, which sounds like the voice did before.
func TestOlderFilesGainCharacterDefaults(t *testing.T) {
	p := NewProject()
	if err := StorePatternState(&p, 0, p.Tracks[0].ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range characterControls {
		delete(p.Tracks[0].Parameters, id)
		delete(p.Tracks[0].Patterns[0].State.Parameters, id)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("older project rejected: %v", err)
	}
	for _, param := range Parameters(Acid) {
		if q.Tracks[0].Parameters[param.ID] != param.Default && contains(characterControls, param.ID) {
			t.Fatalf("%s = %v", param.ID, q.Tracks[0].Parameters[param.ID])
		}
		if _, ok := q.Tracks[0].Patterns[0].State.Parameters[param.ID]; !ok {
			t.Fatalf("stored sound lacks %s", param.ID)
		}
	}

	preset := CaptureSound(NewTrack(Acid, 0), "Old")
	for _, id := range characterControls {
		delete(preset.Parameters, id)
	}
	data, _ = json.Marshal(preset)
	loaded, err := DecodeSoundPreset(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("older preset rejected: %v", err)
	}
	if loaded.Parameters["gate"] != 1 || loaded.Parameters["drive"] != 0 {
		t.Fatalf("preset defaults wrong: %v", loaded.Parameters)
	}
}

func TestRandomiseSoundKeepsCharacterValid(t *testing.T) {
	p := NewProject()
	p.Tracks[0].Parameters["tune"] = -3
	names := map[string]int{}
	for n := 0; n < 200; n++ {
		names[RandomiseSound(&p.Tracks[0])]++
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		if p.Tracks[0].Parameters["tune"] != -3 {
			t.Fatal("randomise changed the 303's tuning")
		}
	}
	if len(names) < 18 || names[""] == 0 || names[""] > 60 {
		t.Fatalf("expected most results from many archetypes and a few independent draws: %v", names)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// TestWriteVoiceSamples renders one 303 line, solo, with each character
// control in turn:
// GRUVBOX_VOICE_WAVS=<dir> go test ./groove -run TestWriteVoiceSamples
func TestWriteVoiceSamples(t *testing.T) {
	dir := os.Getenv("GRUVBOX_VOICE_WAVS")
	if dir == "" {
		t.Skip("set GRUVBOX_VOICE_WAVS to a directory to hear the 303 character controls")
	}
	r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	base := NewProject()
	key := Key{Root: 9, Scale: PhraseMinor}
	base.Key = &key
	base.BPM = 130
	generate(&base.Tracks[0], 0, "Acid", &key, r)
	// A classic squelch patch: low cutoff, high resonance.
	for id, v := range map[string]float64{"cutoff": .28, "resonance": .8, "env": .55, "decay": .3, "accent": .7} {
		base.Tracks[0].Parameters[id] = v
	}
	for i := 1; i < len(base.Tracks); i++ {
		base.Tracks[i].Mute = true
	}
	base.Song = []SongSection{NewSongSection(0)}
	variants := []struct {
		name   string
		values map[string]float64
	}{
		{"1 plain", nil},
		{"2 accent sweep", map[string]float64{"sweep": 1}},
		{"3 drive", map[string]float64{"drive": .7}},
		{"4 short gate", map[string]float64{"gate": .35}},
		{"5 narrow square", map[string]float64{"square": 1, "pulse": .2}},
		{"6 all together", map[string]float64{"sweep": .8, "drive": .5, "gate": .6}},
	}
	for _, v := range variants {
		p := base.Clone()
		for id, value := range v.values {
			p.Tracks[0].Parameters[id] = value
		}
		f, err := os.Create(filepath.Join(dir, v.name+".wav"))
		if err != nil {
			t.Fatal(err)
		}
		if err := RenderWAV(context.Background(), f, p, true, 1, nil); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
