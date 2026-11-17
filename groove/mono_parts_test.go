package groove

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Every idiom writes notes in the key, in a sensible register, for every
// scale and loop length.
func TestMonoIdiomsWriteInKey(t *testing.T) {
	for _, action := range monoIdioms {
		for _, scale := range []PhraseScale{PhraseMinorPentatonic, PhraseMinor, PhraseDorian, PhrasePhrygian, PhraseMajorPentatonic} {
			for _, n := range []int{16, 32, 64} {
				for seed := uint64(1); seed <= 8; seed++ {
					key := Key{Root: int(seed % 12), Scale: scale}
					steps := make([]Step, n)
					if !writeIdiom(steps, action, key, rand.New(rand.NewPCG(seed, 9))) {
						t.Fatalf("%s unknown", action)
					}
					h := harmony{key: key}
					active := 0
					for i, s := range steps {
						if !s.Active {
							continue
						}
						active++
						if !h.inScale(s.Note) {
							t.Fatalf("%s %s/%d seed %d: step %d note %d out of key", action, scale, n, seed, i, s.Note)
						}
						if s.Note < key.BassRoot()-5 || s.Note > key.BassRoot()+31 {
							t.Fatalf("%s %s: note %d far from the bass root %d", action, scale, s.Note, key.BassRoot())
						}
					}
					if active < n/16 {
						t.Fatalf("%s %s/%d seed %d: only %d notes", action, scale, n, seed, active)
					}
				}
			}
		}
	}
}

// The arpeggio plays the chord of each bar, and plays a lot.
func TestArpPlaysTheBarsChord(t *testing.T) {
	for seed := uint64(1); seed <= 30; seed++ {
		key := Key{Root: 9, Scale: PhraseMinor}
		steps := make([]Step, 64)
		r := rand.New(rand.NewPCG(seed, 1))
		writeIdiom(steps, "Arp", key, r)
		for bar := 0; bar < 4; bar++ {
			classes, notes := map[int]bool{}, 0
			for _, s := range steps[bar*16 : bar*16+16] {
				if s.Active {
					classes[s.Note%12] = true
					notes++
				}
			}
			if len(classes) > 4 || notes < 5 {
				t.Fatalf("seed %d bar %d: %d pitch classes, %d notes", seed, bar, len(classes), notes)
			}
		}
	}
}

// A sequence is a short figure that repeats: over one chord the rhythm and
// the notes repeat with a period of 3 to 8 notes (6 to 16 steps in eighths).
func TestSequenceRepeatsAShortFigure(t *testing.T) {
	for seed := uint64(1); seed <= 30; seed++ {
		key := Key{Root: 2, Scale: PhraseDorian}
		steps := make([]Step, 64)
		writeSequence(steps, harmony{key: key, base: key.BassRoot(), roots: []int{0}}, rand.New(rand.NewPCG(seed, 2)))
		found := false
		for period := 3; period <= 16 && !found; period++ {
			found = true
			for i := period; i < len(steps); i++ {
				a, b := steps[i], steps[i-period]
				if a.Active != b.Active || a.Active && a.Note != b.Note {
					found = false
					break
				}
			}
		}
		if !found {
			t.Fatalf("seed %d: no repeating figure", seed)
		}
	}
}

// The Dual's lead is a few long notes; its stabs are short hits in one of
// the stab rhythms.
func TestDualLeadIsHeldAndStabsAreShort(t *testing.T) {
	slides := 0
	for seed := uint64(1); seed <= 30; seed++ {
		r := rand.New(rand.NewPCG(seed, 3))
		lead := make([]Step, 32)
		writeIdiom(lead, "Lead", Key{Root: 4, Scale: PhraseMinorPentatonic}, r)
		for bar := 0; bar < 2; bar++ {
			onsets := 0
			for _, s := range lead[bar*16 : bar*16+16] {
				if s.Active {
					onsets++
					if s.Slide {
						slides++
					}
				}
			}
			if onsets < 1 || onsets > 4 {
				t.Fatalf("seed %d bar %d: %d lead notes", seed, bar, onsets)
			}
		}
		for i, s := range lead {
			if s.Active && s.Length < 2 && i < len(lead)-1 && !lead[i+1].Active {
				t.Fatalf("seed %d: lead note at %d is not held", seed, i)
			}
		}
		stabs := make([]Step, 16)
		writeIdiom(stabs, "Stabs", Key{Root: 4, Scale: PhraseMinor}, r)
		rhythm := []byte("................")
		for i, s := range stabs {
			if s.Active {
				rhythm[i] = 'x'
				if s.GateSteps() != 1 {
					t.Fatalf("seed %d: stab %d lasts %d steps", seed, i, s.GateSteps())
				}
			}
		}
		if !slices.Contains(stabRhythms, string(rhythm)) {
			t.Fatalf("seed %d: stab rhythm %s", seed, rhythm)
		}
	}
	if slides == 0 {
		t.Fatal("no lead note ever slides")
	}
}

