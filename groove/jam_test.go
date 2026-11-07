package groove

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestJamMakesSmallValidChangesAndLeavesHeldTracks(t *testing.T) {
	p, _ := RandomGroove(rand.New(rand.NewPCG(4, 4)))
	p.Tracks[2].Hold = true // the kick
	held := p.Tracks[2]
	r := rand.New(rand.NewPCG(1, 1))
	state := &JamState{}
	kinds := map[string]int{}
	for n := 0; n < 400; n++ {
		before := p.Clone()
		info := Jam(&p, r, state)
		if info == "" {
			t.Fatal("no move")
		}
		kinds[strings.Fields(info)[0]]++
		if err := p.Validate(); err != nil {
			t.Fatalf("%s: %v", info, err)
		}
		if !reflect.DeepEqual(p.Tracks[2], held) {
			t.Fatalf("%s changed a held track", info)
		}
		// One move touches at most one track.
		changed := 0
		for i := range p.Tracks {
			if !reflect.DeepEqual(p.Tracks[i], before.Tracks[i]) {
				changed++
			}
		}
		if changed > 1 {
			t.Fatalf("%s changed %d tracks", info, changed)
		}
		if len(state.Dropped) > state.maxDropped() {
			t.Fatalf("%d parts dropped", len(state.Dropped))
		}
		for _, track := range p.Tracks {
			if track.Kind == Acid {
				for id, limits := range jamRanges {
					if v, was := track.Parameters[id], before.Track(track.ID).Parameters[id]; v != was && (v < limits[0]-1e-9 || v > limits[1]+1e-9) {
						t.Fatalf("%s %s left its range: %v", track.Name, id, v)
					}
				}
			}
		}
	}
	for _, kind := range []string{"varied", "dropped", "brought"} {
		if kinds[kind] == 0 {
			t.Fatalf("never %s: %v", kind, kinds)
		}
	}
	if len(kinds) < 4 {
		t.Fatalf("too few kinds of move: %v", kinds)
	}
}

func TestJamOnlyReturnsWhatItDropped(t *testing.T) {
	p, _ := RandomGroove(rand.New(rand.NewPCG(5, 5)))
	p.Tracks[3].Mute = true // muted by hand
	state := &JamState{}
	r := rand.New(rand.NewPCG(2, 2))
	for n := 0; n < 300; n++ {
		Jam(&p, r, state)
		if !p.Tracks[3].Mute {
			t.Fatal("jam unmuted a track muted by hand")
		}
	}
	// Everything held: nothing to do.
	for i := range p.Tracks {
		p.Tracks[i].Hold = true
	}
	if info := Jam(&p, r, state); info != "" {
		t.Fatalf("moved a held groove: %s", info)
	}
}

func TestNeighbourNoteStaysInKeyAndClose(t *testing.T) {
	key := Key{Root: 2, Scale: PhraseMinor}
	r := rand.New(rand.NewPCG(3, 3))
	scale := map[int]bool{}
	for _, d := range scaleNotes(key.Scale) {
		scale[(key.Root+d)%12] = true
	}
	for n := 0; n < 200; n++ {
		note := 38 + r.IntN(24)
		next := neighbourNote(note, &key, r)
		if !scale[next%12] || abs(next-note) > 7 || next == note && scale[note%12] {
			t.Fatalf("%d → %d", note, next)
		}
	}
}

