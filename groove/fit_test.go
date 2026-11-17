package groove

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func medianNote(steps []Step) (int, bool) {
	notes := []int{}
	for _, s := range steps {
		if s.Active {
			notes = append(notes, s.Note)
		}
	}
	if len(notes) == 0 {
		return 0, false
	}
	slices.Sort(notes)
	return notes[len(notes)/2], true
}

// leadTrack is the groove's first 303 (its bass or lead line).
func leadTrack(p Project) *Track {
	for i := range p.Tracks {
		if p.Tracks[i].Role == "lead" {
			return &p.Tracks[i]
		}
	}
	return nil
}

// The user's case: a Dual added to a running groove, then G. It must come in
// at the groove's length, with the style's level, and sit above the bass
// (or double the lead, for a layer).
func TestAddedDualFitsTheGroove(t *testing.T) {
	for _, style := range GrooveStyles {
		for seed := uint64(1); seed <= 12; seed++ {
			r := rand.New(rand.NewPCG(seed, 99))
			p, _ := newGroove(style, r)
			lead := leadTrack(p)
			if lead == nil || !hasNotes(lead.Patterns[0]) {
				t.Fatalf("%s/%d: groove has no playing lead", style.Name, seed)
			}
			steps := len(lead.Patterns[0].Steps)
			i, info := AddTrack(&p, Dual, r)
			d := &p.Tracks[i]
			if len(d.Patterns[0].Steps) != steps || hasNotes(d.Patterns[0]) {
				t.Fatalf("%s/%d: added Dual has %d steps (groove %d) or notes", style.Name, seed, len(d.Patterns[0].Steps), steps)
			}
			if d.Role == "" || d.Role == "bass" || !strings.Contains(info, style.Name) {
				t.Fatalf("%s/%d: added Dual role %q, info %q", style.Name, seed, d.Role, info)
			}
			if d.Level > .5 {
				t.Fatalf("%s/%d: added Dual %s at level %.2f, louder than the style's part", style.Name, seed, d.Role, d.Level)
			}
			info = FitPart(&p, d, 0, r)
			if len(d.Patterns[0].Steps) != steps || !hasNotes(d.Patterns[0]) || !strings.HasPrefix(info, FitAction) {
				t.Fatalf("%s/%d: Fit wrote %d steps (groove %d): %q", style.Name, seed, len(d.Patterns[0].Steps), steps, info)
			}
			got, _ := medianNote(d.Patterns[0].Steps)
			bass, _ := medianNote(lead.Patterns[0].Steps)
			switch d.Role {
			case "layer":
				if got >= bass {
					t.Fatalf("%s/%d: layer at %d is not under the lead at %d", style.Name, seed, got, bass)
				}
			default:
				if got < bass+12 {
					t.Fatalf("%s/%d: %s at %d is not an octave above the bass at %d", style.Name, seed, d.Role, got, bass)
				}
			}
			if d.Role == "pad" {
				long := false
				for _, s := range d.Patterns[0].Steps {
					long = long || s.Active && s.Length > 1
				}
				if !long {
					t.Fatalf("%s/%d: pad notes are not held", style.Name, seed)
				}
			}
		}
	}
}

// A second 303 added to a groove with a bass goes above it; with no other
// synth it becomes the lead.
func TestAdded303TakesTheFreeRole(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	style, _ := StyleNamed("Dub")
	p, _ := newGroove(style, r)
	i, _ := AddTrack(&p, Acid, r)
	if p.Tracks[i].Role != "second" {
		t.Fatalf("303 added over a bass took role %q", p.Tracks[i].Role)
	}
	FitPart(&p, &p.Tracks[i], 0, r)
	bass, _ := medianNote(leadTrack(p).Patterns[0].Steps)
	if n, _ := medianNote(p.Tracks[i].Patterns[0].Steps); n < bass+12 {
		t.Fatalf("second 303 at %d is not an octave above the bass at %d", n, bass)
	}
	q := p.Clone()
	q.Tracks = slices.DeleteFunc(q.Tracks, func(t Track) bool { return t.Kind.synthPart() })
	j, _ := AddTrack(&q, Acid, r)
	if q.Tracks[j].Role != "lead" {
		t.Fatalf("lone 303 took role %q", q.Tracks[j].Role)
	}
}

// Drums added to a styled groove take the groove's length and the style's
// sound and level, and Fit writes one of the style's patterns for them.
func TestAddedDrumsFitTheGroove(t *testing.T) {
	style, _ := StyleNamed("Dub")
	for seed := uint64(1); seed <= 8; seed++ {
		r := rand.New(rand.NewPCG(seed, 7))
		p, _ := newGroove(style, r)
		steps := grooveSteps(&p, "", 0)
		for _, kind := range []Kind{Kick, Rimshot, Clap, Tom} {
			i, info := AddTrack(&p, kind, r)
			d := &p.Tracks[i]
			if len(d.Patterns[0].Steps) != steps || !strings.Contains(info, "Dub sound") {
				t.Fatalf("%s/%d: %d steps (groove %d), info %q", kind.Name(), seed, len(d.Patterns[0].Steps), steps, info)
			}
			if x, ok := style.extra(kind); ok && d.Level != x.level && style.recipeFor(kind, 0, r).level == 0 {
				t.Fatalf("%s/%d: level %.2f, style gives %.2f", kind.Name(), seed, d.Level, x.level)
			}
			info = FitPart(&p, d, 0, r)
			if !hasNotes(d.Patterns[0]) || len(d.Patterns[0].Steps) != steps || !strings.Contains(info, "Dub ") {
				t.Fatalf("%s/%d: Fit gave no part or wrong length: %q", kind.Name(), seed, info)
			}
		}
	}
}

