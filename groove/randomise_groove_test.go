package groove

import (
	"encoding/json"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

func TestRandomiseGrooveRebuildsOnlyTheSelectedPattern(t *testing.T) {
	p := NewProject()
	for _, kind := range []Kind{Dual, Clap, Sampler} {
		p.Tracks = append(p.Tracks, NewTrack(kind, len(p.Tracks)))
	}
	for i := range p.Tracks {
		GenerateIn(&p, &p.Tracks[i], 0, Actions(p.Tracks[i].Kind)[0])
		p.Tracks[i].Level, p.Tracks[i].Pan, p.Tracks[i].Mute = .5, -.3, i == 2
	}
	p.Pattern = 2
	if err := StorePatternState(&p, 2, p.Tracks[0].ID); err != nil {
		t.Fatal(err)
	}
	p.Tracks[0].Patterns[2].Steps[0].Locks = map[string]float64{"cutoff": .1}
	p.Tracks[0].Effects = []Effect{NewEffect(Delay)}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	before := p.Clone()

	info := RandomiseGroove(&p, rand.New(rand.NewPCG(4, 5)))
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if info == "" || p.Key == nil {
		t.Fatalf("no status or key: %q", info)
	}
	filled := 0
	for i := range p.Tracks {
		now, was := p.Tracks[i], before.Tracks[i]
		if now.ID != was.ID || now.Name != was.Name || now.Kind != was.Kind || now.Level != was.Level || now.Pan != was.Pan || now.Mute != was.Mute {
			t.Fatalf("%s: identity or mixer changed", now.Name)
		}
		if !reflect.DeepEqual(now.Patterns[0], was.Patterns[0]) {
			t.Fatalf("%s: another pattern changed", now.Name)
		}
		if now.Patterns[2].State != nil {
			t.Fatalf("%s: stale stored sound kept", now.Name)
		}
		for _, s := range now.Patterns[2].Steps {
			if s.Active {
				filled++
				break
			}
		}
	}
	if filled < len(p.Tracks)-2 {
		t.Fatalf("only %d of %d tracks got notes", filled, len(p.Tracks))
	}
	if p.Tracks[0].Patterns[2].Steps[0].Locks != nil {
		t.Fatal("old locks survived")
	}
	for _, e := range p.Tracks[0].Effects {
		if e.ID == before.Tracks[0].Effects[0].ID {
			t.Fatal("old effect chain kept")
		}
	}

	// The same seed and starting point give the same groove.
	q := before.Clone()
	if RandomiseGroove(&q, rand.New(rand.NewPCG(4, 5))) != info {
		t.Fatal("not repeatable")
	}
	for i := range q.Tracks {
		if !reflect.DeepEqual(q.Tracks[i].Patterns[2], p.Tracks[i].Patterns[2]) || !reflect.DeepEqual(q.Tracks[i].Parameters, p.Tracks[i].Parameters) {
			t.Fatal("not repeatable")
		}
	}
}

func TestRandomiseGrooveVariesAcrossCalls(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 9))
	styles := map[float64]bool{}
	p := NewProject()
	for n := 0; n < 20; n++ {
		RandomiseGroove(&p, r)
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		styles[p.BPM] = true
	}
	if len(styles) < 6 {
		t.Fatalf("tempos barely vary: %v", styles)
	}
}

// barsDiffer reports whether any bar of steps differs from the first in
// which steps play or which notes they play.
func barsDiffer(steps []Step) bool {
	for i := 16; i < len(steps); i++ {
		a, b := steps[i], steps[i%16]
		if a.Active != b.Active || (a.Active && a.Note != b.Note) {
			return true
		}
	}
	return false
}

