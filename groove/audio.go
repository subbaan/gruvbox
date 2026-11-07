package groove

import (
	"encoding/binary"
	"math"
	"sync/atomic"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/synth/drums"
)

const SampleRate = 44100
const BlockSize = 256
const SourceBufferFrames = SampleRate / 10 // 100 ms: cover device periods and scheduler jitter
const positionHistorySize = 256

// voice is owned exclusively by Render. Control code may retain its pointer,
// but only reads the published atomic playback position.
type voice struct {
	gateRemaining    int
	kind             Kind
	acid             *synth.TB303
	dual             *synth.DualSynth
	drum             drums.DrumVoice
	buffer           [1]float32
	phase            float64
	tick             uint64
	position         atomic.Int32
	velocity         float32
	acidGate         float64 // 303 gate as a fraction of the note's length; 1 rings
	positionHistory  [positionHistorySize]atomic.Uint64
	positionSequence atomic.Uint64
	sampleVoice
}
type playTrack struct {
	resolution          int
	performancePattern  Pattern
	track               Track
	voice               *voice
	left, right         float32
	baseLeft, baseRight float32
	soloMuted           bool
	effects             []preparedEffect
}
type playback struct {
	project      Project
	tracks       []playTrack // Audio-owned current bank.
	live         []playTrack
	banks        [PatternCount][]playTrack
	liveRevision uint64
}

// Player prepares immutable projects and voices outside the callback. Atomic
// publication adopts the newest prepared state only at the next audio block.
type Player struct {
	performanceGeneration                                     atomic.Uint64
	performancePending                                        atomic.Pointer[performanceTake]
	performanceVisible                                        atomic.Pointer[performanceTake]
	performanceLast                                           atomic.Pointer[performanceTake]
	performance                                               *performanceTake
	renderPattern, requestedPattern                           int
	barPhase                                                  float64
	playingSong                                               bool
	sectionMutes                                              map[string]bool
	sectionIndex, sectionBars                                 int
	songMode                                                  atomic.Bool
	currentPattern, queuedPattern, currentSection, currentBar atomic.Int32
	patternLaunch                                             atomic.Uint64
	auditionRequest                                           atomic.Pointer[sampleAudition]
	auditionPlaying                                           atomic.Bool
	audition                                                  *sampleAudition
	pending                                                   atomic.Pointer[playback]
	playing                                                   atomic.Bool
	restart                                                   atomic.Bool
	active                                                    *playback
	prepared                                                  *playback // control thread only; read project/revision, never tracks
	liveControls                                              bool      // audio thread only
	wasPlaying                                                bool
	cache                                                     map[string]*voice // control thread only
	fxCache                                                   map[effectKey]*effectVoice
	random                                                    uint32
	output                                                    [BlockSize * 2]float32
	bytes                                                     [BlockSize * 4]byte
	offset                                                    int
	device                                                    *oto.Player
	renderedFrames                                            uint64 // audio thread only
	barsPlayed                                                uint64 // audio thread only; since Play
	barClock                                                  atomic.Uint64
	readBytes                                                 atomic.Uint64
}

