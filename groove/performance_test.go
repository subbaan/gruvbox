package groove

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

func renderFrames(p *Player, frames int) {
	var out [BlockSize * 2]float32
	for frames > 0 {
		n := min(frames, BlockSize)
		p.Render(out[:n*2])
		frames -= n
	}
}
func TestQueuedPatternSwitchesExactlyAtBarAndKeepsLiveEdits(t *testing.T) {
	project := NewProject()
	project.BPM = 300
	p := NewPlayer()
	p.Publish(project)
	p.SetPlaying(true)
	renderFrames(p, 4410)
	project.Pattern = 1
	p.Publish(project)
	renderFrames(p, 1)
	if p.CurrentPattern() != 0 || p.QueuedPattern() != 1 {
		t.Fatal("pattern selection interrupted current bar")
	}
	project.Tracks[0].Parameters["resonance"] = .91
	p.Publish(project)
	renderFrames(p, 1)
	if p.active.tracks[0].voice.acid == nil || p.CurrentPattern() != 0 {
		t.Fatal("live edit changed queued timing")
	}
	bar := SampleRate * 240 / 300
	renderFrames(p, bar-4412)
	if p.CurrentPattern() != 0 {
		t.Fatal("pattern switched early")
	}
	renderFrames(p, 1)
	if p.CurrentPattern() != 1 || p.QueuedPattern() != -1 || p.Position(project.Tracks[0].ID) != 0 {
		t.Fatal("pattern did not switch/reset at bar boundary")
	}
	project.Pattern = 2
	p.Publish(project)
	renderFrames(p, 100)
	project.Pattern = 1
	p.Publish(project)
	renderFrames(p, 1)
	if p.QueuedPattern() != -1 {
		t.Fatal("latest selection did not cancel pending launch")
	}
	p.SetPlaying(false)
	project.Pattern = 3
	p.Publish(project)
	renderFrames(p, 1)
	if p.CurrentPattern() != 3 {
		t.Fatal("stopped pattern selection was delayed")
	}
}
func TestSongRepeatsSectionsLoopsAndManualExit(t *testing.T) {
	project := NewProject()
	project.BPM = 300
	first, second := NewSongSection(1), NewSongSection(2)
	first.Repeats = 2
	second.Repeats = 1
	project.Song = []SongSection{first, second}
	p := NewPlayer()
	p.Publish(project)
	p.SetSongMode(true)
	p.SetPlaying(true)
	renderFrames(p, 1)
	if p.CurrentPattern() != 1 || p.SongSection() != 0 || p.SongBar() != 1 {
		t.Fatal("song did not start at first section")
	}
	bar := SampleRate * 240 / 300
	renderFrames(p, bar)
	if p.CurrentPattern() != 1 || p.SongBar() != 2 {
		t.Fatal("repeat count was not measured in bars")
	}
	renderFrames(p, bar)
	if p.CurrentPattern() != 2 || p.SongSection() != 1 {
		t.Fatal("second section did not launch")
	}
	renderFrames(p, bar)
	if p.CurrentPattern() != 1 || p.SongSection() != 0 {
		t.Fatal("song did not loop")
	}
	p.SetSongMode(false)
	project.Pattern = 3
	p.Publish(project)
	renderFrames(p, 10)
	if p.CurrentPattern() != 1 {
		t.Fatal("manual exit cut song section mid-bar")
	}
	renderFrames(p, bar-10)
	if p.CurrentPattern() != 3 || p.SongSection() != -1 {
		t.Fatal("manual launch did not leave song mode")
	}
	if allocs := testing.AllocsPerRun(100, func() { var out [BlockSize * 2]float32; p.Render(out[:]) }); allocs != 0 {
		t.Fatalf("performance playback allocated %v", allocs)
	}
}
func TestSongHistoryValidationAndPersistence(t *testing.T) {
	project := NewProject()
	s := NewSongSection(2)
	s.Name = "Intro"
	project.Song = []SongSection{s}
	session := NewSession(project)
	if err := session.Do("Bars", func(p *Project) error { p.Song[0].Repeats = 8; return nil }); err != nil {
		t.Fatal(err)
	}
	if project.Song[0].Repeats != 4 {
		t.Fatal("history aliases song")
	}
	data, _ := json.Marshal(session.Project)
	loaded, err := Decode(bytes.NewReader(data))
	if err != nil || loaded.Song[0].Name != "Intro" || loaded.Song[0].Repeats != 8 {
		t.Fatalf("song round trip: %v", err)
	}
	session.Undo()
	if session.Project.Song[0].Repeats != 4 {
		t.Fatal("song undo failed")
	}
	project.Song = append(project.Song, s)
	if project.Validate() == nil {
		t.Fatal("duplicate song identity accepted")
	}
	project.Song = project.Song[:1]
	project.Song[0].Repeats = 0
	if project.Validate() == nil {
		t.Fatal("zero bar count accepted")
	}
}
func TestAuditionIsIndependentOneShotAndAllocationFree(t *testing.T) {
	project := NewProject()
	track := NewTrack(Sampler, 6)
	values := make([]int16, 8000)
	for i := range values {
		values[i] = 8192
	}
	track.Sample = pcmSample(t, 1, 8000, values...)
	track.Parameters["mode"] = 2
	project.Tracks = append(project.Tracks, track)
	p := NewPlayer()
	p.Publish(project)
	if err := p.Audition(track); err != nil {
		t.Fatal(err)
	}
	var out [BlockSize * 2]float32
	p.Render(out[:])
	if p.Playing() || !p.Auditioning() || p.Position(track.ID) != -1 || out[0] == 0 {
		t.Fatal("audition did not sound independently while stopped")
	}
	before := p.active.tracks[len(project.Tracks)-1].voice.cursor
	p.SetPlaying(true)
	p.Render(out[:])
	if p.active.tracks[len(project.Tracks)-1].voice.cursor != before {
		t.Fatal("audition altered sequenced voice")
	}
	if allocs := testing.AllocsPerRun(100, func() { p.Render(out[:]) }); allocs != 0 {
		t.Fatalf("audition render allocated %v", allocs)
	}
	renderFrames(p, SampleRate)
	if p.Auditioning() {
		t.Fatal("audition looped instead of playing once")
	}
	p.Audition(track)
	p.Render(out[:])
	p.StopAudition()
	p.SetPlaying(false)
	p.Render(out[:])
	for _, sample := range out {
		if sample != 0 || math.IsNaN(float64(sample)) {
			t.Fatal("stop preview left audio")
		}
	}
}

