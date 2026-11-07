package effects

import "math"

// TapeEcho is a delay whose repeats wobble (wow and flutter), saturate and
// darken a little more on every pass, like a worn tape unit. Time changes
// glide, bending the pitch of the repeats as real tape echoes do.
type TapeEcho struct {
	BaseEffect
	buffer                     []float32
	position                   int
	sampleRate                 float32
	time, targetTime           float32 // seconds
	feedback, wow, tone, drive float32
	wowPhase, flutterPhase     float64
	dark, rumble               onePole
	fresh                      bool // the next time setting snaps instead of gliding
}

func NewTapeEcho(sampleRate, maxSeconds float32) *TapeEcho {
	t := &TapeEcho{BaseEffect: NewBaseEffect(), buffer: make([]float32, int(sampleRate*(maxSeconds+.1))+2), sampleRate: sampleRate, time: .35, targetTime: .35, feedback: .45, wow: .3, tone: .5, drive: .3, fresh: true}
	t.rumble.setCutoff(90, sampleRate)
	t.SetTone(.5)
	return t
}
func (t *TapeEcho) SetDelayTime(seconds float32) {
	t.targetTime = min(float32(len(t.buffer)-2)/t.sampleRate-.1, max(.01, seconds))
	if t.fresh {
		t.time = t.targetTime
	}
}
func (t *TapeEcho) SetFeedback(v float32) { t.feedback = min(.95, max(0, v)) }
func (t *TapeEcho) SetWow(v float32)      { t.wow = clamp01(v) }
func (t *TapeEcho) SetDrive(v float32)    { t.drive = clamp01(v) }
func (t *TapeEcho) SetTone(v float32) {
	t.tone = clamp01(v)
	t.dark.setCutoff(1200+t.tone*t.tone*7000, t.sampleRate)
}
func (t *TapeEcho) Process(buffer []float32) {
	if !t.enabled {
		return
	}
	t.fresh = false
	for i, x := range buffer {
		t.time += (t.targetTime - t.time) * .0002
		wobble := 1 + float64(t.wow)*(.004*math.Sin(2*math.Pi*t.wowPhase)+.0012*math.Sin(2*math.Pi*t.flutterPhase))
		read := float64(t.position) - float64(t.time)*wobble*float64(t.sampleRate)
		for read < 0 {
			read += float64(len(t.buffer))
		}
		index := int(read)
		fraction := float32(read - float64(index))
		wet := t.buffer[index%len(t.buffer)]*(1-fraction) + t.buffer[(index+1)%len(t.buffer)]*fraction
		wet = t.dark.lp(wet)
		// The record head saturates, so even high feedback stays bounded.
		gain := 1 + t.drive*3
		loop := softClip(gain*t.rumble.hp(wet)*t.feedback) / gain
		t.buffer[t.position] = x + loop
		t.position = (t.position + 1) % len(t.buffer)
		buffer[i] = wet
		t.wowPhase += .55 / float64(t.sampleRate)
		t.flutterPhase += 6.5 / float64(t.sampleRate)
		if t.wowPhase >= 1 {
			t.wowPhase--
		}
		if t.flutterPhase >= 1 {
			t.flutterPhase--
		}
	}
}
func (t *TapeEcho) Reset() {
	clear(t.buffer)
	t.position = 0
	t.dark.z, t.rumble.z = 0, 0
	t.time = t.targetTime
	t.fresh = true
}

// Compressor is a feed-forward peak compressor with soft knee and makeup gain.
type Compressor struct {
	BaseEffect
	sampleRate                      float32
	threshold, ratio, makeup        float32 // dB, ratio, dB
	attack, release, envelope, gain float32
	target                          float32
	counter                         int
}

func NewCompressor(sampleRate float32) *Compressor {
	c := &Compressor{BaseEffect: NewBaseEffect(), sampleRate: sampleRate, threshold: -18, ratio: 4, makeup: 6, gain: 1, target: 1}
	c.SetAttack(10)
	c.SetRelease(150)
	return c
}
func (c *Compressor) SetThreshold(db float32) { c.threshold = min(0, max(-60, db)) }
func (c *Compressor) SetRatio(r float32)      { c.ratio = min(20, max(1, r)) }
func (c *Compressor) SetMakeup(db float32)    { c.makeup = min(24, max(0, db)) }
func (c *Compressor) coefficient(ms float32) float32 {
	return float32(1 - math.Exp(-1/(float64(ms)/1000*float64(c.sampleRate))))
}
func (c *Compressor) SetAttack(ms float32)  { c.attack = c.coefficient(min(200, max(.1, ms))) }
func (c *Compressor) SetRelease(ms float32) { c.release = c.coefficient(min(2000, max(5, ms))) }
func (c *Compressor) Process(buffer []float32) {
	if !c.enabled {
		return
	}
	for i, x := range buffer {
		level := x
		if level < 0 {
			level = -level
		}
		if level > c.envelope {
			c.envelope += c.attack * (level - c.envelope)
		} else {
			c.envelope += c.release * (level - c.envelope)
		}
		// The gain computer (log and pow) runs every 4 samples; the
		// smoothing below hides the steps.
		if c.counter%4 == 0 {
			db := float32(-120)
			if c.envelope > 1e-6 {
				db = 20 * float32(math.Log10(float64(c.envelope)))
			}
			// Soft knee 6 dB wide.
			over := db - c.threshold
			var reduction float32
			switch {
			case over <= -3:
			case over >= 3:
				reduction = over * (1 - 1/c.ratio)
			default:
				k := over + 3
				reduction = (1 - 1/c.ratio) * k * k / 12
			}
			c.target = float32(math.Pow(10, float64(c.makeup-reduction)/20))
		}
		c.counter++
		c.gain += (c.target - c.gain) * .05
		buffer[i] = x * c.gain
	}
}
func (c *Compressor) Reset() { c.envelope, c.gain, c.target, c.counter = 0, 1, 1, 0 }
