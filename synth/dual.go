package synth

import (
	"math"

	"github.com/subbaan/gruvbox/dsp"
)

type DualParameter struct {
	ID, Name                     string
	Min, Max, Default, Increment float64
	Lockable                     bool
}

// The same definitions drive sound controls, validation and step locks.
var dualDefinitions = []DualParameter{
	{"osc1_wave", "Oscillator 1", 0, 3, 0, 1, false},
	{"osc2_wave", "Oscillator 2", 0, 3, 1, 1, false},
	{"osc2_level", "Oscillator 2 level", 0, 1, 0, .01, true},
	{"osc2_tune", "Oscillator 2 interval", -24, 24, 0, 1, true},
	{"detune", "Oscillator 2 detune (cents)", -30, 30, 6, .1, true},
	{"pulse", "Pulse width", .05, .95, .5, .01, true},
	{"sub", "Sub level", 0, 1, .3, .01, true},
	{"noise", "Noise level", 0, 1, 0, .01, true},
	{"tune", "Tune", -24, 24, 0, 1, true},
	{"cutoff", "Cutoff", 0, 1, .4, .01, true},
	{"resonance", "Resonance", 0, 1, .3, .01, true},
	{"drive", "Filter drive", 0, 1, .15, .01, true},
	{"slope", "Filter slope", 0, 1, 1, 1, false},
	{"env_amount", "Filter envelope", -1, 1, .5, .01, true},
	{"keytrack", "Filter key tracking", 0, 1, .5, .01, true},
	{"amp_attack", "Attack (s)", .001, 4, .005, .001, true},
	{"amp_decay", "Decay (s)", .01, 4, .25, .001, true},
	{"amp_sustain", "Sustain", 0, 1, .6, .01, true},
	{"amp_release", "Release (s)", .005, 5, .12, .001, true},
	{"filter_attack", "Attack (s)", .001, 4, .002, .001, true},
	{"filter_decay", "Decay (s)", .01, 4, .25, .001, true},
	{"filter_sustain", "Sustain", 0, 1, 0, .01, true},
	{"filter_release", "Release (s)", .005, 5, .15, .001, true},
	{"glide", "Glide (s)", 0, 1, .06, .005, true},
	{"legato", "Hold envelopes on slide", 0, 1, 1, 1, false},
	{"accent", "Accent amount", 0, 1, .3, .01, true},
}

func DualParameters() []DualParameter { return dualDefinitions }

const (
	dWave1 = iota
	dWave2
	dLevel2
	dInterval
	dDetune
	dPulse
	dSub
	dNoise
	dTune
	dCutoff
	dResonance
	dDrive
	dSlope
	dEnv
	dKeytrack
	dAmpAttack
	dAmpDecay
	dAmpSustain
	dAmpRelease
	dFilterAttack
	dFilterDecay
	dFilterSustain
	dFilterRelease
	dGlide
	dLegato
	dAccent
	dualParameterCount
)

type dualEnvelope struct {
	stage                         int
	value, attack, decay, release float64
}

func (e *dualEnvelope) next(sustain float64) float64 {
	switch e.stage {
	case 1:
		e.value += e.attack
		if e.value >= 1-1e-12 {
			e.value = 1
			e.stage = 2
		}
	case 2:
		e.value = sustain + (e.value-sustain)*e.decay
		if math.Abs(e.value-sustain) < .0001 {
			e.stage = 3
		}
	case 3:
		e.value += (sustain - e.value) * .002
	case 4:
		e.value *= e.release
		if e.value < .00001 {
			e.value = 0
			e.stage = 0
		}
	}
	return e.value
}

type dualFilter struct{ ic1, ic2 float64 }

// Trapezoidal state-variable lowpass: stable at high resonance and cutoff.
func (f *dualFilter) next(x, a1, a2, a3 float64) float64 {
	v3 := x - f.ic2
	v1 := a1*f.ic1 + a2*v3
	v2 := f.ic2 + a2*f.ic1 + a3*v3
	f.ic1, f.ic2 = 2*v1-f.ic1, 2*v2-f.ic2
	return v2
}

// DualSynth is a new mono voice, independent of the legacy MegaSynth. All state
// belongs to the audio thread; construction and parameter metadata allocate only
// on the control thread. Saw/pulse discontinuities use polyBLEP correction.
type DualSynth struct {
	rate                     float64
	target, current          [dualParameterCount]float64
	phase                    [3]float64
	oldWave                  [2]int
	waveFade                 [2]float64
	amp, filterEnv           dualEnvelope
	filters                  [2]dualFilter
	lfos                     [2]*dsp.LFO
	lfoTarget                [2]int
	lfoDepth                 [2]float64
	lfoEnabled               [2]bool
	gate, accent             bool
	note, targetNote         float64
	glideStep                float64
	glideSamples             int
	frequency                [3]float64
	a1, a2, a3               float64
	frame                    uint32
	velocity, targetVelocity float64
	slope                    float64
	random                   uint32
	dcInput, dcOutput        float64
	dcCoefficient            float64
}

