package synth

import (
	"math"
	"math/cmplx"
	"testing"
)

// render plays n samples and returns them.
func render(s *SH101, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(s.next())
	}
	return out
}

// pitchHz estimates the fundamental from rising zero crossings.
func pitchHz(x []float64, rate float64) float64 {
	first, last, n := -1, -1, 0
	for i := 1; i < len(x); i++ {
		if x[i-1] < 0 && x[i] >= 0 {
			if first < 0 {
				first = i
			}
			last, n = i, n+1
		}
	}
	if n < 2 {
		return 0
	}
	return rate * float64(n-1) / float64(last-first)
}

// bandEnergy is the magnitude of x at hz (a single DFT bin).
func bandEnergy(x []float64, rate, hz float64) float64 {
	var sum complex128
	for i, v := range x {
		sum += complex(v, 0) * cmplx.Exp(complex(0, -2*math.Pi*hz*float64(i)/rate))
	}
	return cmplx.Abs(sum) / float64(len(x))
}

func openVoice() *SH101 {
	s := NewSH101(48000)
	for id, v := range map[string]float64{"cutoff": 1, "resonance": 0, "env_amount": 0, "keytrack": 0, "sustain": 1, "attack": .0015, "sub": 0, "saw": 1} {
		s.SetParameter(id, v)
	}
	s.Reset()
	return s
}

func TestSH101PitchAndRange(t *testing.T) {
	for r, octave := range []float64{-12, 0, 12, 24} {
		s := openVoice()
		s.SetParameter("range", float64(r))
		s.Reset()
		s.NoteOn(45, false, false, 1) // A2, 110 Hz at 8'
		render(s, 4800)
		got := pitchHz(render(s, 48000), 48000)
		want := 110 * math.Exp2(octave/12)
		if math.Abs(got-want)/want > .01 {
			t.Fatalf("range %d: %.2f Hz, want %.2f", r, got, want)
		}
	}
}

// The sub-oscillator sits one or two octaves under the VCO and is locked to
// it: its energy is at f/2 or f/4.
func TestSH101SubOscillatorOctaves(t *testing.T) {
	for mode, divide := range []float64{2, 4, 4} {
		s := openVoice()
		s.SetParameter("saw", 0)
		s.SetParameter("sub", 1)
		s.SetParameter("sub_mode", float64(mode))
		s.Reset()
		s.NoteOn(57, false, false, 1) // 220 Hz
		render(s, 4800)
		x := render(s, 24000)
		sub := bandEnergy(x, 48000, 220/divide)
		vco := bandEnergy(x, 48000, 220)
		if sub < .05 || sub < vco*2 {
			t.Fatalf("sub mode %d: %.3f at f/%v vs %.3f at f", mode, sub, divide, vco)
		}
	}
}

func TestSH101PulseWidthModulationMoves(t *testing.T) {
	s := openVoice()
	s.SetParameter("saw", 0)
	s.SetParameter("pulse", 1)
	s.SetParameter("pw", 0)
	s.Reset()
	s.NoteOn(57, false, false, 1)
	render(s, 4800)
	square := render(s, 24000)
	// A square has almost no second harmonic; a narrow pulse's second is
	// nearly as strong as its fundamental.
	ratio := func(x []float64) float64 { return bandEnergy(x, 48000, 440) / bandEnergy(x, 48000, 220) }
	if r := ratio(square); r > .05 {
		t.Fatalf("50%% pulse second harmonic ratio %.3f", r)
	}
	s.SetParameter("pw", 1)
	render(s, 24000)
	if r := ratio(render(s, 24000)); r < .7 {
		t.Fatalf("narrow pulse second harmonic ratio only %.3f", r)
	}
	// LFO-driven width keeps changing.
	s.SetParameter("pwm_source", 1)
	s.SetParameter("lfo_rate", .6)
	render(s, 24000)
	low, high := 1.0, 0.0
	for i := 0; i < 8; i++ {
		r := ratio(render(s, 4800))
		low, high = min(low, r), max(high, r)
	}
	if high-low < .3 {
		t.Fatalf("LFO pulse width barely moves: %.2f–%.2f", low, high)
	}
}

// At full resonance the filter rings on its own (the SH-101 self-oscillates)
// and stays bounded.
func TestSH101SelfOscillatesBounded(t *testing.T) {
	s := NewSH101(48000)
	for id, v := range map[string]float64{"saw": 0, "sub": 0, "resonance": 1, "cutoff": .6, "env_amount": 0, "sustain": 1} {
		s.SetParameter(id, v)
	}
	s.Reset()
	s.stages[0] = .1 // a nudge to start the ring
	s.NoteOn(48, false, false, 1)
	x := render(s, 48000)
	peak, late := 0.0, 0.0
	for i, v := range x {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatal("non-finite output")
		}
		peak = max(peak, math.Abs(v))
		if i > 40000 {
			late = max(late, math.Abs(v))
		}
	}
	if late < .05 || peak > 1 {
		t.Fatalf("self-oscillation late %.3f, peak %.3f", late, peak)
	}
}

