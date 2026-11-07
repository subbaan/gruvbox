package groove

import "testing"

func TestLiveGestureGroupsValidatedEditsAndInterruptedUndo(t *testing.T) {
	p := NewProject()
	s := NewSession(p)
	original := p.Tracks[0].Parameters["resonance"]
	s.BeginGesture()
	for _, v := range []float64{.1, .3, .6, .9} {
		if err := s.Do("Resonance", func(p *Project) error { p.Tracks[0].Parameters["resonance"] = v; return nil }); err != nil {
			t.Fatal(err)
		}
		if s.Project.Tracks[0].Parameters["resonance"] != v {
			t.Fatal("gesture withheld live value")
		}
	}
	if !s.Dirty() {
		t.Fatal("gesture did not mark unsaved changes")
	}
	s.EndGesture()
	if !s.Undo() || s.Project.Tracks[0].Parameters["resonance"] != original {
		t.Fatal("one undo did not restore whole gesture")
	}
	if s.Undo() {
		t.Fatal("each drag update added a history entry")
	}
	if !s.Redo() || s.Project.Tracks[0].Parameters["resonance"] != .9 {
		t.Fatal("redo did not restore final value")
	}
	s.BeginGesture()
	s.Do("Cutoff", func(p *Project) error { p.Tracks[0].Parameters["cutoff"] = .2; return nil })
	if !s.Undo() || s.Project.Tracks[0].Parameters["cutoff"] != p.Tracks[0].Parameters["cutoff"] {
		t.Fatal("interrupted gesture undo failed")
	}
	s.BeginGesture()
	if err := s.Do("Bad", func(p *Project) error { p.Tracks[0].Parameters["cutoff"] = 2; return nil }); err == nil {
		t.Fatal("invalid live edit accepted")
	}
	s.EndGesture()
	if s.Project.Tracks[0].Parameters["cutoff"] != p.Tracks[0].Parameters["cutoff"] {
		t.Fatal("invalid gesture changed state")
	}
}
func TestAudioShortReadsKeepReservoirBoundedAndAllocateNothing(t *testing.T) {
	p := NewProject()
	Generate(&p.Tracks[0], 0, "16ths")
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	out := make([]byte, SourceBufferFrames*4)
	n, err := player.Read(out)
	if err != nil || n != BlockSize*4 || player.readBytes.Load() != uint64(n) {
		t.Fatalf("source refill %d: %v", n, err)
	}
	if n := testing.AllocsPerRun(100, func() { player.Read(out) }); n != 0 {
		t.Fatalf("stream refill allocated %v", n)
	}
}

func TestLiveParameterAdoptedBetweenSourceRefills(t *testing.T) {
	p := NewProject()
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	out := make([]byte, SourceBufferFrames*4)
	player.Read(out)
	p.Tracks[0].Parameters["resonance"] = .9
	player.Publish(p)
	player.Read(out)
	if player.active.project.Tracks[0].Parameters["resonance"] != .9 {
		t.Fatal("live value was not adopted at the next source refill")
	}
}
