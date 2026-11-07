package groove

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func grownGroove(t *testing.T, seed uint64, full bool) (Project, Project, []int) {
	t.Helper()
	p, _ := RandomGroove(rand.New(rand.NewPCG(seed, 2)))
	p.Pattern = 2
	CopyPattern(&p, 0, 2, "")
	for i := range p.Tracks {
		resetPatternSlot(&p.Tracks[i], 0, 16)
	}
	before := p.Clone()
	if _, err := GrowTrack(&p, rand.New(rand.NewPCG(seed, 3)), full); err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("grown project invalid: %v", err)
	}
	return before, p, GrowSlots(2)
}

func activeCount(p Pattern) int {
	n := 0
	for _, s := range p.Steps {
		if s.Active {
			n++
		}
	}
	return n
}
func trackOf(p Project, kind Kind) *Track {
	for i := range p.Tracks {
		if p.Tracks[i].Kind == kind {
			return &p.Tracks[i]
		}
	}
	return nil
}

func TestGrowLaysOutAPlayableSong(t *testing.T) {
	for _, full := range []bool{false, true} {
		before, p, slots := grownGroove(t, 11, full)
		if p.Pattern != 2 {
			t.Fatal("selected pattern moved")
		}
		for i := range p.Tracks {
			if !reflect.DeepEqual(p.Tracks[i].Patterns[2], before.Tracks[i].Patterns[2]) {
				t.Fatal("the source pattern changed")
			}
		}
		names := []string{}
		bars := 0
		for _, s := range p.Song {
			names = append(names, s.Name)
			bars += s.Repeats
		}
		want := []string{"Intro", "Build", "Riser", "Peak", "Fill", "Breakdown", "Riser", "Peak 2", "Fill", "Outro"}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("sections %v", names)
		}
		if p.Song[3].Pattern != 2 || p.Song[0].Pattern != slots[growIntro] || p.Song[9].Pattern != slots[growIntro] || p.Song[2].Pattern != p.Song[6].Pattern {
			t.Fatal("sections point at the wrong patterns")
		}
		// Each peak plus its fill is a whole number of 16-bar phrases.
		if (p.Song[3].Repeats+p.Song[4].Repeats)%16 != 0 || p.Song[4].Repeats != 1 {
			t.Fatalf("peak %d + fill %d", p.Song[3].Repeats, p.Song[4].Repeats)
		}
		if full && bars < 120 || !full && (bars < 60 || bars > 80) {
			t.Fatalf("%d bars (full %v)", bars, full)
		}
		for _, slot := range slots {
			if silent(p, slot) {
				t.Fatalf("slot %d is silent", slot+1)
			}
		}
	}
}