func TestSH101EnvelopeTriggerModes(t *testing.T) {
	s := NewSH101(48000)
	s.SetParameter("sustain", .5)
	s.SetParameter("env_trigger", 0) // Gate
	s.Reset()
	s.NoteOn(36, false, false, 1)
	render(s, 4800)
	s.NoteOn(38, false, true, 1)
	if s.env.stage == 1 {
		t.Fatal("Gate mode retriggered a legato note")
	}
	s.NoteOn(40, false, false, 1)
	if s.env.stage != 1 {
		t.Fatal("Gate mode did not trigger a new note")
	}
	s.SetParameter("env_trigger", 1) // Gate + trigger
	render(s, 4800)
	s.NoteOn(41, false, true, 1)
	if s.env.stage != 1 {
		t.Fatal("Gate + trigger mode did not retrigger a legato note")
	}
	// LFO trigger: a held note keeps re-attacking with the LFO.
	s.SetParameter("env_trigger", 2)
	s.SetParameter("lfo_rate", .5) // about 1.7 Hz
	s.NoteOff()
	render(s, 48000)
	s.NoteOn(43, false, false, 1)
	attacks, was := 0, s.env.stage
	for i := 0; i < 48000*3; i++ {
		s.next()
		if s.env.stage == 1 && was != 1 {
			attacks++
		}
		was = s.env.stage
	}
	if attacks < 4 {
		t.Fatalf("LFO trigger gave %d attacks in 3 s", attacks)
	}
}

func TestSH101PortamentoModes(t *testing.T) {
	s := NewSH101(48000)
	s.SetParameter("glide", .2)
	s.SetParameter("glide_mode", 2) // Auto: slides only
	s.Reset()
	s.NoteOn(36, false, false, 1)
	render(s, 480)
	s.NoteOn(48, false, false, 1)
	if s.note != 48 {
		t.Fatal("Auto portamento glided a note without slide")
	}
	s.NoteOn(36, false, true, 1)
	render(s, 2400) // 50 ms of a 200 ms exponential glide
	if s.note <= 36.5 || s.note >= 47.5 {
		t.Fatalf("Auto portamento did not glide a slide: %.2f", s.note)
	}
	render(s, 48000)
	if s.note != 36 {
		t.Fatalf("glide did not settle: %.4f", s.note)
	}
	s.SetParameter("glide_mode", 1) // On: every note glides
	s.NoteOff()
	s.NoteOn(48, false, false, 1)
	if s.note == 48 {
		t.Fatal("portamento On did not glide")
	}
	s.SetParameter("glide_mode", 0)
	s.NoteOn(40, false, true, 1)
	if s.note != 40 {
		t.Fatal("portamento Off glided")
	}
}

func TestSH101GateVCAReleasesAndResetSilences(t *testing.T) {
	s := NewSH101(48000)
	s.SetParameter("vca_mode", 1)
	s.SetParameter("sustain", 0)
	s.SetParameter("decay", .01)
	s.Reset()
	s.NoteOn(48, false, false, 1)
	x := render(s, 24000)
	loud := 0.0
	for _, v := range x[12000:] {
		loud = max(loud, math.Abs(v))
	}
	if loud < .05 {
		t.Fatal("Gate VCA died with the envelope")
	}
	s.NoteOff()
	render(s, 4800)
	for _, v := range render(s, 480) {
		if math.Abs(v) > 1e-3 {
			t.Fatal("Gate VCA did not close")
		}
	}
	s.NoteOn(48, false, false, 1)
	render(s, 480)
	s.Reset()
	var out [64]float32
	s.Process(out[:])
	for _, v := range out {
		if v != 0 {
			t.Fatal("reset left an audible voice")
		}
	}
}

func TestSH101LiveSweepsStayFiniteWithoutAllocating(t *testing.T) {
	s := NewSH101(44100)
	s.NoteOn(36, true, false, 1)
	var out [256]float32
	for i := 0; i < 400; i++ {
		x := float64(i%40) / 39
		for _, id := range []string{"cutoff", "resonance", "pw", "env_amount", "filter_lfo", "vco_lfo", "noise"} {
			s.SetParameter(id, x)
		}
		s.SetParameter("lfo_wave", float64(i%4))
		s.SetParameter("pwm_source", float64(i%3))
		s.Process(out[:])
		for _, v := range out {
			if math.IsNaN(float64(v)) || math.Abs(float64(v)) > 1.5 {
				t.Fatalf("unstable output %v at block %d", v, i)
			}
		}
	}
	if n := testing.AllocsPerRun(50, func() { s.Process(out[:]) }); n != 0 {
		t.Fatalf("Process allocates %.0f times", n)
	}
}
