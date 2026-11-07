package groove

import (
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/synth/drums"
)

func (v *voice) lfos() [2]*dsp.LFO {
	if v.dual != nil {
		return v.dual.LFOs()
	}
	if v.acid != nil {
		return [2]*dsp.LFO{v.acid.GetLFO1(), v.acid.GetLFO2()}
	}
	switch d := v.drum.(type) {
	case *drums.KickDrum:
		return [2]*dsp.LFO{d.GetLFO1(), d.GetLFO2()}
	case *drums.SnareDrum:
		return [2]*dsp.LFO{d.GetLFO1(), d.GetLFO2()}
	case *drums.ClapDrum:
		return [2]*dsp.LFO{d.GetLFO1(), d.GetLFO2()}
	case *drums.RimshotDrum:
		return [2]*dsp.LFO{d.GetLFO1(), d.GetLFO2()}
	case *drums.TomDrum:
		return [2]*dsp.LFO{d.GetLFO1(), d.GetLFO2()}
	case *drums.CrashDrum:
		return [2]*dsp.LFO{d.GetLFO1(), d.GetLFO2()}
	case *drums.HiHat:
		return [2]*dsp.LFO{d.GetLFO1(), d.GetLFO2()}
	}
	return [2]*dsp.LFO{}
}

func (v *voice) modulation(t Track, bpm float64, resolution int) {
	a, b := t.LFOs[0], t.LFOs[1]
	if v.acid != nil {
		v.acid.SetLFO1Target(synth.LFOTarget(a.Target))
		v.acid.SetLFO2Target(synth.LFOTarget(b.Target))
		v.acid.SetLFO1Enabled(a.Enabled)
		v.acid.SetLFO2Enabled(b.Enabled)
	} else {
		switch d := v.drum.(type) {
		case *drums.KickDrum:
			d.SetLFO1Target(drums.KickLFOTarget(a.Target))
			d.SetLFO2Target(drums.KickLFOTarget(b.Target))
			d.SetLFO1Enabled(a.Enabled)
			d.SetLFO2Enabled(b.Enabled)
		case *drums.SnareDrum:
			d.SetLFO1Target(drums.SnareLFOTarget(a.Target))
			d.SetLFO2Target(drums.SnareLFOTarget(b.Target))
			d.SetLFO1Enabled(a.Enabled)
			d.SetLFO2Enabled(b.Enabled)
		case *drums.ClapDrum:
			d.SetLFO1Target(drums.ClapLFOTarget(a.Target))
			d.SetLFO2Target(drums.ClapLFOTarget(b.Target))
			d.SetLFO1Enabled(a.Enabled)
			d.SetLFO2Enabled(b.Enabled)
		case *drums.RimshotDrum:
			d.SetLFO1Target(drums.RimshotLFOTarget(a.Target))
			d.SetLFO2Target(drums.RimshotLFOTarget(b.Target))
			d.SetLFO1Enabled(a.Enabled)
			d.SetLFO2Enabled(b.Enabled)
		case *drums.TomDrum:
			d.SetLFO1Target(drums.TomLFOTarget(a.Target))
			d.SetLFO2Target(drums.TomLFOTarget(b.Target))
			d.SetLFO1Enabled(a.Enabled)
			d.SetLFO2Enabled(b.Enabled)
		case *drums.CrashDrum:
			d.SetLFO1Target(drums.CrashLFOTarget(a.Target))
			d.SetLFO2Target(drums.CrashLFOTarget(b.Target))
			d.SetLFO1Enabled(a.Enabled)
			d.SetLFO2Enabled(b.Enabled)
		case *drums.HiHat:
			d.SetLFO1Target(drums.HiHatLFOTarget(a.Target))
			d.SetLFO2Target(drums.HiHatLFOTarget(b.Target))
			d.SetLFO1Enabled(a.Enabled)
			d.SetLFO2Enabled(b.Enabled)
		}
	}
	for i, lfo := range v.lfos() {
		if lfo == nil {
			continue
		}
		configureLFO(lfo, t.LFOs[i], bpm, resolution)
		if v.dual != nil {
			v.dual.SetModulation(i, t.LFOs[i].Target, t.LFOs[i].Enabled, t.LFOs[i].Depth)
		}
	}
}
func configureLFO(lfo *dsp.LFO, c LFO, bpm float64, resolution int) {
	lfo.SetWaveform(dsp.LFOWaveform(c.Waveform))
	lfo.SetDepth(float32(c.Depth))
	lfo.SetRetrigger(c.Retrigger)
	lfo.SetPhaseReset(float32(c.Phase))
	lfo.SetSyncMode(dsp.SyncModeTime)
	lfo.SetTempoSync(c.Sync == 1)
	lfo.SetRate(float32(c.Rate))
	switch c.Sync {
	case 1:
		lfo.SetSyncRate(float32(c.Division))
		lfo.UpdateTempoRate(float32(bpm))
	case 2:
		lfo.SetStepsPerCycle(c.Steps)
		lfo.SetRate(float32(bpm * float64(resolution) / 240 / float64(c.Steps)))
		lfo.SetSyncMode(dsp.SyncModeSteps)
	}
}
