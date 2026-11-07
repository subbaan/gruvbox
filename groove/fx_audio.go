package groove

import (
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/effects"
	"strings"
)

type effectKey struct{ track, effect string }

type preparedEffect struct {
	config Effect
	state  *effectVoice
}
type effectVoice struct {
	applied    map[string]float64
	lfos       [2]*dsp.LFO
	modulation [2]LFO
	modParams  []Parameter
	modFrame   uint32
	tempo      float64
	kind       EffectKind
	processors [2]effects.Effect
	buffers    [2][1]float32
	values     map[string]float64 // allocated on control thread, audio-owned afterwards
	enabled    bool
	mix        float32
	delayTime  float64
}

func newEffectVoice(kind EffectKind) *effectVoice {
	v := &effectVoice{kind: kind, values: map[string]float64{}, applied: map[string]float64{}, modParams: EffectModulationParameters(kind)}
	for i := range v.lfos {
		v.lfos[i] = dsp.NewLFO(SampleRate)
		v.lfos[i].SetRange(-1, 1)
	}
	for _, p := range EffectParameters(kind) {
		v.values[p.ID] = -1
		v.applied[p.ID] = -1
	}
	for ch := range v.processors {
		switch kind {
		case Distortion:
			v.processors[ch] = effects.NewDistortion(SampleRate, 1)
		case Filter:
			v.processors[ch] = effects.NewMultimodeFilter(SampleRate, 1)
		case Delay:
			v.processors[ch] = effects.NewDelay(SampleRate, 1, 6)
		case Reverb:
			v.processors[ch] = effects.NewReverb(SampleRate, 1)
		case Chorus:
			c := effects.NewChorus(SampleRate, 1)
			c.SetPhase(float64(ch) * .25)
			v.processors[ch] = c
		case BitCrusher:
			v.processors[ch] = effects.NewBitCrusher(SampleRate, 1)
		case Overdrive:
			v.processors[ch] = effects.NewOverdrive(SampleRate)
		case Fuzz:
			v.processors[ch] = effects.NewFuzz(SampleRate)
		case EnvelopeFilter:
			v.processors[ch] = effects.NewEnvelopeFilter(SampleRate)
		case Phaser:
			ph := effects.NewPhaser(SampleRate)
			ph.SetPhase(float64(ch) * .25)
			v.processors[ch] = ph
		case Flanger:
			f := effects.NewFlanger(SampleRate)
			f.SetPhase(float64(ch) * .25)
			v.processors[ch] = f
		case TapeEcho:
			v.processors[ch] = effects.NewTapeEcho(SampleRate, 2)
		case Compressor:
			v.processors[ch] = effects.NewCompressor(SampleRate)
		}
	}
	return v
}
func (v *effectVoice) reset() {
	for _, p := range v.processors {
		p.Reset()
	}
	v.mix = 0
	v.modFrame = 0
	for _, lfo := range v.lfos {
		lfo.Reset()
	}
}
func (v *effectVoice) set(id string, value float64) {
	if _, ok := v.values[id]; !ok {
		return
	}
	v.values[id] = value
	v.apply(id, value)
}
func (v *effectVoice) apply(id string, value float64) {
	if old, ok := v.applied[id]; !ok || old == value {
		return
	}
	v.applied[id] = value
	f := float32(value)
	for _, p := range v.processors {
		switch d := p.(type) {
		case *effects.Distortion:
			switch id {
			case "drive":
				d.SetDrive(f)
			case "tone":
				d.SetTone(f)
			}
		case *effects.MultimodeFilter:
			switch id {
			case "cutoff":
				d.SetCutoff(f)
			case "resonance":
				d.SetResonance(f)
			case "mode":
				d.SetFilterMode(effects.FilterMode(value))
			}
		case *effects.Delay:
			if id == "feedback" {
				d.SetFeedback(f)
			}
		case *effects.Chorus:
			switch id {
			case "rate":
				d.SetRate(f)
			case "depth":
				d.SetDepth(f)
			case "delay_ms":
				d.SetDelay(f)
			case "feedback":
				d.SetFeedback(f)
			}
		case *effects.BitCrusher:
			switch id {
			case "bits":
				d.SetBitDepth(f)
			case "rate":
				d.SetCrushRate(f)
			}
		case *effects.Reverb:
			switch id {
			case "room":
				d.SetRoomSize(f)
			case "damping":
				d.SetDamping(f)
			}
		case *effects.Overdrive:
			switch id {
			case "drive":
				d.SetDrive(f)
			case "tone":
				d.SetTone(f)
			case "level":
				d.SetLevel(f)
			}
		case *effects.Fuzz:
			switch id {
			case "fuzz":
				d.SetFuzz(f)
			case "tone":
				d.SetTone(f)
			case "level":
				d.SetLevel(f)
			}
		case *effects.EnvelopeFilter:
			switch id {
			case "sensitivity":
				d.SetSensitivity(f)
			case "cutoff":
				d.SetCutoff(f)
			case "range":
				d.SetRange(f)
			case "resonance":
				d.SetResonance(f)
			case "decay":
				d.SetDecay(f)
			case "shape":
				d.SetShape(value >= .5)
			case "direction":
				d.SetDown(value >= .5)
			}
		case *effects.Phaser:
			switch id {
			case "rate":
				d.SetRate(f)
			case "depth":
				d.SetDepth(f)
			case "feedback":
				d.SetFeedback(f)
			case "stages":
				d.SetStages(PhaserStages[max(0, min(len(PhaserStages)-1, int(value+.5)))])
			}
		case *effects.Flanger:
			switch id {
			case "rate":
				d.SetRate(f)
			case "depth":
				d.SetDepth(f)
			case "manual":
				d.SetManual(f)
			case "feedback":
				d.SetFeedback(f)
			}
		case *effects.TapeEcho:
			switch id {
			case "feedback":
				d.SetFeedback(f)
			case "wow":
				d.SetWow(f)
			case "tone":
				d.SetTone(f)
			case "drive":
				d.SetDrive(f)
			}
		case *effects.Compressor:
			switch id {
			case "threshold":
				d.SetThreshold(f)
			case "ratio":
				d.SetRatio(f)
			case "attack":
				d.SetAttack(f)
			case "release":
				d.SetRelease(f)
			case "makeup":
				d.SetMakeup(f)
			}
		}
	}
}
func (v *effectVoice) updateTempo(bpm float64) {
	if !v.kind.TempoSynced() {
		return
	}
	limit := 6.
	if v.kind == TapeEcho {
		limit = 2
	}
	time := v.applied["time"]
	if v.applied["sync"] >= .5 {
		time = 240 / bpm * v.applied["division"]
	}
	time = max(.001, min(limit, time))
	if time == v.delayTime {
		return
	}
	v.delayTime = time
	for _, p := range v.processors {
		switch d := p.(type) {
		case *effects.Delay:
			d.SetDelayTime(float32(time))
		case *effects.TapeEcho:
			d.SetDelayTime(float32(time))
		}
	}
}
func (t *playTrack) effectParameters(bpm float64) {
	for _, f := range t.effects {
		enabled := f.config.Enabled && !t.track.EffectsBypassed
		resolution := t.resolution
		if resolution == 0 {
			resolution = 16
		}
		f.state.configureModulation(f.config.LFOs, bpm, resolution)
		if enabled && !f.state.enabled {
			f.state.reset()
		}
		f.state.enabled = enabled
		for id, value := range f.config.Parameters {
			f.state.set(id, value)
		}
		f.state.updateTempo(bpm)
	}
}
func (t *playTrack) parameter(id string, value, bpm float64) {
	if !strings.HasPrefix(id, "fx.") {
		t.voice.parameter(id, value)
		return
	}
	key, _ := strings.CutPrefix(id, "fx.")
	effectID, param, _ := strings.Cut(key, ".")
	for _, f := range t.effects {
		if f.config.ID == effectID {
			f.state.set(param, value)
			f.state.updateTempo(bpm)
			return
		}
	}
}
func (t *playTrack) reset() {
	t.voice.reset()
	for _, f := range t.effects {
		f.state.reset()
	}
}
func (v *effectVoice) process(left, right float32) (float32, float32) {
	if !v.enabled {
		return left, right
	}
	v.modulate()
	v.mix += (float32(v.applied["mix"]) - v.mix) * .005
	v.buffers[0][0] = left
	v.buffers[1][0] = right
	for ch, p := range v.processors {
		p.Process(v.buffers[ch][:])
	}
	return left + (v.buffers[0][0]-left)*v.mix, right + (v.buffers[1][0]-right)*v.mix
}

