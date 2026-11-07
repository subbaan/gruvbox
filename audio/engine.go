package audio

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ebitengine/oto/v3"
	"github.com/subbaan/gruvbox/sampler"
	"github.com/subbaan/gruvbox/sequencer"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/synth/drums"
	"github.com/subbaan/gruvbox/util"
)

// Engine is the main audio engine
type Engine struct {
	// Oto context and player
	context *oto.Context
	player  *oto.Player

	// Mixer for combining channels
	mixer *Mixer

	// Synthesizers
	tb303   *synth.TB303
	tb303_2 *synth.TB303

	// Drums
	kick        *drums.KickDrum
	snare       *drums.SnareDrum
	hihatClosed *drums.HiHat
	hihatOpen   *drums.HiHat
	clap        *drums.ClapDrum
	rimshot     *drums.RimshotDrum
	crash       *drums.CrashDrum
	tom         *drums.TomDrum

	// Samplers
	sampler1 *sampler.Sampler
	sampler2 *sampler.Sampler

	// Drone synthesizer
	drone *synth.DroneGen

	// Sequencer
	sequencer *sequencer.Sequencer

	// Communication with UI thread
	paramQueue *util.RingBuffer

	// Pre-allocated buffers (no GC in audio thread)
	mixBuffer  []float32 // Stereo interleaved float32
	int16Buffer []int16   // Stereo interleaved int16 for oto
	byteBuffer []byte    // Raw bytes for oto

	bufferSize int
	running    bool

	// Test tone oscillator (for initial testing)
	testTonePhase float32
	testToneFreq  float32
	testToneEnabled bool
}

// NewEngine creates a new audio engine
func NewEngine() (*Engine, error) {
	// Create oto context options
	op := &oto.NewContextOptions{
		SampleRate:   SampleRate,
		ChannelCount: ChannelCount,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   BufferDuration,
	}

	// Initialize oto context
	ctx, readyChan, err := oto.NewContext(op)
	if err != nil {
		return nil, fmt.Errorf("failed to create oto context: %w", err)
	}

	// Wait for context to be ready
	<-readyChan

	// Create engine
	engine := &Engine{
		context:         ctx,
		mixer:           NewMixer(MaxChannels, BufferSize),
		tb303:           synth.NewTB303(float32(SampleRate), BufferSize),
		tb303_2:         synth.NewTB303(float32(SampleRate), BufferSize),
		kick:            drums.NewKickDrum(float32(SampleRate)),
		snare:           drums.NewSnareDrum(float32(SampleRate)),
		hihatClosed:     drums.NewHiHatClosed(float32(SampleRate)),
		hihatOpen:       drums.NewHiHatOpen(float32(SampleRate)),
		clap:            drums.NewClapDrum(float32(SampleRate)),
		rimshot:         drums.NewRimshotDrum(float32(SampleRate)),
		crash:           drums.NewCrashDrum(float32(SampleRate)),
		tom:             drums.NewTomDrum(float32(SampleRate)),
		sampler1:        sampler.NewSampler(4), // 4 voices for polyphony
		sampler2:        sampler.NewSampler(4),
		drone:           synth.NewDroneGen(float32(SampleRate), BufferSize),
		sequencer:       sequencer.NewSequencer(SampleRate),
		paramQueue:      util.NewRingBuffer(256),
		mixBuffer:       make([]float32, BufferSize*ChannelCount),
		int16Buffer:     make([]int16, BufferSize*ChannelCount),
		byteBuffer:      make([]byte, BufferSize*ChannelCount*BytesPerSample),
		bufferSize:      BufferSize,
		running:         false,
		testToneFreq:    440.0,
		testToneEnabled: false,
	}

	return engine, nil
}

// Start starts the audio engine
func (e *Engine) Start() error {
	if e.running {
		return fmt.Errorf("engine already running")
	}

	// Create player with engine as the reader
	e.player = e.context.NewPlayer(e)
	e.running = true

	// Start playback
	e.player.Play()

	return nil
}

// Stop stops the audio engine
func (e *Engine) Stop() {
	if !e.running {
		return
	}

	e.running = false

	if e.player != nil {
		e.player.Close()
		e.player = nil
	}
}

// Close closes the audio engine and releases resources
func (e *Engine) Close() error {
	e.Stop()
	// Note: oto v3 Context doesn't require explicit closing
	// It will be garbage collected automatically
	return nil
}

