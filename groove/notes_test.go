package groove

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestNoteSelectionMoveCopyCollisionsAndUndo(t *testing.T) {
	p := NewProject()
	pattern := &p.Tracks[0].Patterns[0]
	pattern.Steps[1] = Step{Active: true, Note: 48, Length: 3, Velocity: .37, Accent: true, Slide: true, Probability: .6, Every: 3, Locks: map[string]float64{"cutoff": .72}}
	pattern.Steps[3] = Step{Active: true, Note: 51, Velocity: .8, Probability: 1}
	before := p.Clone()
	session := NewSession(p)
	if err := session.Do("Move phrase", func(p *Project) error { return TransformNotes(&p.Tracks[0].Patterns[0], []int{1, 3}, 2, 7, false) }); err != nil {
		t.Fatal(err)
	}
	got := session.Project.Tracks[0].Patterns[0].Steps
	if got[1].Active || got[3].Note != 55 || got[5].Note != 58 || got[3].Length != 3 || got[3].Locks["cutoff"] != .72 || got[3].Every != 3 || got[3].Velocity != .37 {
		t.Fatal("phrase lost note details")
	}
	session.Undo()
	if !reflect.DeepEqual(session.Project, before) {
		t.Fatal("phrase undo failed")
	}
	pattern = &p.Tracks[0].Patterns[0]
	if err := TransformNotes(pattern, []int{1, 3}, 6, 0, true); err != nil {
		t.Fatal(err)
	}
	pattern.Steps[7].Locks["cutoff"] = .1
	if pattern.Steps[1].Locks["cutoff"] != .72 {
		t.Fatal("copied phrase shares locks")
	}
	for _, tc := range []struct {
		offset, pitch int
		copy          bool
	}{{2, 0, true}, {-2, 0, false}, {0, 100, false}} {
		before := clonePattern(*pattern)
		if err := TransformNotes(pattern, []int{1, 3}, tc.offset, tc.pitch, tc.copy); err == nil {
			t.Fatal("invalid phrase accepted")
		}
		if !reflect.DeepEqual(*pattern, before) {
			t.Fatal("rejected phrase changed pattern")
		}
	}
	pattern.Steps[9].Active = false
	pattern.Steps[9].Locks = map[string]float64{"cutoff": .5}
	beforePattern := clonePattern(*pattern)
	if err := TransformNotes(pattern, []int{1}, 8, 0, false); err == nil || !reflect.DeepEqual(*pattern, beforePattern) {
		t.Fatal("overwrote locks")
	}
}

func TestNoteLengthSaveAndLegacyCompatibility(t *testing.T) {
	p := NewProject()
	p.Tracks[0].Patterns[0].Steps[0].Length = 8
	data, _ := json.Marshal(p)
	loaded, err := Decode(bytes.NewReader(data))
	if err != nil || !reflect.DeepEqual(p, loaded) {
		t.Fatalf("length save/load: %v", err)
	}
	if DefaultStep().GateSteps() != 1 {
		t.Fatal("old notes changed duration")
	}
	p.Tracks[0].Patterns[0].Steps[0].Length = 65
	if p.Validate() == nil {
		t.Fatal("invalid length accepted")
	}
}

func TestSamplerGateNoteLengthsSwingRetriggerAndModes(t *testing.T) {
	values := make([]int16, SampleRate*2)
	for i := range values {
		values[i] = 16000
	}
	sample := pcmSample(t, 1, SampleRate, values...)
	for _, length := range []int{0, 1, 3} {
		p := NewProject()
		p.BPM = 120
		p.Swing = .2
		p.Tracks = []Track{NewTrack(Sampler, 0)}
		p.Tracks[0].Sample = sample
		p.Tracks[0].Parameters["mode"] = 1
		step := &p.Tracks[0].Patterns[0].Steps[0]
		step.Active = true
		step.Length = length
		player := NewPlayer()
		player.Publish(p)
		player.SetPlaying(true)
		// 120 BPM at 1/16: alternating 6615 and 4410 frames under 20% swing.
		duration := 6615
		if length == 3 {
			duration = 6615 + 4410 + 6615
		}
		renderFrames(player, duration)
		if !player.active.tracks[0].voice.samplePlaying {
			t.Fatal("gate ended early")
		}
		renderFrames(player, 1)
		if player.active.tracks[0].voice.samplePlaying {
			t.Fatalf("gate length %d did not end at boundary", length)
		}
	}
	// One-shot and Loop retain their existing sample-driven durations.
	for _, mode := range []float64{0, 2} {
		p := NewProject()
		p.Tracks = []Track{NewTrack(Sampler, 0)}
		p.Tracks[0].Sample = sample
		p.Tracks[0].Parameters["mode"] = mode
		p.Tracks[0].Patterns[0].Steps[0].Active = true
		player := NewPlayer()
		player.Publish(p)
		player.SetPlaying(true)
		renderFrames(player, 10000)
		if !player.active.tracks[0].voice.samplePlaying {
			t.Fatal("length interrupted one-shot/loop")
		}
	}
	// A later onset replaces a held note and gets its own gate.
	p := NewProject()
	p.BPM = 120
	p.Tracks = []Track{NewTrack(Sampler, 0)}
	p.Tracks[0].Sample = sample
	p.Tracks[0].Parameters["mode"] = 1
	p.Tracks[0].Patterns[0].Steps[0].Active = true
	p.Tracks[0].Patterns[0].Steps[0].Length = 8
	p.Tracks[0].Patterns[0].Steps[2].Active = true
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	renderFrames(player, 16540)
	if player.active.tracks[0].voice.samplePlaying {
		t.Fatal("new short onset retained previous long gate")
	}
}

func Test303LengthKeepsSlideGateAcrossRest(t *testing.T) {
	p := NewProject()
	p.BPM = 120
	p.Tracks = []Track{NewTrack(Acid, 0)}
	steps := p.Tracks[0].Patterns[0].Steps
	steps[0].Active = true
	steps[0].Note = 36
	steps[0].Length = 3
	steps[2].Active = true
	steps[2].Note = 60
	steps[2].Slide = true
	short := p.Clone()
	short.Tracks[0].Patterns[0].Steps[0].Length = 0
	held, legacy := NewPlayer(), NewPlayer()
	held.Publish(p)
	legacy.Publish(short)
	held.SetPlaying(true)
	legacy.SetPlaying(true)
	renderFrames(held, 11025)
	renderFrames(legacy, 11025)
	a, b := make([]float32, 512), make([]float32, 512)
	held.Render(a)
	legacy.Render(b)
	difference := 0.
	for i := range a {
		difference += math.Abs(float64(a[i] - b[i]))
	}
	if difference < .001 {
		t.Fatal("long gate did not permit slide across the rest")
	}
	if allocs := testing.AllocsPerRun(100, func() { held.Render(a) }); allocs != 0 {
		t.Fatal("note lengths allocate while rendering")
	}
}