func NewPlayer() *Player {
	p := &Player{cache: map[string]*voice{}, fxCache: map[effectKey]*effectVoice{}, random: 1, offset: BlockSize * 4}
	p.queuedPattern.Store(-1)
	p.currentSection.Store(-1)
	p.currentBar.Store(1)
	return p
}
func (p *Player) Publish(project Project) {
	state := &playback{project: project.Clone(), liveRevision: 1}
	if previous := p.prepared; previous != nil {
		state.liveRevision = previous.liveRevision
		changed := project.Pattern != previous.project.Pattern || len(project.Tracks) != len(previous.project.Tracks)
		if !changed {
			for i, track := range project.Tracks {
				if !sameLiveControls(track, previous.project.Tracks[i]) {
					changed = true
					break
				}
			}
		}
		if changed {
			state.liveRevision++
		}
	}
	nextCache := make(map[string]*voice, len(project.Tracks))
	nextFX := map[effectKey]*effectVoice{}
	for _, t := range state.project.Tracks {
		v := p.cache[t.ID]
		if v == nil || v.kind != t.Kind {
			v = &voice{kind: t.Kind}
			v.position.Store(-1)
			switch t.Kind {
			case Acid:
				v.acid = synth.NewTB303(SampleRate, BlockSize)
			case Dual:
				v.dual = synth.NewDualSynth(SampleRate)
			case Kick:
				v.drum = drums.NewKickDrum(SampleRate)
			case Snare:
				v.drum = drums.NewSnareDrum(SampleRate)
			case ClosedHat:
				v.drum = drums.NewHiHatClosed(SampleRate)
			case OpenHat:
				v.drum = drums.NewHiHatOpen(SampleRate)
			case Clap:
				v.drum = drums.NewClapDrum(SampleRate)
			case Rimshot:
				v.drum = drums.NewRimshotDrum(SampleRate)
			case Tom:
				v.drum = drums.NewTomDrum(SampleRate)
			case Crash:
				v.drum = drums.NewCrashDrum(SampleRate)
			}
		}
		nextCache[t.ID] = v
	}
	prepare := func(slot int, stored bool) []playTrack {
		tracks := make([]playTrack, len(state.project.Tracks))
		solo := false
		for i, track := range state.project.Tracks {
			if stored {
				track = withTrackState(track, track.Patterns[slot].State)
			}
			level := track.Level * .45
			tracks[i] = playTrack{track: track, voice: nextCache[track.ID], baseLeft: float32(level * math.Sqrt((1-track.Pan)/2)), baseRight: float32(level * math.Sqrt((1+track.Pan)/2))}
			solo = solo || track.Solo
		}
		for i := range tracks {
			t := &tracks[i]
			t.soloMuted = solo && !t.track.Solo
			t.effects = make([]preparedEffect, len(t.track.Effects))
			for n, effect := range t.track.Effects {
				key := effectKey{t.track.ID, effect.ID}
				v := nextFX[key]
				if v == nil {
					v = p.fxCache[key]
				}
				if v == nil || v.kind != effect.Kind {
					v = newEffectVoice(effect.Kind)
				}
				nextFX[key] = v
				t.effects[n] = preparedEffect{config: effect, state: v}
			}
		}
		return tracks
	}
	state.live = prepare(project.Pattern, false)
	// A slot's bank differs from the plain tracks only where a track stores
	// state in it; slots without any share one prepared bank.
	var plain []playTrack
	for slot := 0; slot < PatternCount; slot++ {
		stored := false
		for _, t := range state.project.Tracks {
			stored = stored || t.Patterns[slot].State != nil
		}
		if stored {
			state.banks[slot] = prepare(slot, true)
			continue
		}
		if plain == nil {
			plain = prepare(slot, true)
		}
		state.banks[slot] = plain
	}
	state.tracks = state.live
	p.fxCache, p.cache, p.prepared = nextFX, nextCache, state
	p.pending.Store(state)
}
func (p *Player) Position(id string) int {
	if v := p.cache[id]; v != nil {
		if p.device != nil {
			if !p.Playing() {
				return -1
			}
			return v.positionAt(p.audibleFrame())
		}
		return int(v.position.Load())
	}
	return -1
}
func (p *Player) Playing() bool { return p.playing.Load() }
func (p *Player) SetPlaying(playing bool) {
	if !playing {
		p.CancelPerformance()
		p.performanceVisible.Store(nil)
		p.restart.Store(true)
	}
	p.playing.Store(playing)
}
func (p *Player) Open() error {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{SampleRate: SampleRate, ChannelCount: 2, Format: oto.FormatSignedInt16LE, BufferSize: 20 * time.Millisecond})
	if err != nil {
		return err
	}
	<-ready
	p.device = ctx.NewPlayer(p)
	// Source buffering must cover hardware request sizes. Animation follows the
	// consumed sample timeline separately, rather than starving audio for visuals.
	p.device.SetBufferSize(SourceBufferFrames * 2 * 2) // stereo signed int16
	p.device.Play()
	return nil
}
func (p *Player) Close() {
	if p.device != nil {
		p.device.Close()
	}
}

