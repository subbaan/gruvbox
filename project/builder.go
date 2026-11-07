package project

import (
	"github.com/subbaan/gruvbox/audio"
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/effects"
	"github.com/subbaan/gruvbox/sampler"
	"github.com/subbaan/gruvbox/sequencer"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/synth/drums"
)

// ExportFromEngine creates a project from the current engine state
func ExportFromEngine(engine *audio.Engine, projectName string) *Project {
	proj := NewProject(projectName)

	// Get components
	seq := engine.GetSequencer()
	tb303_1 := engine.GetTB303()
	tb303_2 := engine.GetTB303_2()
	mixer := engine.GetMixer()
	kick := engine.GetKick()
	snare := engine.GetSnare()
	hhClosed := engine.GetHihatClosed()
	hhOpen := engine.GetHihatOpen()
	clap := engine.GetClap()
	sampler1 := engine.GetSampler1()
	sampler2 := engine.GetSampler2()
	drone := engine.GetDrone()

	// Export global settings
	proj.BPM = seq.GetBPM()
	proj.Swing = seq.GetSwing()
	proj.CurrentPatternID = seq.GetCurrentPatternID()
	song := seq.GetSong()
	proj.SongMode = song.IsSongMode()
	proj.SongLoop = song.IsLoopEnabled()

	// Export patterns
	proj.Patterns = exportPatternBank(seq)

	// Export song arrangement
	proj.Song = exportSongArrangement(song)

	// Export TB-303 settings
	proj.TB303_1 = exportTB303Settings(tb303_1)
	proj.TB303_2 = exportTB303Settings(tb303_2)

	// Export drum settings
	proj.Kick = exportDrumSettings(kick)
	proj.Snare = exportDrumSettings(snare)
	proj.HHClosed = exportDrumSettings(hhClosed)
	proj.HHOpen = exportDrumSettings(hhOpen)
	proj.Clap = exportDrumSettings(clap)

	// Export sampler settings
	proj.Sampler1 = exportSamplerSettings(sampler1)
	proj.Sampler2 = exportSamplerSettings(sampler2)

	// Export drone settings
	proj.Drone = exportDroneSettings(drone)

	// Export track settings (mute, solo, transpose)
	proj.Tracks = exportTrackSettings(seq)

	// Export effects settings
	for i := 0; i < 10; i++ {
		channel := mixer.GetChannel(i)
		if channel != nil {
			proj.Effects[i] = exportChannelEffects(channel)
		}
	}

	// Export mixer settings
	proj.Mixer = exportMixerSettings(mixer)

	return proj
}

// ImportToEngine applies a project to the engine
func ImportToEngine(proj *Project, engine *audio.Engine) error {
	// Get components
	seq := engine.GetSequencer()
	tb303_1 := engine.GetTB303()
	tb303_2 := engine.GetTB303_2()
	mixer := engine.GetMixer()
	kick := engine.GetKick()
	snare := engine.GetSnare()
	hhClosed := engine.GetHihatClosed()
	hhOpen := engine.GetHihatOpen()
	clap := engine.GetClap()
	sampler1 := engine.GetSampler1()
	sampler2 := engine.GetSampler2()
	drone := engine.GetDrone()

	// Import global settings
	seq.SetBPM(proj.BPM)
	seq.SetSwing(proj.Swing)
	song := seq.GetSong()
	song.SetSongMode(proj.SongMode)
	song.SetLoopEnabled(proj.SongLoop)

	// Import patterns
	importPatternBank(proj.Patterns, seq)

	// Update clock length to match the longest pattern
	seq.UpdateClockLength()

	// Set current pattern
	seq.SetCurrentPatternID(proj.CurrentPatternID)

	// Import song arrangement
	importSongArrangement(proj.Song, song)

	// Import TB-303 settings
	importTB303Settings(proj.TB303_1, tb303_1)
	importTB303Settings(proj.TB303_2, tb303_2)

	// Import drum settings
	importDrumSettings(proj.Kick, kick)
	importDrumSettings(proj.Snare, snare)
	importDrumSettings(proj.HHClosed, hhClosed)
	importDrumSettings(proj.HHOpen, hhOpen)
	importDrumSettings(proj.Clap, clap)

	// Import sampler settings
	importSamplerSettings(proj.Sampler1, sampler1)
	importSamplerSettings(proj.Sampler2, sampler2)

	// Import drone settings
	importDroneSettings(proj.Drone, drone)

	// Import track settings (mute, solo, transpose)
	importTrackSettings(proj.Tracks, seq)

	// Import effects settings
	for i := 0; i < 10; i++ {
		channel := mixer.GetChannel(i)
		if channel != nil {
			importChannelEffects(proj.Effects[i], channel)
		}
	}

	// Import mixer settings
	importMixerSettings(proj.Mixer, mixer)

	return nil
}

// exportPatternBank exports all patterns from the sequencer
func exportPatternBank(seq *sequencer.Sequencer) PatternBank {
	bank := PatternBank{}

	instruments := []sequencer.InstrumentType{
		sequencer.InstrumentTB303_1,
		sequencer.InstrumentTB303_2,
		sequencer.InstrumentKick,
		sequencer.InstrumentSnare,
		sequencer.InstrumentHihatClosed,
		sequencer.InstrumentHihatOpen,
		sequencer.InstrumentClap,
		sequencer.InstrumentSampler1,
		sequencer.InstrumentSampler2,
		sequencer.InstrumentDrone,
	}

	for _, inst := range instruments {
		for i := 0; i < 16; i++ {
			pat := seq.GetPattern(inst, i)
			if pat != nil {
				pattern := exportPattern(pat)
				switch inst {
				case sequencer.InstrumentTB303_1:
					bank.TB303_1[i] = pattern
				case sequencer.InstrumentTB303_2:
					bank.TB303_2[i] = pattern
				case sequencer.InstrumentKick:
					bank.Kick[i] = pattern
				case sequencer.InstrumentSnare:
					bank.Snare[i] = pattern
				case sequencer.InstrumentHihatClosed:
					bank.HHClosed[i] = pattern
				case sequencer.InstrumentHihatOpen:
					bank.HHOpen[i] = pattern
				case sequencer.InstrumentClap:
					bank.Clap[i] = pattern
				case sequencer.InstrumentSampler1:
					bank.Sampler1[i] = pattern
				case sequencer.InstrumentSampler2:
					bank.Sampler2[i] = pattern
				case sequencer.InstrumentDrone:
					bank.Drone[i] = pattern
				}
			}
		}
	}

	return bank
}

