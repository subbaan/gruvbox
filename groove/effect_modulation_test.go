package groove

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/subbaan/gruvbox/effects"
)

func TestEffectModulationCentresOnKnobsAndLocks(t *testing.T) {
	e := NewEffect(BitCrusher)
	e.LFOs[0] = LFO{Enabled: true, Target: 1, Waveform: 2, Rate: 1, Depth: .2, Division: .25, Steps: 4}
	e.LFOs[1] = e.LFOs[0]
	e.LFOs[1].Depth = .1
	state := newEffectVoice(e.Kind)
	track := playTrack{effects: []preparedEffect{{config: e, state: state}}, resolution: 16}
	track.effectParameters(120)
	state.process(.4, 0)
	if math.Abs(state.applied["bits"]-10.25) > .0001 || state.values["bits"] != 8 {
		t.Fatal("LFOs did not add around base knob")
	}
	track.parameter(EffectLockID(e.ID, "bits"), 12, 120)
	for i := 0; i < 16; i++ {
		state.process(.4, 0)
	}
	if math.Abs(state.applied["bits"]-14.25) > .0001 || state.values["bits"] != 12 {
		t.Fatal("modulation replaced step lock centre")
	}
	for i := range track.effects[0].config.LFOs {
		track.effects[0].config.LFOs[i].Depth = 0
	}
	track.effectParameters(120)
	state.process(.4, 0)
	if state.applied["bits"] != 8 {
		t.Fatal("zero depth moved knob")
	}
	track.effects[0].config.LFOs[0].Enabled = false
	track.effects[0].config.LFOs[1].Enabled = false
	track.effectParameters(120)
	if state.processors[0].(*effects.BitCrusher).GetBitDepth() != 8 {
		t.Fatal("disabling LFO did not restore base")
	}
}
func TestEffectModulationSyncStepsAndRetrigger(t *testing.T) {
	e := NewEffect(Filter)
	e.LFOs[0] = LFO{Enabled: true, Target: 1, Waveform: 3, Rate: 1, Depth: .2, Sync: 2, Division: .25, Steps: 4, Retrigger: true, Phase: .25}
	state := newEffectVoice(e.Kind)
	state.configureModulation(e.LFOs, 120, 64)
	if state.lfos[0].GetRate() != 8 {
		t.Fatal("step sync ignores track resolution")
	}
	state.lfos[0].Reset()
	state.set("cutoff", 4000)
	state.enabled = true
	state.process(.2, 0)
	first := state.applied["cutoff"]
	for i := 0; i < 100; i++ {
		state.process(.2, 0)
	}
	if state.applied["cutoff"] != first {
		t.Fatal("step LFO advanced between steps")
	}
	state.lfos[0].StepTrigger()
	for i := 0; i < 16; i++ {
		state.process(.2, 0)
	}
	if state.applied["cutoff"] == first {
		t.Fatal("step LFO did not advance")
	}
	state.lfos[0].Trigger()
	for i := 0; i < 16; i++ {
		state.process(.2, 0)
	}
	if math.Abs(state.applied["cutoff"]-first) > .001 {
		t.Fatal("FX note retrigger ignored phase")
	}
	e.LFOs[0].Sync = 1
	state.configureModulation(e.LFOs, 120, 16)
	if state.lfos[0].GetRate() != 2 {
		t.Fatal("tempo division wrong")
	}
}
func TestNewEffectsAndLFOSaveUndoLocksAndRealtimeRender(t *testing.T) {
	for _, kind := range EffectKinds {
		t.Run(kind.Name(), func(t *testing.T) {
			p := fxProject(kind)
			p.Tracks[0].Effects[0].LFOs[0].Enabled = true
			p.Tracks[0].Effects[0].LFOs[0].Depth = .1
			param := EffectModulationParameters(kind)[0]
			e := p.Tracks[0].Effects[0]
			p.Tracks[0].Patterns[0].Steps[0].Locks = map[string]float64{EffectLockID(e.ID, param.ID): param.Default}
			data, _ := json.Marshal(p)
			loaded, err := Decode(bytes.NewReader(data))
			if err != nil || !reflect.DeepEqual(p, loaded) {
				t.Fatalf("FX/LFO save: %v", err)
			}
			session := NewSession(p)
			if err := session.Do("Random FX", func(p *Project) error { RandomiseEffects(&p.Tracks[0]); return nil }); err != nil {
				t.Fatal(err)
			}
			session.Undo()
			if !reflect.DeepEqual(session.Project, p) {
				t.Fatal("random FX undo lost modulation")
			}
			player := NewPlayer()
			player.Publish(p)
			player.SetPlaying(true)
			out := make([]float32, BlockSize*2)
			for block := 0; block < 100; block++ {
				player.Render(out)
				for _, v := range out {
					if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
						t.Fatal("invalid modulated audio")
					}
				}
			}
			if allocs := testing.AllocsPerRun(100, func() { player.Render(out) }); allocs != 0 {
				t.Fatal("effect modulation allocates")
			}
			p.Tracks[0].Effects[0].LFOs[0].Target = 999
			if p.Validate() == nil {
				t.Fatal("invalid FX target accepted")
			}
		})
	}
	// Omitted effect LFOs in earlier projects stay disabled and valid.
	p := fxProject(Chorus)
	p.Tracks[0].Effects[0].LFOs = [2]LFO{}
	data, _ := json.Marshal(p)
	if _, err := Decode(bytes.NewReader(data)); err != nil {
		t.Fatal("legacy FX rejected")
	}
}

func BenchmarkModulatedEffects(b *testing.B) {
	p := NewProject()
	p.Tracks = p.Tracks[:1]
	Generate(&p.Tracks[0], 0, "16ths")
	for _, kind := range EffectKinds {
		e := NewEffect(kind)
		for i := range e.LFOs {
			e.LFOs[i].Enabled = true
			e.LFOs[i].Depth = .15
		}
		p.Tracks[0].Effects = append(p.Tracks[0].Effects, e)
	}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var out [BlockSize * 2]float32
	player.Render(out[:])
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		player.Render(out[:])
	}
}

func TestEffectLFOInitialEnableHonoursStartPhase(t *testing.T) {
	e := NewEffect(Filter)
	e.LFOs[0] = LFO{Enabled: true, Target: 1, Waveform: 3, Rate: 1, Depth: .2, Phase: .25, Division: .25, Steps: 4}
	state := newEffectVoice(e.Kind)
	track := playTrack{effects: []preparedEffect{{config: e, state: state}}}
	track.effectParameters(120)
	state.process(.2, 0)
	if math.Abs(state.applied["cutoff"]-3001) > .001 {
		t.Fatal("first enable ignored FX LFO start phase")
	}
}