// Read implements io.Reader for oto
// This is the audio callback - runs in real-time audio thread
func (e *Engine) Read(p []byte) (n int, err error) {
	if !e.running {
		return 0, io.EOF
	}

	// 1. Process parameter updates from UI (lock-free)
	e.processParameterUpdates()

	// 2. Process sequencer and get note events
	noteEvents := e.sequencer.Process(e.bufferSize)

	// 2a. Advance drum LFOs on each sequencer step (for step-locked LFOs)
	if stepCount := e.sequencer.GetTriggeredStepCount(); stepCount > 0 {
		e.triggerDrumLFOSteps(stepCount)
	}

	// 3. Handle note events (trigger synths)
	e.handleNoteEvents(noteEvents)

	// 4. Clear mixer channels
	e.mixer.ClearAllChannels()

	// 5. Render audio sources into mixer channels
	if e.testToneEnabled {
		e.renderTestTone()
	}

	// Render TB-303s into channels 0 and 1
	e.renderTB303()

	// Render drums into channels 2-5
	e.renderDrums()

	// Render samplers into channels 6-7
	e.renderSamplers()

	// Render drone into channel 8
	e.renderDrone()

	// 6. Process effects on all channels
	e.mixer.ProcessChannelEffects()

	// 7. Mix all channels to stereo output
	e.mixer.Mix(e.mixBuffer)

	// 8. Convert float32 to int16
	e.float32ToInt16(e.mixBuffer, e.int16Buffer)

	// 9. Convert int16 to bytes
	e.int16ToBytes(e.int16Buffer, e.byteBuffer)

	// 10. Copy to output buffer
	n = copy(p, e.byteBuffer)
	return n, nil
}

// processParameterUpdates reads and applies parameter updates from the UI thread
func (e *Engine) processParameterUpdates() {
	// Process up to 32 parameter updates per audio callback
	// This prevents the audio thread from spending too much time processing updates
	for i := 0; i < 32; i++ {
		update, ok := e.paramQueue.Read()
		if !ok {
			break
		}

		// Apply parameter update
		e.applyParameterUpdate(update)
	}
}

// applyParameterUpdate applies a single parameter update
func (e *Engine) applyParameterUpdate(update util.ParamUpdate) {
	// TODO: Implement parameter routing to synths/effects
	// For now, just handle mixer parameters

	switch update.TargetID {
	case 0: // Master level
		e.mixer.SetMasterLevel(update.Value)
	default:
		// Channel level/pan updates
		if update.ParameterID == 0 {
			e.mixer.SetChannelLevel(update.TargetID, update.Value)
		} else if update.ParameterID == 1 {
			e.mixer.SetChannelPan(update.TargetID, update.Value)
		}
	}
}

// renderTestTone generates a test tone into channel 0
func (e *Engine) renderTestTone() {
	ch := e.mixer.GetChannel(0)
	if ch == nil {
		return
	}

	// Calculate phase increment
	phaseIncrement := e.testToneFreq / float32(SampleRate)

	for i := 0; i < e.bufferSize; i++ {
		// Generate sine wave
		ch.Buffer[i] = util.Sine(e.testTonePhase) * 0.3 // 30% amplitude

		// Advance phase
		e.testTonePhase += phaseIncrement
		if e.testTonePhase >= 1.0 {
			e.testTonePhase -= 1.0
		}
	}
}

// renderTB303 renders TB-303s into channels 0 and 1
func (e *Engine) renderTB303() {
	// TB-303 #1 on channel 0
	if ch := e.mixer.GetChannel(0); ch != nil {
		e.tb303.Process(ch.Buffer)
	}

	// TB-303 #2 on channel 1
	if ch := e.mixer.GetChannel(1); ch != nil {
		e.tb303_2.Process(ch.Buffer)
	}
}

