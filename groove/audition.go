package groove

import (
	"fmt"
	"math"
)

type sampleAudition struct {
	voice       sampleVoice
	left, right float32
}

// Audition prepares a private one-shot voice on the control thread. It neither
// advances the sequencer nor changes its voices, effects or project history.
func (p *Player) Audition(track Track) error {
	if track.Kind != Sampler || track.Sample == nil {
		return fmt.Errorf("load a sample to audition")
	}
	t := track
	t.Parameters = make(map[string]float64, len(track.Parameters))
	for id, value := range track.Parameters {
		t.Parameters[id] = value
	}
	t.Parameters["mode"] = 0
	a := &sampleAudition{voice: sampleVoice{sample: t.Sample}}
	for id, value := range t.Parameters {
		a.voice.parameter(id, value)
	}
	step := DefaultStep()
	step.Velocity = 1
	a.voice.trigger(t, step)
	a.left = float32(.35 * t.Level * math.Sqrt((1-t.Pan)/2))
	a.right = float32(.35 * t.Level * math.Sqrt((1+t.Pan)/2))
	p.auditionRequest.Store(a)
	return nil
}
func (p *Player) StopAudition()     { p.auditionRequest.Store(&sampleAudition{}) }
func (p *Player) Auditioning() bool { return p.auditionPlaying.Load() }
func (p *Player) renderAudition(output []float32) {
	if a := p.auditionRequest.Swap(nil); a != nil {
		p.audition = a
	}
	a := p.audition
	if a == nil {
		return
	}
	for i := 0; i < len(output)/2; i++ {
		left, right := a.voice.next()
		output[i*2] += left * a.left
		output[i*2+1] += right * a.right
	}
	p.auditionPlaying.Store(a.voice.samplePlaying)
}
