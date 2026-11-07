package groove

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"reflect"
	"sync"
	"testing"
)

func TestDualTrackGateLocksSaveAndDiscovery(t *testing.T) {
	track := NewTrack(Dual, 0)
	track.Parameters["amp_attack"], track.Parameters["amp_sustain"], track.Parameters["amp_release"] = .001, .8, .02
	track.Parameters["tune"], track.Parameters["osc2_tune"] = -2, 7
	step := &track.Patterns[0].Steps[0]
	step.Active, step.Length = true, 3
	step.Locks = map[string]float64{"cutoff": .7, "pulse": .3}
	project := Project{FormatVersion: FormatVersion, BPM: 120, Tracks: []Track{track}}
	session := NewSession(project)
	if err := session.Do("Discover", func(p *Project) error { RandomiseSound(&p.Tracks[0]); return nil }); err != nil {
		t.Fatal(err)
	}
	discovered := session.Project.Tracks[0]
	if discovered.Parameters["tune"] != -2 || discovered.Parameters["osc2_tune"] != 7 || discovered.Parameters["amp_attack"] != .001 || discovered.Parameters["amp_sustain"] != .8 || !reflect.DeepEqual(discovered.Patterns, track.Patterns) {
		t.Fatal("randomisation changed tuning, envelope shape or notes")
	}
	data, _ := json.Marshal(session.Project)
	loaded, err := Decode(bytes.NewReader(data))
	if err != nil || !reflect.DeepEqual(loaded.Tracks, session.Project.Tracks) {
		t.Fatal("Dual sound/locks/LFOs did not round trip", err)
	}
	if !session.Undo() || !reflect.DeepEqual(session.Project, project) {
		t.Fatal("discovery undo failed")
	}
	player := NewPlayer()
	player.Publish(project)
	player.SetPlaying(true)
	// Each step is 5512.5 frames. Three steps hold the gate beyond the first
	// two empty steps, then the short release decays to exact silence.
	player.Render(make([]float32, 12000*2))
	v := player.active.tracks[0].voice
	if v.dual == nil || v.lfos()[0] == nil || v.gateRemaining != 1 {
		t.Fatal("Dual gate or modulation was not wired")
	}
	var block [BlockSize * 2]float32
	player.Render(make([]float32, 7000*2))
	player.Render(block[:])
	for _, value := range block {
		if value != 0 {
			t.Fatal("Dual gate failed to release on its empty step", value)
		}
	}
	// Selector fractions must be rejected, rather than silently truncated by DSP.
	for _, id := range []string{"osc1_wave", "osc2_wave", "slope", "legato"} {
		bad := project.Clone()
		bad.Tracks[0].Parameters[id] = .5
		if err := bad.Validate(); err == nil {
			t.Fatal("fractional selector accepted", id)
		}
	}
}

func TestDualLivePublicationAndExportTail(t *testing.T) {
	track := NewTrack(Dual, 0)
	Generate(&track, 0, "Bassline")
	track.LFOs[0].Enabled, track.LFOs[1].Enabled = true, true
	track.LFOs[1].Sync = 2
	p := Project{FormatVersion: FormatVersion, BPM: 126, Tracks: []Track{track}}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var out [BlockSize * 2]float32
		for i := 0; i < 200; i++ {
			player.Render(out[:])
		}
	}()
	for i := 0; i < 80; i++ {
		p.Tracks[0].Parameters["resonance"] = float64(i%20) / 20
		p.Tracks[0].Parameters["cutoff"] = .3 + float64(i%20)/40
		player.Publish(p)
	}
	wg.Wait()
	var out [BlockSize * 2]float32
	if n := testing.AllocsPerRun(100, func() { player.Render(out[:]) }); n != 0 {
		t.Fatal("Dual render allocates", n)
	}
	// End-of-export note-off must release a sustained Dual voice, including its
	// filter envelope, without scheduling another note during the tail.
	player.endExportNotes()
	for i := 0; i < 150; i++ {
		player.renderTail(out[:])
	}
	energy := 0.0
	for _, v := range out {
		energy += math.Abs(float64(v))
	}
	if energy != 0 {
		t.Fatal("export tail left the Dual envelope gated", energy)
	}
	var wav bytes.Buffer
	if err := RenderWAV(context.Background(), &wav, p, false, .5, nil); err != nil || wav.Len() < 44 {
		t.Fatal("Dual offline export failed", err)
	}
	player.SetPlaying(false)
	player.Render(out[:])
	if !silentAfterMasterLatency(out[:]) {
		t.Fatal("stop left Dual audio")
	}
}

func TestDrumFillLeavesDualPatternsAlone(t *testing.T) {
	p := NewProject()
	dual := NewTrack(Dual, 0)
	Generate(&dual, 0, "Bassline")
	p.Tracks = append(p.Tracks, dual)
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	if err := player.QueuePerformance(p, DrumFill, ""); err != nil {
		t.Fatal(err)
	}
	take := player.performancePending.Load()
	if !reflect.DeepEqual(take.patterns[dual.ID], dual.Patterns[0]) {
		t.Fatal("drum fill rewrote synth notes")
	}
}
