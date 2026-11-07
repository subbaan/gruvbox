package groove

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestPatternStateStoresAllControlsWithoutAliasingOrChangingNotes(t *testing.T) {
	p := NewProject()
	p.Tracks = nil
	for i, kind := range Kinds {
		track := NewTrack(kind, i)
		track.Mute, track.Solo, track.Level, track.Pan = i%2 == 0, i%3 == 0, .4, -.3
		track.Effects = []Effect{NewEffect(Chorus), NewEffect(Delay)}
		track.Effects[0].LFOs[0].Enabled, track.Effects[0].LFOs[0].Target = true, 1
		if kind != Sampler {
			track.LFOs[0].Enabled, track.LFOs[0].Target = true, 1
		}
		track.EffectsBypassed = true
		track.Patterns[0].Steps[2].Locks = map[string]float64{EffectLockID(track.Effects[0].ID, "mix"): .35}
		track.Patterns[0].Steps[2].Active = true
		if kind == Sampler {
			pcm := make([]byte, 1600)
			for i := 0; i < 800; i++ {
				binary.LittleEndian.PutUint16(pcm[i*2:], uint16(i*20))
			}
			var err error
			track.Sample, err = SampleFromPCM("stored take", pcm, 44100, 1)
			if err != nil {
				t.Fatal(err)
			}
			track.Parameters["start"], track.Parameters["end"], track.Parameters["reverse"], track.Parameters["gain_db"], track.Parameters["fade_out"] = .1, .8, 1, -3, 8
		}
		p.Tracks = append(p.Tracks, track)
	}
	before := p.Clone()
	session := NewSession(p)
	if err := session.Do("Store", func(p *Project) error { return StorePatternState(p, 0, "") }); err != nil {
		t.Fatal(err)
	}
	stored := session.Project.Clone()
	for i, track := range stored.Tracks {
		state := track.Patterns[0].State
		if !reflect.DeepEqual(state, CaptureTrackState(before.Tracks[i])) || !reflect.DeepEqual(track.Patterns[0].Steps, before.Tracks[i].Patterns[0].Steps) {
			t.Fatal("store missed controls or changed note data", track.Kind)
		}
	}
	if err := session.Do("Live edits", func(p *Project) error {
		for i := range p.Tracks {
			track := &p.Tracks[i]
			track.Level, track.Pan, track.Mute, track.Solo = .9, .7, false, false
			track.Parameters[Parameters(track.Kind)[0].ID] = Parameters(track.Kind)[0].Min
			track.Effects[0].Parameters["mix"] = .9
			track.LFOs, track.Sample, track.EffectsBypassed = DefaultLFOs(), nil, false
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for i, track := range session.Project.Tracks {
		if !reflect.DeepEqual(track.Patterns[0].State, stored.Tracks[i].Patterns[0].State) {
			t.Fatal("live edits overwrote stored state")
		}
	}
	live := session.Project.Clone()
	if err := session.Do("Recall", func(p *Project) error { return RecallPatternState(p, 0, "") }); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(session.Project, stored) {
		t.Fatal("recall missed controls")
	}
	if !session.Undo() || !reflect.DeepEqual(session.Project, live) || !session.Redo() || !reflect.DeepEqual(session.Project, stored) {
		t.Fatal("recall was not one undo/redo")
	}
	data, err := json.Marshal(stored)
	loaded, decodeErr := Decode(bytes.NewReader(data))
	if err != nil || decodeErr != nil || !reflect.DeepEqual(stored, loaded) {
		t.Fatalf("snapshot roundtrip failed: %v %v", err, decodeErr)
	}
	if err := Save(filepath.Join(t.TempDir(), "states.gruvbox3"), stored); err != nil {
		t.Fatal(err)
	}
	clone := stored.Clone()
	clone.Tracks[0].Patterns[0].State.Parameters["cutoff"] = .1
	clone.Tracks[0].Patterns[0].State.Effects[0].Parameters["mix"] = .1
	if stored.Tracks[0].Patterns[0].State.Parameters["cutoff"] == .1 || stored.Tracks[0].Patterns[0].State.Effects[0].Parameters["mix"] == .1 {
		t.Fatal("history shares nested stored maps")
	}
}

func TestPatternStateCopyFXLifetimeClearAndValidation(t *testing.T) {
	p := NewProject()
	p.Tracks = p.Tracks[:1]
	track := &p.Tracks[0]
	fx := NewEffect(Delay)
	track.Effects = []Effect{fx}
	key := EffectLockID(fx.ID, "mix")
	track.Patterns[0].Steps[0].Locks = map[string]float64{key: .4}
	if err := StorePatternState(&p, 0, ""); err != nil {
		t.Fatal(err)
	}
	if err := CopyPattern(&p, 0, 1, ""); err != nil {
		t.Fatal(err)
	}
	track.Patterns[1].State.Parameters["cutoff"] = .1
	if track.Patterns[0].State.Parameters["cutoff"] == .1 {
		t.Fatal("copy aliases source state")
	}
	RemoveEffect(track, fx.ID)
	if p.Validate() != nil || track.Patterns[0].Steps[0].Locks[key] != .4 {
		t.Fatal("removing live FX damaged stored chain/locks")
	}
	if err := SelectPattern(&p, 1); err != nil || p.Validate() != nil || len(track.Effects) != 1 || track.Effects[0].ID != fx.ID {
		t.Fatal("recall failed to restore FX identity/locks")
	}
	RemoveEffect(track, fx.ID)
	if err := StorePatternState(&p, 1, ""); err != nil || len(track.Patterns[1].Steps[0].Locks) != 0 || track.Patterns[0].Steps[0].Locks[key] != .4 {
		t.Fatal("overwriting snapshot cleaned wrong pattern's locks")
	}
	if err := ClearPatternState(&p, 0); err != nil || p.Validate() != nil || len(track.Patterns[0].Steps[0].Locks) != 0 {
		t.Fatal("clear left orphan FX locks")
	}
	if NextEmptyPattern(p, 0) != 2 {
		t.Fatal("empty-slot search would overwrite a stored state")
	}
	for _, bad := range []func(*TrackState){
		func(s *TrackState) { s.Level = math.NaN() }, func(s *TrackState) { s.Pan = 2 },
		func(s *TrackState) { s.Parameters["cutoff"] = 2 }, func(s *TrackState) { s.LFOs[0].Target = 999 },
		func(s *TrackState) { s.Effects = []Effect{NewEffect(Delay)}; s.Effects[0].Parameters["feedback"] = 9 },
	} {
		copy := p.Clone()
		bad(copy.Tracks[0].Patterns[1].State)
		if copy.Validate() == nil {
			t.Fatal("accepted invalid stored controls")
		}
	}
	if StorePatternState(&p, -1, "") == nil || RecallPatternState(&p, 8, "") == nil || StorePatternState(&p, 0, "missing") == nil {
		t.Fatal("invalid snapshot target accepted")
	}
}

func TestPatternStateRecallAtLaunchBoundaryAndSamplerAsset(t *testing.T) {
	p := NewProject()
	p.BPM = 300
	p.Tracks = []Track{NewTrack(Acid, 0), NewTrack(Sampler, 1)}
	firstSample, _ := SampleFromPCM("first", []byte{100, 0, 200, 0, 100, 0, 200, 0}, 44100, 1)
	secondSample, _ := SampleFromPCM("second", []byte{200, 0, 100, 0, 200, 0, 100, 0}, 44100, 1)
	p.Tracks[1].Sample = firstSample
	p.Tracks[0].Parameters["cutoff"] = .2
	StorePatternState(&p, 0, "")
	p.Tracks[0].Parameters["cutoff"], p.Tracks[0].Mute, p.Tracks[0].Pan = .8, true, 1
	p.Tracks[0].Effects = []Effect{NewEffect(BitCrusher)}
	p.Tracks[0].Effects[0].Parameters["bits"] = 5
	p.Tracks[0].LFOs[0].Enabled, p.Tracks[0].LFOs[0].Target = true, 1
	p.Tracks[1].Sample = secondSample
	StorePatternState(&p, 1, "")
	SelectPattern(&p, 0)
	p.Tracks[0].Parameters["cutoff"] = .35 // Live outgoing tweak, deliberately not stored.
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	renderFrames(player, 100)
	SelectPattern(&p, 1)
	player.Publish(p)
	renderFrames(player, 1)
	if player.CurrentPattern() != 0 || player.active.tracks[0].track.Parameters["cutoff"] != .35 || player.active.tracks[0].left == 0 || player.active.tracks[1].voice.sample != firstSample {
		t.Fatal("incoming recall leaked into outgoing bar")
	}
	// Further incoming edits must also wait for the launch.
	p.Tracks[0].Parameters["cutoff"] = .9
	player.Publish(p)
	renderFrames(player, SampleRate*240/300-101)
	if player.CurrentPattern() != 0 {
		t.Fatal("switched early")
	}
	renderFrames(player, 1)
	current := player.active.tracks[0]
	if player.CurrentPattern() != 1 || current.track.Parameters["cutoff"] != .9 || current.left != 0 || current.track.Pan != 1 || len(current.effects) != 1 || current.effects[0].config.Parameters["bits"] != 5 || !current.track.LFOs[0].Enabled || player.active.tracks[1].voice.sample != secondSample {
		t.Fatal("full controls did not switch at the note boundary")
	}
	SelectPattern(&p, 0)
	player.Publish(p)
	renderFrames(player, SampleRate*240/300)
	if player.active.tracks[0].track.Parameters["cutoff"] != .2 || player.active.tracks[0].left == 0 || player.active.tracks[1].voice.sample != firstSample {
		t.Fatal("return did not restore stored state")
	}
}

func TestSongPatternStatesLiveTweaksOverridesAndAllocationFreeTransitions(t *testing.T) {
	p := NewProject()
	p.BPM = 300
	p.Tracks = p.Tracks[:2]
	p.Tracks[0].Parameters["cutoff"], p.Tracks[0].Mute = .2, true
	p.Tracks[0].Effects = []Effect{NewEffect(Chorus)}
	StorePatternState(&p, 0, "")
	p.Tracks[0].Parameters["cutoff"], p.Tracks[0].Mute = .7, false
	p.Tracks[0].Effects = []Effect{NewEffect(BitCrusher)}
	StorePatternState(&p, 1, "")
	SelectPattern(&p, 0)
	p.Tracks[0].Parameters["cutoff"] = .9
	first, second := NewSongSection(0), NewSongSection(1)
	first.Repeats, second.Repeats = 1, 1
	first.Mutes = map[string]bool{p.Tracks[0].ID: false}
	p.Song = []SongSection{first, second}
	player := NewPlayer()
	player.Publish(p)
	player.SetSongMode(true)
	player.SetPlaying(true)
	renderFrames(player, 1)
	if player.active.tracks[0].track.Parameters["cutoff"] != .2 || player.active.tracks[0].left == 0 {
		t.Fatal("song failed to recall stored state or section override")
	}
	p.Tracks[0].Parameters["cutoff"] = .4
	player.Publish(p)
	renderFrames(player, 1)
	if player.active.tracks[0].track.Parameters["cutoff"] != .4 {
		t.Fatal("live knob no longer works over stored sound")
	}
	p.Tracks[0].Patterns[0].Steps[2].Active = true
	player.Publish(p)
	renderFrames(player, 1)
	if player.active.tracks[0].track.Parameters["cutoff"] != .4 {
		t.Fatal("sequence edit reset live controls")
	}
	renderFrames(player, SampleRate*240/300)
	if player.CurrentPattern() != 1 || player.active.tracks[0].track.Parameters["cutoff"] != .7 || player.active.tracks[0].effects[0].config.Kind != BitCrusher {
		t.Fatal("song did not recall next pattern's chain")
	}
	// A returning unit must not resume a stale frozen wet buffer.
	player.active.banks[0][0].effects[0].state.mix = .9
	renderFrames(player, SampleRate*240/300)
	if player.CurrentPattern() != 0 || player.active.tracks[0].track.Parameters["cutoff"] != .2 {
		t.Fatal("song return retained unstored tweaks")
	}
	if player.active.tracks[0].effects[0].state.mix > .8 {
		t.Fatal("returning FX retained a stale wet state")
	}
	p.Tracks[0].Patterns[0].Steps[3].Active = true
	player.Publish(p)
	renderFrames(player, 1)
	if player.active.tracks[0].track.Parameters["cutoff"] != .2 {
		t.Fatal("unrelated publication resurrected an old live tweak")
	}
	var buffer [BlockSize * 2]float32
	allocs := testing.AllocsPerRun(20, func() { player.barPhase = 4; player.Render(buffer[:]) })
	if allocs != 0 {
		t.Fatalf("pattern control recall allocated in audio callback: %.1f", allocs)
	}
	var wav bytes.Buffer
	if err := RenderWAV(context.Background(), &wav, p, true, 0, nil); err != nil || wav.Len() <= 44 || binary.LittleEndian.Uint32(wav.Bytes()[40:44]) == 0 {
		t.Fatal("song state export failed", err)
	}
}

func TestStoredStateConcurrentPublication(t *testing.T) {
	p := NewProject()
	p.Tracks = p.Tracks[:1]
	StorePatternState(&p, 0, "")
	StorePatternState(&p, 1, "")
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var buffer [BlockSize * 2]float32
		for i := 0; i < 600; i++ {
			player.Render(buffer[:])
		}
	}()
	for i := 0; i < 80; i++ {
		SelectPattern(&p, i%2)
		p.Tracks[0].Parameters["cutoff"] = float64(i) / 100
		StorePatternState(&p, p.Pattern, "")
		player.Publish(p)
	}
	wg.Wait()
}
