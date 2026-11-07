package effects

import "math"

// svf is a topology-preserving state-variable filter (low, band and high
// outputs from one structure), stable under fast cutoff sweeps.
type svf struct {
	g, k, a1, a2, a3 float32
	ic1, ic2         float32
}

func (f *svf) set(hz, q, sampleRate float32) {
	hz = min(sampleRate*.45, max(20, hz))
	f.g = float32(math.Tan(math.Pi * float64(hz/sampleRate)))
	f.k = 1 / q
	f.a1 = 1 / (1 + f.g*(f.g+f.k))
	f.a2 = f.g * f.a1
	f.a3 = f.g * f.a2
}
func (f *svf) process(x float32) (low, band float32) {
	v3 := x - f.ic2
	v1 := f.a1*f.ic1 + f.a2*v3
	v2 := f.ic2 + f.a2*f.ic1 + f.a3*v3
	f.ic1 = 2*v1 - f.ic1
	f.ic2 = 2*v2 - f.ic2
	return v2, v1
}

// EnvelopeFilter is an auto-wah: an envelope follower on the input moves a
// resonant filter, up or down, so harder notes open (or close) it further.
type EnvelopeFilter struct {
	BaseEffect
	sampleRate                    float32
	sensitivity, cutoff, sweep, q float32
	decay                         float32 // release in ms
	bandPass, down                bool
	envelope, attack, release     float32
	filter                        svf
	counter                       int
}

func NewEnvelopeFilter(sampleRate float32) *EnvelopeFilter {
	e := &EnvelopeFilter{BaseEffect: NewBaseEffect(), sampleRate: sampleRate, sensitivity: .6, cutoff: .25, sweep: .7, q: 3}
	e.attack = float32(1 - math.Exp(-1/(.002*float64(sampleRate))))
	e.SetDecay(120)
	e.filter.set(400, e.q, sampleRate)
	return e
}
func (e *EnvelopeFilter) SetSensitivity(v float32) { e.sensitivity = clamp01(v) }
func (e *EnvelopeFilter) SetCutoff(v float32)      { e.cutoff = clamp01(v) }
func (e *EnvelopeFilter) SetRange(v float32)       { e.sweep = clamp01(v) }

// SetResonance maps 0–0.95 to a filter Q of 0.7–12.
func (e *EnvelopeFilter) SetResonance(v float32) { e.q = .7 + min(.95, max(0, v))*12 }
func (e *EnvelopeFilter) SetDecay(ms float32) {
	e.decay = min(1000, max(10, ms))
	e.release = float32(1 - math.Exp(-1/(float64(e.decay)/1000*float64(e.sampleRate))))
}
func (e *EnvelopeFilter) SetShape(bandPass bool) { e.bandPass = bandPass }
func (e *EnvelopeFilter) SetDown(down bool)      { e.down = down }
func (e *EnvelopeFilter) Process(buffer []float32) {
	if !e.enabled {
		return
	}
	for i, x := range buffer {
		level := x
		if level < 0 {
			level = -level
		}
		if level > e.envelope {
			e.envelope += e.attack * (level - e.envelope)
		} else {
			e.envelope += e.release * (level - e.envelope)
		}
		// Retune every 8 samples; the follower is far slower than that.
		if e.counter%8 == 0 {
			amount := min(1, e.envelope*e.sensitivity*8)
			if e.down {
				amount = 1 - amount
			}
			octaves := amount * e.sweep * 5
			hz := 70 * float32(math.Exp2(float64(e.cutoff*5+octaves)))
			e.filter.set(hz, e.q, e.sampleRate)
		}
		e.counter++
		low, band := e.filter.process(x)
		if e.bandPass {
			// A band-pass peaks at Q × input; cap the peak near 10× so
			// high resonance does not jump out (a narrow swept band
			// needs that much to sit at about the dry level).
			buffer[i] = band * min(2, 10*e.filter.k)
		} else {
			buffer[i] = low
		}
	}
}
func (e *EnvelopeFilter) Reset() {
	e.envelope, e.filter.ic1, e.filter.ic2, e.counter = 0, 0, 0, 0
}

// Phaser runs the signal through swept first-order all-pass stages with
// feedback. Its output is wet only; a 50% mix gives the classic notches.
type Phaser struct {
	BaseEffect
	sampleRate, phase, initialPhase float64
	rate, depth, feedback           float32
	stages                          int
	coefficient, last               float32
	xs, ys                          [12]float32
	counter                         int
}