// exportPattern exports a single pattern
func exportPattern(pat *sequencer.Pattern) Pattern {
	pattern := Pattern{
		Name:   pat.Name,
		Length: pat.Length,
		Steps:  make([]Step, len(pat.Steps)),
	}

	for i, step := range pat.Steps {
		pattern.Steps[i] = ConvertSequencerStep(step)
	}

	// Copy SoundParams (includes all sound settings and mute/solo states)
	if len(pat.SoundParams) > 0 {
		pattern.SoundParams = make(map[string]float32, len(pat.SoundParams))
		for k, v := range pat.SoundParams {
			pattern.SoundParams[k] = v
		}
	}

	return pattern
}

// exportSongArrangement exports the song arrangement
func exportSongArrangement(song *sequencer.Song) SongArrangement {
	arr := SongArrangement{
		Steps: make([]SongStep, song.GetStepCount()),
	}

	for i := 0; i < song.GetStepCount(); i++ {
		step := song.GetStep(i)
		if step != nil {
			arr.Steps[i] = SongStep{
				PatternID: step.PatternID,
				Repeats:   step.Repeats,
			}
		}
	}

	return arr
}

// exportTB303Settings exports TB-303 settings
func exportTB303Settings(tb *synth.TB303) TB303Settings {
	waveform := "saw"
	if tb.GetWaveform() == synth.WaveformSquare {
		waveform = "square"
	}

	lfo1 := tb.GetLFO1()
	lfo2 := tb.GetLFO2()

	return TB303Settings{
		Waveform:  waveform,
		Cutoff:    tb.GetCutoff(),
		Resonance: tb.GetResonance(),
		EnvMod:    tb.GetEnvMod(),
		Decay:     tb.GetDecay(),
		Accent:    tb.GetAccent(),
		SlideTime: tb.GetSlideTime(),
		SubLevel:  tb.GetSubLevel(),
		Tuning:    0.0, // Not implemented yet

		// LFO1 settings
		LFO1Enabled:       tb.GetLFO1Enabled(),
		LFO1Target:        lfoTargetToString(tb.GetLFO1Target()),
		LFO1Waveform:      lfoWaveformToString(lfo1.GetWaveform()),
		LFO1Rate:          lfo1.GetRate(),
		LFO1Depth:         lfo1.GetDepth(),
		LFO1Min:           lfo1.GetMin(),
		LFO1Max:           lfo1.GetMax(),
		LFO1TempoSync:     lfo1.GetTempoSync(),
		LFO1SyncRate:      lfo1.GetSyncRate(),
		LFO1SyncMode:      int(lfo1.GetSyncMode()),
		LFO1StepsPerCycle: lfo1.GetStepsPerCycle(),

		// LFO2 settings
		LFO2Enabled:       tb.GetLFO2Enabled(),
		LFO2Target:        lfoTargetToString(tb.GetLFO2Target()),
		LFO2Waveform:      lfoWaveformToString(lfo2.GetWaveform()),
		LFO2Rate:          lfo2.GetRate(),
		LFO2Depth:         lfo2.GetDepth(),
		LFO2Min:           lfo2.GetMin(),
		LFO2Max:           lfo2.GetMax(),
		LFO2TempoSync:     lfo2.GetTempoSync(),
		LFO2SyncRate:      lfo2.GetSyncRate(),
		LFO2SyncMode:      int(lfo2.GetSyncMode()),
		LFO2StepsPerCycle: lfo2.GetStepsPerCycle(),
	}
}

