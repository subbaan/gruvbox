package groove

import (
	"reflect"
	"sync"
	"testing"
)

func TestPerformanceFillLaunchReturnAndKeep(t *testing.T) {
	project := NewProject()
	project.BPM = 300
	project.Tracks = []Track{NewTrack(Tom, 0)}
	original := project.Clone()
	player := NewPlayer()
	player.Publish(project)
	player.SetPlaying(true)
	renderFrames(player, 4410)
	if err := player.QueuePerformance(project, DrumFill, ""); err != nil {
		t.Fatal(err)
	}
	bar := SampleRate * 240 / 300
	renderFrames(player, bar-4410)
	if player.RunningPerformance() != NoPerformance || player.QueuedPerformance() != DrumFill {
		t.Fatal("cue launched before bar")
	}
	renderFrames(player, 1)
	if player.RunningPerformance() != DrumFill || player.QueuedPerformance() != NoPerformance || player.CurrentPattern() != 0 {
		t.Fatal("fill did not launch on bar")
	}
	if !reflect.DeepEqual(project, original) || !reflect.DeepEqual(player.active.project, original) {
		t.Fatal("cue changed saved patterns")
	}
	// Live controls adopt while the prepared notes remain active.
	project.Tracks[0].Parameters["decay"] = .8
	player.Publish(project)
	renderFrames(player, 1)
	if player.RunningPerformance() != DrumFill || player.active.tracks[0].voice.drum == nil {
		t.Fatal("live edit stopped cue")
	}
	if allocs := testing.AllocsPerRun(10, func() { var out [BlockSize * 2]float32; player.Render(out[:]) }); allocs != 0 {
		t.Fatal("performance render allocates")
	}
	renderFrames(player, bar-2-11*BlockSize)
	if player.RunningPerformance() != DrumFill {
		t.Fatal("cue returned early")
	}
	renderFrames(player, 1)
	if player.RunningPerformance() != NoPerformance || !player.HasPerformanceTake() {
		t.Fatal("cue did not return after one bar")
	}
	session := NewSession(project)
	if err := session.Do("Keep fill", func(p *Project) error { return player.KeepPerformance(p, 1) }); err != nil {
		t.Fatal(err)
	}
	steps := session.Project.Tracks[0].Patterns[1].Steps
	for i := 12; i < 16; i++ {
		if !steps[i].Active || steps[i].Locks["pitch"] == 0 {
			t.Fatal("saved fill lacks tom notes")
		}
	}
	if session.Project.Pattern != 0 {
		t.Fatal("keeping interrupted main groove")
	}
	session.Undo()
	if !reflect.DeepEqual(session.Project, project) {
		t.Fatal("keep undo failed")
	}
	player.SetPlaying(false)
	renderFrames(player, 1)
	if err := player.KeepPerformance(&project, 1); err != nil {
		t.Fatal("cannot keep take after stopping")
	}
}

func TestPerformanceLatestQueueCancellationAndManualSelection(t *testing.T) {
	project := NewProject()
	project.BPM = 300
	player := NewPlayer()
	player.Publish(project)
	player.SetPlaying(true)
	renderFrames(player, 1)
	player.QueuePerformance(project, DrumFill, "")
	player.QueuePerformance(project, TrackVariation, project.Tracks[0].ID)
	if player.QueuedPerformance() != TrackVariation {
		t.Fatal("latest cue did not replace queue")
	}
	player.CancelPerformance()
	renderFrames(player, SampleRate*240/300)
	if player.RunningPerformance() != NoPerformance {
		t.Fatal("cancelled cue played")
	}
	player.QueuePerformance(project, DrumFill, "")
	project.Pattern = 2
	player.Publish(project)
	renderFrames(player, 1)
	if player.QueuedPerformance() != NoPerformance {
		t.Fatal("manual launch retained old cue")
	}
	player.QueuePerformance(project, GrooveVariation, "")
	player.SetPlaying(false)
	renderFrames(player, 1)
	player.SetPlaying(true)
	renderFrames(player, SampleRate*240/300+1)
	if player.RunningPerformance() != NoPerformance || player.QueuedPerformance() != NoPerformance {
		t.Fatal("stopped queue returned on restart")
	}
	player.SetPlaying(false)
	if err := player.QueuePerformance(project, DrumFill, ""); err == nil {
		t.Fatal("queued while stopped")
	}
}

