package groove

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

func TestSH101TrackPlaysSavesAndValidates(t *testing.T) {
	track := NewTrack(SH101, 0)
	if track.Name != "SH-101" || !SH101.Capabilities().Slide || !SH101.Capabilities().NoteLengths {
		t.Fatal("SH-101 kind not set up")
	}
	Generate(&track, 0, "Bassline")
	track.Patterns[0].Steps[0].Locks = map[string]float64{"cutoff": .8, "pw": .6}
	track.LFOs[0].Enabled = true
	p := Project{FormatVersion: FormatVersion, BPM: 124, Tracks: []Track{track}}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(p)
	loaded, err := Decode(bytes.NewReader(data))
	if err != nil || !reflect.DeepEqual(loaded.Tracks, p.Tracks) {
		t.Fatal("SH-101 track did not round trip", err)
	}
	bad := p.Clone()
	bad.Tracks[0].Parameters["sub_mode"] = 1.5
	if bad.Validate() == nil {
		t.Fatal("half a switch position validated")
	}
	if got := ParameterChoices(SH101, "range"); len(got) != 4 || got[1] != "8'" {
		t.Fatalf("range choices %v", got)
	}
	if len(LFOTargets(SH101)) != 7 {
		t.Fatal("SH-101 LFO targets missing")
	}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	out := make([]float32, BlockSize*2)
	energy := 0.0
	for i := 0; i < 400; i++ {
		player.Render(out)
		for _, v := range out {
			if math.IsNaN(float64(v)) {
				t.Fatal("NaN from the SH-101 track")
			}
			energy += math.Abs(float64(v))
		}
	}
	if energy < 1 {
		t.Fatal("SH-101 track is silent")
	}
	if v := player.active.tracks[0].voice; v.mono == nil || v.lfos()[0] == nil {
		t.Fatal("SH-101 voice or LFOs not wired")
	}
	if n := testing.AllocsPerRun(50, func() { player.Render(out) }); n != 0 {
		t.Fatal("SH-101 render allocates", n)
	}
	player.endExportNotes()
	for i := 0; i < 200; i++ {
		player.renderTail(out)
	}
	for _, v := range out {
		if v != 0 {
			t.Fatal("export tail left the SH-101 gated")
		}
	}
	var wav bytes.Buffer
	if err := RenderWAV(context.Background(), &wav, p, false, .5, nil); err != nil || wav.Len() < 44 {
		t.Fatal("SH-101 export failed", err)
	}
}

func TestSH101RandomiseSoundKeepsRangeAndTune(t *testing.T) {
	track := NewTrack(SH101, 0)
	track.Parameters["range"], track.Parameters["tune"] = 2, -3
	names := map[string]bool{}
	for seed := uint64(1); seed <= 40; seed++ {
		tr := Project{Tracks: []Track{track}}.Clone().Tracks[0]
		names[RandomiseSoundFrom(&tr, rand.New(rand.NewPCG(seed, 1)))] = true
		if tr.Parameters["range"] != 2 || tr.Parameters["tune"] != -3 {
			t.Fatal("Randomise sound moved the SH-101's octave or tuning")
		}
		p := Project{FormatVersion: FormatVersion, BPM: 120, Tracks: []Track{tr}}
		if err := p.Validate(); err != nil {
			t.Fatalf("randomised SH-101 invalid: %v", err)
		}
	}
	if len(names) < 6 {
		t.Fatalf("Randomise sound drew only %d SH-101 sounds", len(names))
	}
}

// Added to a styled groove, the SH-101 takes a role and the style's level and
// gets an SH-101 sound for it; Fit writes its part like a Dual's.
func TestSH101JoinsGroovesInARole(t *testing.T) {
	bank := map[string]bool{}
	for _, preset := range FactorySounds(SH101) {
		bank[preset.Name] = true
	}
	for _, style := range GrooveStyles {
		for seed := uint64(1); seed <= 6; seed++ {
			r := rand.New(rand.NewPCG(seed, 77))
			p, _ := newGroove(style, r)
			i, info := AddTrack(&p, SH101, r)
			s := &p.Tracks[i]
			if s.Role == "" || s.Role == "bass" || !strings.Contains(info, style.Name) {
				t.Fatalf("%s/%d: SH-101 role %q: %q", style.Name, seed, s.Role, info)
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("%s/%d: %v", style.Name, seed, err)
			}
			FitPart(&p, s, 0, r)
			if !hasNotes(s.Patterns[0]) {
				t.Fatalf("%s/%d: Fit wrote no SH-101 part", style.Name, seed)
			}
		}
	}
	// With no other synth it becomes the bass, with a bass sound.
	style, _ := StyleNamed("Acid")
	p, _ := newGroove(style, rand.New(rand.NewPCG(2, 2)))
	p.Tracks = p.Tracks[:0:0]
	i, _ := AddTrack(&p, SH101, rand.New(rand.NewPCG(3, 3)))
	if p.Tracks[i].Role != "bass" {
		t.Fatalf("lone SH-101 took role %q", p.Tracks[i].Role)
	}
}

// New grooves sometimes put an SH-101 in the Dual's place, with a role.
func TestNewGroovesSometimesUseTheSH101(t *testing.T) {
	seen := 0
	for seed := uint64(1); seed <= 120; seed++ {
		p, _ := RandomGroove(rand.New(rand.NewPCG(seed, 5)))
		for _, tr := range p.Tracks {
			if tr.Kind == SH101 {
				seen++
				if tr.Role == "" {
					t.Fatalf("seed %d: SH-101 without a role", seed)
				}
			}
		}
	}
	if seen < 8 || seen > 60 {
		t.Fatalf("SH-101 in %d of 120 new grooves", seen)
	}
}