// exportDrumSettings exports drum settings (generic for all drums)
func exportDrumSettings(drum interface{}) DrumSettings {
	settings := DrumSettings{}

	// Type assert to get specific drum parameters
	switch d := drum.(type) {
	case *drums.KickDrum:
		settings.Pitch = d.GetTune()
		settings.Decay = d.GetDecay()
		settings.Tone = d.GetPitchDecay()
		settings.Drive = d.GetDrive()
		settings.Attack = d.GetClickAmount()

		// LFO1 settings
		lfo1 := d.GetLFO1()
		settings.LFO1Enabled = d.GetLFO1Enabled()
		settings.LFO1Target = int(d.GetLFO1Target())
		settings.LFO1Waveform = int(lfo1.GetWaveform())
		settings.LFO1Rate = lfo1.GetRate()
		settings.LFO1Depth = lfo1.GetDepth()
		settings.LFO1Min = lfo1.GetMin()
		settings.LFO1Max = lfo1.GetMax()
		settings.LFO1TempoSync = lfo1.GetTempoSync()
		settings.LFO1SyncRate = lfo1.GetSyncRate()
		settings.LFO1SyncMode = int(lfo1.GetSyncMode())
		settings.LFO1StepsPerCycle = lfo1.GetStepsPerCycle()

	case *drums.SnareDrum:
		settings.Pitch = d.GetTune()
		settings.Decay = d.GetNoiseDecay()
		settings.Tone = d.GetToneMix()
		settings.Drive = 0.0 // Not available
		settings.Snap = d.GetSnap()

		// LFO1 settings
		lfo1 := d.GetLFO1()
		settings.LFO1Enabled = d.GetLFO1Enabled()
		settings.LFO1Target = int(d.GetLFO1Target())
		settings.LFO1Waveform = int(lfo1.GetWaveform())
		settings.LFO1Rate = lfo1.GetRate()
		settings.LFO1Depth = lfo1.GetDepth()
		settings.LFO1Min = lfo1.GetMin()
		settings.LFO1Max = lfo1.GetMax()
		settings.LFO1TempoSync = lfo1.GetTempoSync()
		settings.LFO1SyncRate = lfo1.GetSyncRate()
		settings.LFO1SyncMode = int(lfo1.GetSyncMode())
		settings.LFO1StepsPerCycle = lfo1.GetStepsPerCycle()

	case *drums.HiHat:
		settings.Pitch = 0.0 // Not available
		settings.Decay = d.GetDecay()
		settings.Tone = d.GetTone()
		settings.Drive = 0.0 // Not available

		// LFO1 settings
		lfo1 := d.GetLFO1()
		settings.LFO1Enabled = d.GetLFO1Enabled()
		settings.LFO1Target = int(d.GetLFO1Target())
		settings.LFO1Waveform = int(lfo1.GetWaveform())
		settings.LFO1Rate = lfo1.GetRate()
		settings.LFO1Depth = lfo1.GetDepth()
		settings.LFO1Min = lfo1.GetMin()
		settings.LFO1Max = lfo1.GetMax()
		settings.LFO1TempoSync = lfo1.GetTempoSync()
		settings.LFO1SyncRate = lfo1.GetSyncRate()
		settings.LFO1SyncMode = int(lfo1.GetSyncMode())
		settings.LFO1StepsPerCycle = lfo1.GetStepsPerCycle()

	case *drums.ClapDrum:
		settings.Pitch = d.GetTune()
		settings.Decay = d.GetDecay()
		settings.Tone = d.GetTone()
		settings.Drive = d.GetDrive()

		// LFO1 settings
		lfo1 := d.GetLFO1()
		settings.LFO1Enabled = d.GetLFO1Enabled()
		settings.LFO1Target = int(d.GetLFO1Target())
		settings.LFO1Waveform = int(lfo1.GetWaveform())
		settings.LFO1Rate = lfo1.GetRate()
		settings.LFO1Depth = lfo1.GetDepth()
		settings.LFO1Min = lfo1.GetMin()
		settings.LFO1Max = lfo1.GetMax()
		settings.LFO1TempoSync = lfo1.GetTempoSync()
		settings.LFO1SyncRate = lfo1.GetSyncRate()
		settings.LFO1SyncMode = int(lfo1.GetSyncMode())
		settings.LFO1StepsPerCycle = lfo1.GetStepsPerCycle()

	case *drums.RimshotDrum:
		settings.Pitch = d.GetPitch()
		settings.Decay = d.GetDecay()
		settings.Tone = d.GetTone()
		settings.Snap = d.GetSnap()

		// LFO1 settings
		lfo1 := d.GetLFO1()
		settings.LFO1Enabled = d.GetLFO1Enabled()
		settings.LFO1Target = int(d.GetLFO1Target())
		settings.LFO1Waveform = int(lfo1.GetWaveform())
		settings.LFO1Rate = lfo1.GetRate()
		settings.LFO1Depth = lfo1.GetDepth()
		settings.LFO1Min = lfo1.GetMin()
		settings.LFO1Max = lfo1.GetMax()
		settings.LFO1TempoSync = lfo1.GetTempoSync()
		settings.LFO1SyncRate = lfo1.GetSyncRate()
		settings.LFO1SyncMode = int(lfo1.GetSyncMode())
		settings.LFO1StepsPerCycle = lfo1.GetStepsPerCycle()

	case *drums.CrashDrum:
		settings.Pitch = 0.0 // Not available
		settings.Decay = d.GetDecay()
		settings.Tone = d.GetTone()
		settings.Drive = 0.0 // Not available

		// LFO1 settings
		lfo1 := d.GetLFO1()
		settings.LFO1Enabled = d.GetLFO1Enabled()
		settings.LFO1Target = int(d.GetLFO1Target())
		settings.LFO1Waveform = int(lfo1.GetWaveform())
		settings.LFO1Rate = lfo1.GetRate()
		settings.LFO1Depth = lfo1.GetDepth()
		settings.LFO1Min = lfo1.GetMin()
		settings.LFO1Max = lfo1.GetMax()
		settings.LFO1TempoSync = lfo1.GetTempoSync()
		settings.LFO1SyncRate = lfo1.GetSyncRate()
		settings.LFO1SyncMode = int(lfo1.GetSyncMode())
		settings.LFO1StepsPerCycle = lfo1.GetStepsPerCycle()

	case *drums.TomDrum:
		settings.Pitch = d.GetTune()
		settings.Decay = d.GetDecay()
		settings.Tone = d.GetTone()
		settings.Drive = 0.0 // Not available

		// LFO1 settings
		lfo1 := d.GetLFO1()
		settings.LFO1Enabled = d.GetLFO1Enabled()
		settings.LFO1Target = int(d.GetLFO1Target())
		settings.LFO1Waveform = int(lfo1.GetWaveform())
		settings.LFO1Rate = lfo1.GetRate()
		settings.LFO1Depth = lfo1.GetDepth()
		settings.LFO1Min = lfo1.GetMin()
		settings.LFO1Max = lfo1.GetMax()
		settings.LFO1TempoSync = lfo1.GetTempoSync()
		settings.LFO1SyncRate = lfo1.GetSyncRate()
		settings.LFO1SyncMode = int(lfo1.GetSyncMode())
		settings.LFO1StepsPerCycle = lfo1.GetStepsPerCycle()
	}

	return settings
}