func (v *voice) reset() {
	v.gateRemaining = 0
	v.phase, v.tick = 0, 0
	v.position.Store(-1)
	v.samplePlaying = false
	for _, lfo := range v.lfos() {
		if lfo != nil {
			lfo.Reset()
		}
	}
	if v.acid != nil {
		v.acid.Reset()
	}
	if v.dual != nil {
		v.dual.Reset()
	}
	if v.drum != nil {
		v.drum.Reset()
	}
}
func (v *voice) parameter(id string, value float64) {
	if v.dual != nil {
		v.dual.SetParameter(id, value)
		return
	}
	if v.kind == Sampler {
		v.sampleVoice.parameter(id, value)
		return
	}
	f := float32(value)
	if v.acid != nil {
		switch id {
		case "cutoff":
			v.acid.SetCutoff(f)
		case "resonance":
			v.acid.SetResonance(f)
		case "env":
			v.acid.SetEnvMod(f)
		case "decay":
			v.acid.SetDecay(f)
		case "accent":
			v.acid.SetAccent(f)
		case "slide":
			v.acid.SetSlideTime(f)
		case "tune":
			v.acid.SetTune(int(value))
		case "sub":
			v.acid.SetSubLevel(f)
		case "pulse":
			v.acid.SetPulseWidth(f)
		case "drive":
			v.acid.SetDrive(f)
		case "sweep":
			v.acid.SetAccentSweep(f)
		case "gate":
			v.acidGate = value
		case "blend":
			v.acid.SetBlend(f)
		case "noise":
			v.acid.SetNoise(f)
		case "filter_mode":
			v.acid.SetFilterMode(int(value + .5))
		case "square":
			if value >= .5 {
				v.acid.SetWaveform(synth.WaveformSquare)
			} else {
				v.acid.SetWaveform(synth.WaveformSaw)
			}
		}
		return
	}
	switch d := v.drum.(type) {
	case *drums.KickDrum:
		switch id {
		case "decay":
			d.SetDecay(f)
		case "drive":
			d.SetDrive(f)
		case "tune":
			d.SetTune(f)
		case "click":
			d.SetClickAmount(f)
		case "pitch_decay":
			d.SetPitchDecay(f)
		}
	case *drums.SnareDrum:
		switch id {
		case "snap":
			d.SetSnap(f)
		case "tone":
			d.SetToneMix(f)
		case "decay":
			d.SetNoiseDecay(f)
		case "tune":
			d.SetTune(f)
		case "tone_decay":
			d.SetToneDecay(f)
		}
	case *drums.ClapDrum:
		switch id {
		case "tune":
			d.SetTune(f)
		case "decay":
			d.SetDecay(f)
		case "tone":
			d.SetTone(f)
		case "drive":
			d.SetDrive(f)
		}
	case *drums.RimshotDrum:
		switch id {
		case "pitch":
			d.SetPitch(f)
		case "decay":
			d.SetDecay(f)
		case "tone":
			d.SetTone(f)
		case "snap":
			d.SetSnap(f)
		}
	case *drums.TomDrum:
		switch id {
		case "pitch":
			d.SetPitch(f)
		case "tune":
			d.SetTune(f)
		case "decay":
			d.SetDecay(f)
		case "pitch_decay":
			d.SetPitchDecay(f)
		case "tone":
			d.SetTone(f)
		case "attack":
			d.SetAttack(f)
		}
	case *drums.CrashDrum:
		switch id {
		case "decay":
			d.SetDecay(f)
		case "tone":
			d.SetTone(f)
		case "metallic":
			d.SetMetallic(f)
		}
	case *drums.HiHat:
		switch id {
		case "decay":
			d.SetDecay(f)
		case "tone":
			d.SetTone(f)
		case "noise":
			d.SetNoiseMix(f)
		}
	}
}

