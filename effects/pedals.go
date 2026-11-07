package effects

import "math"

// Pedal effects modelled loosely on the stompboxes often put after a 303.
// Like the other processors they run one channel each, output a fully wet
// signal (the track's FX mix blends dry and wet) and allocate nothing while
// processing. Parameter setters store targets; Process glides towards them.

// softClip is a smooth saturator with unity slope at zero, limited to ±1.
func softClip(x float32) float32 {
	if x > 3 {
		return 1
	}
	if x < -3 {
		return -1
	}
	x2 := x * x
	return x * (27 + x2) / (27 + 9*x2)
}

// onePole is a one-pole low-pass; high-pass is x - lp(x).
type onePole struct{ a, z float32 }

func (f *onePole) setCutoff(hz, sampleRate float32) {
	f.a = float32(1 - math.Exp(-2*math.Pi*float64(hz)/float64(sampleRate)))
}
func (f *onePole) lp(x float32) float32 {
	f.z += f.a * (x - f.z)
	return f.z
}
func (f *onePole) hp(x float32) float32 { return x - f.lp(x) }

func clamp01(v float32) float32 { return min(1, max(0, v)) }

// levelMatch keeps a drive pedal about as loud as its input: slow (~1 s)
// followers of input and output power set a makeup gain, and the Level knob
// trims ±12 dB around that. Drive then changes the character, not the
// volume, whatever the source.
type levelMatch struct {
	in, out, gain, coefficient float32
	counter                    int
}

func newLevelMatch(sampleRate float32) levelMatch {
	return levelMatch{gain: 1, coefficient: float32(1 - math.Exp(-1/float64(sampleRate)))}
}
func (m *levelMatch) process(x, y, level float32) float32 {
	m.in += m.coefficient * (x*x - m.in)
	m.out += m.coefficient * (y*y - m.out)
	if m.counter%32 == 0 {
		target := m.gain
		if m.out > 1e-10 {
			target = min(8, max(.05, float32(math.Sqrt(float64(m.in/m.out)))))
		}
		m.gain += (target - m.gain) * .05
	}
	m.counter++
	return y * m.gain * float32(math.Exp2(float64(level*4-2)))
}
func (m *levelMatch) reset() { m.in, m.out, m.gain, m.counter = 0, 0, 1, 0 }

// glide moves value towards target; the per-sample step keeps knob moves
// free of zipper noise.
func glide(value *float32, target float32) bool {
	if *value == target {
		return false
	}
	*value += (target - *value) * .002
	if d := *value - target; d < 1e-4 && d > -1e-4 {
		*value = target
	}
	return true
}

// Overdrive is a mid-forward soft clipper: lows are thinned before clipping
// (the classic green-pedal hump), clipping is slightly asymmetric, and a tone
// control sets the post low-pass.
type Overdrive struct {
	BaseEffect
	sampleRate                       float32
	drive, tone, level               float32
	targetDrive, targetTone, targetL float32
	match                            levelMatch
	bass, post, dc                   onePole
}

func NewOverdrive(sampleRate float32) *Overdrive {
	o := &Overdrive{BaseEffect: NewBaseEffect(), sampleRate: sampleRate, drive: .5, tone: .5, level: .5, targetDrive: .5, targetTone: .5, targetL: .5, match: newLevelMatch(sampleRate)}
	o.bass.setCutoff(720, sampleRate)
	o.dc.setCutoff(12, sampleRate)
	o.updateTone()
	return o
}
func (o *Overdrive) SetDrive(v float32) { o.targetDrive = clamp01(v) }
func (o *Overdrive) SetTone(v float32)  { o.targetTone = clamp01(v) }
func (o *Overdrive) SetLevel(v float32) { o.targetL = clamp01(v) }
func (o *Overdrive) updateTone()        { o.post.setCutoff(900+o.tone*o.tone*7000, o.sampleRate) }
func (o *Overdrive) Process(buffer []float32) {
	if !o.enabled {
		return
	}
	for i, x := range buffer {
		glide(&o.drive, o.targetDrive)
		glide(&o.level, o.targetL)
		if glide(&o.tone, o.targetTone) {
			o.updateTone()
		}
		low := o.bass.lp(x)
		mid := x - low*.75
		gain := 1 + o.drive*o.drive*60
		const bias = .12
		y := softClip(gain*mid+bias) - softClip(bias)
		y += low * .3 // keep some body under the hump
		y = o.dc.hp(o.post.lp(y))
		buffer[i] = o.match.process(x, y, o.level)
	}
}
func (o *Overdrive) Reset() {
	o.bass.z, o.post.z, o.dc.z = 0, 0, 0
	o.drive, o.level = o.targetDrive, o.targetL
	o.match.reset()
	if o.tone != o.targetTone {
		o.tone = o.targetTone
		o.updateTone()
	}
}

// Fuzz is two cascaded high-gain clipping stages into a mid-scooped tone
// stack: dark and thick at one end, thin and buzzy at the other.
type Fuzz struct {
	BaseEffect
	sampleRate                      float32
	fuzz, tone, level               float32
	targetFuzz, targetTone, targetL float32
	match                           levelMatch
	input, between, low, high, dc   onePole
}

func NewFuzz(sampleRate float32) *Fuzz {
	f := &Fuzz{BaseEffect: NewBaseEffect(), sampleRate: sampleRate, fuzz: .6, tone: .5, level: .5, targetFuzz: .6, targetTone: .5, targetL: .5, match: newLevelMatch(sampleRate)}
	f.input.setCutoff(60, sampleRate)
	f.between.setCutoff(3500, sampleRate)
	f.low.setCutoff(450, sampleRate)
	f.high.setCutoff(1100, sampleRate)
	f.dc.setCutoff(15, sampleRate)
	return f
}
func (f *Fuzz) SetFuzz(v float32)  { f.targetFuzz = clamp01(v) }
func (f *Fuzz) SetTone(v float32)  { f.targetTone = clamp01(v) }
func (f *Fuzz) SetLevel(v float32) { f.targetL = clamp01(v) }
func (f *Fuzz) Process(buffer []float32) {
	if !f.enabled {
		return
	}
	for i, x := range buffer {
		glide(&f.fuzz, f.targetFuzz)
		glide(&f.tone, f.targetTone)
		glide(&f.level, f.targetL)
		stage := 2 + f.fuzz*f.fuzz*40
		y := softClip(stage * f.input.hp(x))
		y = softClip(stage * f.between.lp(y))
		// Tone stack: blend a low-passed and a high-passed copy, which
		// scoops the mids in the middle of the knob.
		y = f.low.lp(y)*(1-f.tone) + f.high.hp(y)*f.tone*1.6
		buffer[i] = f.match.process(x, f.dc.hp(y), f.level)
	}
}
func (f *Fuzz) Reset() {
	f.input.z, f.between.z, f.low.z, f.high.z, f.dc.z = 0, 0, 0, 0, 0
	f.fuzz, f.tone, f.level = f.targetFuzz, f.targetTone, f.targetL
	f.match.reset()
}
