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
	// alpha.62: what a two-oscillator synth can do that the SH-101 cannot.
	// All default off (oscillator 1 at full level), so older sounds are
	// unchanged.
	{"osc1_level", "Oscillator 1 level", 0, 1, 1, .01, true},
	{"sync", "Oscillator 2 sync", 0, 1, 0, 1, false},
	{"osc2_env", "Osc 2 sweep", 0, 1, 0, .01, true},
	{"fm", "FM amount", 0, 1, 0, .01, true},
	{"ring", "Ring mod", 0, 1, 0, .01, true},
	{"unison", "Unison", 0, 1, 0, .01, true},
}

// DualChoices names the settings of the Dual synth's switches.
var DualChoices = map[string][]string{
	"osc1_wave": {"Saw", "Pulse", "Triangle", "Sine"},
	"osc2_wave": {"Saw", "Pulse", "Triangle", "Sine"},
	"slope":     {"12 dB low-pass", "24 dB low-pass"},
	"legato":    {"Retrigger envelopes on slide", "Hold envelopes on slide"},
	"sync":      {"Off", "Sync to oscillator 1"},
}

// DualLFOTargets are the track LFOs' destinations, in order. New ones go at
// the end: saved LFOs store the index.
var DualLFOTargets = []string{"None", "Cutoff", "Resonance", "Pitch", "Pulse width", "Oscillator 2 detune", "Oscillator 2 level", "Filter drive", "Volume", "Oscillator 2 pitch", "FM amount", "Ring mod"}

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
	dOsc1Level
	dSync
	dOsc2Env
	dFM
	dRing
	dUnison
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
// on the control thread. Saw/pulse discontinuities use polyBLEP correction,
// including the jump when sync resets oscillator 2.
//
// Oscillators: 0 and 1 are the two oscillators, 2 the sub, 3 and 4 the
// unison copies of oscillator 1 (computed only while unison is up).
type DualSynth struct {
	rate                     float64
	target, current          [dualParameterCount]float64
	phase                    [5]float64
	syncCarry                float64 // polyBLEP residual of a sync reset, due on the next sample
	unisonGain               float64
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
	frequency                [5]float64
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
	s.phase[3], s.phase[4], s.unisonGain = unisonPhases[0], unisonPhases[1], 1
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
	s.phase, s.filters = [5]float64{}, [2]dualFilter{}
	s.phase[3], s.phase[4], s.syncCarry = unisonPhases[0], unisonPhases[1], 0
	s.waveFade = [2]float64{1, 1}
	s.dcInput, s.dcOutput, s.frame, s.glideSamples = 0, 0, 0, 0
	s.current = s.target
	s.slope = s.target[dSlope]
	s.velocity = s.targetVelocity
	for _, lfo := range s.lfos {
		lfo.Reset()
	}
}

// oscillator is oscillator i's band-limited value (0 also for the unison
// copies), crossfading from its old waveform after a change.
func (s *DualSynth) oscillator(i int, phase, dt, pulse float64) float64 {
	value := dualOscillator(phase, dt, pulse, int(s.target[i]))
	if s.waveFade[i] < 1 {
		old := dualOscillator(phase, dt, pulse, s.oldWave[i])
		value = old + (value-old)*s.waveFade[i]
	}
	return value
}

