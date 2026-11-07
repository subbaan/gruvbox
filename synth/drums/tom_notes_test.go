package drums

import (
	"math"
	"testing"
)

func TestTomNotesTransposeWithoutChangingRootOrTune(t *testing.T) {
	for _, tc := range []struct {
		note int
		root float32
	}{{24, 50}, {36, 100}, {48, 200}} {
		played, reference := NewTomDrum(44100), NewTomDrum(44100)
		for _, drum := range []*TomDrum{played, reference} {
			drum.SetTone(1)
			drum.SetAttack(0)
			drum.SetTune(3)
			drum.SetPitchDecay(.6)
		}
		played.SetNote(tc.note)
		reference.SetPitch(tc.root)
		played.Trigger(.7)
		reference.Trigger(.7)
		a, b := make([]float32, 4096), make([]float32, 4096)
		played.Process(a)
		reference.Process(b)
		for i := range a {
			if math.Abs(float64(a[i]-b[i])) > 1e-5 {
				t.Fatalf("note %d sample %d: %g != %g", tc.note, i, a[i], b[i])
			}
		}
		if played.GetPitch() != 100 || played.GetTune() != 3 {
			t.Fatal("note changed root/tune controls")
		}
	}
}

func TestTomExtremeNotesRemainFinite(t *testing.T) {
	for _, note := range []int{-100, 0, 12, 127, 300} {
		drum := NewTomDrum(44100)
		drum.SetNote(note)
		drum.SetPitch(300)
		drum.SetTune(12)
		drum.SetPitchDecay(1)
		drum.Trigger(1)
		out := make([]float32, 4096)
		drum.Process(out)
		for _, v := range out {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || math.Abs(float64(v)) > 1 {
				t.Fatalf("invalid audio for note %d", note)
			}
		}
	}
}