func NewPhaser(sampleRate float32) *Phaser {
	return &Phaser{BaseEffect: NewBaseEffect(), sampleRate: float64(sampleRate), rate: .4, depth: .7, feedback: .4, stages: 6}
}
func (p *Phaser) SetRate(v float32)     { p.rate = min(8, max(.02, v)) }
func (p *Phaser) SetDepth(v float32)    { p.depth = clamp01(v) }
func (p *Phaser) SetFeedback(v float32) { p.feedback = min(.9, max(0, v)) }

// SetStages takes 4, 6, 8 or 12 stages.
func (p *Phaser) SetStages(n int) { p.stages = min(len(p.xs), max(2, n)) }
func (p *Phaser) SetPhase(v float64) {
	p.initialPhase = v - math.Floor(v)
	p.phase = p.initialPhase
}
func (p *Phaser) Process(buffer []float32) {
	if !p.enabled {
		return
	}
	for i, x := range buffer {
		if p.counter%16 == 0 {
			sweep := .5 + .5*math.Sin(2*math.Pi*p.phase)
			hz := 160 * math.Exp2(sweep*float64(p.depth)*4.5)
			t := math.Tan(math.Pi * min(.45, hz/p.sampleRate))
			p.coefficient = float32((t - 1) / (t + 1)) // 90° at hz; 0° at DC, 180° at Nyquist
		}
		p.counter++
		y := x + p.last*p.feedback
		for s := 0; s < p.stages; s++ {
			out := p.coefficient*y + p.xs[s] - p.coefficient*p.ys[s]
			p.xs[s], p.ys[s] = y, out
			y = out
		}
		p.last = y
		buffer[i] = y
		p.phase += float64(p.rate) / p.sampleRate
		if p.phase >= 1 {
			p.phase--
		}
	}
}
func (p *Phaser) Reset() {
	p.xs, p.ys, p.last, p.counter = [12]float32{}, [12]float32{}, 0, 0
	p.phase = p.initialPhase
}

// Flanger is a short modulated delay with feedback; like the phaser its
// output is wet only, so a 50% mix gives the jet sweep.
type Flanger struct {
	BaseEffect
	buffer                          []float32
	position                        int
	sampleRate, phase, initialPhase float64
	rate, depth, manual, feedback   float32
	targetManual                    float32
}

func NewFlanger(sampleRate float32) *Flanger {
	return &Flanger{BaseEffect: NewBaseEffect(), buffer: make([]float32, int(sampleRate*.025)+2), sampleRate: float64(sampleRate), rate: .25, depth: .7, manual: 2.5, targetManual: 2.5, feedback: .5}
}
func (f *Flanger) SetRate(v float32)     { f.rate = min(8, max(.02, v)) }
func (f *Flanger) SetDepth(v float32)    { f.depth = clamp01(v) }
func (f *Flanger) SetManual(ms float32)  { f.targetManual = min(10, max(.3, ms)) }
func (f *Flanger) SetFeedback(v float32) { f.feedback = min(.92, max(0, v)) }
func (f *Flanger) SetPhase(v float64) {
	f.initialPhase = v - math.Floor(v)
	f.phase = f.initialPhase
}
func (f *Flanger) Process(buffer []float32) {
	if !f.enabled {
		return
	}
	for i, x := range buffer {
		glide(&f.manual, f.targetManual)
		ms := float64(f.manual) * (1 + .9*float64(f.depth)*math.Sin(2*math.Pi*f.phase))
		read := float64(f.position) - max(.05, ms)*f.sampleRate/1000
		if read < 0 {
			read += float64(len(f.buffer))
		}
		index := int(read)
		fraction := float32(read - float64(index))
		wet := f.buffer[index]*(1-fraction) + f.buffer[(index+1)%len(f.buffer)]*fraction
		f.buffer[f.position] = softClip(x + wet*f.feedback)
		f.position = (f.position + 1) % len(f.buffer)
		buffer[i] = wet
		f.phase += float64(f.rate) / f.sampleRate
		if f.phase >= 1 {
			f.phase--
		}
	}
}
func (f *Flanger) Reset() {
	clear(f.buffer)
	f.position = 0
	f.phase = f.initialPhase
	f.manual = f.targetManual
}
