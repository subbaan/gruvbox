package effects

import (
	"math"
	"testing"
)

func TestChorusDelayResetAndLiveParameters(t *testing.T) {
	c := NewChorus(44100, 1)
	c.SetDepth(0)
	c.SetFeedback(0)
	c.SetDelay(10)
	c.Reset()
	out := make([]float32, 1000)
	out[0] = .5
	c.Process(out)
	if out[0] != 0 || math.Abs(float64(out[441]-.5)) > 1e-5 {
		t.Fatal("fixed chorus tap missed its delay")
	}
	c.Reset()
	clear(out)
	c.Process(out)
	for _, v := range out {
		if v != 0 {
			t.Fatal("chorus reset left a tail")
		}
	}
	for block := 0; block < 200; block++ {
		c.SetDepth(float32(block%10) / 10)
		c.SetRate(float32(block%8) + .05)
		c.SetDelay(float32(5 + block%30))
		c.SetFeedback(.7)
		for i := range out {
			out[i] = float32(math.Sin(float64(i)*.1)) * .5
		}
		c.Process(out)
		for _, v := range out {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || math.Abs(float64(v)) > 1 {
				t.Fatal("chorus unstable during live sweeps")
			}
		}
	}
	if allocs := testing.AllocsPerRun(100, func() { c.Process(out) }); allocs != 0 {
		t.Fatal("chorus process allocates")
	}
}
func TestChorusStreamingMatchesWholeBuffer(t *testing.T) {
	a, b := NewChorus(44100, 1), NewChorus(44100, 1024)
	input := make([]float32, 4096)
	for i := range input {
		input[i] = float32(math.Sin(float64(i)*.07)) * .4
	}
	want := append([]float32(nil), input...)
	b.Process(want)
	for i := range input {
		one := []float32{input[i]}
		a.Process(one)
		if one[0] != want[i] {
			t.Fatal("chorus depends on callback size")
		}
	}
}