func NewDualSynth(sampleRate float64) *DualSynth {
	s := &DualSynth{rate: sampleRate, note: 36, targetNote: 36, velocity: .8, targetVelocity: .8, random: 1}
	s.dcCoefficient = math.Exp(-2 * math.Pi * 5 / sampleRate)
	for i, p := range dualDefinitions {
		s.target[i] = math.NaN() // ensure default envelope coefficients are prepared
		s.SetParameter(p.ID, p.Default)
		s.current[i] = p.Default
	}
	s.slope = s.current[dSlope]
	s.waveFade = [2]float64{1, 1}
	for i := range s.lfos {
		s.lfos[i] = dsp.NewLFO(float32(sampleRate))
		s.lfos[i].SetRange(-1, 1)
		s.lfos[i].SetDepth(1)
	}
	return s
}

func (s *DualSynth) SetParameter(id string, value float64) {
	for i, p := range dualDefinitions {
		if p.ID != id {
			continue
		}
		value = max(p.Min, min(p.Max, value))
		if s.target[i] == value {
			return
		}
		if (i == dWave1 || i == dWave2) && !math.IsNaN(s.target[i]) {
			s.oldWave[i], s.waveFade[i] = int(s.target[i]), 0
		}
		s.target[i] = value
		switch i {
		case dAmpAttack:
			s.amp.attack = 1 / (value * s.rate)
		case dAmpDecay:
			s.amp.decay = math.Exp(-9.21034 / (value * s.rate))
		case dAmpRelease:
			s.amp.release = math.Exp(-9.21034 / (value * s.rate))
		case dFilterAttack:
			s.filterEnv.attack = 1 / (value * s.rate)
		case dFilterDecay:
			s.filterEnv.decay = math.Exp(-9.21034 / (value * s.rate))
		case dFilterRelease:
			s.filterEnv.release = math.Exp(-9.21034 / (value * s.rate))
		}
		return
	}
}

func (s *DualSynth) LFOs() [2]*dsp.LFO { return s.lfos }
func (s *DualSynth) SetModulation(index, target int, enabled bool, depth float64) {
	firstEnable := enabled && !s.lfoEnabled[index]
	s.lfoTarget[index], s.lfoEnabled[index], s.lfoDepth[index] = target, enabled, depth
	// Legacy LFO depth is unipolar. Scale its full bipolar output ourselves.
	s.lfos[index].SetDepth(1)
	if firstEnable {
		s.lfos[index].Reset()
	}
}

func (s *DualSynth) NoteOn(note int, accent, slide bool, velocity float64) {
	held := slide && s.gate
	s.targetNote = float64(note)
	s.glideSamples = 0
	if held && s.target[dGlide] > 0 {
		s.glideSamples = max(1, int(s.target[dGlide]*s.rate))
		s.glideStep = (s.targetNote - s.note) / float64(s.glideSamples)
	} else {
		s.note = s.targetNote
	}
	if !held || s.target[dLegato] < .5 {
		s.amp.stage, s.filterEnv.stage = 1, 1
	}
	s.gate, s.accent, s.targetVelocity = true, accent, velocity
}

func (s *DualSynth) NoteOff() {
	if s.gate {
		s.amp.stage, s.filterEnv.stage = 4, 4
		s.gate = false
	}
}

func (s *DualSynth) Reset() {
	s.gate = false
	s.amp.stage, s.amp.value, s.filterEnv.stage, s.filterEnv.value = 0, 0, 0, 0
	s.phase, s.filters = [3]float64{}, [2]dualFilter{}
	s.waveFade = [2]float64{1, 1}
	s.dcInput, s.dcOutput, s.frame, s.glideSamples = 0, 0, 0, 0
	s.current = s.target
	s.slope = s.target[dSlope]
	s.velocity = s.targetVelocity
	for _, lfo := range s.lfos {
		lfo.Reset()
	}
}

func dualBLEP(t, dt float64) float64 {
	if t < dt {
		t /= dt
		return t + t - t*t - 1
	}
	if t > 1-dt {
		t = (t - 1) / dt
		return t*t + t + t + 1
	}
	return 0
}

func dualOscillator(phase, dt, pulse float64, wave int) float64 {
	switch wave {
	case 1:
		v := -1.0
		if phase < pulse {
			v = 1
		}
		edge := phase - pulse
		if edge < 0 {
			edge++
		}
		return v + dualBLEP(phase, dt) - dualBLEP(edge, dt)
	case 2:
		return 1 - 4*math.Abs(phase-.5)
	case 3:
		return math.Sin(phase * 2 * math.Pi)
	default:
		return 2*phase - 1 - dualBLEP(phase, dt)
	}
}

