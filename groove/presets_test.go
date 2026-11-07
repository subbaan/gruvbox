package groove

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestSoundRecallPreservesSequenceAndUndo(t *testing.T) {
	p := NewProject()
	target := &p.Tracks[0]
	target.Name, target.Level, target.Pan, target.Mute, target.Solo = "My bass", .42, -.3, true, true
	old := NewEffect(Delay)
	target.Effects = []Effect{old}
	for i := range target.Patterns {
		s := &target.Patterns[i].Steps[0]
		s.Active, s.Note, s.Length, s.Slide = true, 47, 4, true
		s.Locks = map[string]float64{"cutoff": .7, EffectLockID(old.ID, "mix"): .5}
	}
	before := p.Clone()
	source := NewTrack(Acid, 0)
	source.Parameters["cutoff"] = .23
	source.LFOs[0].Enabled = true
	source.Effects = []Effect{NewEffect(Chorus)}
	source.Effects[0].LFOs[1].Enabled = true
	preset := CaptureSound(source, "New timbre")
	session := NewSession(p)
	for _, includeFX := range []bool{false, true} {
		if err := session.Do("Recall", func(p *Project) error { return ApplySound(&p.Tracks[0], preset, includeFX) }); err != nil {
			t.Fatal(err)
		}
		target := session.Project.Tracks[0]
		if target.Parameters["cutoff"] != .23 || target.LFOs != preset.LFOs || target.ID != before.Tracks[0].ID || target.Name != "My bass" || target.Level != .42 || target.Pan != -.3 || !target.Mute || !target.Solo {
			t.Fatal("sound recall changed track identity or mixer, or lost LFOs")
		}
		expected := before.Clone().Tracks[0].Patterns
		if includeFX {
			for i := range expected {
				delete(expected[i].Steps[0].Locks, EffectLockID(old.ID, "mix"))
			}
			if len(target.Effects) != 1 || target.Effects[0].Kind != Chorus || target.Effects[0].ID == preset.Effects[0].ID || target.Effects[0].LFOs != preset.Effects[0].LFOs {
				t.Fatal("FX recall lost modulation or reused a preset's identity")
			}
		} else if !reflect.DeepEqual(target.Effects, before.Tracks[0].Effects) {
			t.Fatal("instrument-only recall changed FX")
		}
		if !reflect.DeepEqual(expected, target.Patterns) {
			t.Fatal("sound recall changed notes, lengths or instrument locks")
		}
		// Recalled parameters cannot mutate the reusable preset.
		target.Parameters["cutoff"] = .99
		if preset.Parameters["cutoff"] != .23 {
			t.Fatal("preset and track share a mutable map")
		}
		if !session.Undo() || !reflect.DeepEqual(session.Project, before) {
			t.Fatal("preset recall did not undo in one command")
		}
	}
}

func TestSoundLibrarySamplerPortabilityAndIdentity(t *testing.T) {
	library := SoundLibrary{Dir: t.TempDir()}
	source := NewTrack(Sampler, 0)
	source.Sample = pcmSample(t, 2, 48000, 12000, -10000, 8000, -6000)
	source.Parameters["start"], source.Parameters["end"], source.Parameters["reverse"] = .25, .75, 1
	source.Parameters["gain_db"], source.Parameters["fade_in"] = -3, 50
	source.Effects = []Effect{NewEffect(BitCrusher)}
	source.Effects[0].LFOs[0].Enabled = true
	preset := CaptureSound(source, " ../../A found sound ")
	id, err := library.Save(preset)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := library.List(Sampler)
	if err != nil || len(entries) != 1 || entries[0].ID != id || entries[0].Name != "../../A found sound" {
		t.Fatalf("library metadata: %v %v", entries, err)
	}
	loaded, err := library.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded.Sample.wav, source.Sample.wav) || loaded.Sample.Channels() != 2 || loaded.Sample.Rate() != 48000 || !reflect.DeepEqual(loaded.Parameters, preset.Parameters) || !reflect.DeepEqual(loaded.Effects, preset.Effects) {
		t.Fatal("portable preset lost WAV, trim, processing or FX modulation")
	}
	other := SoundLibrary{Dir: t.TempDir()}
	secondID, err := other.Save(loaded)
	if err != nil || secondID == id {
		t.Fatal("import did not create an independent library identity")
	}
	target := NewTrack(Sampler, 0)
	if err := ApplySound(&target, loaded, true); err != nil || target.Sample != loaded.Sample || target.Parameters["end"] != .75 {
		t.Fatal("sampler recall failed")
	}
	if err := library.Delete(id); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Load(secondID); err != nil {
		t.Fatal("deleting one library preset removed another")
	}
	for _, bad := range []string{"../escape", "../../x", "factory-tb303-0", "", strings.Repeat("a", 25)} {
		if _, err := library.Load(bad); err == nil && !strings.HasPrefix(bad, "factory-") {
			t.Fatalf("unsafe ID accepted: %q", bad)
		}
		if err := library.Delete(bad); err == nil {
			t.Fatalf("unsafe or factory deletion accepted: %q", bad)
		}
	}
}