// exportChannelEffects exports effect settings for one channel
func exportChannelEffects(channel *audio.Channel) ChannelEffects {
	ce := ChannelEffects{}

	// Export distortion
	if dist := channel.Effects.GetEffect(0); dist != nil {
		if d, ok := dist.(*effects.Distortion); ok {
			ce.Distortion = EffectSettings{
				Enabled: d.IsEnabled(),
				Params: map[string]float32{
					"drive": d.GetDrive(),
					"tone":  d.GetTone(),
					"mix":   d.GetMix(),
				},
			}
		}
	}

	// Export delay
	if del := channel.Effects.GetEffect(1); del != nil {
		if d, ok := del.(*effects.Delay); ok {
			ce.Delay = EffectSettings{
				Enabled: d.IsEnabled(),
				Params: map[string]float32{
					"time":     d.GetDelayTime(),
					"feedback": d.GetFeedback(),
					"mix":      d.GetMix(),
				},
			}
		}
	}

	// Export reverb
	if rev := channel.Effects.GetEffect(2); rev != nil {
		if r, ok := rev.(*effects.Reverb); ok {
			ce.Reverb = EffectSettings{
				Enabled: r.IsEnabled(),
				Params: map[string]float32{
					"room_size": r.GetRoomSize(),
					"damping":   r.GetDamping(),
					"mix":       r.GetMix(),
				},
			}
		}
	}

	// Export pitch shifter
	if ps := channel.Effects.GetEffect(3); ps != nil {
		if p, ok := ps.(*effects.PitchShifter); ok {
			ce.PitchShifter = EffectSettings{
				Enabled: p.IsEnabled(),
				Params: map[string]float32{
					"pitch": p.GetPitch(),
					"mix":   p.GetMix(),
				},
			}
		}
	}

	// Export multimode filter
	if mf := channel.Effects.GetEffect(4); mf != nil {
		if f, ok := mf.(*effects.MultimodeFilter); ok {
			ce.MultimodeFilter = EffectSettings{
				Enabled: f.IsEnabled(),
				Params: map[string]float32{
					"cutoff":    f.GetCutoff(),
					"resonance": f.GetResonance(),
					"mode":      float32(f.GetFilterMode()),
					"mix":       f.GetMix(),
				},
			}
		}
	}

	// Export bit crusher
	if bc := channel.Effects.GetEffect(5); bc != nil {
		if c, ok := bc.(*effects.BitCrusher); ok {
			ce.BitCrusher = EffectSettings{
				Enabled: c.IsEnabled(),
				Params: map[string]float32{
					"bit_depth":  c.GetBitDepth(),
					"crush_rate": c.GetCrushRate(),
					"mix":        c.GetMix(),
				},
			}
		}
	}

	return ce
}

// exportMixerSettings exports mixer settings
func exportMixerSettings(mixer *audio.Mixer) MixerSettings {
	ms := MixerSettings{
		MasterLevel: mixer.GetMasterLevel(),
	}

	for i := 0; i < 10; i++ {
		channel := mixer.GetChannel(i)
		if channel != nil {
			ms.Channels[i] = ChannelSettings{
				Level: channel.Level,
				Pan:   channel.Pan,
				Mute:  channel.Mute,
				Solo:  channel.Solo,
			}
		}
	}

	return ms
}

// Import functions

func importPatternBank(bank PatternBank, seq *sequencer.Sequencer) {
	// Import TB-303 #1 patterns
	for i := 0; i < 16; i++ {
		importPattern(bank.TB303_1[i], seq, sequencer.InstrumentTB303_1, i)
	}

	// Import TB-303 #2 patterns
	for i := 0; i < 16; i++ {
		importPattern(bank.TB303_2[i], seq, sequencer.InstrumentTB303_2, i)
	}

	// Import kick patterns
	for i := 0; i < 16; i++ {
		importPattern(bank.Kick[i], seq, sequencer.InstrumentKick, i)
	}

	// Import snare patterns
	for i := 0; i < 16; i++ {
		importPattern(bank.Snare[i], seq, sequencer.InstrumentSnare, i)
	}

	// Import hi-hat closed patterns
	for i := 0; i < 16; i++ {
		importPattern(bank.HHClosed[i], seq, sequencer.InstrumentHihatClosed, i)
	}

	// Import hi-hat open patterns
	for i := 0; i < 16; i++ {
		importPattern(bank.HHOpen[i], seq, sequencer.InstrumentHihatOpen, i)
	}

	// Import clap patterns
	for i := 0; i < 16; i++ {
		importPattern(bank.Clap[i], seq, sequencer.InstrumentClap, i)
	}

	// Import sampler #1 patterns
	for i := 0; i < 16; i++ {
		importPattern(bank.Sampler1[i], seq, sequencer.InstrumentSampler1, i)
	}

	// Import sampler #2 patterns
	for i := 0; i < 16; i++ {
		importPattern(bank.Sampler2[i], seq, sequencer.InstrumentSampler2, i)
	}

	// Import drone patterns
	for i := 0; i < 16; i++ {
		importPattern(bank.Drone[i], seq, sequencer.InstrumentDrone, i)
	}
}

func importPattern(pat Pattern, seq *sequencer.Sequencer, inst sequencer.InstrumentType, patternID int) {
	seqPat := seq.GetPattern(inst, patternID)
	if seqPat == nil {
		return
	}

	seqPat.Name = pat.Name
	seqPat.SetLength(pat.Length)

	for i, step := range pat.Steps {
		if i < len(seqPat.Steps) {
			seqPat.Steps[i] = ConvertToSequencerStep(step)
		}
	}

	// Copy SoundParams (includes all sound settings and mute/solo states)
	if len(pat.SoundParams) > 0 {
		seqPat.SoundParams = make(map[string]float32, len(pat.SoundParams))
		for k, v := range pat.SoundParams {
			seqPat.SoundParams[k] = v
		}
	} else {
		// Initialize empty map if not present
		seqPat.SoundParams = make(map[string]float32)
	}
}

func importSongArrangement(arr SongArrangement, song *sequencer.Song) {
	// Clear existing song
	for song.GetStepCount() > 0 {
		song.RemoveStep(0)
	}

	// Add new steps
	for _, step := range arr.Steps {
		song.AddStep(step.PatternID, step.Repeats)
	}

	song.Reset()
}