// fmPhase offsets oscillator 1's phase by oscillator 2 (phase modulation, the
// usual "FM"); with fm at 0 it is the phase itself.
func (s *DualSynth) fmPhase(phase, fm, o2 float64) float64 {
	if fm == 0 {
		return phase
	}
	phase += fm * o2
	return phase - math.Floor(phase)
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

// unisonPhases start the unison copies away from oscillator 1, so they do
// not begin in phase with it.
var unisonPhases = [2]float64{.31, .67}

// dualSyncBLEP switches the sync reset's band-limiting; tests turn it off to
// measure what it saves.
var dualSyncBLEP = true

// dualNaive is an oscillator's value at phase without band-limiting.
func dualNaive(phase, pulse float64, wave int) float64 {
	switch wave {
	case 1:
		if phase < pulse {
			return 1
		}
		return -1
	case 2:
		return 1 - 4*math.Abs(phase-.5)
	case 3:
		return math.Sin(phase * 2 * math.Pi)
	default:
		return 2*phase - 1
	}
}

// dualWrapStep is the jump dualOscillator band-limits at phase 0.
func dualWrapStep(wave int) float64 {
	switch wave {
	case 0:
		return -2
	case 1:
		return 2
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
	// Targets: DualLFOTargets. Two routes to one target sum around its base.
	var mod [12]float64
	for i, lfo := range s.lfos {
		if s.lfoEnabled[i] && s.lfoTarget[i] > 0 && s.lfoTarget[i] < len(mod) {
			mod[s.lfoTarget[i]] += float64(lfo.Process()) * s.lfoDepth[i]
		}
	}
	unison := s.current[dUnison]
	if s.frame%8 == 0 {
		note := s.note + s.current[dTune] + mod[3]*2
		s.frequency[0] = min(s.rate*.45, 440*math.Exp2((note-69)/12))
		// Interval, detune, the pitch envelope (up to two octaves, the classic
		// sync sweep) and the LFO (an octave either way).
		interval := s.current[dInterval] + (s.current[dDetune]+mod[5]*30)/100 + env*s.current[dOsc2Env]*24 + mod[9]*12
		s.frequency[1] = min(s.rate*.45, s.frequency[0]*math.Exp2(interval/12))
		s.frequency[2] = s.frequency[0] * .5
		if unison > 0 {
			// Two copies of oscillator 1 up to 25 cents either side, mixed in
			// at the same power as the spread grows.
			spread := math.Exp2(unison * .25 / 12)
			s.frequency[3] = min(s.rate*.45, s.frequency[0]*spread)
			s.frequency[4] = s.frequency[0] / spread
			s.unisonGain = 1 / math.Sqrt(1+2*unison*unison)
		}
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
	level1 := s.current[dOsc1Level]
	level2 := max(0, min(1, s.current[dLevel2]+mod[6]*.5))
	ring := max(0, min(1, s.current[dRing]+mod[11]*.5))
	fm := max(0, min(1, s.current[dFM]+mod[10]*.5))
	fm *= fm * 1.5 // phase offset in cycles at full oscillator 2 swing
	drive := 1 + 7*max(0, min(1, s.current[dDrive]+mod[7]*.5))
	oscillatorScale := max(.25, level1+level2+s.current[dSub]+s.current[dNoise]+ring)
	driveScale := math.Tanh(drive)
	s.slope += (s.target[dSlope] - s.slope) * .002
	for i := range s.waveFade {
		s.waveFade[i] = min(1, s.waveFade[i]+1/(.005*s.rate))
	}
	sync := s.target[dSync] >= .5
	x := 0.0
	// Oscillators, nonlinear drive and filtering run at twice the output rate.
	// The lowpass stages and pair average reduce folding from the drive stage.
	for half := 0; half < 2; half++ {
		dt0 := s.frequency[0] / (s.rate * 2)
		dt1 := s.frequency[1] / (s.rate * 2)
		// Oscillator 2 first: FM and ring mod use it.
		o2 := s.oscillator(1, s.phase[1], dt1, pulse) + s.syncCarry
		s.syncCarry = 0
		next2 := s.phase[1] + dt1
		if sync && s.phase[0]+dt0 >= 1 {
			// Oscillator 1 wraps before the next sample, a fraction t of the
			// way there, and resets oscillator 2. Band-limit the jump with a
			// polyBLEP: half now, half on the next sample, less the wrap
			// correction dualOscillator will apply there itself.
			t := (1 - s.phase[0]) / dt0
			reached := s.phase[1] + t*dt1
			reached -= math.Floor(reached)
			wave := int(s.target[dWave2])
			jump := dualNaive(0, pulse, wave) - dualNaive(reached, pulse, wave)
			if dualSyncBLEP {
				o2 += jump * (1 - t) * (1 - t) * .5
				s.syncCarry = -(jump - dualWrapStep(wave)) * t * t * .5
			}
			next2 = (1 - t) * dt1
		}
		o1 := s.oscillator(0, s.fmPhase(s.phase[0], fm, o2), dt0, pulse)
		if unison > 0 {
			dt3, dt4 := s.frequency[3]/(s.rate*2), s.frequency[4]/(s.rate*2)
			o1 += unison * (s.oscillator(0, s.fmPhase(s.phase[3], fm, o2), dt3, pulse) + s.oscillator(0, s.fmPhase(s.phase[4], fm, o2), dt4, pulse))
			o1 *= s.unisonGain
			s.phase[3] += dt3
			s.phase[3] -= math.Floor(s.phase[3])
			s.phase[4] += dt4
			s.phase[4] -= math.Floor(s.phase[4])
		}
		input := o1*level1 + o2*level2 + o1*o2*ring
		input += dualOscillator(s.phase[2], s.frequency[2]/(s.rate*2), .5, 1) * s.current[dSub]
		s.random = s.random*1664525 + 1013904223
		input += (float64(s.random)/2147483648 - 1) * s.current[dNoise]
		input = math.Tanh(input*drive/oscillatorScale) / driveScale
		low := s.filters[0].next(input, s.a1, s.a2, s.a3)
		steep := s.filters[1].next(low, s.a1, s.a2, s.a3)
		x += (low + (steep-low)*s.slope) * .5
		s.phase[0] += dt0
		if s.phase[0] >= 1 {
			s.phase[0]--
		}
		s.phase[1] = next2
		if s.phase[1] >= 1 {
			s.phase[1]--
		}
		s.phase[2] += s.frequency[2] / (s.rate * 2)
		if s.phase[2] >= 1 {
			s.phase[2]--
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
