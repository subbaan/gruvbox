package synth

import (
	"math"

	"github.com/subbaan/gruvbox/dsp"
)

// SH101Parameter describes one SH-101 control. The same definitions drive the
// sound panel, validation and step locks.
type SH101Parameter struct {
	ID, Name                     string
	Min, Max, Default, Increment float64
	Lockable                     bool
}

// sh101Definitions follow the front panel: VCO, source mixer, VCF, VCA and
// envelope, LFO and portamento. Accent is not on the original; it lets the
// step sequencer's accent flag open the filter and lift the level.
var sh101Definitions = []SH101Parameter{
	{"range", "Range", 0, 3, 1, 1, false},
	{"tune", "Tune", -24, 24, 0, 1, true},
	{"vco_lfo", "Pitch LFO", 0, 1, 0, .01, true},
	{"pwm_source", "Pulse width mod", 0, 2, 0, 1, false},
	{"pw", "Pulse width", 0, 1, .3, .01, true},
	{"saw", "Saw level", 0, 1, .8, .01, true},
	{"pulse", "Pulse level", 0, 1, 0, .01, true},
	{"sub", "Sub level", 0, 1, .4, .01, true},
	{"sub_mode", "Sub oscillator", 0, 2, 0, 1, false},
	{"noise", "Noise level", 0, 1, 0, .01, true},
	{"cutoff", "Cutoff", 0, 1, .45, .01, true},
	{"resonance", "Resonance", 0, 1, .3, .01, true},
	{"env_amount", "Filter envelope", 0, 1, .45, .01, true},
	{"filter_lfo", "Filter LFO", 0, 1, 0, .01, true},
	{"keytrack", "Key tracking", 0, 1, .5, .01, true},
	{"vca_mode", "Amp", 0, 1, 0, 1, false},
	{"env_trigger", "Envelope trigger", 0, 2, 1, 1, false},
	{"attack", "Attack (s)", .0015, 4, .002, .001, true},
	{"decay", "Decay (s)", .002, 10, .3, .001, true},
	{"sustain", "Sustain", 0, 1, .3, .01, true},
	{"release", "Release (s)", .002, 10, .08, .001, true},
	{"lfo_rate", "LFO rate", 0, 1, .5, .01, true},
	{"lfo_wave", "LFO wave", 0, 3, 0, 1, false},
	{"glide", "Portamento (s)", 0, 1.5, .08, .005, true},
	{"glide_mode", "Portamento", 0, 2, 2, 1, false},
	{"accent", "Accent amount", 0, 1, .3, .01, true},
}

// SH101Parameters lists the controls in panel order.
func SH101Parameters() []SH101Parameter { return sh101Definitions }

// SH101Choices names the settings of the SH-101's switches.
var SH101Choices = map[string][]string{
	"range":       {"16'", "8'", "4'", "2'"},
	"pwm_source":  {"Manual", "LFO", "Envelope"},
	"sub_mode":    {"1 octave down", "2 octaves down", "2 octaves down, narrow pulse"},
	"vca_mode":    {"Envelope", "Gate"},
	"env_trigger": {"Gate", "Gate + trigger", "LFO"},
	"lfo_wave":    {"Triangle", "Square", "Random", "Noise"},
	"glide_mode":  {"Off", "On", "Auto (slides only)"},
}

const (
	sRange = iota
	sTune
	sVCOLFO
	sPWMSource
	sPW
	sSaw
	sPulse
	sSub
	sSubMode
	sNoise
	sCutoff
	sResonance
	sEnv
	sFilterLFO
	sKeytrack
	sVCAMode
	sEnvTrigger
	sAttack
	sDecay
	sSustain
	sRelease
	sLFORate
	sLFOWave
	sGlide
	sGlideMode
	sAccent
	sh101ParameterCount
)

// SH101LFOTargets are the track LFOs' destinations, in order.
var SH101LFOTargets = []string{"None", "Cutoff", "Resonance", "Pitch", "Pulse width", "Sub level", "Volume"}