func TestPresetValidationAndAtomicReplacement(t *testing.T) {
	preset := FactorySounds(Acid)[0]
	path := filepath.Join(t.TempDir(), "sound.gruvsound")
	if err := SaveSoundPreset(path, preset); err != nil {
		t.Fatal(err)
	}
	good, _ := os.ReadFile(path)
	bad := preset
	bad.Parameters = cloneMap(preset.Parameters)
	bad.Parameters["cutoff"] = math.NaN()
	if err := SaveSoundPreset(path, bad); err == nil {
		t.Fatal("non-finite preset accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, good) {
		t.Fatal("invalid save damaged the existing preset")
	}
	target := NewTrack(Kick, 0)
	before := CaptureSound(target, "before")
	if err := ApplySound(&target, preset, true); err == nil || !reflect.DeepEqual(before, CaptureSound(target, "before")) {
		t.Fatal("wrong-kind recall partially edited the track")
	}
	for _, edit := range []func(*SoundPreset){
		func(p *SoundPreset) { p.FormatVersion = 999 },
		func(p *SoundPreset) { p.Name = "   " },
		func(p *SoundPreset) { p.Kind = "megasynth" },
		func(p *SoundPreset) { p.LFOs[0].Target = 99 },
		func(p *SoundPreset) { p.Effects = []Effect{NewEffect(Chorus)}; p.Effects[0].LFOs[0].Target = 99 },
	} {
		bad := preset
		edit(&bad)
		data, _ := json.Marshal(bad)
		if _, err := DecodeSoundPreset(bytes.NewReader(data)); err == nil {
			t.Fatal("invalid portable preset accepted")
		}
	}
	if _, err := DecodeSoundPreset(bytes.NewReader(make([]byte, MaxSoundPresetBytes+1))); err == nil {
		t.Fatal("oversized preset accepted")
	}
	// Escaped descriptions still fit the header reader's limit.
	preset.Description = strings.Repeat("\x01", 2000)
	library := SoundLibrary{Dir: t.TempDir()}
	if _, err := library.Save(preset); err != nil {
		t.Fatal(err)
	}
	if entries, err := library.List(Acid); err != nil || len(entries) != len(FactorySounds(Acid))+1 {
		t.Fatal("valid saved preset cannot be listed", err)
	}
	if err := os.WriteFile(filepath.Join(library.Dir, strings.Repeat("b", 24)+".gruvsound"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if entries, err := library.List(Acid); err == nil || len(entries) != len(FactorySounds(Acid))+1 {
		t.Fatal("one broken preset hid the usable library")
	}
}

func TestFactorySoundsRenderAndAreIndependent(t *testing.T) {
	count := 0
	for _, kind := range Kinds {
		for i, p := range FactorySounds(kind) {
			count++
			if err := p.Validate(); err != nil {
				t.Fatalf("%s: %v", p.Name, err)
			}
			track := NewTrack(kind, 0)
			if err := ApplySound(&track, p, true); err != nil {
				t.Fatal(err)
			}
			track.Patterns[0].Steps[0].Active = true
			player := NewPlayer()
			player.Publish(Project{FormatVersion: FormatVersion, BPM: 126, Tracks: []Track{track}})
			player.SetPlaying(true)
			var block [BlockSize * 2]float32
			energy := 0.0
			for n := 0; n < 100; n++ {
				player.Render(block[:])
				for _, v := range block {
					if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
						t.Fatal("non-finite factory sound", p.Name)
					}
					energy += math.Abs(float64(v))
				}
			}
			if energy == 0 {
				t.Fatal("silent factory sound", p.Name)
			}
			p.Parameters[Parameters(kind)[0].ID] = -999
			fresh := FactorySounds(kind)[i]
			if err := fresh.Validate(); err != nil {
				t.Fatal("factory recipe shares mutable maps")
			}
		}
	}
	if count != 89 {
		t.Fatalf("expected 89 starting sounds, got %d", count)
	}
}

func TestPresetRecallWhileRenderingRemainsAllocationFree(t *testing.T) {
	p := NewProject()
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var block [BlockSize * 2]float32
		for i := 0; i < 500; i++ {
			player.Render(block[:])
		}
	}()
	for i := 0; i < 30; i++ {
		preset := FactorySounds(Acid)[i%4]
		if err := ApplySound(&p.Tracks[0], preset, true); err != nil {
			t.Fatal(err)
		}
		player.Publish(p)
	}
	wg.Wait()
	var block [BlockSize * 2]float32
	if n := testing.AllocsPerRun(100, func() { player.Render(block[:]) }); n != 0 {
		t.Fatal("recalled sound allocates in Render", n)
	}
}
