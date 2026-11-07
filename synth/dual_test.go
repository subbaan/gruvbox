package synth

import (
	"github.com/subbaan/gruvbox/dsp"
	"math"
	"testing"
)

func TestDualEnvelopesGateAndLegatoGlide(t *testing.T) {
	s := NewDualSynth(48000)
	s.SetParameter("amp_attack", .01)
	s.SetParameter("amp_sustain", .7)
	s.SetParameter("amp_release", .05)
	s.SetParameter("glide", .1)
	s.Reset()
	s.NoteOn(36, false, false, .8)
	for i := 0; i < 480; i++ {
		s.next()
	}
	if s.amp.value < .999 {
		t.Fatal("attack did not reach its peak at the requested time", s.amp.value)
	}
	s.NoteOn(48, false, true, .8)
	if s.amp.stage == 1 || s.note != 36 {
		t.Fatal("legato slide retriggered or jumped pitch")
	}
	for i := 0; i < 2400; i++ {
		s.next()
	}
	if math.Abs(s.note-42) > 1e-7 {
		t.Fatal("glide is not progressing in semitones", s.note)
	}
	for i := 0; i < 2400; i++ {
		s.next()
	}
	if s.note != 48 || s.glideSamples != 0 {
		t.Fatal("glide did not reach the target")
	}
	s.NoteOff()
	for i := 0; i < 3000; i++ {
		s.next()
	}
	if s.amp.stage != 0 || s.amp.value != 0 {
		t.Fatal("gate release never becomes silent")
	}
	s.NoteOn(60, false, true, .8)
	if s.note != 60 || s.amp.stage != 1 {
		t.Fatal("slide after a gap must start a fresh note")
	}
	s.SetParameter("legato", 0)
	s.next()
	s.NoteOn(62, false, true, .8)
	if s.amp.stage != 1 {
		t.Fatal("slide retrigger mode ignored")
	}
	s.Reset()
	var out [64]float32
	s.Process(out[:])
	for _, v := range out {
		if v != 0 {
			t.Fatal("reset left an audible voice")
		}
	}
}

func TestDualBipolarLFOZeroDepthAndDisable(t *testing.T) {
	s := NewDualSynth(44100)
	s.SetParameter("env_amount", 0)
	s.SetParameter("keytrack", 0)
	s.NoteOn(36, false, false, 1)
	for _, lfo := range s.LFOs() {
		lfo.SetWaveform(dsp.LFOSquare)
		lfo.SetPhaseReset(.75)
		lfo.SetSyncMode(dsp.SyncModeTime)
		lfo.SetRate(.01)
	}
	s.SetModulation(0, 3, true, .5)
	s.next()
	want := 440 * math.Exp2((35.0-69)/12)
	if math.Abs(s.frequency[0]-want) > .001 {
		t.Fatal("first enable did not honour start phase", s.frequency[0], want)
	}
	s.SetModulation(0, 3, true, 0)
	for i := 0; i < 8; i++ {
		s.next()
	}
	want = 440 * math.Exp2((36.0-69)/12)
	if math.Abs(s.frequency[0]-want) > .001 {
		t.Fatal("zero depth changed the base pitch")
	}
	s.SetModulation(0, 3, false, .5)
	for i := 0; i < 8; i++ {
		s.next()
	}
	if math.Abs(s.frequency[0]-want) > .001 {
		t.Fatal("disabled LFO left residual pitch modulation")
	}
}

func TestDualStableLiveSweepsAndNoAllocations(t *testing.T) {
	for _, rate := range []float64{44100, 48000} {
		s := NewDualSynth(rate)
		s.NoteOn(48, true, false, 1)
		var block [256]float32
		for n := 0; n < 180; n++ {
			s.SetParameter("cutoff", float64(n%100)/99)
			s.SetParameter("resonance", 1)
			s.SetParameter("drive", 1)
			s.SetParameter("osc2_level", 1)
			s.SetParameter("osc1_wave", float64(n%4))
			s.SetParameter("slope", float64(n%2))
			s.Process(block[:])
			for _, v := range block {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || math.Abs(float64(v)) > 1 {
					t.Fatal("unstable swept voice", v)
				}
			}
		}
		if n := testing.AllocsPerRun(100, func() { s.Process(block[:]) }); n != 0 {
			t.Fatal("DSP allocates", n)
		}
	}
}

func TestDualBLEPReducesFoldedSawHarmonics(t *testing.T) {
	// A coherent high note has only two legal harmonics below Nyquist. Remove
	// those components and compare the remaining folded harmonic energy.
	const size = 1024
	const bin = 173
	residual := func(corrected bool) float64 {
		var samples [size]float64
		energy, mean := 0.0, 0.0
		for i := range samples {
			phase := float64((i*bin)%size) / size
			v := 2*phase - 1
			if corrected {
				v = dualOscillator(phase, float64(bin)/size, .5, 0)
			}
			samples[i] = v
			energy += v * v
			mean += v
		}
		energy -= mean * mean / size
		for harmonic := 1; harmonic <= 2; harmonic++ {
			cos, sin := 0.0, 0.0
			for i, v := range samples {
				angle := 2 * math.Pi * float64(i*bin*harmonic) / size
				cos += v * math.Cos(angle)
				sin += v * math.Sin(angle)
			}
			energy -= 2 * (cos*cos + sin*sin) / size
		}
		return energy
	}
	if corrected, naive := residual(true), residual(false); corrected > naive*.4 {
		t.Fatal("polyBLEP did not sufficiently suppress folded harmonics", corrected, naive)
	}
}

func BenchmarkDualSynth(b *testing.B) {
	s := NewDualSynth(44100)
	s.SetParameter("osc2_level", .6)
	s.NoteOn(48, false, false, .8)
	for i, lfo := range s.LFOs() {
		lfo.SetSyncMode(dsp.SyncModeTime)
		s.SetModulation(i, i+1, true, .3)
	}
	var block [256]float32
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Process(block[:])
	}
}