// sh101Envelope is the single ADSR. The attack charges towards an overshoot
// like the analog RC stage, so it curves; decay and release are exponential.
type sh101Envelope struct {
	stage                  int // 0 idle, 1 attack, 2 decay/sustain, 3 release
	value                  float64
	attack, decay, release float64
}

func (e *sh101Envelope) next(sustain float64) float64 {
	switch e.stage {
	case 1:
		e.value += (1.3 - e.value) * e.attack
		if e.value >= 1 {
			e.value, e.stage = 1, 2
		}
	case 2:
		e.value = sustain + (e.value-sustain)*e.decay
	case 3:
		e.value *= e.release
		if e.value < 1e-5 {
			e.value, e.stage = 0, 0
		}
	}
	return e.value
}

// SH101 is a mono voice modelled on the Roland SH-101: one VCO whose saw,
// variable pulse, locked sub-oscillator and noise are mixed, a self-
// oscillating four-pole OTA low-pass, one ADSR for filter and amp, its own
// LFO and exponential portamento. All state belongs to the audio thread;
// construction allocates only on the control thread. Process never
// allocates.
type SH101 struct {
	rate            float64
	target, current [sh101ParameterCount]float64
	phase           float64
	wraps           uint32 // VCO cycles, so the sub-oscillator stays locked
	env             sh101Envelope
	gate, envGate   bool
	ampGate         float64
	accent          bool
	note, goal      float64
	glideCoef       float64
	gliding, played bool
	velocity, vel   float64
	lfoPhase        float64
	lfoHeld         float64
	lfoNoise        float64
	stages          [4]float64
	frequency       float64
	g, k            float64
	frame           uint32
	random          uint32
	lfos            [2]*dsp.LFO
	lfoTarget       [2]int
	lfoDepth        [2]float64
	lfoEnabled      [2]bool
	dcInput         float64
	dcOutput        float64
	dcCoefficient   float64
}

// NewSH101 makes a voice at the default settings.
func NewSH101(sampleRate float64) *SH101 {
	s := &SH101{rate: sampleRate, note: 36, goal: 36, velocity: .8, vel: .8, random: 22222}
	s.dcCoefficient = math.Exp(-2 * math.Pi * 5 / sampleRate)
	for i, p := range sh101Definitions {
		s.target[i] = math.NaN() // so every coefficient is prepared below
		s.SetParameter(p.ID, p.Default)
		s.current[i] = p.Default
	}
	for i := range s.lfos {
		s.lfos[i] = dsp.NewLFO(float32(sampleRate))
		s.lfos[i].SetRange(-1, 1)
		s.lfos[i].SetDepth(1)
	}
	return s
}

// SetParameter sets a control by ID, clamped to its range.
func (s *SH101) SetParameter(id string, value float64) {
	for i, p := range sh101Definitions {
		if p.ID != id {
			continue
		}
		value = max(p.Min, min(p.Max, value))
		if s.target[i] == value {
			return
		}
		s.target[i] = value
		switch i {
		case sAttack:
			// The overshooting charge reaches 1 after 1.466 time constants.
			s.env.attack = 1 - math.Exp(-1.466/(value*s.rate))
		case sDecay:
			s.env.decay = math.Exp(-4.6 / (value * s.rate))
		case sRelease:
			s.env.release = math.Exp(-4.6 / (value * s.rate))
		case sGlide:
			s.glideCoef = 1
			if value > 0 {
				s.glideCoef = 1 - math.Exp(-3/(value*s.rate))
			}
		}
		return
	}
}

// LFOs are the track's two modulation LFOs (the SH-101's own LFO is internal).
func (s *SH101) LFOs() [2]*dsp.LFO { return s.lfos }

// SetModulation routes track LFO index to a target in SH101LFOTargets.
func (s *SH101) SetModulation(index, target int, enabled bool, depth float64) {
	firstEnable := enabled && !s.lfoEnabled[index]
	s.lfoTarget[index], s.lfoEnabled[index], s.lfoDepth[index] = target, enabled, depth
	s.lfos[index].SetDepth(1)
	if firstEnable {
		s.lfos[index].Reset()
	}
}