func TestGroovesComeInOneTwoAndFourBars(t *testing.T) {
	r := rand.New(rand.NewPCG(31, 41))
	lengths := map[int]int{}
	developed, long := 0, 0
	for n := 0; n < 60; n++ {
		p, info := RandomGroove(r)
		steps := len(p.Tracks[0].Patterns[0].Steps)
		lengths[steps]++
		for _, track := range p.Tracks {
			if len(track.Patterns[0].Steps) != steps || track.Patterns[0].Resolution != 16 {
				t.Fatalf("%s: tracks disagree on length", track.Name)
			}
		}
		if steps > 16 {
			long++
			if !strings.Contains(info, " bars") {
				t.Fatalf("status lacks length: %q", info)
			}
			if barsDiffer(p.Tracks[0].Patterns[0].Steps) {
				developed++
			}
		}
	}
	if lengths[16] == 0 || lengths[32] == 0 || lengths[64] == 0 {
		t.Fatalf("lengths %v", lengths)
	}
	if developed*2 < long {
		t.Fatalf("only %d of %d multi-bar lead lines change between bars", developed, long)
	}

	// Randomise all sets the selected pattern's length too.
	p := NewProject()
	seen := map[int]bool{}
	for n := 0; n < 30; n++ {
		RandomiseGroove(&p, r)
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		seen[len(p.Tracks[0].Patterns[p.Pattern].Steps)] = true
	}
	if len(seen) < 3 {
		t.Fatalf("Randomise all lengths %v", seen)
	}
}

func TestRandomiseKeepsHeldPartsAndTheirTempoAndKey(t *testing.T) {
	p, _ := RandomGroove(rand.New(rand.NewPCG(7, 8)))
	p.Tracks[0].Hold = true // the lead 303
	held := p.Tracks[0]
	bpm, swing, key := p.BPM, p.Swing, *p.Key
	for seed := uint64(0); seed < 20; seed++ {
		q := p.Clone()
		info := RandomiseGroove(&q, rand.New(rand.NewPCG(seed, 1)))
		if err := q.Validate(); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(q.Tracks[0], held) {
			t.Fatal("held 303 changed")
		}
		if q.BPM != bpm || q.Swing != swing || *q.Key != key {
			t.Fatalf("tempo or key moved under a held line: %v %v %v", q.BPM, q.Swing, q.Key.Name())
		}
		if !strings.Contains(info, "held: "+held.Name) {
			t.Fatalf("status does not mention the held part: %q", info)
		}
		changed := false
		for i := 1; i < len(q.Tracks); i++ {
			changed = changed || !reflect.DeepEqual(q.Tracks[i].Patterns[0], p.Tracks[i].Patterns[0])
		}
		if !changed {
			t.Fatal("nothing else was randomised")
		}
	}
}

func TestRandomiseDrumsOrSynthsOnly(t *testing.T) {
	p, _ := RandomGroove(rand.New(rand.NewPCG(9, 9)))
	for _, part := range []GroovePart{DrumParts, SynthParts} {
		q := p.Clone()
		RandomiseGrooveParts(&q, rand.New(rand.NewPCG(3, 3)), part)
		// Kept drums keep the tempo; kept 303 lines also keep the key.
		if q.BPM != p.BPM || part == DrumParts && *q.Key != *p.Key {
			t.Fatalf("part %d changed tempo or key", part)
		}
		for i, track := range q.Tracks {
			same := reflect.DeepEqual(track, p.Tracks[i])
			if synth := track.Kind == Acid || track.Kind == Dual; synth == (part == DrumParts) && !same {
				t.Fatalf("part %d changed %s", part, track.Name)
			}
		}
		if RerolledTracks(q, part) == 0 {
			t.Fatalf("part %d has nothing to reroll", part)
		}
	}
	// Drums-only on a project with no drums leaves nothing to change.
	q := p.Clone()
	q.Tracks = q.Tracks[:2]
	if RerolledTracks(q, DrumParts) != 0 {
		t.Fatal("expected no drum tracks")
	}
	q.Tracks[0].Hold, q.Tracks[1].Hold = true, true
	if RerolledTracks(q, AllParts) != 0 {
		t.Fatal("held tracks counted")
	}
}

func TestHoldSurvivesSaveAndLoad(t *testing.T) {
	p := NewProject()
	p.Tracks[2].Hold = true
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Decode(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	if !q.Tracks[2].Hold || q.Tracks[1].Hold {
		t.Fatal("hold not saved")
	}
}
