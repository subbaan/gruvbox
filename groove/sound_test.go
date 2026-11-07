package groove

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestSoundDiscoveryPreservesGrooveAndUndoes(t *testing.T) {
	for _, kind := range Kinds {
		p := NewProject()
		track := NewTrack(kind, 0)
		p.Tracks = []Track{track}
		Generate(&p.Tracks[0], 0, "Busy")
		if kind != Sampler {
			p.Tracks[0].LFOs[0].Enabled = true
		}
		session := NewSession(p)
		for n := 0; n < 30; n++ {
			if err := session.Do("Randomise sound", func(p *Project) error { RandomiseSound(&p.Tracks[0]); return nil }); err != nil {
				t.Fatalf("%s: %v", kind, err)
			}
			got := session.Project.Tracks[0]
			if !reflect.DeepEqual(got.Patterns, p.Tracks[0].Patterns) || got.Level != track.Level || got.Pan != track.Pan || got.ID != track.ID || got.Sample != track.Sample {
				t.Fatal("randomise sound changed groove or mixer")
			}
			if got.LFOs[1] != p.Tracks[0].LFOs[1] {
				t.Fatal("disabled LFO changed")
			}
		}
		for session.Undo() {
		}
		if !reflect.DeepEqual(session.Project, p) {
			t.Fatal("sound discovery cannot be undone")
		}
	}
}
func TestLFORoundTripAndAlphaCompatibility(t *testing.T) {
	p := NewProject()
	p.Tracks[0].LFOs[1] = LFO{Enabled: true, Target: 6, Waveform: 5, Rate: 3, Depth: .7, Sync: 2, Division: .125, Steps: 13, Retrigger: true, Phase: .25}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Decode(bytes.NewReader(data))
	if err != nil || !reflect.DeepEqual(p, q) {
		t.Fatalf("LFO round trip: %v", err)
	}
	var old map[string]any
	json.Unmarshal(data, &old)
	for _, raw := range old["tracks"].([]any) {
		track := raw.(map[string]any)
		delete(track, "lfos")
		params := track["parameters"].(map[string]any)
		delete(params, "sub")
		delete(params, "pitch_decay")
		delete(params, "tone_decay")
		if track["kind"] != "kick" {
			delete(params, "tune")
		}
	}
	legacy, _ := json.Marshal(old)
	q, err = Decode(bytes.NewReader(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if q.Tracks[0].LFOs != DefaultLFOs() || q.Tracks[0].Parameters["sub"] != .3 {
		t.Fatal("alpha project defaults were not restored")
	}
	q.Tracks[0].LFOs[0].Rate = 0
	if q.Validate() == nil {
		t.Fatal("invalid LFO accepted")
	}
}
func TestAudibleLFOTimingAndNoAllocations(t *testing.T) {
	p := NewProject()
	p.Tracks = p.Tracks[:1]
	p.BPM = 120
	p.Tracks[0].Patterns[0].Steps[0].Active = true
	p.Tracks[0].LFOs[0].Enabled = true
	p.Tracks[0].LFOs[0].Sync = 2
	p.Tracks[0].LFOs[0].Steps = 8
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var out [BlockSize * 2]float32
	player.Render(out[:2])
	lfo := player.active.tracks[0].voice.lfos()[0]
	first := lfo.GetValue()
	player.Render(make([]float32, 5512*2))
	if lfo.GetValue() != first {
		t.Fatal("step-synced LFO advanced between steps")
	}
	player.Render(out[:2])
	if lfo.GetValue() == first {
		t.Fatal("step-synced LFO did not advance at next step")
	}
	p.Tracks[0].LFOs[0].Sync = 1
	p.Tracks[0].LFOs[0].Division = .25
	p.BPM = 180
	player.Publish(p)
	player.Render(out[:])
	if math.Abs(float64(lfo.GetRate())-3) > .0001 {
		t.Fatal("LFO ignored BPM update")
	}
	p.Tracks[0].LFOs[0].Sync = 0
	p.Tracks[0].LFOs[0].Rate = 5
	player.Publish(p)
	player.Render(out[:])
	if lfo.GetRate() != 5 {
		t.Fatal("free LFO rate was not applied")
	}
	if n := testing.AllocsPerRun(100, func() { player.Render(out[:]) }); n != 0 {
		t.Fatalf("modulation allocated %v", n)
	}
	// Identical bass notes with and without cutoff modulation must sound different.
	p.Tracks[0].LFOs[0].Sync = 0
	p.Tracks[0].LFOs[0].Rate = 7
	mod := NewPlayer()
	mod.Publish(p)
	mod.SetPlaying(true)
	p.Tracks[0].LFOs[0].Enabled = false
	dry := NewPlayer()
	dry.Publish(p)
	dry.SetPlaying(true)
	var a, b [BlockSize * 2]float32
	mod.Render(a[:])
	dry.Render(b[:])
	if a == b {
		t.Fatal("LFO controls did not alter audio")
	}
}