func importTB303Settings(settings TB303Settings, tb *synth.TB303) {
	// Set waveform
	if settings.Waveform == "square" {
		tb.SetWaveform(synth.WaveformSquare)
	} else {
		tb.SetWaveform(synth.WaveformSaw)
	}

	tb.SetCutoff(settings.Cutoff)
	tb.SetResonance(settings.Resonance)
	tb.SetEnvMod(settings.EnvMod)
	tb.SetDecay(settings.Decay)
	tb.SetAccent(settings.Accent)
	tb.SetSlideTime(settings.SlideTime)
	tb.SetSubLevel(settings.SubLevel)
	// SetTuning not implemented yet

	// Import LFO1 settings
	lfo1 := tb.GetLFO1()
	tb.SetLFO1Enabled(settings.LFO1Enabled)
	tb.SetLFO1Target(stringToLFOTarget(settings.LFO1Target))
	lfo1.SetWaveform(stringToLFOWaveform(settings.LFO1Waveform))
	lfo1.SetRate(settings.LFO1Rate)
	lfo1.SetDepth(settings.LFO1Depth)
	lfo1.SetRange(settings.LFO1Min, settings.LFO1Max)
	lfo1.SetTempoSync(settings.LFO1TempoSync)
	lfo1.SetSyncRate(settings.LFO1SyncRate)
	lfo1.SetSyncMode(dsp.LFOSyncMode(settings.LFO1SyncMode))
	lfo1.SetStepsPerCycle(settings.LFO1StepsPerCycle)

	// Import LFO2 settings
	lfo2 := tb.GetLFO2()
	tb.SetLFO2Enabled(settings.LFO2Enabled)
	tb.SetLFO2Target(stringToLFOTarget(settings.LFO2Target))
	lfo2.SetWaveform(stringToLFOWaveform(settings.LFO2Waveform))
	lfo2.SetRate(settings.LFO2Rate)
	lfo2.SetDepth(settings.LFO2Depth)
	lfo2.SetRange(settings.LFO2Min, settings.LFO2Max)
	lfo2.SetTempoSync(settings.LFO2TempoSync)
	lfo2.SetSyncRate(settings.LFO2SyncRate)
	lfo2.SetSyncMode(dsp.LFOSyncMode(settings.LFO2SyncMode))
	lfo2.SetStepsPerCycle(settings.LFO2StepsPerCycle)
}

func importDrumSettings(settings DrumSettings, drum interface{}) {
	switch d := drum.(type) {
	case *drums.KickDrum:
		d.SetTune(settings.Pitch)
		d.SetDecay(settings.Decay)
		d.SetPitchDecay(settings.Tone)
		d.SetDrive(settings.Drive)
		if settings.Attack > 0 {
			d.SetClickAmount(settings.Attack)
		}

		// Import LFO1 settings
		lfo1 := d.GetLFO1()
		d.SetLFO1Enabled(settings.LFO1Enabled)
		d.SetLFO1Target(drums.KickLFOTarget(settings.LFO1Target))
		lfo1.SetWaveform(dsp.LFOWaveform(settings.LFO1Waveform))
		lfo1.SetRate(settings.LFO1Rate)
		lfo1.SetDepth(settings.LFO1Depth)
		lfo1.SetRange(settings.LFO1Min, settings.LFO1Max)
		lfo1.SetTempoSync(settings.LFO1TempoSync)
		lfo1.SetSyncRate(settings.LFO1SyncRate)
		lfo1.SetSyncMode(dsp.LFOSyncMode(settings.LFO1SyncMode))
		lfo1.SetStepsPerCycle(settings.LFO1StepsPerCycle)

	case *drums.SnareDrum:
		d.SetTune(settings.Pitch)
		d.SetNoiseDecay(settings.Decay)
		d.SetToneMix(settings.Tone)
		// Drive not available
		if settings.Snap > 0 {
			d.SetSnap(settings.Snap)
		}

		// Import LFO1 settings
		lfo1 := d.GetLFO1()
		d.SetLFO1Enabled(settings.LFO1Enabled)
		d.SetLFO1Target(drums.SnareLFOTarget(settings.LFO1Target))
		lfo1.SetWaveform(dsp.LFOWaveform(settings.LFO1Waveform))
		lfo1.SetRate(settings.LFO1Rate)
		lfo1.SetDepth(settings.LFO1Depth)
		lfo1.SetRange(settings.LFO1Min, settings.LFO1Max)
		lfo1.SetTempoSync(settings.LFO1TempoSync)
		lfo1.SetSyncRate(settings.LFO1SyncRate)
		lfo1.SetSyncMode(dsp.LFOSyncMode(settings.LFO1SyncMode))
		lfo1.SetStepsPerCycle(settings.LFO1StepsPerCycle)

	case *drums.HiHat:
		// Pitch not available
		d.SetDecay(settings.Decay)
		d.SetTone(settings.Tone)
		// Drive not available

		// Import LFO1 settings
		lfo1 := d.GetLFO1()
		d.SetLFO1Enabled(settings.LFO1Enabled)
		d.SetLFO1Target(drums.HiHatLFOTarget(settings.LFO1Target))
		lfo1.SetWaveform(dsp.LFOWaveform(settings.LFO1Waveform))
		lfo1.SetRate(settings.LFO1Rate)
		lfo1.SetDepth(settings.LFO1Depth)
		lfo1.SetRange(settings.LFO1Min, settings.LFO1Max)
		lfo1.SetTempoSync(settings.LFO1TempoSync)
		lfo1.SetSyncRate(settings.LFO1SyncRate)
		lfo1.SetSyncMode(dsp.LFOSyncMode(settings.LFO1SyncMode))
		lfo1.SetStepsPerCycle(settings.LFO1StepsPerCycle)

	case *drums.ClapDrum:
		d.SetTune(settings.Pitch)
		d.SetDecay(settings.Decay)
		d.SetTone(settings.Tone)
		d.SetDrive(settings.Drive)

		// Import LFO1 settings
		lfo1 := d.GetLFO1()
		d.SetLFO1Enabled(settings.LFO1Enabled)
		d.SetLFO1Target(drums.ClapLFOTarget(settings.LFO1Target))
		lfo1.SetWaveform(dsp.LFOWaveform(settings.LFO1Waveform))
		lfo1.SetRate(settings.LFO1Rate)
		lfo1.SetDepth(settings.LFO1Depth)
		lfo1.SetRange(settings.LFO1Min, settings.LFO1Max)
		lfo1.SetTempoSync(settings.LFO1TempoSync)
		lfo1.SetSyncRate(settings.LFO1SyncRate)
		lfo1.SetSyncMode(dsp.LFOSyncMode(settings.LFO1SyncMode))
		lfo1.SetStepsPerCycle(settings.LFO1StepsPerCycle)

	case *drums.RimshotDrum:
		d.SetPitch(settings.Pitch)
		d.SetDecay(settings.Decay)
		d.SetTone(settings.Tone)
		if settings.Snap > 0 {
			d.SetSnap(settings.Snap)
		}

		// Import LFO1 settings
		lfo1 := d.GetLFO1()
		d.SetLFO1Enabled(settings.LFO1Enabled)
		d.SetLFO1Target(drums.RimshotLFOTarget(settings.LFO1Target))
		lfo1.SetWaveform(dsp.LFOWaveform(settings.LFO1Waveform))
		lfo1.SetRate(settings.LFO1Rate)
		lfo1.SetDepth(settings.LFO1Depth)
		lfo1.SetRange(settings.LFO1Min, settings.LFO1Max)
		lfo1.SetTempoSync(settings.LFO1TempoSync)
		lfo1.SetSyncRate(settings.LFO1SyncRate)
		lfo1.SetSyncMode(dsp.LFOSyncMode(settings.LFO1SyncMode))
		lfo1.SetStepsPerCycle(settings.LFO1StepsPerCycle)

	case *drums.CrashDrum:
		d.SetDecay(settings.Decay)
		d.SetTone(settings.Tone)

		// Import LFO1 settings
		lfo1 := d.GetLFO1()
		d.SetLFO1Enabled(settings.LFO1Enabled)
		d.SetLFO1Target(drums.CrashLFOTarget(settings.LFO1Target))
		lfo1.SetWaveform(dsp.LFOWaveform(settings.LFO1Waveform))
		lfo1.SetRate(settings.LFO1Rate)
		lfo1.SetDepth(settings.LFO1Depth)
		lfo1.SetRange(settings.LFO1Min, settings.LFO1Max)
		lfo1.SetTempoSync(settings.LFO1TempoSync)
		lfo1.SetSyncRate(settings.LFO1SyncRate)
		lfo1.SetSyncMode(dsp.LFOSyncMode(settings.LFO1SyncMode))
		lfo1.SetStepsPerCycle(settings.LFO1StepsPerCycle)

	case *drums.TomDrum:
		d.SetTune(settings.Pitch)
		d.SetDecay(settings.Decay)
		d.SetTone(settings.Tone)

		// Import LFO1 settings
		lfo1 := d.GetLFO1()
		d.SetLFO1Enabled(settings.LFO1Enabled)
		d.SetLFO1Target(drums.TomLFOTarget(settings.LFO1Target))
		lfo1.SetWaveform(dsp.LFOWaveform(settings.LFO1Waveform))
		lfo1.SetRate(settings.LFO1Rate)
		lfo1.SetDepth(settings.LFO1Depth)
		lfo1.SetRange(settings.LFO1Min, settings.LFO1Max)
		lfo1.SetTempoSync(settings.LFO1TempoSync)
		lfo1.SetSyncRate(settings.LFO1SyncRate)
		lfo1.SetSyncMode(dsp.LFOSyncMode(settings.LFO1SyncMode))
		lfo1.SetStepsPerCycle(settings.LFO1StepsPerCycle)
	}
}

