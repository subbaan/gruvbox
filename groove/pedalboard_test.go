package groove

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

// soloRMS renders two bars of track i alone and returns its RMS and peak.
func soloRMS(t *testing.T, p Project, i int) (rms, peak float64) {
	t.Helper()
	for n := range p.Tracks {
		p.Tracks[n].Mute = n != i
	}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	out := make([]float32, int(SampleRate*60/p.BPM*8)*2)
	player.Render(out)
	sum := 0.0
	for _, v := range out {
		x := math.Abs(float64(v))
		peak = max(peak, x)
		sum += x * x
	}
	if math.IsNaN(sum) {
		t.Fatal("non-finite render")
	}
	return math.Sqrt(sum / float64(len(out))), peak
}

// Every board must make a valid chain and keep its track near the dry level:
// pedals change the colour, not the volume, by more than a few dB.
func TestPedalboardsApplyAndKeepLevel(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 8))
	for _, board := range Pedalboards {
		p := NewProject()
		index := 0 // 303
		if board.Drums {
			index = 2 // kick
		}
		track := &p.Tracks[index]
		key := Key{Root: 9, Scale: PhraseMinor}
		p.Key = &key
		generate(track, 0, map[bool]string{false: "Acid", true: "4 Floor"}[board.Drums], &key, rand.New(rand.NewPCG(13, 21)))
		dry, _ := soloRMS(t, p.Clone(), index)
		worst := 0.0
		for n := 0; n < 4; n++ {
			q := p.Clone()
			ApplyPedalboard(&q.Tracks[index], board, r)
			if len(q.Tracks[index].Effects) == 0 {
				t.Fatalf("%s made an empty chain", board.Name)
			}
			if err := q.Validate(); err != nil {
				t.Fatalf("%s: %v", board.Name, err)
			}
			wet, peak := soloRMS(t, q, index)
			db := 20 * math.Log10(wet/dry)
			if math.Abs(db) > math.Abs(worst) {
				worst = db
			}
			if peak > 1.0001 || math.Abs(db) > 7 {
				t.Errorf("%s: %.1f dB against dry, peak %.3f (%s)", board.Name, db, peak, PedalboardInfo(board, q.Tracks[index]))
			}
		}
		t.Logf("%-15s worst %+.1f dB", board.Name, worst)
	}
}

func TestRandomiseEmptyChainUsesSuitedBoard(t *testing.T) {
	p := NewProject()
	for i := range p.Tracks {
		track := &p.Tracks[i]
		RandomiseEffects(track)
		if len(track.Effects) == 0 {
			t.Fatalf("%s: no effects", track.Kind)
		}
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	r := rand.New(rand.NewPCG(1, 2))
	for _, kind := range []Kind{Acid, Dual, Kick, OpenHat, Sampler} {
		for n := 0; n < 20; n++ {
			if b := RandomPedalboard(kind, r); !b.suitsTrack(kind) {
				t.Fatalf("%s got %s", kind, b.Name)
			}
		}
	}
	for _, b := range Pedalboards {
		for _, pedal := range b.pedals {
			defs := map[string]bool{}
			for _, param := range EffectParameters(pedal.kind) {
				defs[param.ID] = true
			}
			for id := range pedal.params {
				if !defs[id] {
					t.Errorf("%s: %s has no %q", b.Name, pedal.kind, id)
				}
			}
		}
	}
}

// Every effect kind, chained on one track, renders without allocating.
func TestEveryEffectKindRendersWithoutAllocating(t *testing.T) {
	p := NewProject()
	track := &p.Tracks[0]
	GenerateIn(&p, track, 0, "Acid")
	for _, kind := range EffectKinds {
		track.Effects = append(track.Effects, NewEffect(kind))
	}
	track.Effects[0].LFOs[0].Enabled, track.Effects[0].LFOs[0].Target = true, 1
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var out [BlockSize * 2]float32
	player.Render(out[:])
	if n := testing.AllocsPerRun(200, func() { player.Render(out[:]) }); n != 0 {
		t.Fatalf("render allocated %v", n)
	}
	for _, v := range out {
		if math.IsNaN(float64(v)) || math.Abs(float64(v)) > 1.0001 {
			t.Fatalf("bad sample %v", v)
		}
	}
}

// TestWritePedalSamples renders one Acid groove through every bass board,
// so boards can be compared on the same line:
// GRUVBOX_PEDAL_WAVS=<dir> go test ./groove -run TestWritePedalSamples
// (add GRUVBOX_PEDAL_SOLO=1 to hear the 303 alone).
func TestWritePedalSamples(t *testing.T) {
	dir := os.Getenv("GRUVBOX_PEDAL_WAVS")
	if dir == "" {
		t.Skip("set GRUVBOX_PEDAL_WAVS to a directory to hear each pedalboard")
	}
	r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	base := NewGrooveInStyle(GrooveStyles[0], r)
	base.Song = []SongSection{NewSongSection(0)}
	if os.Getenv("GRUVBOX_PEDAL_SOLO") != "" {
		for i := 1; i < len(base.Tracks); i++ {
			base.Tracks[i].Mute = true
		}
	}
	write := func(name string, p Project) {
		f, err := os.Create(filepath.Join(dir, name+".wav"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := RenderWAV(context.Background(), f, p, true, 1, nil); err != nil {
			t.Fatal(err)
		}
	}
	dry := base.Clone()
	for len(dry.Tracks[0].Effects) > 0 {
		RemoveEffect(&dry.Tracks[0], dry.Tracks[0].Effects[0].ID)
	}
	write("00 dry", dry)
	for i, board := range Pedalboards {
		if board.Drums {
			continue
		}
		p := base.Clone()
		ApplyPedalboard(&p.Tracks[0], board, r)
		write(fmt.Sprintf("%02d %s", i+1, board.Name), p)
	}
}