func TestJamEvolvingAndWildMakeBiggerChanges(t *testing.T) {
	changedSteps := map[JamAmount]int{}
	for _, amount := range []JamAmount{JamGentle, JamEvolving, JamWild} {
		p, _ := RandomGroove(rand.New(rand.NewPCG(8, 9)))
		p.Tracks[2].Hold = true
		held := p.Tracks[2]
		start := p.Clone()
		r := rand.New(rand.NewPCG(uint64(amount)+1, 7))
		state := &JamState{Amount: amount}
		seen := map[string]int{}
		for n := 0; n < 300; n++ {
			before := p.Clone()
			info := Jam(&p, r, state)
			if info == "" {
				t.Fatal("no move")
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("%s: %v", info, err)
			}
			if !reflect.DeepEqual(p.Tracks[2], held) {
				t.Fatalf("%s changed a held track", info)
			}
			changed := 0
			for i := range p.Tracks {
				if !reflect.DeepEqual(p.Tracks[i], before.Tracks[i]) {
					changed++
				}
			}
			if limit := []int{1, 2, 3}[amount]; changed > limit || len(strings.Split(info, " · ")) > limit {
				t.Fatalf("%s: %d tracks changed by %q", JamAmounts[amount], changed, info)
			}
			if len(state.Dropped) > state.maxDropped() {
				t.Fatalf("%d parts dropped", len(state.Dropped))
			}
			hits := 0
			if crash := trackOf(p, Crash); crash != nil {
				hits = hitCount(crash.Patterns[p.Pattern].Steps)
			}
			if hits > 1 {
				t.Fatalf("%s: crash hits %d times a loop", info, hits)
			}
			for _, move := range []string{"→", "new ", "brought in", "sound →", " mix ", "dropped", "brought back", "varied"} {
				if strings.Contains(info, move) {
					seen[move]++
				}
			}
			if n == 23 {
				// How far the groove has travelled after 24 changes.
				for i := range p.Tracks {
					for j, s := range p.Tracks[i].Patterns[p.Pattern].Steps {
						if was := start.Tracks[i].Patterns[p.Pattern].Steps[j]; s.Active != was.Active || s.Active && s.Note != was.Note {
							changedSteps[amount]++
						}
					}
				}
			}
		}
		if amount != JamGentle {
			for _, move := range []string{"→", "new ", "brought in", "sound →", " mix ", "dropped", "brought back", "varied"} {
				if seen[move] == 0 {
					t.Errorf("%s never made a %q move: %v", JamAmounts[amount], move, seen)
				}
			}
		} else if seen["→"]+seen["new "]+seen["brought in"] > 0 {
			t.Errorf("Gentle made big moves: %v", seen)
		}
	}
	if !(changedSteps[JamGentle] < changedSteps[JamEvolving] && changedSteps[JamEvolving] < changedSteps[JamWild]) {
		t.Fatalf("steps changed after 24 changes: %v", changedSteps)
	}
}

func TestJamBuildsThenReleases(t *testing.T) {
	r := rand.New(rand.NewPCG(10, 10))
	drops := map[bool]int{}
	for run := 0; run < 20; run++ {
		p, _ := RandomGroove(r)
		state := &JamState{Amount: JamEvolving}
		for n := 0; n < 2*jamWave; n++ {
			rising := state.Rising()
			info := Jam(&p, r, state)
			drops[rising] += strings.Count(info, "dropped ")
			if strings.Contains(info, "brought in") && !rising {
				t.Fatalf("brought a part in while releasing: %s", info)
			}
		}
	}
	if drops[false] <= drops[true] {
		t.Fatalf("drops while building %d, releasing %d", drops[true], drops[false])
	}
}

func TestJamRephraseKeepsKeyRegisterAndOpening(t *testing.T) {
	r := rand.New(rand.NewPCG(12, 13))
	for n := 0; n < 40; n++ {
		p, _ := RandomGroove(r)
		lead := &p.Tracks[0]
		if !hasNotes(lead.Patterns[0]) || len(lead.Patterns[0].Steps) < 32 {
			continue
		}
		opening := append([]Step(nil), lead.Patterns[0].Steps[:8]...)
		state := &JamState{Amount: JamEvolving, touched: map[string]bool{}}
		for i := range p.Tracks[1:] {
			state.touched[p.Tracks[1+i].ID] = true
		}
		info := jamRephrase(&p, r, state)
		if !strings.HasPrefix(info, lead.Name+" new phrase in bar ") && info != lead.Name+" new end to bar 1" {
			t.Fatalf("rephrase: %q", info)
		}
		if !reflect.DeepEqual(opening, lead.Patterns[0].Steps[:8]) {
			t.Fatal("rephrase changed the opening bar")
		}
		scale := map[int]bool{}
		for _, d := range scaleNotes(p.Key.Scale) {
			scale[(p.Key.Root+d)%12] = true
		}
		for _, s := range lead.Patterns[0].Steps {
			if s.Active && !scale[s.Note%12] {
				t.Fatalf("note %d out of %s", s.Note, p.Key.Name())
			}
		}
	}
}