// NoteOn plays note. A slide step while the gate is open is legato: in Gate
// trigger mode it keeps the envelope running, and with Auto portamento it is
// the only kind of note that glides.
func (s *SH101) NoteOn(note int, accent, slide bool, velocity float64) {
	legato := slide && s.gate
	s.goal = float64(note)
	mode := int(s.target[sGlideMode])
	s.gliding = s.played && s.target[sGlide] > 0 && (mode == 1 || mode == 2 && legato)
	if !s.gliding {
		s.note = s.goal
	}
	s.played = true
	switch int(s.target[sEnvTrigger]) {
	case 0:
		if !legato {
			s.env.stage = 1
		}
	case 1:
		s.env.stage = 1
	case 2:
		// The LFO triggers the envelope; a note from silence starts it at once.
		if !s.gate {
			s.env.stage, s.lfoPhase = 1, 0
		}
	}
	s.gate, s.envGate, s.accent, s.vel = true, true, accent, velocity
}

// NoteOff closes the gate: the envelope releases.
func (s *SH101) NoteOff() {
	if !s.gate {
		return
	}
	s.gate, s.envGate = false, false
	if s.env.stage != 0 {
		s.env.stage = 3
	}
}

// Reset silences the voice at once.
func (s *SH101) Reset() {
	s.gate, s.envGate, s.gliding, s.played = false, false, false, false
	s.env.stage, s.env.value, s.ampGate = 0, 0, 0
	s.phase, s.wraps, s.stages = 0, 0, [4]float64{}
	s.lfoPhase, s.lfoHeld, s.lfoNoise = 0, 0, 0
	s.dcInput, s.dcOutput, s.frame = 0, 0, 0
	s.current = s.target
	s.velocity = s.vel
	for _, lfo := range s.lfos {
		lfo.Reset()
	}
}

// Process fills output with mono samples.
func (s *SH101) Process(output []float32) {
	for i := range output {
		output[i] = s.next()
	}
}

func (s *SH101) noise() float64 {
	s.random = s.random*1664525 + 1013904223
	return float64(s.random)/2147483648 - 1
}

// subLevel is the sub-oscillator's output in VCO cycle n: a square one or two
// octaves down, or a quarter-width pulse two octaves down.
func subLevel(mode int, n uint32) float64 {
	switch mode {
	case 0:
		if n%2 == 0 {
			return 1
		}
	case 1:
		if n%4 < 2 {
			return 1
		}
	default:
		if n%4 == 0 {
			return 1
		}
	}
	return -1
}

func (s *SH101) internalLFO() float64 {
	hz := .1 * math.Exp2(s.current[sLFORate]*8.2)
	s.lfoPhase += hz / s.rate
	if s.lfoPhase >= 1 {
		s.lfoPhase--
		s.lfoHeld = s.noise()
	}
	// Noise: white noise smoothed to roughly 200 Hz, a fast random wobble.
	s.lfoNoise += (s.noise() - s.lfoNoise) * .03
	switch int(s.target[sLFOWave]) {
	case 1:
		if s.lfoPhase < .5 {
			return 1
		}
		return -1
	case 2:
		return s.lfoHeld
	case 3:
		return max(-1, min(1, s.lfoNoise*2.5))
	}
	return 1 - 4*math.Abs(s.lfoPhase-.5)
}

