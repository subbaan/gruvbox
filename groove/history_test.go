package groove

import (
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRestoreBringsBackARandomisedGroove(t *testing.T) {
	p, _ := RandomGroove(rand.New(rand.NewPCG(6, 6)))
	snap := CaptureGroove(p, "first")
	want := p.Clone()
	RandomiseGroove(&p, rand.New(rand.NewPCG(7, 7)))
	if n, err := RestoreGroove(&p, snap); err != nil || n != len(p.Tracks) {
		t.Fatalf("matched %d: %v", n, err)
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if p.BPM != want.BPM || p.Swing != want.Swing || *p.Key != *want.Key {
		t.Fatal("tempo or key not restored")
	}
	for i := range p.Tracks {
		a, b := p.Tracks[i], want.Tracks[i]
		if !reflect.DeepEqual(a.Patterns[0], b.Patterns[0]) || !reflect.DeepEqual(a.Parameters, b.Parameters) || !reflect.DeepEqual(a.Effects, b.Effects) || a.LFOs != b.LFOs {
			t.Fatalf("%s not restored", a.Name)
		}
	}
}

func TestFavouritesTravelBetweenProjectsAndFiles(t *testing.T) {
	src, _ := RandomGroove(rand.New(rand.NewPCG(8, 8)))
	path := filepath.Join(t.TempDir(), "favourites.json")
	if list, err := LoadFavourites(path); err != nil || list != nil {
		t.Fatalf("missing file: %v %v", list, err)
	}
	if err := SaveFavourites(path, []GrooveSnapshot{CaptureGroove(src, "keeper")}); err != nil {
		t.Fatal(err)
	}
	list, err := LoadFavourites(path)
	if err != nil || len(list) != 1 || list[0].Label != "keeper" {
		t.Fatalf("%v %v", list, err)
	}
	// Another project with the same kinds of track, other track IDs, an
	// extra clap, pattern 4 selected.
	p, _ := RandomGroove(rand.New(rand.NewPCG(9, 9)))
	p.Tracks = nil
	for i, track := range src.Tracks {
		p.Tracks = append(p.Tracks, NewTrack(track.Kind, i))
	}
	p.Tracks = append(p.Tracks, NewTrack(Clap, len(p.Tracks)))
	p.Pattern = 3
	clap := p.Tracks[len(p.Tracks)-1]
	n, err := RestoreGroove(&p, list[0])
	if err != nil || n != len(src.Tracks) {
		t.Fatalf("matched %d: %v", n, err)
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	for i := range src.Tracks {
		if !reflect.DeepEqual(p.Tracks[i].Patterns[3].Steps, src.Tracks[i].Patterns[0].Steps) {
			t.Fatalf("%s pattern not restored into the selected slot", p.Tracks[i].Name)
		}
	}
	if !reflect.DeepEqual(p.Tracks[len(p.Tracks)-1], clap) {
		t.Fatal("unmatched clap changed")
	}

	// Held tracks and their key stay.
	q, _ := RandomGroove(rand.New(rand.NewPCG(10, 10)))
	q.Tracks[0].Hold = true
	held, key := q.Tracks[0], *q.Key
	if _, err := RestoreGroove(&q, list[0]); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(q.Tracks[0], held) || *q.Key != key {
		t.Fatal("restore changed a held part or its key")
	}

	// Nothing in common.
	e := NewProject()
	e.Tracks = []Track{NewTrack(Sampler, 0)}
	if _, err := RestoreGroove(&e, list[0]); err == nil {
		t.Fatal("restored into a project with no matching tracks")
	}
}