func TestSongReorderKeepsSectionIdentityAndRemovalIsSafe(t *testing.T) {
	project := NewProject()
	project.BPM = 300
	a, b := NewSongSection(1), NewSongSection(2)
	a.Repeats = 2
	b.Repeats = 1
	project.Song = []SongSection{a, b}
	p := NewPlayer()
	p.Publish(project)
	p.SetSongMode(true)
	p.SetPlaying(true)
	renderFrames(p, 1)
	project.Song[0], project.Song[1] = project.Song[1], project.Song[0]
	p.Publish(project)
	renderFrames(p, 1)
	if p.SongSection() != 1 || p.CurrentPattern() != 1 {
		t.Fatal("reorder moved playback to a different section")
	}
	project.Song = project.Song[:1]
	p.Publish(project)
	renderFrames(p, 1)
	if p.CurrentPattern() != 1 {
		t.Fatal("removing section interrupted the bar")
	}
	renderFrames(p, SampleRate*240/300)
	if p.CurrentPattern() != 2 || p.SongSection() != 0 {
		t.Fatal("removed section did not recover at next bar")
	}
	project.Song = nil
	p.Publish(project)
	renderFrames(p, SampleRate*240/300)
	if p.SongMode() || p.SongSection() != -1 {
		t.Fatal("empty song did not fall back safely")
	}
}
func TestPatternPositionHistoryWaitsForAudibleBoundary(t *testing.T) {
	v := &voice{}
	v.recordPatternPosition(0, 15, 0)
	v.recordPatternPosition(100, 0, 1)
	if v.patternAt(99) != 0 || v.positionAt(99) != 15 || v.patternAt(100) != 1 || v.positionAt(100) != 0 {
		t.Fatal("pattern playhead ignored queued source audio")
	}
}
func TestWaveformViewportKeepsLocalPeaks(t *testing.T) {
	sample := pcmSample(t, 1, 8000, 8192, 8192, -16384, -16384)
	first, second := sample.PeaksRange(0, .5), sample.PeaksRange(.5, 1)
	for _, peak := range first {
		if peak[0] != 0 || peak[1] != .25 {
			t.Fatal("first viewport contains outside audio")
		}
	}
	for _, peak := range second {
		if peak[0] != -.5 || peak[1] != 0 {
			t.Fatal("second viewport contains outside audio")
		}
	}
}
