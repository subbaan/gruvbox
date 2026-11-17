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
			s.SetParameter("sync", float64(n/7%2))
			s.SetParameter("fm", float64(n%50)/49)
			s.SetParameter("ring", float64(n%30)/29)
			s.SetParameter("unison", float64(n%40)/39)
			s.SetParameter("osc2_env", 1)
			s.SetParameter("osc2_tune", float64(n%49-24))
			s.SetParameter("osc1_level", float64(n%3)/2)
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

// With sync on, oscillator 2 restarts every time oscillator 1 wraps.
func TestDualSyncLocksOscillator2ToOscillator1(t *testing.T) {
	s := NewDualSynth(44100)
	s.SetParameter("sync", 1)
	s.SetParameter("osc2_tune", 7)
	s.Reset()
	s.NoteOn(45, false, false, .8)
	resets := 0
	for n := 0; n < 20000; n++ {
		before := s.phase[0]
		s.next()
		if s.phase[0] < before {
			// One of the two half-samples wrapped; oscillator 2 restarted no
			// more than one half-sample ago.
			if limit := 2 * s.frequency[1] / (s.rate * 2); s.phase[1] > limit {
				t.Fatalf("oscillator 2 at %.3f after a wrap (limit %.3f)", s.phase[1], limit)
			}
			resets++
		}
	}
	if resets < 45 { // 110 Hz over 20000 samples is about 50 wraps
		t.Fatal("too few wraps", resets)
	}
}

// The sync reset is a jump in oscillator 2; band-limiting it keeps the
// aliasing down. The note is chosen so 37 periods fill the window exactly:
// everything off the harmonics of oscillator 1 is folded energy.
func TestDualSyncBLEPReducesAliasing(t *testing.T) {
	const size, periods = 4096, 37
	residual := func() float64 {
		s := NewDualSynth(44100)
		for id, v := range map[string]float64{"sync": 1, "osc2_tune": 10, "osc1_level": 0, "osc2_level": 1, "sub": 0, "cutoff": 1, "resonance": 0,
			"env_amount": 0, "keytrack": 0, "drive": 0, "amp_sustain": 1, "amp_attack": .001} {
			s.SetParameter(id, v)
		}
		s.Reset()
		s.NoteOn(60, false, false, .8)
		f := float64(periods) * 44100 / size
		s.note, s.targetNote = 69+12*math.Log2(f/440), 69+12*math.Log2(f/440)
		out := make([]float32, size)
		for i := 0; i < 8; i++ {
			s.Process(out)
		}
		energy, mean := 0.0, 0.0
		for _, v := range out {
			energy += float64(v) * float64(v)
			mean += float64(v)
		}
		energy -= mean * mean / size
		for k := periods; k < size/2; k += periods {
			cos, sin := 0.0, 0.0
			for i, v := range out {
				angle := 2 * math.Pi * float64(i*k) / size
				cos += float64(v) * math.Cos(angle)
				sin += float64(v) * math.Sin(angle)
			}
			energy -= 2 * (cos*cos + sin*sin) / size
		}
		return energy
	}
	corrected := residual()
	dualSyncBLEP = false
	naive := residual()
	dualSyncBLEP = true
	t.Logf("folded energy: %.4g band-limited, %.4g naive", corrected, naive)
	if corrected > naive*.5 {
		t.Fatal("sync polyBLEP did not reduce folded energy", corrected, naive)
	}
}

// Each new control changes the sound clearly and keeps it bounded.
func TestDualSyncFMRingAndUnisonShapeTheSound(t *testing.T) {
	render := func(values map[string]float64) []float32 {
		s := NewDualSynth(44100)
		for id, v := range values {
			s.SetParameter(id, v)
		}
		s.Reset()
		out := make([]float32, 0, 44100)
		var block [512]float32
		for n, note := range []int{36, 43, 48, 41} {
			s.NoteOn(note, n == 0, false, .8)
			for i := 0; i < 20; i++ {
				s.Process(block[:])
				out = append(out, block[:]...)
			}
		}
		return out
	}
	plain := render(map[string]float64{"osc2_tune": 7})
	for name, values := range map[string]map[string]float64{
		"sync":     {"osc2_tune": 7, "sync": 1, "osc2_level": 1, "osc1_level": .2},
		"sweep":    {"osc2_tune": 7, "sync": 1, "osc2_level": 1, "osc2_env": .6, "filter_sustain": .3},
		"fm":       {"osc2_tune": 7, "fm": .5, "osc1_wave": 3, "osc2_wave": 3},
		"ring":     {"osc2_tune": 7, "ring": .9, "osc1_level": .2},
		"unison":   {"osc2_tune": 7, "unison": .8},
		"together": {"osc2_tune": 7, "sync": 1, "osc2_level": 1, "osc2_env": 1, "fm": 1, "ring": 1, "unison": 1, "resonance": 1, "drive": 1},
	} {
		out := render(values)
		diff, power, peak := 0.0, 0.0, 0.0
		for i, v := range out {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("%s: not a number", name)
			}
			d := float64(v - plain[i])
			diff += d * d
			power += float64(plain[i]) * float64(plain[i])
			peak = max(peak, math.Abs(float64(v)))
		}
		t.Logf("%-8s difference %.2f of plain, peak %.2f", name, math.Sqrt(diff/power), peak)
		if math.Sqrt(diff/power) < .2 {
			t.Errorf("%s barely changes the sound", name)
		}
		if peak > 1 {
			t.Errorf("%s peaks at %.2f", name, peak)
		}
	}
}

func BenchmarkDualSynthEverything(b *testing.B) {
	s := NewDualSynth(44100)
	for id, v := range map[string]float64{"osc2_level": .6, "sync": 1, "osc2_env": .5, "fm": .3, "ring": .3, "unison": .5} {
		s.SetParameter(id, v)
	}
	s.NoteOn(48, false, false, .8)
	var block [256]float32
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Process(block[:])
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