// Render performs no I/O, locking or heap allocation. It is also usable offline
// for timing, DSP and concurrency tests without opening an audio device.
func (p *Player) Render(output []float32) {
	firstFrame := p.renderedFrames
	p.renderedFrames += uint64(len(output) / 2)
	restart := p.restart.Swap(false)
	if next := p.pending.Swap(nil); next != nil {
		reset := restart || p.active == nil
		p.remapSong(next.project)
		previous := p.active
		p.active = next
		if !reset {
			p.adoptTrackBank(previous)
		}
		p.adoptTransport(reset)
		p.configureTracks(reset, reset)
	} else if restart && p.active != nil {
		p.beginTransport(false)
		for i := range p.active.tracks {
			p.active.tracks[i].reset()
		}
	}
	clear(output)
	playing := p.playing.Load()
	if p.active == nil {
		p.renderAudition(output)
		return
	}
	if !playing {
		p.CancelPerformance()
		p.performance = nil
		p.performanceVisible.Store(nil)
		if p.wasPlaying {
			for i := range p.active.tracks {
				p.active.tracks[i].reset()
			}
		}
		p.wasPlaying = false
		p.renderAudition(output)
		return
	}
	if !p.wasPlaying {
		p.beginTransport(p.songMode.Load())
	}
	p.wasPlaying = true
	project := &p.active.project
	for frame := 0; frame < len(output)/2; frame++ {
		if p.barPhase >= 4-1e-10 {
			p.barPhase -= 4
			p.advanceBar()
		}
		for i := range p.active.tracks {
			t := &p.active.tracks[i]
			v := t.voice
			pattern := p.performancePattern(t)
			if v.phase <= 0 {
				v.gateRemaining--
				for _, lfo := range v.lfos() {
					if lfo != nil {
						lfo.StepTrigger()
					}
				}
				index := int(v.tick % uint64(len(pattern.Steps)))
				step := &pattern.Steps[index]
				v.position.Store(int32(index))
				v.recordPatternPosition(firstFrame+uint64(frame), index, p.renderPattern)
				for id, value := range t.track.Parameters {
					v.parameter(id, value)
				}
				t.resolution = pattern.Resolution
				t.effectParameters(project.BPM)
				for _, f := range t.effects {
					for _, lfo := range f.state.lfos {
						lfo.StepTrigger()
					}
				}
				for id, value := range step.Locks {
					t.parameter(id, value, project.BPM)
				}
				p.random = p.random*1664525 + 1013904223
				probability := float64(p.random) / float64(math.MaxUint32)
				loop := v.tick / uint64(len(pattern.Steps))
				if step.Active && probability < step.Probability && (step.Every <= 1 || loop%uint64(step.Every) == 0) {
					for _, f := range t.effects {
						for _, lfo := range f.state.lfos {
							lfo.Trigger()
						}
					}
					v.gateRemaining = step.GateSteps()
					v.velocity = float32(step.Velocity)
					for _, lfo := range v.lfos() {
						if lfo != nil {
							lfo.Trigger()
						}
					}
					if v.acid != nil {
						v.acid.NoteOnFor(step.Note, step.Accent, step.Slide, acidGateSamples(v.acidGate, project.BPM, pattern, index))
					} else if v.dual != nil {
						v.dual.NoteOn(step.Note, step.Accent, step.Slide, step.Velocity)
					} else if v.drum != nil {
						velocity := v.velocity
						if step.Accent {
							velocity = min(1, velocity*1.25)
						}
						if tom, ok := v.drum.(*drums.TomDrum); ok {
							tom.SetNote(step.Note)
						}
						v.drum.Trigger(velocity)
					} else if v.kind == Sampler {
						v.sampleVoice.trigger(t.track, *step)
					}
				} else if v.acid != nil && v.gateRemaining <= 0 {
					v.acid.NoteOff()
				} else if v.dual != nil && v.gateRemaining <= 0 {
					v.dual.NoteOff()
				} else if v.kind == Sampler && t.track.Parameters["mode"] == 1 && v.gateRemaining <= 0 {
					v.samplePlaying = false
				}
				duration := float64(SampleRate) * 60 / project.BPM * 4 / float64(pattern.Resolution)
				if v.tick%2 == 0 {
					duration *= 1 + project.Swing
				} else {
					duration *= 1 - project.Swing
				}
				v.phase += duration
				v.tick++
			}
			v.phase--
			left, right := t.renderSample()
			output[frame*2] += left
			output[frame*2+1] += right
		}
		output[frame*2] = max(-.98, min(.98, output[frame*2]))
		output[frame*2+1] = max(-.98, min(.98, output[frame*2+1]))
		p.barPhase += project.BPM / (60 * SampleRate)
	}
	p.barClock.Store(p.barsPlayed<<16 | uint64(min(65535, p.barPhase/4*65536)))
	p.renderAudition(output)
}
func (p *Player) Read(out []byte) (int, error) {
	// Keep the source reservoir large, but refill it in DSP-sized pieces. Oto
	// accepts short reads and keeps filling; this avoids 100 ms render bursts and
	// lets live parameter snapshots be adopted between blocks.
	out = out[:min(len(out), len(p.bytes))]
	for n := 0; n < len(out); {
		if p.offset >= len(p.bytes) {
			p.Render(p.output[:])
			for i, sample := range p.output {
				binary.LittleEndian.PutUint16(p.bytes[i*2:], uint16(int16(sample*32767)))
			}
			p.offset = 0
		}
		count := copy(out[n:], p.bytes[p.offset:])
		n += count
		p.offset += count
	}
	p.readBytes.Add(uint64(len(out)))
	return len(out), nil
}

// acidGateSamples is how long a 303 note sounds before its quick release:
// Gate × its length in steps. A note that slides into the next step is held,
// as is every note when Gate is 1 (0 means ring).
func acidGateSamples(gate, bpm float64, pattern *Pattern, index int) int {
	if gate >= 1 || gate <= 0 {
		return 0
	}
	next := pattern.Steps[(index+1)%len(pattern.Steps)]
	step := pattern.Steps[index]
	if next.Active && next.Slide && step.GateSteps() <= 1 {
		return 0
	}
	stepSamples := float64(SampleRate) * 60 / bpm * 4 / float64(pattern.Resolution)
	return max(1, int(gate*stepSamples*float64(step.GateSteps())))
}