func (s *SH101) next() float32 {
	for i := range s.current {
		s.current[i] += (s.target[i] - s.current[i]) * .002
	}
	s.velocity += (s.vel - s.velocity) * .002
	if s.gliding {
		s.note += (s.goal - s.note) * s.glideCoef
		if math.Abs(s.goal-s.note) < 1e-4 {
			s.note, s.gliding = s.goal, false
		}
	}
	lfo := s.internalLFO()
	if int(s.target[sEnvTrigger]) == 2 && s.gate {
		// LFO trigger: the envelope's gate follows the LFO's first half-cycle.
		high := s.lfoPhase < .5
		if high && !s.envGate {
			s.env.stage = 1
		} else if !high && s.envGate && s.env.stage != 0 {
			s.env.stage = 3
		}
		s.envGate = high
	}
	var mod [7]float64
	for i, l := range s.lfos {
		if s.lfoEnabled[i] && s.lfoTarget[i] > 0 && s.lfoTarget[i] < len(mod) {
			mod[s.lfoTarget[i]] += float64(l.Process()) * s.lfoDepth[i]
		}
	}
	env := s.env.next(s.current[sSustain])
	accent := 0.0
	if s.accent {
		accent = s.current[sAccent]
	}
	if s.frame%8 == 0 {
		vibrato := s.current[sVCOLFO] * s.current[sVCOLFO] * 12 * lfo
		note := s.note + s.current[sTune] + (math.Round(s.target[sRange])-1)*12 + vibrato + mod[3]*2
		s.frequency = min(s.rate*.45, 440*math.Exp2((note-69)/12))
		cutoff := s.current[sCutoff] + env*s.current[sEnv]*.85 + s.current[sFilterLFO]*lfo*.35 +
			s.current[sKeytrack]*(s.note-60)/120 + mod[1]*.5 + accent*.12
		hz := min(s.rate*.85, 16*math.Exp2(max(0, min(1, cutoff))*10.3))
		s.g = math.Tan(math.Pi * hz / (2 * s.rate)) // the filter runs at twice the rate
		s.k = 4.15 * max(0, min(1, s.current[sResonance]+mod[2]*.5))
	}
	s.frame++
	depth := s.current[sPW]
	switch int(s.target[sPWMSource]) {
	case 1:
		depth *= (lfo + 1) / 2
	case 2:
		depth *= env
	}
	width := .5 - .45*max(0, min(1, depth+mod[4]*.4))
	subMode := int(s.target[sSubMode])
	subGain := s.current[sSub] * max(0, min(1.5, 1+mod[5]*.5))
	G := s.g / (1 + s.g)
	G2 := G * G
	G4 := G2 * G2
	dt := s.frequency / (2 * s.rate)
	x := 0.0
	for half := 0; half < 2; half++ {
		p := s.phase
		saw := 2*p - 1 - dualBLEP(p, dt)
		pulse := -1.0
		if p < width {
			pulse = 1
		}
		edge := p - width
		if edge < 0 {
			edge++
		}
		pulse += dualBLEP(p, dt) - dualBLEP(edge, dt)
		// The sub flips only on VCO wraps, so its edges share the saw's.
		sub := subLevel(subMode, s.wraps)
		if p < dt {
			sub += (sub - subLevel(subMode, s.wraps-1)) / 2 * dualBLEP(p, dt)
		} else if p > 1-dt {
			sub += (subLevel(subMode, s.wraps+1) - sub) / 2 * dualBLEP(p, dt)
		}
		in := (saw*s.current[sSaw] + pulse*s.current[sPulse] + sub*subGain + s.noise()*s.current[sNoise]) * .55
		// Zero-delay four-pole ladder: solve the feedback, saturate the input
		// like an OTA, then run the four one-pole stages.
		S := (G2*G*s.stages[0] + G2*s.stages[1] + G*s.stages[2] + s.stages[3]) / (1 + s.g)
		y := (G4*in + S) / (1 + s.k*G4)
		u := math.Tanh(in - s.k*y)
		for i := range s.stages {
			v := (u - s.stages[i]) * G
			out := v + s.stages[i]
			s.stages[i] = out + v
			u = out
		}
		// The OTA ladder thins the bass as resonance rises; give some back.
		x += u * (1 + s.k*.22) * .5
		s.phase += dt
		if s.phase >= 1 {
			s.phase--
			s.wraps++
		}
	}
	dc := x - s.dcInput + s.dcCoefficient*s.dcOutput
	s.dcInput, s.dcOutput = x, dc
	amp := env
	if int(s.target[sVCAMode]) == 1 {
		gate := 0.0
		if s.gate {
			gate = 1
		}
		s.ampGate += (gate - s.ampGate) * .02 // a few milliseconds, no click
		amp = s.ampGate
	}
	volume := max(0, min(1.5, 1+mod[6]*.5))
	return float32(math.Tanh(dc*1.1) * amp * s.velocity * volume * (1 + accent*.5) * .62)
}
