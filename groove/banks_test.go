package groove

import (
	"bytes"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSlotNamesAndBanks(t *testing.T) {
	for slot, want := range map[int]string{0: "A1", 7: "A8", 8: "B1", 18: "C3", 63: "H8", 64: "?", -1: "?"} {
		if got := SlotName(slot); got != want {
			t.Errorf("SlotName(%d) = %s, want %s", slot, got, want)
		}
	}
	if BankOf(18) != 2 || NextInBank(15) != 8 || NextInBank(9) != 10 || PatternCount != 64 {
		t.Fatal("bank arithmetic")
	}
}

func TestVersionOneProjectsGetSixtyFourSlots(t *testing.T) {
	p, _ := RandomGroove(rand.New(rand.NewPCG(3, 4)))
	// Write it as a version 1 file: 8 full slots per track.
	old := p.Clone()
	old.FormatVersion = 1
	for i := range old.Tracks {
		old.Tracks[i].Patterns = old.Tracks[i].Patterns[:8]
	}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FormatVersion != FormatVersion {
		t.Fatalf("version %d", loaded.FormatVersion)
	}
	for i, tr := range loaded.Tracks {
		if len(tr.Patterns) != PatternCount {
			t.Fatalf("track %d has %d slots", i, len(tr.Patterns))
		}
		if !reflect.DeepEqual(tr.Patterns[:8], p.Tracks[i].Patterns[:8]) {
			t.Fatalf("track %d patterns changed", i)
		}
		if !reflect.DeepEqual(tr.Patterns[40], NewPattern()) {
			t.Fatal("added slots are not blank")
		}
	}
	// Formats from the future are still refused.
	future := strings.Replace(string(data), `"format_version":1`, `"format_version":9`, 1)
	if _, err := Decode(strings.NewReader(future)); err == nil {
		t.Fatal("accepted format 9")
	}
}

func TestBlankSlotsSaveSmallAndRoundTrip(t *testing.T) {
	p, _ := RandomGroove(rand.New(rand.NewPCG(5, 6)))
	p.Tracks[0].Patterns[42].Steps[3].Active = true // one note in F3
	p.Tracks[1].Patterns[50].Steps[0].Velocity = .3 // an edited but silent step
	path := filepath.Join(t.TempDir(), "g.gruvbox3")
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if len(data) > 160_000 {
		t.Fatalf("a groove with 64 slots saved as %d bytes", len(data))
	}
	f, _ := os.Open(path)
	defer f.Close()
	loaded, err := Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Tracks, p.Tracks) {
		t.Fatal("tracks changed in the round trip")
	}
	if len(p.Tracks[0].Patterns[63].Steps) != 16 {
		t.Fatal("saving changed the project in memory")
	}
}

func TestStatelessSlotsShareOnePlaybackBank(t *testing.T) {
	p := NewProject()
	if err := StorePatternState(&p, 12, ""); err != nil {
		t.Fatal(err)
	}
	player := NewPlayer()
	player.Publish(p)
	banks := player.prepared.banks
	if &banks[0][0] != &banks[40][0] {
		t.Fatal("stateless slots prepared separately")
	}
	if &banks[12][0] == &banks[0][0] {
		t.Fatal("a slot with stored state shares the plain bank")
	}
}

func TestGrowFillsTheSourceBank(t *testing.T) {
	if got := GrowSlots(18); !reflect.DeepEqual(got, []int{16, 17, 19, 20, 21, 22, 23}) {
		t.Fatalf("GrowSlots(C3) = %v", got)
	}
	p, _ := RandomGroove(rand.New(rand.NewPCG(9, 9)))
	if err := CopyPattern(&p, 0, 18, ""); err != nil {
		t.Fatal(err)
	}
	p.Pattern = 18
	status, err := GrowTrack(&p, rand.New(rand.NewPCG(1, 2)), false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "peak is pattern C3") {
		t.Fatalf("status: %s", status)
	}
	for _, s := range p.Song {
		if BankOf(s.Pattern) != 2 {
			t.Fatalf("section %s uses %s", s.Name, SlotName(s.Pattern))
		}
	}
	for slot := 1; slot < BankSize; slot++ {
		if !PatternEmpty(p, slot) {
			t.Fatalf("Grow wrote outside its bank: %s", SlotName(slot))
		}
	}
}