// renderDrums renders drums into channels 2-9
func (e *Engine) renderDrums() {
	// Kick on channel 2
	if ch := e.mixer.GetChannel(2); ch != nil {
		e.kick.Process(ch.Buffer)
	}

	// Snare on channel 3
	if ch := e.mixer.GetChannel(3); ch != nil {
		e.snare.Process(ch.Buffer)
	}

	// Closed hi-hat on channel 4
	if ch := e.mixer.GetChannel(4); ch != nil {
		e.hihatClosed.Process(ch.Buffer)
	}

	// Open hi-hat on channel 5
	if ch := e.mixer.GetChannel(5); ch != nil {
		e.hihatOpen.Process(ch.Buffer)
	}

	// Clap on channel 6
	if ch := e.mixer.GetChannel(6); ch != nil {
		e.clap.Process(ch.Buffer)
	}

	// Rimshot on channel 7
	if ch := e.mixer.GetChannel(7); ch != nil {
		e.rimshot.Process(ch.Buffer)
	}

	// Crash on channel 8
	if ch := e.mixer.GetChannel(8); ch != nil {
		e.crash.Process(ch.Buffer)
	}

	// Tom on channel 9
	if ch := e.mixer.GetChannel(9); ch != nil {
		e.tom.Process(ch.Buffer)
	}
}

// renderSamplers renders samplers into channels 10-11
func (e *Engine) renderSamplers() {
	// Sampler #1 on channel 10
	if ch := e.mixer.GetChannel(10); ch != nil {
		left, right := e.sampler1.Process(e.bufferSize, SampleRate)
		// Mix stereo to mono for the channel buffer
		for i := 0; i < e.bufferSize; i++ {
			ch.Buffer[i] = (left[i] + right[i]) * 0.5
		}
	}

	// Sampler #2 on channel 11
	if ch := e.mixer.GetChannel(11); ch != nil {
		left, right := e.sampler2.Process(e.bufferSize, SampleRate)
		// Mix stereo to mono for the channel buffer
		for i := 0; i < e.bufferSize; i++ {
			ch.Buffer[i] = (left[i] + right[i]) * 0.5
		}
	}
}

// renderDrone renders the drone synthesizer
func (e *Engine) renderDrone() {
	// Drone on channel 12
	if ch := e.mixer.GetChannel(12); ch != nil {
		// Update BPM for arpeggiator timing
		e.drone.SetBPM(e.sequencer.GetBPM())
		e.drone.Process(ch.Buffer)
	}
}

// handleNoteEvents processes note events from the sequencer
func (e *Engine) handleNoteEvents(events []sequencer.NoteEvent) {
	for _, event := range events {
		// Apply parameter locks before triggering
		if len(event.ParamLocks) > 0 {
			e.applyParamLocks(event.Instrument, event.ParamLocks)
		}

		switch event.Instrument {
		case sequencer.InstrumentTB303_1:
			// Trigger TB-303
			e.tb303.NoteOn(event.Note, event.Accent, event.Slide)

		case sequencer.InstrumentTB303_2:
			// Trigger TB-303 #2
			e.tb303_2.NoteOn(event.Note, event.Accent, event.Slide)

		case sequencer.InstrumentKick:
			// Trigger kick drum
			e.kick.Trigger(event.Velocity)

		case sequencer.InstrumentSnare:
			// Trigger snare drum
			e.snare.Trigger(event.Velocity)

		case sequencer.InstrumentHihatClosed:
			// Trigger closed hi-hat
			e.hihatClosed.Trigger(event.Velocity)

		case sequencer.InstrumentHihatOpen:
			// Trigger open hi-hat
			e.hihatOpen.Trigger(event.Velocity)

		case sequencer.InstrumentClap:
			// Trigger clap
			e.clap.Trigger(event.Velocity)

		case sequencer.InstrumentRimshot:
			// Trigger rimshot
			e.rimshot.Trigger(event.Velocity)

		case sequencer.InstrumentCrash:
			// Trigger crash
			e.crash.Trigger(event.Velocity)

		case sequencer.InstrumentTom:
			// Trigger tom
			e.tom.Trigger(event.Velocity)

		case sequencer.InstrumentSampler1:
			// Trigger sampler #1
			e.sampler1.Trigger(event.Velocity)

		case sequencer.InstrumentSampler2:
			// Trigger sampler #2
			e.sampler2.Trigger(event.Velocity)

		case sequencer.InstrumentDrone:
			// Trigger drone synthesizer
			e.drone.NoteOn(event.Note, event.Velocity)
		}
	}
}