func TestGrowSectionsHaveTheirShape(t *testing.T) {
	_, p, slots := grownGroove(t, 5, false)
	kick, snare, hat, lead := trackOf(p, Kick), trackOf(p, Snare), trackOf(p, ClosedHat), &p.Tracks[0]
	if activeCount(kick.Patterns[slots[growIntro]]) == 0 || activeCount(hat.Patterns[slots[growIntro]]) == 0 {
		t.Fatal("intro lacks kick or hats")
	}
	if activeCount(snare.Patterns[slots[growIntro]]) != 0 || activeCount(lead.Patterns[slots[growIntro]]) != 0 {
		t.Fatal("intro should be kick and hats only")
	}
	if activeCount(snare.Patterns[slots[growBuild]]) != 0 {
		t.Fatal("build has a snare")
	}
	if activeCount(kick.Patterns[slots[growBreakdown]]) != 0 || activeCount(snare.Patterns[slots[growBreakdown]]) != 0 {
		t.Fatal("breakdown keeps kick or snare")
	}

	// The build's synth sits filtered below the peak.
	cutoff := lead.Parameters["cutoff"]
	for _, s := range lead.Patterns[slots[growBuild]].Steps {
		if s.Locks["cutoff"] >= cutoff {
			t.Fatalf("build cutoff %v not below %v", s.Locks["cutoff"], cutoff)
		}
	}

	// The riser is four bars: the cutoff climbs past the peak's, the roll
	// gets denser and louder, the kick drops out on the last beat.
	riser := lead.Patterns[slots[growRiser]]
	if len(riser.Steps) != 64 {
		t.Fatalf("riser %d steps", len(riser.Steps))
	}
	for i := 1; i < len(riser.Steps); i++ {
		if riser.Steps[i].Locks["cutoff"] < riser.Steps[i-1].Locks["cutoff"] {
			t.Fatal("riser cutoff falls")
		}
	}
	if riser.Steps[0].Locks["cutoff"] >= cutoff || riser.Steps[63].Locks["cutoff"] <= cutoff {
		t.Fatalf("riser ramp %v → %v around %v", riser.Steps[0].Locks["cutoff"], riser.Steps[63].Locks["cutoff"], cutoff)
	}
	roll := snare.Patterns[slots[growRiser]]
	if a, b := activeCount(Pattern{Steps: roll.Steps[:16]}), activeCount(Pattern{Steps: roll.Steps[48:]}); a >= b {
		t.Fatalf("roll not getting denser: %d then %d", a, b)
	}
	if roll.Steps[48].Velocity <= roll.Steps[0].Velocity {
		t.Fatal("roll not getting louder")
	}
	for _, s := range kick.Patterns[slots[growRiser]].Steps[60:] {
		if s.Active {
			t.Fatal("kick plays through the last beat of the riser")
		}
	}

	// The breakdown's lead is longer; peak 2 is hotter and varied; neither
	// stores a sound that following the song would carry into other slots.
	for _, track := range p.Tracks {
		for _, slot := range slots {
			if track.Patterns[slot].State != nil {
				t.Fatalf("%s: slot %d stores a sound", track.Name, slot+1)
			}
		}
	}
	if bd := lead.Patterns[slots[growBreakdown]].Steps[0].Locks; bd["decay"] <= lead.Parameters["decay"] {
		t.Fatal("breakdown lead not longer")
	}
	p2 := lead.Patterns[slots[growPeak2]]
	if p2.Steps[0].Locks["sweep"] <= lead.Parameters["sweep"] {
		t.Fatal("peak 2 lead not hotter")
	}
	if reflect.DeepEqual(p2.Steps, lead.Patterns[2].Steps) {
		t.Fatal("peak 2 line not varied")
	}
	if fill2 := lead.Patterns[slots[growFill2]]; fill2.Steps[0].Locks["sweep"] != p2.Steps[0].Locks["sweep"] {
		t.Fatal("fill 2 should keep peak 2's sound")
	}
}

func TestGrowFillIsTheLastBarWithAFill(t *testing.T) {
	p, _ := RandomGroove(rand.New(rand.NewPCG(1, 1)))
	p.Tracks = append(p.Tracks, NewTrack(Tom, len(p.Tracks)))
	for i := range p.Tracks {
		resetPatternSlot(&p.Tracks[i], 0, 64)
		for n := range p.Tracks[i].Patterns[0].Steps {
			p.Tracks[i].Patterns[0].Steps[n].Active = n%3 == 0
			p.Tracks[i].Patterns[0].Steps[n].Note = 36 + n%12
		}
	}
	if _, err := GrowTrack(&p, rand.New(rand.NewPCG(1, 1)), false); err != nil {
		t.Fatal(err)
	}
	slots := GrowSlots(0)
	lead, tom := p.Tracks[0], trackOf(p, Tom)
	fill := lead.Patterns[slots[growFill]]
	// The peak runs 15 bars of a 4-bar pattern, so the fill is its bar 4.
	if len(fill.Steps) != 16 || !reflect.DeepEqual(fill.Steps, lead.Patterns[0].Steps[48:]) {
		t.Fatal("fill bar does not continue the peak")
	}
	toms := tom.Patterns[slots[growFill]].Steps
	for i := 12; i < 16; i++ {
		if !toms[i].Active {
			t.Fatal("tom fill missing")
		}
		if i > 12 && toms[i].Locks["pitch"] >= toms[i-1].Locks["pitch"] {
			t.Fatal("toms should fall")
		}
	}
}

