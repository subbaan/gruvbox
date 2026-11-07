package groove

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"sync"
	"testing"
)

func fxProject(kind EffectKind) Project {
	p := NewProject()
	p.Tracks = p.Tracks[:1]
	p.BPM = 120
	p.Tracks[0].Effects = []Effect{NewEffect(kind)}
	p.Tracks[0].Patterns[0].Steps[0].Active = true
	return p
}
func TestEffectHistoryRoundTripAndLockRemoval(t *testing.T) {
	p := fxProject(Delay)
	e := p.Tracks[0].Effects[0]
	id := EffectLockID(e.ID, "feedback")
	p.Tracks[0].Patterns[3].Steps[5].Locks = map[string]float64{id: .7, "cutoff": .4}
	s := NewSession(p)
	if err := s.Do("Remove", func(p *Project) error { RemoveEffect(&p.Tracks[0], e.ID); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(s.Project.Tracks[0].Effects) != 0 || len(s.Project.Tracks[0].Patterns[3].Steps[5].Locks) != 1 {
		t.Fatal("effect removal did not clean its locks")
	}
	s.Undo()
	if !reflect.DeepEqual(p, s.Project) {
		t.Fatal("effect removal undo lost nested parameters/locks")
	}
	clone := p.Clone()
	clone.Tracks[0].Effects[0].Parameters["mix"] = .9
	if p.Tracks[0].Effects[0].Parameters["mix"] == .9 {
		t.Fatal("effect parameters shared with undo snapshot")
	}
	data, _ := json.Marshal(p)
	q, err := Decode(bytes.NewReader(data))
	if err != nil || !reflect.DeepEqual(p, q) {
		t.Fatalf("FX roundtrip: %v", err)
	}
}
func TestEffectValidationAndDiscovery(t *testing.T) {
	p := NewProject()
	before := p.Clone()
	for n := 0; n < 50; n++ {
		RandomiseEffects(&p.Tracks[0])
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(p.Tracks[0].Patterns, before.Tracks[0].Patterns) || p.Tracks[0].Level != before.Tracks[0].Level {
		t.Fatal("effect discovery changed groove/mixer")
	}
	ids := []string{}
	for _, e := range p.Tracks[0].Effects {
		ids = append(ids, e.ID)
	}
	RandomiseEffects(&p.Tracks[0])
	for i, e := range p.Tracks[0].Effects {
		if e.ID != ids[i] {
			t.Fatal("discovery replaced effect identities/order")
		}
	}
	bad := p.Clone()
	bad.Tracks[0].Effects[0].Parameters["mix"] = math.NaN()
	if bad.Validate() == nil {
		t.Fatal("non-finite FX accepted")
	}
	bad = p.Clone()
	bad.Tracks[0].Patterns[0].Steps[0].Locks = map[string]float64{EffectLockID("missing", "mix"): .5}
	if bad.Validate() == nil {
		t.Fatal("dangling effect lock accepted")
	}
	bad = p.Clone()
	bad.Tracks[0].Effects = append(bad.Tracks[0].Effects, bad.Tracks[0].Effects[0])
	if bad.Validate() == nil {
		t.Fatal("duplicate effect ID accepted")
	}
}
func TestStereoEffectTailsAndReset(t *testing.T) {
	for _, kind := range EffectKinds {
		e := NewEffect(kind)
		e.Parameters["mix"] = 1
		if kind.TempoSynced() {
			e.Parameters["sync"] = 0
			e.Parameters["time"] = .01
			e.Parameters["feedback"] = .5
		}
		state := newEffectVoice(kind)
		track := playTrack{track: Track{}, effects: []preparedEffect{{config: e, state: state}}}
		track.effectParameters(120)
		state.mix = 1
		energy, tail := 0., 0.
		for i := 0; i < 4000; i++ {
			input := float32(0)
			if i < 80 {
				input = .5
			}
			l, r := state.process(input, 0)
			if math.IsNaN(float64(l)) || math.IsInf(float64(l), 0) {
				t.Fatal("nonfinite effect")
			}
			if r != 0 {
				t.Fatal("FX merged stereo channels")
			}
			energy += math.Abs(float64(l))
			if i > 80 {
				tail += math.Abs(float64(l))
			}
		}
		if energy == 0 {
			t.Fatalf("%s failed to process impulse", kind)
		}
		if (kind.TempoSynced() || kind == Reverb) && tail == 0 {
			t.Fatalf("%s lost its tail", kind)
		}
		state.reset()
		state.mix = 1
		for i := 0; i < 4000; i++ {
			l, r := state.process(0, 0)
			if l != 0 || r != 0 {
				t.Fatalf("%s reset retained audio", kind)
			}
		}
	}
}
func TestEffectBypassAndStepLockReset(t *testing.T) {
	p := fxProject(Distortion)
	e := &p.Tracks[0].Effects[0]
	e.Parameters["drive"] = 3
	p.Tracks[0].Patterns[0].Steps[0].Locks = map[string]float64{EffectLockID(e.ID, "drive"): 9}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var out [BlockSize * 2]float32
	player.Render(out[:2])
	state := player.active.tracks[0].effects[0].state
	if state.values["drive"] != 9 {
		t.Fatal("effect lock was not applied")
	}
	player.Render(make([]float32, 5512*2))
	player.Render(out[:2])
	if state.values["drive"] != 3 {
		t.Fatal("effect lock leaked into following step")
	}
	p.Tracks[0].EffectsBypassed = true
	p.Tracks[0].Patterns[0].Steps[0].Locks = nil
	dryProject := p.Clone()
	dryProject.Tracks[0].Effects = nil
	bypass, dry := NewPlayer(), NewPlayer()
	bypass.Publish(p)
	dry.Publish(dryProject)
	bypass.SetPlaying(true)
	dry.SetPlaying(true)
	var a, b [BlockSize * 2]float32
	for n := 0; n < 100; n++ {
		bypass.Render(a[:])
		dry.Render(b[:])
		if a != b {
			t.Fatal("chain bypass changed dry audio")
		}
	}
	p.Tracks[0].EffectsBypassed = false
	p.Tracks[0].Effects[0].Enabled = false
	bypass = NewPlayer()
	bypass.Publish(p)
	bypass.SetPlaying(true)
	dry = NewPlayer()
	dry.Publish(dryProject)
	dry.SetPlaying(true)
	bypass.Render(a[:])
	dry.Render(b[:])
	if a != b {
		t.Fatal("unit bypass changed dry audio")
	}
}
func TestEffectRenderNoAllocationsAndConcurrentEdits(t *testing.T) {
	p := NewProject()
	for i := range p.Tracks {
		RandomiseEffects(&p.Tracks[i])
		Generate(&p.Tracks[i], 0, "16ths")
		for n := range p.Tracks[i].Effects {
			p.Tracks[i].Effects[n].Enabled = true
		}
	}
	p.Tracks[0].Patterns[0].Steps[0].Locks = map[string]float64{EffectLockID(p.Tracks[0].Effects[0].ID, "mix"): .4}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var out [BlockSize * 2]float32
	player.Render(out[:])
	if n := testing.AllocsPerRun(100, func() { player.Render(out[:]) }); n != 0 {
		t.Fatalf("FX render allocated %v", n)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 120; i++ {
			player.Render(out[:])
		}
	}()
	for i := 0; i < 60; i++ {
		p.Tracks[0].Effects[0].Parameters["mix"] = float64(i) / 100
		player.Publish(p)
	}
	wg.Wait()
	player.SetPlaying(false)
	player.Render(out[:])
	if !silentAfterMasterLatency(out[:]) {
		t.Fatal("stop retained effect output")
	}
}
func BenchmarkRenderSixTracksWithFX(b *testing.B) {
	p := NewProject()
	for i := range p.Tracks {
		for _, kind := range EffectKinds {
			p.Tracks[i].Effects = append(p.Tracks[i].Effects, NewEffect(kind))
		}
		Generate(&p.Tracks[i], 0, "16ths")
	}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var out [BlockSize * 2]float32
	player.Render(out[:])
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		player.Render(out[:])
	}
}