// applyParamLocks applies parameter locks to an instrument before triggering
func (e *Engine) applyParamLocks(instrument sequencer.InstrumentType, locks map[string]float32) {
	for paramName, value := range locks {
		switch instrument {
		case sequencer.InstrumentTB303_1:
			e.applyTB303ParamLock(e.tb303, e.mixer.GetChannel(0), paramName, value)
		case sequencer.InstrumentTB303_2:
			e.applyTB303ParamLock(e.tb303_2, e.mixer.GetChannel(1), paramName, value)
		case sequencer.InstrumentKick:
			e.applyKickParamLock(paramName, value)
		case sequencer.InstrumentSnare:
			e.applySnareParamLock(paramName, value)
		case sequencer.InstrumentHihatClosed:
			e.applyHihatClosedParamLock(paramName, value)
		case sequencer.InstrumentHihatOpen:
			e.applyHihatOpenParamLock(paramName, value)
		case sequencer.InstrumentClap:
			e.applyClapParamLock(paramName, value)
		case sequencer.InstrumentRimshot:
			e.applyRimshotParamLock(paramName, value)
		case sequencer.InstrumentCrash:
			e.applyCrashParamLock(paramName, value)
		case sequencer.InstrumentTom:
			e.applyTomParamLock(paramName, value)
		case sequencer.InstrumentDrone:
			e.applyDroneParamLock(paramName, value)
		}
	}
}

// applyTB303ParamLock applies a parameter lock to a TB-303
func (e *Engine) applyTB303ParamLock(tb *synth.TB303, channel *Channel, paramName string, value float32) {
	switch paramName {
	case "tb303_waveform":
		tb.SetWaveform(synth.Waveform(int(value)))
	case "tb303_cutoff":
		tb.SetCutoff(value)
	case "tb303_resonance":
		tb.SetResonance(value)
	case "tb303_envmod":
		tb.SetEnvMod(value)
	case "tb303_decay":
		tb.SetDecay(value * 2.0) // Denormalize from 0-1 to 0-2 seconds
	case "tb303_accent":
		tb.SetAccent(value)
	case "tb303_slidetime":
		tb.SetSlideTime(value*0.49 + 0.01) // Denormalize from 0-1 to 0.01-0.5 seconds
	case "tb303_sublevel":
		tb.SetSubLevel(value)
	case "tb303_volume":
		channel.Level = value
	}
}

// applyKickParamLock applies a parameter lock to the kick drum
func (e *Engine) applyKickParamLock(paramName string, value float32) {
	switch paramName {
	case "kick_tune":
		e.kick.SetTune(value)
	case "kick_decay":
		e.kick.SetDecay(value)
	case "kick_pitchdecay":
		e.kick.SetPitchDecay(value)
	case "kick_drive":
		e.kick.SetDrive(value)
	case "kick_click":
		e.kick.SetClickAmount(value)
	case "kick_volume":
		e.mixer.GetChannel(2).Level = value
	}
}

// applySnareParamLock applies a parameter lock to the snare drum
func (e *Engine) applySnareParamLock(paramName string, value float32) {
	switch paramName {
	case "snare_tune":
		e.snare.SetTune(value)
	case "snare_snap":
		e.snare.SetSnap(value)
	case "snare_tonedecay":
		e.snare.SetToneDecay(value)
	case "snare_noisedecay":
		e.snare.SetNoiseDecay(value)
	case "snare_tonemix":
		e.snare.SetToneMix(value)
	case "snare_volume":
		e.mixer.GetChannel(3).Level = value
	}
}

// applyHihatClosedParamLock applies a parameter lock to the closed hi-hat
func (e *Engine) applyHihatClosedParamLock(paramName string, value float32) {
	switch paramName {
	case "hhclosed_decay":
		e.hihatClosed.SetDecay(value)
	case "hhclosed_tone":
		e.hihatClosed.SetTone(value)
	case "hhclosed_noisemix":
		e.hihatClosed.SetNoiseMix(value)
	case "hhclosed_volume":
		e.mixer.GetChannel(4).Level = value
	}
}

// applyHihatOpenParamLock applies a parameter lock to the open hi-hat
func (e *Engine) applyHihatOpenParamLock(paramName string, value float32) {
	switch paramName {
	case "hhopen_decay":
		e.hihatOpen.SetDecay(value)
	case "hhopen_tone":
		e.hihatOpen.SetTone(value)
	case "hhopen_noisemix":
		e.hihatOpen.SetNoiseMix(value)
	case "hhopen_volume":
		e.mixer.GetChannel(5).Level = value
	}
}

