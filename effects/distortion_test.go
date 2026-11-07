package effects

import (
	"math"
	"testing"
)

func TestToneSettingChangesDistortionFilter(t *testing.T) {
	dark, bright := NewDistortion(44100, 1), NewDistortion(44100, 1)
	dark.SetTone(0)
	bright.SetTone(1)
	var a, b [1]float32
	darkEnergy, brightEnergy := 0., 0.
	for n := 0; n < 4000; n++ {
		x := float32(.2 * math.Sin(2*math.Pi*6000*float64(n)/44100))
		a[0] = x
		b[0] = x
		dark.Process(a[:])
		bright.Process(b[:])
		darkEnergy += math.Abs(float64(a[0]))
		brightEnergy += math.Abs(float64(b[0]))
	}
	if brightEnergy < darkEnergy*4 {
		t.Fatal("tone changed its label but not its filter")
	}
}
