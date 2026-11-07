package effects

import "math"

// Chorus uses a continuously interpolated delay tap with bounded feedback.
// Storage is allocated once; live parameter changes glide to their new values.
type Chorus struct {
	BaseEffect
	buffer                                               []float32
	position                                             int
	sampleRate, phase, initialPhase                      float64
	rate, depth, delay, feedback                         float64
	targetRate, targetDepth, targetDelay, targetFeedback float64
}

func NewChorus(sampleRate float32, bufferSize int) *Chorus {
	return &Chorus{BaseEffect: NewBaseEffect(), buffer: make([]float32, int(sampleRate*.08)+2), sampleRate: float64(sampleRate), rate: .6, targetRate: .6, depth: .5, targetDepth: .5, delay: 18, targetDelay: 18}
}
func (c *Chorus) SetRate(v float32)     { c.targetRate = math.Max(.05, math.Min(8, float64(v))) }
func (c *Chorus) SetDepth(v float32)    { c.targetDepth = math.Max(0, math.Min(1, float64(v))) }
func (c *Chorus) SetDelay(v float32)    { c.targetDelay = math.Max(5, math.Min(35, float64(v))) }
func (c *Chorus) SetFeedback(v float32) { c.targetFeedback = math.Max(0, math.Min(.7, float64(v))) }
func (c *Chorus) SetPhase(v float64)    { c.initialPhase = v - math.Floor(v); c.phase = c.initialPhase }
func (c *Chorus) Process(buffer []float32) {
	if !c.enabled {
		return
	}
	for i, dry := range buffer {
		c.rate += (c.targetRate - c.rate) * .001
		c.depth += (c.targetDepth - c.depth) * .001
		c.delay += (c.targetDelay - c.delay) * .001
		c.feedback += (c.targetFeedback - c.feedback) * .001
		frames := (c.delay + c.delay*.45*c.depth*math.Sin(2*math.Pi*c.phase)) * c.sampleRate / 1000
		read := float64(c.position) - frames
		if read < 0 {
			read += float64(len(c.buffer))
		}
		index := int(read)
		fraction := float32(read - float64(index))
		wet := c.buffer[index]*(1-fraction) + c.buffer[(index+1)%len(c.buffer)]*fraction
		c.buffer[c.position] = dry + wet*float32(c.feedback)
		wet *= float32(1 - c.feedback)
		buffer[i] = dry + (wet-dry)*c.mix
		c.position = (c.position + 1) % len(c.buffer)
		c.phase += c.rate / c.sampleRate
		if c.phase >= 1 {
			c.phase -= 1
		}
	}
}
func (c *Chorus) Reset() {
	clear(c.buffer)
	c.position = 0
	c.phase = c.initialPhase
	c.rate, c.depth, c.delay, c.feedback = c.targetRate, c.targetDepth, c.targetDelay, c.targetFeedback
}