func (s *DualSynth) Process(output []float32) {
	for i := range output {
		output[i] = s.next()
	}
}

func (s *DualSynth) next() float32 {
	for i := range s.current {
		s.current[i] += (s.target[i] - s.current[i]) * .002
	}
	s.velocity += (s.targetVelocity - s.velocity) * .002
	if s.glideSamples > 0 {
		s.note += s.glideStep
		s.glideSamples--
		if s.glideSamples == 0 {
			s.note = s.targetNote
		}
	}
	amp := s.amp.next(s.current[dAmpSustain])
	env := s.filterEnv.next(s.current[dFilterSustain])
	// Targets: None, Cutoff, Resonance, Pitch, Pulse width, Osc2 detune,
	// Osc2 level, Drive, Volume. Two routes to one target sum around its base.
	var mod [9]float64
	for i, lfo := range s.lfos {
		if s.lfoEnabled[i] && s.lfoTarget[i] > 0 && s.lfoTarget[i] < len(mod) {
			mod[s.lfoTarget[i]] += float64(lfo.Process()) * s.lfoDepth[i]
		}
	}
	if s.frame%8 == 0 {
		note := s.note + s.current[dTune] + mod[3]*2
		s.frequency[0] = min(s.rate*.45, 440*math.Exp2((note-69)/12))
		s.frequency[1] = min(s.rate*.45, s.frequency[0]*math.Exp2((s.current[dInterval]+(s.current[dDetune]+mod[5]*30)/100)/12))
		s.frequency[2] = s.frequency[0] * .5
		cutoff := max(0, min(1, s.current[dCutoff]+mod[1]*.5+env*s.current[dEnv]*.6+(s.note-36)*s.current[dKeytrack]/108))
		hz := min(s.rate*.4, 30*math.Exp2(cutoff*9))
		g := math.Tan(math.Pi * hz / (s.rate * 2))
		k := 2 - 1.85*max(0, min(1, s.current[dResonance]+mod[2]*.5))
		s.a1 = 1 / (1 + g*(g+k))
		s.a2 = g * s.a1
		s.a3 = g * s.a2
	}
	s.frame++
	pulse := max(.05, min(.95, s.current[dPulse]+mod[4]*.4))
	level2 := max(0, min(1, s.current[dLevel2]+mod[6]*.5))
	drive := 1 + 7*max(0, min(1, s.current[dDrive]+mod[7]*.5))
	oscillatorScale := 1 + level2 + s.current[dSub] + s.current[dNoise]
	driveScale := math.Tanh(drive)
	s.slope += (s.target[dSlope] - s.slope) * .002
	for i := range s.waveFade {
		s.waveFade[i] = min(1, s.waveFade[i]+1/(.005*s.rate))
	}
	x := 0.0
	// Oscillators, nonlinear drive and filtering run at twice the output rate.
	// The lowpass stages and pair average reduce folding from the drive stage.
	for half := 0; half < 2; half++ {
		var oscillators [2]float64
		for i := range oscillators {
			dt := s.frequency[i] / (s.rate * 2)
			value := dualOscillator(s.phase[i], dt, pulse, int(s.target[i]))
			if s.waveFade[i] < 1 {
				old := dualOscillator(s.phase[i], dt, pulse, s.oldWave[i])
				value = old + (value-old)*s.waveFade[i]
			}
			oscillators[i] = value
		}
		input := oscillators[0] + oscillators[1]*level2
		input += dualOscillator(s.phase[2], s.frequency[2]/(s.rate*2), .5, 1) * s.current[dSub]
		s.random = s.random*1664525 + 1013904223
		input += (float64(s.random)/2147483648 - 1) * s.current[dNoise]
		input = math.Tanh(input*drive/oscillatorScale) / driveScale
		low := s.filters[0].next(input, s.a1, s.a2, s.a3)
		steep := s.filters[1].next(low, s.a1, s.a2, s.a3)
		x += (low + (steep-low)*s.slope) * .5
		for i := range s.phase {
			s.phase[i] += s.frequency[i] / (s.rate * 2)
			if s.phase[i] >= 1 {
				s.phase[i]--
			}
		}
	}
	// Remove pulse/drive DC before the amplitude envelope. Soft saturation bounds
	// resonance peaks without relying on the project's master clipper.
	dc := x - s.dcInput + s.dcCoefficient*s.dcOutput
	s.dcInput, s.dcOutput = x, dc
	volume := max(0, min(1.5, 1+mod[8]*.5))
	accent := 1.0
	if s.accent {
		accent += s.current[dAccent] * .5
	}
	return float32(math.Tanh(dc*.7) * amp * s.velocity * volume * accent * .65)
}