func TestGrowHandlesThinProjects(t *testing.T) {
	// Only a 303: there is no kick or hat for the intro, so it plays the
	// filtered line instead of silence.
	p := NewProject()
	p.Tracks = p.Tracks[:1]
	key := Key{Root: 9, Scale: PhraseMinor}
	p.Key = &key
	generate(&p.Tracks[0], 0, "Acid", &key, rand.New(rand.NewPCG(1, 2)))
	if _, err := GrowTrack(&p, rand.New(rand.NewPCG(1, 2)), false); err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, slot := range GrowSlots(0) {
		if silent(p, slot) {
			t.Fatalf("slot %d silent", slot+1)
		}
	}

	// Only drums, at 1/32: the riser stays within 64 steps.
	q := NewProject()
	q.Tracks = q.Tracks[2:]
	for i := range q.Tracks {
		q.Tracks[i].Patterns[0].Resolution = 32
		resetPatternSlot(&q.Tracks[i], 0, 32)
		q.Tracks[i].Patterns[0].Resolution = 32
		q.Tracks[i].Patterns[0].Steps[0].Active = true
		q.Tracks[i].Patterns[0].Steps[16].Active = true
	}
	if _, err := GrowTrack(&q, rand.New(rand.NewPCG(1, 2)), true); err != nil {
		t.Fatal(err)
	}
	if err := q.Validate(); err != nil {
		t.Fatal(err)
	}
	if n := len(q.Tracks[0].Patterns[GrowSlots(0)[growRiser]].Steps); n != 64 {
		t.Fatalf("riser %d steps", n)
	}

	// An empty pattern has nothing to grow.
	e := NewProject()
	e.Pattern = 5
	if _, err := GrowTrack(&e, rand.New(rand.NewPCG(1, 2)), false); err == nil {
		t.Fatal("grew an empty pattern")
	}
}

func TestGrowIsRepeatableAndReportsWhatItReplaces(t *testing.T) {
	before, a, _ := grownGroove(t, 21, false)
	b := before.Clone()
	if _, err := GrowTrack(&b, rand.New(rand.NewPCG(21, 3)), false); err != nil {
		t.Fatal(err)
	}
	for i := range a.Tracks {
		if !reflect.DeepEqual(a.Tracks[i].Patterns, b.Tracks[i].Patterns) {
			t.Fatal("not repeatable")
		}
	}
	slots, song := GrowReplaces(a)
	if len(slots) != growSlots || !song {
		t.Fatalf("replaces %v, song %v", slots, song)
	}
	fresh, _ := RandomGroove(rand.New(rand.NewPCG(3, 3)))
	if slots, song := GrowReplaces(fresh); len(slots) != 0 || song {
		t.Fatalf("fresh groove replaces %v, song %v", slots, song)
	}
}

// TestWriteGrownTracks renders whole grown tracks in each style:
// GRUVBOX_GROW_WAVS=<dir> go test ./groove -run TestWriteGrownTracks
func TestWriteGrownTracks(t *testing.T) {
	dir := os.Getenv("GRUVBOX_GROW_WAVS")
	if dir == "" {
		t.Skip("set GRUVBOX_GROW_WAVS to a directory to hear grown tracks")
	}
	for i, style := range GrooveStyles {
		r := rand.New(rand.NewPCG(uint64(i)+40, 7))
		p, _ := newGroove(style, r)
		if _, err := GrowTrack(&p, r, false); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%d %s.wav", i+1, style.Name)))
		if err != nil {
			t.Fatal(err)
		}
		if err := RenderWAV(context.Background(), f, p, true, 1, nil); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}

// Song mode selects each section's pattern as it plays; that must not leave
// one section's sound behind in the next.
func TestFollowingAGrownSongKeepsTheSound(t *testing.T) {
	_, p, _ := grownGroove(t, 8, false)
	want := p.Clone()
	for round := 0; round < 2; round++ {
		for _, section := range p.Song {
			if err := SelectPattern(&p, section.Pattern); err != nil {
				t.Fatal(err)
			}
		}
	}
	SelectPattern(&p, want.Pattern)
	for i := range p.Tracks {
		if !reflect.DeepEqual(p.Tracks[i].Parameters, want.Tracks[i].Parameters) || !reflect.DeepEqual(p.Tracks[i].Effects, want.Tracks[i].Effects) {
			t.Fatalf("%s sound changed by following the song", p.Tracks[i].Name)
		}
	}
}