func importChannelEffects(ce ChannelEffects, channel *audio.Channel) {
	// Import distortion
	if dist := channel.Effects.GetEffect(0); dist != nil {
		if d, ok := dist.(*effects.Distortion); ok {
			d.SetEnabled(ce.Distortion.Enabled)
			if drive, ok := ce.Distortion.Params["drive"]; ok {
				d.SetDrive(drive)
			}
			if tone, ok := ce.Distortion.Params["tone"]; ok {
				d.SetTone(tone)
			}
			if mix, ok := ce.Distortion.Params["mix"]; ok {
				d.SetMix(mix)
			}
		}
	}

	// Import delay
	if del := channel.Effects.GetEffect(1); del != nil {
		if d, ok := del.(*effects.Delay); ok {
			d.SetEnabled(ce.Delay.Enabled)
			if time, ok := ce.Delay.Params["time"]; ok {
				d.SetDelayTime(time)
			}
			if feedback, ok := ce.Delay.Params["feedback"]; ok {
				d.SetFeedback(feedback)
			}
			if mix, ok := ce.Delay.Params["mix"]; ok {
				d.SetMix(mix)
			}
		}
	}

	// Import reverb
	if rev := channel.Effects.GetEffect(2); rev != nil {
		if r, ok := rev.(*effects.Reverb); ok {
			r.SetEnabled(ce.Reverb.Enabled)
			if roomSize, ok := ce.Reverb.Params["room_size"]; ok {
				r.SetRoomSize(roomSize)
			}
			if damping, ok := ce.Reverb.Params["damping"]; ok {
				r.SetDamping(damping)
			}
			if mix, ok := ce.Reverb.Params["mix"]; ok {
				r.SetMix(mix)
			}
		}
	}

	// Import pitch shifter
	if ps := channel.Effects.GetEffect(3); ps != nil {
		if p, ok := ps.(*effects.PitchShifter); ok {
			p.SetEnabled(ce.PitchShifter.Enabled)
			if pitch, ok := ce.PitchShifter.Params["pitch"]; ok {
				p.SetPitch(pitch)
			}
			if mix, ok := ce.PitchShifter.Params["mix"]; ok {
				p.SetMix(mix)
			}
		}
	}

	// Import multimode filter
	if mf := channel.Effects.GetEffect(4); mf != nil {
		if f, ok := mf.(*effects.MultimodeFilter); ok {
			f.SetEnabled(ce.MultimodeFilter.Enabled)
			if cutoff, ok := ce.MultimodeFilter.Params["cutoff"]; ok {
				f.SetCutoff(cutoff)
			}
			if resonance, ok := ce.MultimodeFilter.Params["resonance"]; ok {
				f.SetResonance(resonance)
			}
			if mode, ok := ce.MultimodeFilter.Params["mode"]; ok {
				f.SetFilterMode(effects.FilterMode(int(mode)))
			}
			if mix, ok := ce.MultimodeFilter.Params["mix"]; ok {
				f.SetMix(mix)
			}
		}
	}

	// Import bit crusher
	if bc := channel.Effects.GetEffect(5); bc != nil {
		if c, ok := bc.(*effects.BitCrusher); ok {
			c.SetEnabled(ce.BitCrusher.Enabled)
			if bitDepth, ok := ce.BitCrusher.Params["bit_depth"]; ok {
				c.SetBitDepth(bitDepth)
			}
			if crushRate, ok := ce.BitCrusher.Params["crush_rate"]; ok {
				c.SetCrushRate(crushRate)
			}
			if mix, ok := ce.BitCrusher.Params["mix"]; ok {
				c.SetMix(mix)
			}
		}
	}
}

