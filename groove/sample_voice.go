package groove

import "math"

// All cursor state belongs to the render thread. Sample storage is immutable.
type sampleVoice struct {
	sample                  *Sample
	samplePlaying           bool
	cursor, speed, pitch    float64
	start, end              int
	loop                    bool
	gain                    float32
	gainDB, fadeIn, fadeOut float64
	normalize               bool
	level, targetLevel      float64
}

func (v *sampleVoice) trigger(t Track, step Step) {
	if v.sample == nil {
		return
	}
	frames := v.sample.Frames()
	v.start = min(frames-1, int(t.Parameters["start"]*float64(frames)))
	v.end = min(frames, max(v.start+1, int(t.Parameters["end"]*float64(frames))))
	v.speed = float64(v.sample.rate) / SampleRate * math.Exp2((float64(step.Note-36)+v.pitch)/12)
	v.cursor = float64(v.start)
	if t.Parameters["reverse"] >= .5 {
		v.cursor = float64(v.end - 1)
		v.speed = -v.speed
	}
	v.loop = t.Parameters["mode"] == 2
	v.gain = float32(step.Velocity)
	if step.Accent {
		v.gain = min(1, v.gain*1.25)
	}
	v.level = v.sampleLevel()
	v.targetLevel = v.level
	v.samplePlaying = true
}

func (v *sampleVoice) next() (float32, float32) {
	if !v.samplePlaying || v.sample == nil {
		return 0, 0
	}
	if v.cursor < float64(v.start) || v.cursor >= float64(v.end) {
		if !v.loop {
			v.samplePlaying = false
			return 0, 0
		}
		width := float64(v.end - v.start)
		v.cursor = float64(v.start) + math.Mod(math.Mod(v.cursor-float64(v.start), width)+width, width)
	}
	i := int(v.cursor)
	j := min(i+1, v.end-1)
	if v.loop && j == i {
		j = v.start
	}
	fraction := float32(v.cursor - float64(i))
	channels := v.sample.channels
	left := v.sample.data[i*channels]*(1-fraction) + v.sample.data[j*channels]*fraction
	right := left
	if channels == 2 {
		right = v.sample.data[i*channels+1]*(1-fraction) + v.sample.data[j*channels+1]*fraction
	}
	envelope := v.fadeEnvelope(v.cursor)
	v.level += (v.targetLevel - v.level) * .005
	v.cursor += v.speed
	gain := float64(v.gain) * v.level * envelope
	return float32(float64(left) * gain), float32(float64(right) * gain)
}

func (v *sampleVoice) sampleLevel() float64 {
	level := math.Pow(10, v.gainDB/20)
	if v.normalize {
		level *= v.sample.NormalizationGain()
	}
	return level
}
func (v *sampleVoice) parameter(id string, value float64) {
	switch id {
	case "pitch":
		v.pitch = value
	case "gain_db":
		v.gainDB = value
	case "normalize":
		v.normalize = value >= .5
	case "fade_in":
		v.fadeIn = value
	case "fade_out":
		v.fadeOut = value
	}
	if id == "gain_db" || id == "normalize" {
		v.targetLevel = v.sampleLevel()
	}
}
func (v *sampleVoice) fadeEnvelope(cursor float64) float64 {
	if v.sample == nil {
		return 1
	}
	fromStart, toEnd := cursor-float64(v.start), float64(v.end-1)-cursor
	if v.speed < 0 {
		fromStart, toEnd = toEnd, fromStart
	}
	envelope := 1.
	width := float64(v.end - v.start - 1)
	if v.fadeIn > 0 {
		frames := min(width, v.fadeIn*float64(v.sample.rate)/1000)
		if frames <= 0 {
			return 0
		}
		envelope = min(envelope, fromStart/frames)
	}
	if v.fadeOut > 0 {
		frames := min(width, v.fadeOut*float64(v.sample.rate)/1000)
		if frames <= 0 {
			return 0
		}
		envelope = min(envelope, toEnd/frames)
	}
	return max(0, min(1, envelope))
}