// Without a style (a template or an older file) Fit still matches the
// length and keeps a new synth line out of the bass's register.
func TestFitWithoutStyle(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	p := NewProject()
	p.Tracks = []Track{NewTrack(Acid, 0), NewTrack(Kick, 1)}
	resetPatternSlot(&p.Tracks[0], 0, 32)
	FitPart(&p, &p.Tracks[0], 0, r)
	if p.Style != "" || p.Key == nil || p.Tracks[0].Role != "lead" || len(p.Tracks[0].Patterns[0].Steps) != 32 {
		t.Fatalf("unstyled 303 fit: style %q role %q steps %d", p.Style, p.Tracks[0].Role, len(p.Tracks[0].Patterns[0].Steps))
	}
	i, _ := AddTrack(&p, Dual, r)
	d := &p.Tracks[i]
	if d.Role != "" || len(d.Patterns[0].Steps) != 32 {
		t.Fatalf("unstyled add: role %q, %d steps", d.Role, len(d.Patterns[0].Steps))
	}
	FitPart(&p, d, 0, r)
	bass, _ := medianNote(p.Tracks[0].Patterns[0].Steps)
	if n, _ := medianNote(d.Patterns[0].Steps); d.Role != "upper" || n < bass+12 {
		t.Fatalf("unstyled Dual fit: role %q at %d", d.Role, n)
	}
	// Generate without a project treats Fit groove as the usual first pattern.
	k := NewTrack(Kick, 0)
	Generate(&k, 0, FitAction)
	if !k.Patterns[0].Steps[0].Active || !k.Patterns[0].Steps[4].Active {
		t.Fatal("Fit without a project did not write four to the floor")
	}
}

// Style and roles survive saving, groove history and randomising.
func TestStyleAndRolesPersist(t *testing.T) {
	r := rand.New(rand.NewPCG(8, 9))
	p, _ := newGroove(GrooveStyles[1], r)
	if p.Style != GrooveStyles[1].Name {
		t.Fatalf("new groove style %q", p.Style)
	}
	path := filepath.Join(t.TempDir(), "g.gruvbox3")
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if q.Style != p.Style || leadTrack(q) == nil {
		t.Fatalf("saved style %q / lead role lost", q.Style)
	}
	snap := CaptureGroove(p, "x")
	RandomiseGroove(&q, r)
	if q.Style == "" {
		t.Fatal("randomise left no style")
	}
	for _, tr := range q.Tracks {
		if tr.Kind.synthPart() && tr.Role == "" {
			t.Fatalf("randomise left %s without a role", tr.Name)
		}
	}
	if _, err := RestoreGroove(&q, snap); err != nil {
		t.Fatal(err)
	}
	if q.Style != p.Style || leadTrack(q) == nil {
		t.Fatalf("restored style %q / roles", q.Style)
	}
}

// collisionRate is the share of a line's notes that start with a bass note,
// and the bass's density (what an unrelated line would hit).
func collisionRate(bass Pattern, line []Step) (rate, density float64) {
	hits := bassOnsets(bass, len(line))
	notes, both, on := 0, 0, 0
	for i, s := range line {
		if hits[i] {
			on++
		}
		if s.Active {
			notes++
			if hits[i] {
				both++
			}
		}
	}
	return float64(both) / float64(max(notes, 1)), float64(on) / float64(max(len(line), 1))
}

// Counter lines, stabs and second lines play in the bass's rests: far fewer
// of their notes land on bass notes than an unrelated line's would, and they
// keep at least half their notes.
func TestUpperPartsAnswerTheBass(t *testing.T) {
	var rate, density float64
	n := 0
	for _, style := range GrooveStyles {
		for seed := uint64(1); seed <= 20; seed++ {
			r := rand.New(rand.NewPCG(seed, 31))
			p, _ := newGroove(style, r)
			for _, kind := range []Kind{Dual, Acid} {
				q := p.Clone()
				i, _ := AddTrack(&q, kind, r)
				d := &q.Tracks[i]
				if !answers(d.Role) {
					continue
				}
				FitPart(&q, d, 0, r)
				bass, _ := bassLine(&q, d.ID, 0)
				notes := 0
				for _, s := range d.Patterns[0].Steps {
					if s.Active {
						notes++
					}
				}
				if notes == 0 {
					t.Fatalf("%s/%d: %s %s lost every note", style.Name, seed, kind.Name(), d.Role)
				}
				a, b := collisionRate(bass.Patterns[0], d.Patterns[0].Steps)
				rate, density, n = rate+a, density+b, n+1
			}
		}
	}
	rate, density = rate/float64(n), density/float64(n)
	t.Logf("%d parts: %.0f%% of notes on bass onsets (bass density %.0f%%)", n, rate*100, density*100)
	if n < 40 || rate > density*.5 {
		t.Fatalf("upper parts do not answer the bass: %.2f on onsets vs density %.2f over %d parts", rate, density, n)
	}
}

func TestBassOnsetsAcrossLoopLengths(t *testing.T) {
	bass := NewPattern()
	bass.Steps[3].Active = true
	hits := bassOnsets(bass, 32) // a 1-bar bass under a 2-bar line
	if !hits[3] || !hits[19] || hits[4] {
		t.Fatalf("1-bar bass onsets on a 2-bar line: %v", hits)
	}
	long := Pattern{Steps: make([]Step, 32), Resolution: 16}
	long.Steps[20].Active = true
	if hits := bassOnsets(long, 16); !hits[4] || hits[3] { // a 2-bar bass under a 1-bar line
		t.Fatalf("2-bar bass onsets on a 1-bar line: %v", hits)
	}
}
