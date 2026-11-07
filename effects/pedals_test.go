package effects

import (
	"math"
	"testing"
)

const sr = 44100

func sine(n int, hz, amp float64) float32 {
	return float32(amp * math.Sin(2*math.Pi*hz*float64(n)/sr))
}

// run feeds frames samples of input through e one at a time, as the engine
// does, and returns the output.
func run(e Effect, frames int, input func(int) float32) []float32 {
	out := make([]float32, frames)
	var b [1]float32
	for n := range out {
		b[0] = input(n)
		e.Process(b[:])
		out[n] = b[0]
	}
	return out
}

func rms(x []float32) float64 {
	sum := 0.
	for _, v := range x {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum / float64(len(x)))
}

func peak(x []float32) float64 {
	p := 0.
	for _, v := range x {
		p = max(p, math.Abs(float64(v)))
	}
	return p
}

func pedals() map[string]Effect {
	phaser, flanger := NewPhaser(sr), NewFlanger(sr)
	return map[string]Effect{
		"overdrive": NewOverdrive(sr), "fuzz": NewFuzz(sr), "envelope": NewEnvelopeFilter(sr),
		"phaser": phaser, "flanger": flanger, "tape": NewTapeEcho(sr, 2), "compressor": NewCompressor(sr),
	}
}

func TestPedalsStayFiniteAndBounded(t *testing.T) {
	for name, e := range pedals() {
		loud := run(e, sr, func(n int) float32 { return sine(n, 110, 1.5) })
		for _, v := range loud {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("%s produced %v", name, v)
			}
		}
		if p := peak(loud); p > 8 {
			t.Errorf("%s peak %.2f on a loud input", name, p)
		}
		e.Reset()
		silent := run(e, sr/4, func(int) float32 { return 0 })
		if p := peak(silent); p > 1e-3 {
			t.Errorf("%s is not silent after Reset (peak %.4f)", name, p)
		}
	}
}

func TestDriveAndFuzzAddHarmonics(t *testing.T) {
	// A pure low sine gains energy above 1 kHz when clipped.
	high := func(e Effect) float64 {
		out := run(e, sr/2, func(n int) float32 { return sine(n, 110, .3) })
		// Four cascaded one-pole high-passes at 1 kHz all but remove 110 Hz.
		var hp [4]onePole
		filtered := make([]float32, len(out))
		for i, v := range out {
			for s := range hp {
				if i == 0 {
					hp[s].setCutoff(1000, sr)
				}
				v = hp[s].hp(v)
			}
			filtered[i] = v
		}
		return rms(filtered[sr/8:]) / max(1e-9, rms(out[sr/8:]))
	}
	clean := NewOverdrive(sr)
	clean.SetDrive(0)
	clean.SetTone(1)
	driven := NewOverdrive(sr)
	driven.SetDrive(1)
	driven.SetTone(1)
	fuzz := NewFuzz(sr)
	fuzz.SetFuzz(1)
	fuzz.SetTone(1)
	c, d, f := high(clean), high(driven), high(fuzz)
	if d < c*4 || f < c*4 {
		t.Fatalf("clipping did not add harmonics: clean %.3f drive %.3f fuzz %.3f", c, d, f)
	}
}

func TestEnvelopeFilterFollowsLevel(t *testing.T) {
	// Louder input opens the filter, so more of a bright tone gets through.
	bright := func(amp float64) float64 {
		e := NewEnvelopeFilter(sr)
		e.SetSensitivity(1)
		e.SetCutoff(0)
		e.SetRange(1)
		out := run(e, sr/2, func(n int) float32 { return sine(n, 2000, amp) })
		return rms(out[sr/4:]) / amp
	}
	if quiet, loud := bright(.02), bright(.4); loud < quiet*3 {
		t.Fatalf("envelope did not open the filter: quiet %.4f loud %.4f", quiet, loud)
	}
}

func TestTapeEchoRepeatsAfterItsTime(t *testing.T) {
	e := NewTapeEcho(sr, 2)
	e.SetDelayTime(.1)
	e.SetWow(0)
	e.Reset()
	out := run(e, sr/2, func(n int) float32 {
		if n < 200 {
			return .8
		}
		return 0
	})
	if peak(out[:4000]) > .01 {
		t.Fatal("echo arrived before its delay time")
	}
	if peak(out[4300:5000]) < .2 {
		t.Fatal("no echo at 100 ms")
	}
	if peak(out[8700:9500]) < .05 {
		t.Fatal("no second repeat")
	}
}

func TestCompressorReducesLoudMoreThanQuiet(t *testing.T) {
	gain := func(amp float64) float64 {
		c := NewCompressor(sr)
		c.SetMakeup(0)
		c.SetThreshold(-24)
		c.SetRatio(8)
		out := run(c, sr/2, func(n int) float32 { return sine(n, 220, amp) })
		return rms(out[sr/4:]) / (amp / math.Sqrt2)
	}
	if quiet, loud := gain(.01), gain(.8); loud > quiet*.5 {
		t.Fatalf("compressor gain quiet %.3f loud %.3f", quiet, loud)
	}
}

func TestPhaserAndFlangerNotchWhenMixed(t *testing.T) {
	// Mixed 50/50 with the dry signal, the sweep must vary the level of a
	// steady tone over time (notches passing through it).
	for _, e := range []Effect{NewPhaser(sr), NewFlanger(sr)} {
		var lows, highs []float64
		out := run(e, sr*4, func(n int) float32 { return sine(n, 900, .3) })
		for block := 0; block+2205 <= len(out); block += 2205 {
			var mixed []float32
			for n := block; n < block+2205; n++ {
				mixed = append(mixed, .5*sine(n, 900, .3)+.5*out[n])
			}
			r := rms(mixed)
			lows, highs = append(lows, r), append(highs, r)
		}
		lo, hi := math.Inf(1), 0.
		for i := range lows {
			lo, hi = min(lo, lows[i]), max(hi, highs[i])
		}
		if hi < lo*1.5 {
			t.Fatalf("%T: no audible sweep (%.3f..%.3f)", e, lo, hi)
		}
	}
}
