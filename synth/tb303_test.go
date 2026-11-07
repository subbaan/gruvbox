package synth

import (
	"math"
	"testing"

	"github.com/subbaan/gruvbox/util"
)

const testRate = 44100

func render303(tb *TB303, n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		var b [1]float32
		tb.Process(b[:])
		out[i] = b[0]
	}
	return out
}

func rms303(x []float32) float64 {
	sum := 0.0
	for _, v := range x {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum / float64(len(x)))
}

func TestFastSineAccuracy(t *testing.T) {
	for i := 0; i < 1000; i++ {
		p := float32(i) / 1000
		if d := math.Abs(float64(util.FastSine(p)) - math.Sin(2*math.Pi*float64(p))); d > .002 {
			t.Fatalf("FastSine(%v) off by %v", p, d)
		}
	}
}

func TestGateReleasesAndRingIsDefault(t *testing.T) {
	ring, gated := NewTB303(testRate, 1), NewTB303(testRate, 1)
	ring.NoteOn(36, false, false)
	gated.NoteOnFor(36, false, false, 2000)
	a, b := render303(ring, 6000), render303(gated, 6000)
	for i := 0; i < 2000; i++ {
		if a[i] != b[i] {
			t.Fatalf("gated note differs before its gate at %d", i)
		}
	}
	if tail := rms303(b[4000:]); tail > 1e-4 {
		t.Fatalf("gated note still sounding: %v", tail)
	}
	if rms303(a[4000:]) < .01 {
		t.Fatal("ringing note died early")
	}
	if gated.IsActive() && gated.ampEnv.IsActive() {
		t.Fatal("gated amp envelope still active")
	}
}

func TestDriveAddsHarmonicsWithoutBlowingUp(t *testing.T) {
	clean, driven := NewTB303(testRate, 1), NewTB303(testRate, 1)
	for _, tb := range []*TB303{clean, driven} {
		tb.SetCutoff(.9)
		tb.SetEnvMod(0)
		tb.SetWaveform(WaveformSquare)
		tb.SetSubLevel(0)
	}
	driven.SetDrive(1)
	clean.NoteOn(45, false, false)
	driven.NoteOn(45, false, false)
	a, b := render303(clean, 8000), render303(driven, 8000)
	same := true
	for i := range a {
		if math.IsNaN(float64(b[i])) || math.Abs(float64(b[i])) > 2 {
			t.Fatalf("driven sample %v", b[i])
		}
		same = same && a[i] == b[i]
	}
	if same {
		t.Fatal("drive changed nothing")
	}
	if ratio := rms303(b) / rms303(a); ratio < .5 || ratio > 2.5 {
		t.Fatalf("drive changed level by %.2f×", ratio)
	}
}

// peakCutoff plays one note and returns the highest cutoff it reached.
func peakCutoff(tb *TB303, accent bool) float32 {
	tb.NoteOn(36, accent, false)
	peak := float32(0)
	for i := 0; i < 4000; i++ {
		var b [1]float32
		tb.Process(b[:])
		peak = max(peak, tb.GetFilterCutoff())
	}
	return peak
}

func TestAccentSweepBuildsOverConsecutiveAccents(t *testing.T) {
	plain, swept := NewTB303(testRate, 1), NewTB303(testRate, 1)
	for _, tb := range []*TB303{plain, swept} {
		tb.SetCutoff(.2)
		tb.SetEnvMod(.3)
	}
	swept.SetAccentSweep(1)
	var p, s [3]float32
	for i := range p {
		p[i], s[i] = peakCutoff(plain, true), peakCutoff(swept, true)
	}
	if math.Abs(float64(p[2]-p[0])) > 1e-4 {
		t.Fatalf("without sweep accents should match: %v", p)
	}
	if !(s[1] > s[0]+.01 && s[2] > s[1]+.005) {
		t.Fatalf("sweep should climb over accents: %v", s)
	}
	// A plain note drains the build-up.
	peakCutoff(swept, false)
	if again := peakCutoff(swept, true); again >= s[2] {
		t.Fatalf("plain note did not drain the sweep: %v then %v", s[2], again)
	}
}

func TestVoiceProcessDoesNotAllocate(t *testing.T) {
	tb := NewTB303(testRate, 1)
	tb.SetDrive(.6)
	tb.SetAccentSweep(.7)
	tb.SetSubLevel(.5)
	tb.NoteOnFor(40, true, false, 3000)
	var b [1]float32
	if n := testing.AllocsPerRun(1000, func() { tb.Process(b[:]) }); n != 0 {
		t.Fatalf("Process allocated %v", n)
	}
}

func TestBlendZeroMatchesPlainAndNoiseIsBounded(t *testing.T) {
	plain, blend0 := NewTB303(testRate, 1), NewTB303(testRate, 1)
	blend0.SetBlend(0)
	plain.NoteOn(40, false, false)
	blend0.NoteOn(40, false, false)
	a, b := render303(plain, 3000), render303(blend0, 3000)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("blend 0 changed the sound")
		}
	}
	mixed, noisy := NewTB303(testRate, 1), NewTB303(testRate, 1)
	mixed.SetBlend(1)
	noisy.SetNoise(1)
	mixed.NoteOn(40, false, false)
	noisy.NoteOn(40, false, false)
	m, n := render303(mixed, 3000), render303(noisy, 3000)
	differs := false
	for i := range a {
		if math.IsNaN(float64(m[i])) || math.IsNaN(float64(n[i])) || math.Abs(float64(n[i])) > 2 {
			t.Fatal("bad sample")
		}
		differs = differs || m[i] != a[i]
	}
	if !differs || rms303(n) == rms303(a) {
		t.Fatal("blend or noise changed nothing")
	}
}

// lowShare is the fraction of energy below ~200 Hz, from a one-pole split.
func lowShare(x []float32) float64 {
	var lp float32
	low, all := 0.0, 0.0
	a := float32(1 - math.Exp(-2*math.Pi*200/testRate))
	for _, v := range x {
		lp += a * (v - lp)
		low += float64(lp * lp)
		all += float64(v * v)
	}
	return low / all
}

func TestFilterModesShapeTheSpectrum(t *testing.T) {
	shares := [3]float64{}
	for mode := range shares {
		tb := NewTB303(testRate, 1)
		tb.SetCutoff(.45)
		tb.SetResonance(.3)
		tb.SetEnvMod(0)
		tb.SetSubLevel(0)
		tb.SetFilterMode(mode)
		tb.NoteOn(33, false, false) // 55 Hz
		out := render303(tb, 8000)
		if rms303(out) < 1e-3 {
			t.Fatalf("mode %d silent", mode)
		}
		shares[mode] = lowShare(out)
	}
	if !(shares[0] > shares[1] && shares[1] > shares[2]) {
		t.Fatalf("low-frequency share should fall LP > BP > HP: %v", shares)
	}
}