// In styled grooves the two synths now play different parts: the SH-101
// arpeggios, sequences and basslines, the Dual stabs, melodies, pads,
// counter lines and layers. The SH-101's parts are busier and the Dual's
// notes longer.
func TestDualAndSH101PlayDifferentParts(t *testing.T) {
	type tally struct{ notes, steps, length int }
	parts := map[Kind]*tally{Dual: {}, SH101: {}}
	roles := map[Kind]map[string]bool{Dual: {}, SH101: {}}
	for _, style := range GrooveStyles {
		for seed := uint64(1); seed <= 12; seed++ {
			for _, kind := range []Kind{Dual, SH101} {
				r := rand.New(rand.NewPCG(seed, 31))
				p, _ := newGroove(style, r)
				i, _ := AddTrack(&p, kind, r)
				tr := &p.Tracks[i]
				FitPart(&p, tr, 0, r)
				if err := p.Validate(); err != nil {
					t.Fatal(err)
				}
				roles[kind][tr.Role] = true
				for _, s := range tr.Patterns[0].Steps {
					parts[kind].steps++
					if s.Active {
						parts[kind].notes++
						parts[kind].length += s.GateSteps()
					}
				}
			}
		}
	}
	for role := range roles[SH101] {
		if role != "arp" && role != "sequence" && role != "bass" {
			t.Errorf("SH-101 took the Dual's role %q", role)
		}
	}
	for _, role := range []string{"melody", "stab"} {
		if !roles[Dual][role] {
			t.Errorf("no Dual %s in any groove", role)
		}
	}
	d, s := parts[Dual], parts[SH101]
	dualDensity, shDensity := float64(d.notes)/float64(d.steps), float64(s.notes)/float64(s.steps)
	dualLength, shLength := float64(d.length)/float64(d.notes), float64(s.length)/float64(s.notes)
	t.Logf("Dual: %.0f%% of steps, %.1f steps a note; SH-101: %.0f%%, %.1f", dualDensity*100, dualLength, shDensity*100, shLength)
	if shDensity < dualDensity*1.5 || dualLength < shLength*1.5 {
		t.Fatal("the two synths' parts are too alike")
	}
}

// Generate buttons: the new actions write notes on their own synth.
func TestIdiomActionsAreOnTheirSynths(t *testing.T) {
	for kind, actions := range map[Kind][]string{Dual: {"Lead", "Stabs"}, SH101: {"Arp", "Sequence"}} {
		for _, action := range actions {
			if !slices.Contains(Actions(kind), action) {
				t.Fatalf("%s lacks %s", kind, action)
			}
			p := NewProject()
			p.Tracks = []Track{NewTrack(kind, 0)}
			GenerateIn(&p, &p.Tracks[0], 0, action)
			if !hasNotes(p.Tracks[0].Patterns[0]) || p.Key == nil {
				t.Fatalf("%s %s wrote nothing", kind, action)
			}
		}
	}
}

// GRUVBOX_MONO_WAVS=<dir> go test ./groove -run TestWriteMonoPartSamples
// writes, per style, a groove with a Dual added and one with an SH-101
// added (Fit groove), eight bars each, in full and with the new part solo.
func TestWriteMonoPartSamples(t *testing.T) {
	dir := os.Getenv("GRUVBOX_MONO_WAVS")
	if dir == "" {
		t.Skip("set GRUVBOX_MONO_WAVS to a directory to write example parts")
	}
	r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	for _, style := range GrooveStyles {
		for _, kind := range []Kind{Dual, SH101} {
			p := NewGrooveInStyle(style, r)
			i, info := AddTrack(&p, kind, r)
			FitPart(&p, &p.Tracks[i], 0, r)
			t.Log(info)
			section := NewSongSection(0)
			section.Repeats = 8
			p.Song = []SongSection{section}
			for _, solo := range []bool{false, true} {
				q := p.Clone()
				q.Tracks[i].Solo = solo
				name := fmt.Sprintf("%s %s %s%s.wav", style.Name, kind, q.Tracks[i].Role, map[bool]string{true: " solo"}[solo])
				f, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				if err := RenderWAV(context.Background(), f, q, true, 1, nil); err != nil {
					t.Fatal(err)
				}
				f.Close()
				t.Log(name)
			}
		}
	}
}