func importMixerSettings(ms MixerSettings, mixer *audio.Mixer) {
	mixer.SetMasterLevel(ms.MasterLevel)
	for i := 0; i < 10; i++ {
		channel := mixer.GetChannel(i)
		if channel != nil {
			channel.Level = ms.Channels[i].Level
			channel.Pan = ms.Channels[i].Pan
			channel.Mute = ms.Channels[i].Mute
			channel.Solo = ms.Channels[i].Solo
		}
	}
}

// exportSamplerSettings exports sampler settings
func exportSamplerSettings(samp *sampler.Sampler) SamplerSettings {
	settings := SamplerSettings{
		Volume: 1.0, // Default volume
	}

	sample := samp.GetSample()
	if sample != nil {
		settings.SampleName = sample.Name
		settings.SamplePath = sample.FilePath // Save the file path
		settings.TrimStart = sample.TrimStart
		settings.TrimEnd = sample.TrimEnd
	}

	return settings
}

// importSamplerSettings imports sampler settings
func importSamplerSettings(settings SamplerSettings, samp *sampler.Sampler) {
	// If a sample path is provided, try to reload it
	if settings.SamplePath != "" {
		sample := sampler.NewSample(settings.SampleName)
		err := sample.LoadWAV(settings.SamplePath)
		if err == nil {
			// Restore trim points
			sample.TrimStart = settings.TrimStart
			sample.TrimEnd = settings.TrimEnd

			// Load the sample into the sampler
			samp.LoadSample(sample)
		}
		// If loading fails, silently continue - the sample file may have been moved/deleted
	}
}

// exportDroneSettings exports drone synth settings
func exportDroneSettings(drone *synth.DroneGen) DroneSettings {
	lfo1 := drone.GetLFO1()
	lfo2 := drone.GetLFO2()

	return DroneSettings{
		MasterVolume:  drone.GetMasterVolume(),
		MasterTune:    drone.GetMasterTune(),
		Osc1Level:     drone.GetOsc1Level(),
		Osc2Level:     drone.GetOsc2Level(),
		Osc3Level:     drone.GetOsc3Level(),
		Osc2Detune:    drone.GetOsc2Detune(),
		Osc3Detune:    drone.GetOsc3Detune(),
		Sub1Level:     drone.GetSub1Level(),
		Sub2Level:     drone.GetSub2Level(),
		NoiseLevel:    drone.GetNoiseLevel(),
		RingModAmount: drone.GetRingModAmount(),
		DriveAmount:   drone.GetDriveAmount(),
		Cutoff:        drone.GetCutoff(),
		Resonance:     drone.GetResonance(),
		EnvMod:        drone.GetEnvMod(),
		Attack:        drone.GetAttack(),
		Decay:         drone.GetDecay(),
		Sustain:       drone.GetSustain(),
		Release:       drone.GetRelease(),
		LFO1Enabled:       drone.GetLFO1Enabled(),
		LFO1Target:        lfoTargetToString(drone.GetLFO1Target()),
		LFO1Waveform:      lfoWaveformToString(lfo1.GetWaveform()),
		LFO1Rate:          lfo1.GetRate(),
		LFO1Depth:         lfo1.GetDepth(),
		LFO1Min:           lfo1.GetMin(),
		LFO1Max:           lfo1.GetMax(),
		LFO1TempoSync:     lfo1.GetTempoSync(),
		LFO1SyncMode:      int(lfo1.GetSyncMode()),
		LFO1StepsPerCycle: lfo1.GetStepsPerCycle(),
		LFO2Enabled:       drone.GetLFO2Enabled(),
		LFO2Target:        lfoTargetToString(drone.GetLFO2Target()),
		LFO2Waveform:      lfoWaveformToString(lfo2.GetWaveform()),
		LFO2Rate:          lfo2.GetRate(),
		LFO2Depth:         lfo2.GetDepth(),
		LFO2Min:           lfo2.GetMin(),
		LFO2Max:           lfo2.GetMax(),
		LFO2TempoSync:     lfo2.GetTempoSync(),
		LFO2SyncMode:      int(lfo2.GetSyncMode()),
		LFO2StepsPerCycle: lfo2.GetStepsPerCycle(),
	}
}