func TestJamRewriteChangesFamily(t *testing.T) {
	r := rand.New(rand.NewPCG(14, 15))
	for n := 0; n < 60; n++ {
		p, _ := RandomGroove(r)
		state := &JamState{Amount: JamEvolving, touched: map[string]bool{}}
		info := jamRewrite(&p, r, state)
		if info == "" {
			continue
		}
		parts := strings.SplitN(info, " → ", 2)
		for _, track := range p.Tracks {
			if track.Name == parts[0] && closestFamily(track.Kind, track.Patterns[0].Steps) != parts[1] {
				// Families can overlap ("?" steps); the new hits must at least
				// match the family named.
				f, _ := drumFamilyNamed(track.Kind, parts[1])
				for i := 0; i < 16; i++ {
					if c := f.template[i]; c != '?' && (c != '.') != track.Patterns[0].Steps[i].Active {
						t.Fatalf("%s: step %d does not match", info, i)
					}
				}
			}
		}
	}
}

// TestWriteJams renders the same groove jammed for 96 bars at each amount,
// a change every 8 bars, with a log of the changes beside each WAV:
// GRUVBOX_JAM_WAVS=<dir> go test ./groove -run TestWriteJams
func TestWriteJams(t *testing.T) {
	dir := os.Getenv("GRUVBOX_JAM_WAVS")
	if dir == "" {
		t.Skip("set GRUVBOX_JAM_WAVS to a directory to write jams")
	}
	const bars, every = 96, 8
	start, info := RandomGroove(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	for _, amount := range []JamAmount{JamGentle, JamEvolving, JamWild} {
		p := start.Clone()
		r := rand.New(rand.NewPCG(uint64(amount)+40, 41))
		state := &JamState{Amount: amount}
		frames := int(float64(SampleRate) * 60 / p.BPM * 4 * bars)
		var pcm []byte
		player := NewPlayer()
		player.Publish(p)
		player.SetPlaying(true)
		log := []string{info}
		next := float64(every)
		var block [BlockSize * 2]float32
		for done := 0; done < frames; done += BlockSize {
			if player.BarClock() >= next-1./8 {
				log = append(log, fmt.Sprintf("bar %3.0f: %s", next+1, Jam(&p, r, state)))
				player.Publish(p)
				next += every
			}
			out := block[:min(BlockSize, frames-done)*2]
			player.Render(out)
			for _, v := range out {
				pcm = binary.LittleEndian.AppendUint16(pcm, uint16(int16(max(-1, min(1, v))*32767)))
			}
		}
		var header [44]byte
		copy(header[:], "RIFF")
		binary.LittleEndian.PutUint32(header[4:], uint32(36+len(pcm)))
		copy(header[8:], "WAVEfmt ")
		binary.LittleEndian.PutUint32(header[16:], 16)
		binary.LittleEndian.PutUint16(header[20:], 1)
		binary.LittleEndian.PutUint16(header[22:], 2)
		binary.LittleEndian.PutUint32(header[24:], SampleRate)
		binary.LittleEndian.PutUint32(header[28:], SampleRate*4)
		binary.LittleEndian.PutUint16(header[32:], 4)
		binary.LittleEndian.PutUint16(header[34:], 16)
		copy(header[36:], "data")
		binary.LittleEndian.PutUint32(header[40:], uint32(len(pcm)))
		name := filepath.Join(dir, "jam-"+strings.ToLower(JamAmounts[amount]))
		if err := os.WriteFile(name+".wav", append(header[:], pcm...), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name+".txt", []byte(strings.Join(log, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log(name + ".wav")
	}
}