// applyClapParamLock applies a parameter lock to the clap
func (e *Engine) applyClapParamLock(paramName string, value float32) {
	switch paramName {
	case "clap_tune":
		e.clap.SetTune(value)
	case "clap_decay":
		e.clap.SetDecay(value)
	case "clap_tone":
		e.clap.SetTone(value)
	case "clap_drive":
		e.clap.SetDrive(value)
	case "clap_volume":
		e.mixer.GetChannel(6).Level = value
	}
}

// applyRimshotParamLock applies a parameter lock to the rimshot
func (e *Engine) applyRimshotParamLock(paramName string, value float32) {
	switch paramName {
	case "rimshot_pitch":
		e.rimshot.SetPitch(value)
	case "rimshot_decay":
		e.rimshot.SetDecay(value)
	case "rimshot_tone":
		e.rimshot.SetTone(value)
	case "rimshot_snap":
		e.rimshot.SetSnap(value)
	case "rimshot_volume":
		e.mixer.GetChannel(7).Level = value
	}
}

// applyCrashParamLock applies a parameter lock to the crash
func (e *Engine) applyCrashParamLock(paramName string, value float32) {
	switch paramName {
	case "crash_decay":
		e.crash.SetDecay(value)
	case "crash_tone":
		e.crash.SetTone(value)
	case "crash_metallic":
		e.crash.SetMetallic(value)
	case "crash_volume":
		e.mixer.GetChannel(8).Level = value
	}
}

// applyTomParamLock applies a parameter lock to the tom
func (e *Engine) applyTomParamLock(paramName string, value float32) {
	switch paramName {
	case "tom_tune":
		e.tom.SetTune(value)
	case "tom_decay":
		e.tom.SetDecay(value)
	case "tom_pitchdecay":
		e.tom.SetPitchDecay(value)
	case "tom_volume":
		e.mixer.GetChannel(9).Level = value
	}
}

// applyDroneParamLock applies a parameter lock to the drone synth
func (e *Engine) applyDroneParamLock(paramName string, value float32) {
	switch paramName {
	case "drone_volume":
		e.drone.SetMasterVolume(value)
	case "drone_tune":
		e.drone.SetMasterTune(int(value))
	case "drone_osc1level":
		e.drone.SetOsc1Level(value)
	case "drone_osc2level":
		e.drone.SetOsc2Level(value)
	case "drone_osc3level":
		e.drone.SetOsc3Level(value)
	case "drone_osc2detune":
		e.drone.SetOsc2Detune(value)
	case "drone_osc3detune":
		e.drone.SetOsc3Detune(value)
	case "drone_sub1level":
		e.drone.SetSub1Level(value)
	case "drone_sub2level":
		e.drone.SetSub2Level(value)
	case "drone_cutoff":
		e.drone.SetCutoff(value)
	case "drone_resonance":
		e.drone.SetResonance(value)
	case "drone_envmod":
		e.drone.SetEnvMod(value)
	case "drone_attack":
		e.drone.SetAttack(value)
	case "drone_decay":
		e.drone.SetDecay(value)
	case "drone_sustain":
		e.drone.SetSustain(value)
	case "drone_release":
		e.drone.SetRelease(value)
	}
}

// float32ToInt16 converts float32 samples to int16
func (e *Engine) float32ToInt16(input []float32, output []int16) {
	for i := range input {
		// Clamp to [-1.0, 1.0]
		sample := input[i]
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}

		// Convert to int16
		output[i] = int16(sample * MaxAmplitude)
	}
}

// int16ToBytes converts int16 samples to bytes (little-endian)
func (e *Engine) int16ToBytes(input []int16, output []byte) {
	for i, sample := range input {
		binary.LittleEndian.PutUint16(output[i*2:], uint16(sample))
	}
}

// EnableTestTone enables/disables the test tone
func (e *Engine) EnableTestTone(enabled bool) {
	e.testToneEnabled = enabled
}

// SetTestToneFrequency sets the test tone frequency
func (e *Engine) SetTestToneFrequency(freq float32) {
	e.testToneFreq = freq
}

// GetMixer returns the mixer
func (e *Engine) GetMixer() *Mixer {
	return e.mixer
}

// SendParameterUpdate sends a parameter update to the audio thread
func (e *Engine) SendParameterUpdate(targetID, parameterID int, value float32) bool {
	return e.paramQueue.Write(util.ParamUpdate{
		TargetID:    targetID,
		ParameterID: parameterID,
		Value:       value,
	})
}

