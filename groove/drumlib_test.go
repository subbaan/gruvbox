package groove

import (
	"math/rand/v2"
	"testing"
)

func TestDrumFamilyTemplates(t *testing.T) {
	for kind, families := range drumFamilies {
		for _, f := range families {
			if len(f.template) != 16 {
				t.Errorf("%s %s: template has %d steps", kind, f.name, len(f.template))
			}
			for _, c := range f.template {
				if c != 'x' && c != 'o' && c != 'g' && c != '?' && c != '.' {
					t.Errorf("%s %s: bad character %q", kind, f.name, c)
				}
			}
		}
	}
}

func TestDrumPatternKeepsLengthAndVaries(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	steps := make([]Step, 32)
	if !DrumPattern(steps, Kick, "Four", r) {
		t.Fatal("Four kick family missing")
	}
	for _, i := range []int{0, 4, 8, 12, 16, 20, 24} {
		if !steps[i].Active || steps[i].Velocity < .75 {
			t.Fatalf("four-on-the-floor kick missing at %d: %+v", i, steps[i])
		}
	}
	if DrumPattern(steps, Kick, "No such family", r) {
		t.Fatal("unknown family accepted")
	}
	// Random drum generation produces several different patterns.
	seen := map[string]bool{}
	for n := 0; n < 30; n++ {
		track := NewTrack(ClosedHat, 0)
		Generate(&track, 0, "Random")
		key := ""
		for _, s := range track.Patterns[0].Steps {
			if s.Active {
				key += "x"
			} else {
				key += "."
			}
		}
		seen[key] = true
		p := NewProject()
		p.Tracks = []Track{track}
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) < 4 {
		t.Fatalf("drum Random produced only %d distinct patterns", len(seen))
	}
}

func TestMelodicPartsShareTheProjectKey(t *testing.T) {
	p := NewProject()
	GenerateIn(&p, &p.Tracks[0], 0, "Acid")
	if p.Key == nil {
		t.Fatal("first melodic generation did not set a key")
	}
	key := *p.Key
	GenerateIn(&p, &p.Tracks[1], 0, "Sparse")
	if *p.Key != key {
		t.Fatal("second part changed the key")
	}
	scale := scaleNotes(key.Scale)
	for _, track := range p.Tracks[:2] {
		for _, s := range track.Patterns[0].Steps {
			if !s.Active {
				continue
			}
			degree := ((s.Note-key.BassRoot())%12 + 12) % 12
			inScale := false
			for _, n := range scale {
				if n%12 == degree {
					inScale = true
				}
			}
			if !inScale {
				t.Fatalf("note %d is outside %s", s.Note, key.Name())
			}
		}
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := Key{Root: 12, Scale: PhraseMinor}
	p.Key = &bad
	if p.Validate() == nil {
		t.Fatal("invalid key accepted")
	}
}
