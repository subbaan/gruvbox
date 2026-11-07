package groove

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/subbaan/gruvbox/synth/drums"
)

var addedDrums = []Kind{Clap, Rimshot, Tom, Crash}

func TestAddedDrumsSoundModulateSaveAndUndo(t *testing.T) {
	for _, kind := range addedDrums {
		t.Run(string(kind), func(t *testing.T) {
			project := NewProject()
			track := NewTrack(kind, 0)
			project.Tracks = []Track{track}
			Generate(&project.Tracks[0], 0, Actions(kind)[0])
			project.Tracks[0].LFOs[0].Enabled = true
			project.Tracks[0].LFOs[0].Sync = 0
			project.Tracks[0].LFOs[1].Enabled = true
			project.Tracks[0].LFOs[1].Sync = 2
			session := NewSession(project)
			if err := session.Do("Random sound", func(p *Project) error { RandomiseSound(&p.Tracks[0]); return nil }); err != nil {
				t.Fatal(err)
			}
			saved, _ := json.Marshal(session.Project)
			loaded, err := Decode(bytes.NewReader(saved))
			if err != nil || !reflect.DeepEqual(loaded.Tracks[0], session.Project.Tracks[0]) {
				t.Fatalf("sound/LFO save: %v", err)
			}
			session.Undo()
			if !reflect.DeepEqual(session.Project, project) {
				t.Fatal("sound undo failed")
			}
			player := NewPlayer()
			player.Publish(loaded)
			player.SetPlaying(true)
			var out [BlockSize * 2]float32
			player.Render(out[:])
			voice := player.active.tracks[0].voice
			if voice.drum == nil || voice.lfos()[0] == nil || voice.lfos()[1] == nil {
				t.Fatal("drum or LFO not connected")
			}
			energy := 0.
			for n := 0; n < 160; n++ {
				player.Render(out[:])
				for _, value := range out {
					if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || math.Abs(float64(value)) > .98 {
						t.Fatal("invalid audio")
					}
					energy += math.Abs(float64(value))
				}
			}
			if energy == 0 {
				t.Fatal("instrument is silent")
			}
			if allocs := testing.AllocsPerRun(100, func() { player.Render(out[:]) }); allocs != 0 {
				t.Fatalf("render allocates %v", allocs)
			}
			player.SetPlaying(false)
			player.Render(out[:])
			for _, value := range out {
				if value != 0 {
					t.Fatal("stopped drum left audio")
				}
			}
		})
	}
}
func TestAddedDrumParameterLocksAndEveryLFOTarget(t *testing.T) {
	for _, kind := range addedDrums {
		for target := 1; target < len(LFOTargets(kind)); target++ {
			p := NewProject()
			track := NewTrack(kind, 0)
			p.Tracks = []Track{track}
			param := Parameters(kind)[0]
			p.Tracks[0].Patterns[0].Steps[0].Active = true
			p.Tracks[0].Patterns[0].Steps[0].Locks = map[string]float64{param.ID: param.Max}
			p.Tracks[0].LFOs[0] = LFO{Enabled: true, Target: target, Waveform: 5, Rate: 5, Depth: .8, Division: .25, Steps: 4}
			player := NewPlayer()
			player.Publish(p)
			player.SetPlaying(true)
			var out [BlockSize * 2]float32
			player.Render(out[:])
			if player.active.tracks[0].voice.lfos()[0].GetValue() == 0 && param.Min > 0 {
				t.Fatalf("%s target %d missing range", kind, target)
			}
			for _, value := range out {
				if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
					t.Fatalf("%s LFO target %d produced invalid audio", kind, target)
				}
			}
		}
	}
	// Locks arrive before triggering the new drum, and base values return next step.
	p := NewProject()
	p.BPM = 300
	p.Tracks = []Track{NewTrack(Tom, 0)}
	p.Tracks[0].Patterns[0].Steps[0].Active = true
	p.Tracks[0].Patterns[0].Steps[0].Locks = map[string]float64{"pitch": 250}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	renderFrames(player, 1)
	tom := player.active.tracks[0].voice.drum.(*drums.TomDrum)
	if tom.GetPitch() != 250 {
		t.Fatal("tom pitch lock missing")
	}
	renderFrames(player, SampleRate*60/300/4+1)
	if tom.GetPitch() != 100 {
		t.Fatal("tom pitch lock did not reset")
	}
}
func TestTomFillAndCrashDownbeatGenerators(t *testing.T) {
	tom := NewTrack(Tom, 0)
	Generate(&tom, 0, "Fill")
	for index, step := range tom.Patterns[0].Steps {
		if step.Active != (index >= 12) {
			t.Fatal("tom fill did not stay in final beat")
		}
		if step.Active && (step.Locks["pitch"] < 40 || step.Locks["pitch"] > 300) {
			t.Fatal("tom fill lacks valid pitch locks")
		}
	}
	crash := NewTrack(Crash, 0)
	Generate(&crash, 0, "Downbeat")
	for index, step := range crash.Patterns[0].Steps {
		if step.Active != (index == 0) {
			t.Fatal("crash downbeat added repeated hits")
		}
	}
}
func BenchmarkAddedDrums(b *testing.B) {
	p := NewProject()
	p.Tracks = nil
	for i, kind := range addedDrums {
		track := NewTrack(kind, i)
		Generate(&track, 0, "Busy")
		track.LFOs[0].Enabled = true
		p.Tracks = append(p.Tracks, track)
	}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var out [BlockSize * 2]float32
	player.Render(out[:])
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		player.Render(out[:])
	}
}