// GetTB303 returns the first TB-303 synthesizer
func (e *Engine) GetTB303() *synth.TB303 {
	return e.tb303
}

// GetTB303_2 returns the second TB-303 synthesizer
func (e *Engine) GetTB303_2() *synth.TB303 {
	return e.tb303_2
}

// GetSequencer returns the sequencer
func (e *Engine) GetSequencer() *sequencer.Sequencer {
	return e.sequencer
}

// GetKick returns the kick drum
func (e *Engine) GetKick() *drums.KickDrum {
	return e.kick
}

// GetSnare returns the snare drum
func (e *Engine) GetSnare() *drums.SnareDrum {
	return e.snare
}

// GetHihatClosed returns the closed hi-hat
func (e *Engine) GetHihatClosed() *drums.HiHat {
	return e.hihatClosed
}

// GetHihatOpen returns the open hi-hat
func (e *Engine) GetHihatOpen() *drums.HiHat {
	return e.hihatOpen
}

// GetClap returns the clap drum
func (e *Engine) GetClap() *drums.ClapDrum {
	return e.clap
}

// GetRimshot returns the rimshot drum
func (e *Engine) GetRimshot() *drums.RimshotDrum {
	return e.rimshot
}

// GetCrash returns the crash cymbal
func (e *Engine) GetCrash() *drums.CrashDrum {
	return e.crash
}

// GetTom returns the tom drum
func (e *Engine) GetTom() *drums.TomDrum {
	return e.tom
}

// GetSampler1 returns sampler #1
func (e *Engine) GetSampler1() *sampler.Sampler {
	return e.sampler1
}

// GetSampler2 returns sampler #2
func (e *Engine) GetSampler2() *sampler.Sampler {
	return e.sampler2
}

// GetDrone returns the drone synthesizer
func (e *Engine) GetDrone() *synth.DroneGen {
	return e.drone
}

// UpdateAllLFOTempo updates LFO rates on all instruments based on current BPM
func (e *Engine) UpdateAllLFOTempo() {
	bpm := e.sequencer.GetBPM()

	// Update TB-303 synths
	e.tb303.UpdateLFOTempo(bpm)
	e.tb303_2.UpdateLFOTempo(bpm)

	// Update drums
	e.kick.UpdateLFOTempo(bpm)
	e.snare.UpdateLFOTempo(bpm)
	e.hihatClosed.UpdateLFOTempo(bpm)
	e.hihatOpen.UpdateLFOTempo(bpm)
	e.clap.UpdateLFOTempo(bpm)
	e.rimshot.UpdateLFOTempo(bpm)
	e.crash.UpdateLFOTempo(bpm)
	e.tom.UpdateLFOTempo(bpm)

	// Update drone LFO tempo
	e.drone.UpdateLFOTempo(bpm)
}

// triggerDrumLFOSteps advances all instrument LFOs by one step (for step-locked mode)
func (e *Engine) triggerDrumLFOSteps(stepCount int) {
	for i := 0; i < stepCount; i++ {
	// Advance LFOs for TB-303 synths
		e.tb303.GetLFO1().StepTrigger()
		e.tb303.GetLFO2().StepTrigger()

		e.tb303_2.GetLFO1().StepTrigger()
		e.tb303_2.GetLFO2().StepTrigger()

	// Advance LFOs for all drums
		e.kick.GetLFO1().StepTrigger()
		e.kick.GetLFO2().StepTrigger()

		e.snare.GetLFO1().StepTrigger()
		e.snare.GetLFO2().StepTrigger()

		e.hihatClosed.GetLFO1().StepTrigger()
		e.hihatClosed.GetLFO2().StepTrigger()

		e.hihatOpen.GetLFO1().StepTrigger()
		e.hihatOpen.GetLFO2().StepTrigger()

		e.clap.GetLFO1().StepTrigger()
		e.clap.GetLFO2().StepTrigger()

		e.rimshot.GetLFO1().StepTrigger()
		e.rimshot.GetLFO2().StepTrigger()

		e.crash.GetLFO1().StepTrigger()
		e.crash.GetLFO2().StepTrigger()

		e.tom.GetLFO1().StepTrigger()
		e.tom.GetLFO2().StepTrigger()

	// Advance LFOs for drone synth
		e.drone.GetLFO1().StepTrigger()
		e.drone.GetLFO2().StepTrigger()
	}
}