// importDroneSettings imports drone synth settings
func importDroneSettings(settings DroneSettings, drone *synth.DroneGen) {
	drone.SetMasterVolume(settings.MasterVolume)
	drone.SetMasterTune(settings.MasterTune)
	drone.SetOsc1Level(settings.Osc1Level)
	drone.SetOsc2Level(settings.Osc2Level)
	drone.SetOsc3Level(settings.Osc3Level)
	drone.SetOsc2Detune(settings.Osc2Detune)
	drone.SetOsc3Detune(settings.Osc3Detune)
	drone.SetSub1Level(settings.Sub1Level)
	drone.SetSub2Level(settings.Sub2Level)
	drone.SetNoiseLevel(settings.NoiseLevel)
	drone.SetRingModAmount(settings.RingModAmount)
	drone.SetDriveAmount(settings.DriveAmount)
	drone.SetCutoff(settings.Cutoff)
	drone.SetResonance(settings.Resonance)
	drone.SetEnvMod(settings.EnvMod)
	drone.SetAttack(settings.Attack)
	drone.SetDecay(settings.Decay)
	drone.SetSustain(settings.Sustain)
	drone.SetRelease(settings.Release)

	// LFO1
	drone.SetLFO1Enabled(settings.LFO1Enabled)
	drone.SetLFO1Target(stringToLFOTarget(settings.LFO1Target))

	lfo1 := drone.GetLFO1()
	lfo1.SetWaveform(stringToLFOWaveform(settings.LFO1Waveform))
	lfo1.SetRate(settings.LFO1Rate)
	lfo1.SetDepth(settings.LFO1Depth)
	lfo1.SetRange(settings.LFO1Min, settings.LFO1Max)
	lfo1.SetTempoSync(settings.LFO1TempoSync)
	lfo1.SetSyncMode(dsp.LFOSyncMode(settings.LFO1SyncMode))
	lfo1.SetStepsPerCycle(settings.LFO1StepsPerCycle)

	// LFO2
	drone.SetLFO2Enabled(settings.LFO2Enabled)
	drone.SetLFO2Target(stringToLFOTarget(settings.LFO2Target))

	lfo2 := drone.GetLFO2()
	lfo2.SetWaveform(stringToLFOWaveform(settings.LFO2Waveform))
	lfo2.SetRate(settings.LFO2Rate)
	lfo2.SetDepth(settings.LFO2Depth)
	lfo2.SetRange(settings.LFO2Min, settings.LFO2Max)
	lfo2.SetTempoSync(settings.LFO2TempoSync)
	lfo2.SetSyncMode(dsp.LFOSyncMode(settings.LFO2SyncMode))
	lfo2.SetStepsPerCycle(settings.LFO2StepsPerCycle)
}

// Helper functions for LFO target conversion

func lfoTargetToString(target synth.LFOTarget) string {
	switch target {
	case synth.LFOTargetNone:
		return "none"
	case synth.LFOTargetCutoff:
		return "cutoff"
	case synth.LFOTargetResonance:
		return "resonance"
	case synth.LFOTargetPitch:
		return "pitch"
	case synth.LFOTargetEnvMod:
		return "envmod"
	case synth.LFOTargetPulseWidth:
		return "pulsewidth"
	case synth.LFOTargetVolume:
		return "volume"
	default:
		return "none"
	}
}

func stringToLFOTarget(s string) synth.LFOTarget {
	switch s {
	case "cutoff":
		return synth.LFOTargetCutoff
	case "resonance":
		return synth.LFOTargetResonance
	case "pitch":
		return synth.LFOTargetPitch
	case "envmod":
		return synth.LFOTargetEnvMod
	case "pulsewidth":
		return synth.LFOTargetPulseWidth
	case "volume":
		return synth.LFOTargetVolume
	default:
		return synth.LFOTargetNone
	}
}

// Helper functions for LFO waveform conversion

func lfoWaveformToString(waveform dsp.LFOWaveform) string {
	switch waveform {
	case dsp.LFOSine:
		return "sine"
	case dsp.LFOTriangle:
		return "triangle"
	case dsp.LFOSquare:
		return "square"
	case dsp.LFOSawUp:
		return "sawup"
	case dsp.LFOSawDown:
		return "sawdown"
	case dsp.LFORandom:
		return "random"
	default:
		return "sine"
	}
}

func stringToLFOWaveform(s string) dsp.LFOWaveform {
	switch s {
	case "triangle":
		return dsp.LFOTriangle
	case "square":
		return dsp.LFOSquare
	case "sawup":
		return dsp.LFOSawUp
	case "sawdown":
		return dsp.LFOSawDown
	case "random":
		return dsp.LFORandom
	default:
		return dsp.LFOSine
	}
}

// exportTrackSettings exports track settings (mute, solo, transpose)
func exportTrackSettings(seq *sequencer.Sequencer) TrackSettings {
	ts := TrackSettings{}

	// Export for each instrument
	instruments := []struct {
		inst sequencer.InstrumentType
		ptr  *TrackState
	}{
		{sequencer.InstrumentTB303_1, &ts.TB303_1},
		{sequencer.InstrumentTB303_2, &ts.TB303_2},
		{sequencer.InstrumentKick, &ts.Kick},
		{sequencer.InstrumentSnare, &ts.Snare},
		{sequencer.InstrumentHihatClosed, &ts.HHClosed},
		{sequencer.InstrumentHihatOpen, &ts.HHOpen},
		{sequencer.InstrumentClap, &ts.Clap},
		{sequencer.InstrumentSampler1, &ts.Sampler1},
		{sequencer.InstrumentSampler2, &ts.Sampler2},
		{sequencer.InstrumentDrone, &ts.Drone},
	}

	for _, item := range instruments {
		track := seq.GetTrack(item.inst)
		if track != nil {
			*item.ptr = TrackState{
				Mute:      track.Mute,
				Solo:      track.Solo,
				Transpose: track.Transpose,
			}
		}
	}

	return ts
}

// importTrackSettings imports track settings (mute, solo, transpose)
func importTrackSettings(ts TrackSettings, seq *sequencer.Sequencer) {
	// Import for each instrument
	instruments := []struct {
		inst  sequencer.InstrumentType
		state TrackState
	}{
		{sequencer.InstrumentTB303_1, ts.TB303_1},
		{sequencer.InstrumentTB303_2, ts.TB303_2},
		{sequencer.InstrumentKick, ts.Kick},
		{sequencer.InstrumentSnare, ts.Snare},
		{sequencer.InstrumentHihatClosed, ts.HHClosed},
		{sequencer.InstrumentHihatOpen, ts.HHOpen},
		{sequencer.InstrumentClap, ts.Clap},
		{sequencer.InstrumentSampler1, ts.Sampler1},
		{sequencer.InstrumentSampler2, ts.Sampler2},
		{sequencer.InstrumentDrone, ts.Drone},
	}

	for _, item := range instruments {
		track := seq.GetTrack(item.inst)
		if track != nil {
			track.Mute = item.state.Mute
			track.Solo = item.state.Solo
			track.Transpose = item.state.Transpose
		}
	}
}