func (v *effectVoice) configureModulation(settings [2]LFO, bpm float64, resolution int) {
	v.tempo = bpm
	for i, c := range settings {
		if c == (LFO{}) {
			c = DefaultLFOs()[i]
		}
		v.modulation[i] = c
		configureLFO(v.lfos[i], c, bpm, resolution)
		v.lfos[i].SetDepth(1) // Scale the bipolar signal ourselves around the current knob/lock value.
	}
}
func (v *effectVoice) modulate() {
	var values [2]float64
	active := false
	for i, c := range v.modulation {
		if c.Enabled && c.Target > 0 {
			values[i] = float64(v.lfos[i].Process()) * c.Depth
			active = true
		}
	}
	if !active {
		return
	}
	v.modFrame++
	if v.modFrame%16 != 1 {
		return
	}
	for _, c := range v.modulation {
		if !c.Enabled || c.Target < 1 || c.Target > len(v.modParams) {
			continue
		}
		param := v.modParams[c.Target-1]
		value := v.values[param.ID]
		for j, other := range v.modulation {
			if other.Enabled && other.Target == c.Target {
				value += values[j] * (param.Max - param.Min) * .5
			}
		}
		v.apply(param.ID, max(param.Min, min(param.Max, value)))
	}
	v.updateTempo(v.tempo)
}