func TestPerformanceSongProgressAndShortPolymeter(t *testing.T) {
	project := NewProject()
	project.BPM = 300
	project.Tracks = []Track{NewTrack(Tom, 0)}
	project.Tracks[0].Patterns[0].Steps = project.Tracks[0].Patterns[0].Steps[:4]
	project.Song = []SongSection{NewSongSection(0), NewSongSection(1), NewSongSection(2)}
	for i := range project.Song {
		project.Song[i].Repeats = 1
	}
	player := NewPlayer()
	player.Publish(project)
	player.SetSongMode(true)
	player.SetPlaying(true)
	renderFrames(player, 1)
	player.QueuePerformance(project, DrumFill, "")
	bar := SampleRate * 240 / 300
	renderFrames(player, bar)
	if player.RunningPerformance() != DrumFill || player.SongSection() != 1 {
		t.Fatal("fill paused song progression")
	}
	pattern := player.performancePattern(&player.active.tracks[0])
	if len(pattern.Steps) != 16 || !pattern.Steps[15].Active {
		t.Fatal("short pattern missed final beat fill")
	}
	renderFrames(player, bar)
	if player.RunningPerformance() != NoPerformance || player.SongSection() != 2 || player.CurrentPattern() != 2 {
		t.Fatal("fill failed to return to advancing song")
	}
}

func TestPerformanceVariationsScopeAndInvalidEffectLocks(t *testing.T) {
	project := NewProject()
	project.BPM = 300
	project.Tracks[0].Effects = []Effect{NewEffect(Filter)}
	id := project.Tracks[0].Effects[0].ID
	project.Tracks[0].Patterns[0].Steps[0].Locks = map[string]float64{EffectLockID(id, "cutoff"): 900}
	player := NewPlayer()
	player.Publish(project)
	player.SetPlaying(true)
	renderFrames(player, 1)
	player.QueuePerformance(project, TrackVariation, project.Tracks[0].ID)
	take := player.performancePending.Load()
	if reflect.DeepEqual(take.patterns[project.Tracks[0].ID], project.Tracks[0].Patterns[0]) {
		t.Fatal("variation did not change selected track")
	}
	for _, track := range project.Tracks[1:] {
		if !reflect.DeepEqual(take.patterns[track.ID], track.Patterns[0]) {
			t.Fatal("track variation changed another track")
		}
	}
	renderFrames(player, SampleRate*240/300)
	RemoveEffect(&project.Tracks[0], id)
	if err := player.KeepPerformance(&project, 1); err != nil || project.Validate() != nil {
		t.Fatal("keep retained removed effect lock")
	}
	if err := player.KeepPerformance(&project, 1); err == nil {
		t.Fatal("keep overwrote occupied slot")
	}
	player.ClearPerformanceTake()
	if player.HasPerformanceTake() {
		t.Fatal("new project retained previous take")
	}
}

func TestConcurrentPerformanceQueueAndRender(t *testing.T) {
	project := NewProject()
	project.BPM = 300
	player := NewPlayer()
	player.Publish(project)
	player.SetPlaying(true)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var out [BlockSize * 2]float32
		for i := 0; i < 1000; i++ {
			player.Render(out[:])
		}
	}()
	for i := 0; i < 100; i++ {
		player.QueuePerformance(project, GrooveVariation, "")
		if i%3 == 0 {
			player.CancelPerformance()
		}
		player.Publish(project)
		_ = player.RunningPerformance()
		_ = player.QueuedPerformance()
	}
	wg.Wait()
}

func TestPerformancePreviousProjectRequestCannotResurrectTake(t *testing.T) {
	project := NewProject()
	project.BPM = 300
	player := NewPlayer()
	player.Publish(project)
	player.SetPlaying(true)
	renderFrames(player, 1)
	player.QueuePerformance(project, DrumFill, "")
	old := player.performancePending.Load()
	player.ClearPerformanceTake()
	// Simulate an old control-side preparation finishing after a project reset.
	player.performancePending.Store(old)
	if player.QueuedPerformance() != NoPerformance {
		t.Fatal("old project cue shown")
	}
	renderFrames(player, SampleRate*240/300)
	if player.RunningPerformance() != NoPerformance || player.HasPerformanceTake() {
		t.Fatal("old request resurrected after project reset")
	}
}
