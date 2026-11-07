package synth

import (
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/util"
)

// DroneGen is a drone/techno rumble synthesizer with multiple oscillators,
// sub-bass, filter modulation, and long envelopes for sustained pads and rumbles
type DroneGen struct {
	// Audio components
	osc1   *TB303Oscillator // Main oscillator 1
	osc2   *TB303Oscillator // Main oscillator 2 (detuned)
	osc3   *TB303Oscillator // Main oscillator 3 (detuned)
	sub1   *TB303Oscillator // Sub-oscillator (-1 octave)
	sub2   *TB303Oscillator // Sub-oscillator (-2 octaves)
	filter *TB303Filter     // Diode ladder filter
	env    *Envelope        // ADSR envelope (not decay-only)
	lfo1   *dsp.LFO              // LFO1 for modulation
	lfo2   *dsp.LFO              // LFO2 for modulation
	noise  *dsp.NoiseGenerator   // Noise generator
	arp    *Arpeggiator          // Arpeggiator

	// Parameters
	masterVolume float32 // Master volume (0.0-1.0)
	masterTune   int     // Master tune in semitones (-24 to +24)
	osc1Level    float32 // Oscillator 1 level (0.0-1.0)
	osc2Level    float32 // Oscillator 2 level (0.0-1.0)
	osc3Level    float32 // Oscillator 3 level (0.0-1.0)
	osc1PW       float32 // Oscillator 1 pulse width (0.0-1.0)
	osc2PW       float32 // Oscillator 2 pulse width (0.0-1.0)
	osc3PW       float32 // Oscillator 3 pulse width (0.0-1.0)
	osc2Detune   float32 // Oscillator 2 detune in cents (-50 to +50)
	osc3Detune   float32 // Oscillator 3 detune in cents (-50 to +50)
	sub1Level    float32 // Sub1 level (0.0-1.0)
	sub2Level    float32 // Sub2 level (0.0-1.0)
	noiseLevel   float32 // Noise level (0.0-1.0)
	ringModAmount float32 // Ring modulation amount (0.0-1.0)
	driveAmount   float32 // Waveshaper drive amount (0.0-1.0)

	// Unison parameters
	unisonVoices int     // Number of unison voices (1-7, 1=off)
	unisonDetune float32 // Unison detune amount in cents (0-50)
	unisonWidth  float32 // Stereo width (0.0-1.0)

	// Envelope parameters (stored for UI access)
	attack  float32 // Attack time in seconds
	decay   float32 // Decay time in seconds
	sustain float32 // Sustain level (0.0-1.0)
	release float32 // Release time in seconds

	// LFO parameters
	lfo1Target  LFOTarget // What LFO1 modulates
	lfo1Enabled bool      // Is LFO1 enabled
	lfo2Target  LFOTarget // What LFO2 modulates
	lfo2Enabled bool      // Is LFO2 enabled

	// Voice state
	currentNote int     // Current MIDI note
	noteActive  bool    // Is a note currently playing
	velocity    float32 // Note velocity (0.0-1.0)

	// Filter parameters (stored for UI access)
	cutoff    float32
	resonance float32
	envMod    float32

	// Internal
	sampleRate float32
	bpm        float32 // Current BPM for arpeggiator timing
}

// NewDroneGen creates a new drone synthesizer
func NewDroneGen(sampleRate float32, bufferSize int) *DroneGen {
	d := &DroneGen{
		osc1:   NewTB303Oscillator(sampleRate),
		osc2:   NewTB303Oscillator(sampleRate),
		osc3:   NewTB303Oscillator(sampleRate),
		sub1:   NewTB303Oscillator(sampleRate),
		sub2:   NewTB303Oscillator(sampleRate),
		filter: NewTB303Filter(sampleRate),
		env:    NewEnvelope(sampleRate),
		lfo1:   dsp.NewLFO(sampleRate),
		lfo2:   dsp.NewLFO(sampleRate),
		noise:  dsp.NewNoiseGenerator(),
		arp:    NewArpeggiator(),

		// Default parameter values
		masterVolume: 0.8,
		masterTune:   0,
		osc1Level:    0.8,
		osc2Level:    0.7,
		osc3Level:    0.7,
		osc1PW:       0.5, // 50% pulse width (square wave)
		osc2PW:       0.5,
		osc3PW:       0.5,
		osc2Detune:   10.0,  // 10 cents detune
		osc3Detune:   -15.0, // -15 cents detune
		sub1Level:    0.6,
		sub2Level:    0.4,
		noiseLevel:   0.0,   // Off by default
		ringModAmount: 0.0,   // Off by default
		driveAmount:   0.0,   // Off by default

		// Unison defaults
		unisonVoices: 1,   // Off by default (1 voice = no unison)
		unisonDetune: 15.0, // 15 cents default detune
		unisonWidth:  0.75, // 75% stereo width

		// Default envelope values
		attack:  0.01, // 10ms
		decay:   2.0,  // 2s
		sustain: 0.8,  // 80%
		release: 1.0,  // 1s

		lfo1Target:  LFOTargetCutoff,
		lfo1Enabled: true, // Enabled by default for dark rumbles
		lfo2Target:  LFOTargetResonance,
		lfo2Enabled: true, // Enabled by default for dark rumbles

		currentNote: 36, // C2
		noteActive:  false,
		velocity:    1.0,
		sampleRate:  sampleRate,

		// Default filter values (will be set properly below)
		cutoff:    0.5,
		resonance: 0.5,
		envMod:    0.3,
	}

	// Set all oscillators to sawtooth (good for drones)
	d.osc1.SetWaveform(WaveformSaw)
	d.osc2.SetWaveform(WaveformSaw)
	d.osc3.SetWaveform(WaveformSaw)
	d.sub1.SetWaveform(WaveformSine) // Sine for clean sub-bass
	d.sub2.SetWaveform(WaveformSine)

	// Set default envelope (long sustain for drone character)
	d.env.SetADSR(0.01, 2.0, 0.8, 1.0) // 10ms attack, 2s decay, 80% sustain, 1s release

	// Set default filter parameters
	d.filter.SetCutoff(0.5)
	d.filter.SetResonance(0.5)
	d.filter.SetEnvMod(0.3)
	d.filter.SetDecay(1.0) // Longer filter decay for drones

	// Set default LFO1 parameters (slow sweep for evolving pads)
	d.lfo1.SetRate(0.5)         // 0.5 Hz
	d.lfo1.SetWaveform(dsp.LFOSine) // Smooth sine wave
	d.lfo1.SetRange(0.2, 1.0)   // Wide cutoff range for dramatic sweeps
	d.lfo1.SetDepth(0.8)        // 80% depth for obvious modulation
	d.lfo1.SetTempoSync(true)   // Enable tempo sync by default
	d.lfo1.SetSyncMode(dsp.SyncModeSteps) // Step-locked mode
	d.lfo1.SetStepsPerCycle(64) // Complete cycle every 64 steps (4 patterns)

	// Set default LFO2 parameters (faster modulation for dark rumbles)
	d.lfo2.SetRate(2.0)         // 2 Hz (faster than LFO1)
	d.lfo2.SetWaveform(dsp.LFOTriangle) // Triangle wave for different character
	d.lfo2.SetRange(0.3, 0.9)   // Wide resonance range
	d.lfo2.SetDepth(0.7)        // 70% depth for clear modulation
	d.lfo2.SetTempoSync(true)   // Enable tempo sync by default
	d.lfo2.SetSyncMode(dsp.SyncModeSteps) // Step-locked mode
	d.lfo2.SetStepsPerCycle(16) // Complete cycle every 16 steps (1 pattern)

	return d
}

// Process renders a buffer of audio samples
func (d *DroneGen) Process(buffer []float32) {
	bufferSize := len(buffer)

	// Update arpeggiator and handle note events
	if d.arp.GetEnabled() {
		for i := 0; i < bufferSize; i++ {
			note, noteOn, changed := d.arp.Update(d.sampleRate, d.bpm)
			if changed {
				if noteOn && note >= 0 {
					// Trigger new note
					d.NoteOn(note, d.velocity)
				} else if !noteOn {
					// Note off (gate)
					d.NoteOff()
				}
			}
		}
	}

	// Skip if no note is active and envelope is silent
	if !d.noteActive && !d.env.IsActive() {
		return // Mixer already cleared buffer
	}

	// Calculate base frequency
	note := d.currentNote + d.masterTune
	baseFreq := util.MIDIToFrequency(note)

	// Calculate detuned frequencies
	freq1 := baseFreq
	freq2 := baseFreq * detuneCentsToRatio(d.osc2Detune)
	freq3 := baseFreq * detuneCentsToRatio(d.osc3Detune)
	freqSub1 := baseFreq * 0.5  // -1 octave
	freqSub2 := baseFreq * 0.25 // -2 octaves

	// Generate samples
	for i := 0; i < bufferSize; i++ {
		// Process LFOs
		var lfo1Value, lfo2Value float32
		if d.lfo1Enabled {
			lfo1Value = d.lfo1.Process()
		}
		if d.lfo2Enabled {
			lfo2Value = d.lfo2.Process()
		}

		// Apply LFO modulation to filter cutoff
		modCutoff := d.cutoff
		if d.lfo1Enabled && d.lfo1Target == LFOTargetCutoff {
			modCutoff = lfo1Value
		}
		if d.lfo2Enabled && d.lfo2Target == LFOTargetCutoff {
			modCutoff = lfo2Value
		}

		// Apply LFO modulation to filter resonance
		modResonance := d.resonance
		if d.lfo1Enabled && d.lfo1Target == LFOTargetResonance {
			modResonance = lfo1Value
		}
		if d.lfo2Enabled && d.lfo2Target == LFOTargetResonance {
			modResonance = lfo2Value
		}

		// Apply LFO modulation to pitch (vibrato)
		pitchMod := float32(1.0)
		if d.lfo1Enabled && d.lfo1Target == LFOTargetPitch {
			pitchMod = 0.98 + (lfo1Value * 0.04)
		}
		if d.lfo2Enabled && d.lfo2Target == LFOTargetPitch {
			pitchMod *= 0.98 + (lfo2Value * 0.04)
		}

		d.osc1.SetFrequency(freq1 * pitchMod)
		d.osc2.SetFrequency(freq2 * pitchMod)
		d.osc3.SetFrequency(freq3 * pitchMod)
		d.sub1.SetFrequency(freqSub1 * pitchMod)
		d.sub2.SetFrequency(freqSub2 * pitchMod)

		// Apply pulse width modulation
		d.osc1.SetPulseWidth(d.osc1PW)
		d.osc2.SetPulseWidth(d.osc2PW)
		d.osc3.SetPulseWidth(d.osc3PW)

		// Set filter modulation values
		d.filter.SetCutoffModulation(modCutoff)
		d.filter.SetResonanceModulation(modResonance)

		// Generate oscillator samples
		osc1Sample := d.osc1.Process()
		osc2Sample := d.osc2.Process()
		osc3Sample := d.osc3.Process()
		sub1Sample := d.sub1.Process()
		sub2Sample := d.sub2.Process()

		// Generate noise sample
		noiseSample := d.noise.NextWhite()

		// Mix dry oscillators
		dryMix := osc1Sample*d.osc1Level +
			osc2Sample*d.osc2Level +
			osc3Sample*d.osc3Level

		// Apply ring modulation with cross-fade
		var oscMix float32
		if d.ringModAmount > 0.0 {
			// Ring mod: multiply oscillators together for metallic/inharmonic sound
			ringMod1 := osc1Sample * osc2Sample
			ringMod2 := osc1Sample * osc3Sample
			ringMix := (ringMod1 + ringMod2) * 0.5

			// Cross-fade between dry and ring-modulated signal
			// At 0.0: all dry, at 1.0: all ring mod
			oscMix = dryMix*(1.0-d.ringModAmount) + ringMix*d.ringModAmount
		} else {
			oscMix = dryMix
		}

		// Add subs and noise (not affected by ring mod)
		mix := oscMix +
			sub1Sample*d.sub1Level +
			sub2Sample*d.sub2Level +
			noiseSample*d.noiseLevel

		// Scale down to prevent clipping
		mix *= 0.2

		// Apply waveshaping/distortion if enabled
		if d.driveAmount > 0.0 {
			// Soft clipping waveshaper with adjustable drive
			drive := 1.0 + d.driveAmount*4.0 // 1.0 to 5.0
			mix = waveshape(mix*drive) * 1.5 // Makeup gain to compensate for compression
		}

		// Get envelope value
		envValue := d.env.Process()

		// Process filter with envelope modulation
		filtered := d.filter.Process(mix)

		// Apply volume LFO if enabled
		volumeMod := float32(1.0)
		if d.lfo1Enabled && d.lfo1Target == LFOTargetVolume {
			volumeMod *= lfo1Value
		}
		if d.lfo2Enabled && d.lfo2Target == LFOTargetVolume {
			volumeMod *= lfo2Value
		}

		// Apply envelope, velocity, and master volume
		buffer[i] = filtered * envValue * d.masterVolume * d.velocity * volumeMod
	}
}

// NoteOn triggers a note
func (d *DroneGen) NoteOn(note int, velocity float32) {
	d.currentNote = note
	d.velocity = velocity
	d.noteActive = true

	// Trigger envelope
	d.env.Trigger()

	// Retrigger filter envelope
	d.filter.Trigger(false) // false = no accent
}

// NoteOff releases the note
func (d *DroneGen) NoteOff() {
	d.env.Release()
}

// detuneCentsToRatio converts cents to frequency ratio
// 100 cents = 1 semitone = 2^(1/12) ratio
func detuneCentsToRatio(cents float32) float32 {
	// 2^(cents/1200) - use util.Pow2 if available, or inline calculation
	exponent := cents / 1200.0
	// Simple approximation: 2^x ≈ 1 + x*ln(2) for small x
	// For better accuracy with larger detune, we should use a proper pow2 implementation
	// For now, use a linear approximation which is close enough for small detune values
	if exponent < 0.1 && exponent > -0.1 {
		return 1.0 + exponent*0.693147 // ln(2) ≈ 0.693147
	}
	// For larger values, use repeated squaring approximation
	result := float32(1.0)
	x := exponent
	if x < 0 {
		x = -x
		for x >= 1.0 {
			result *= 2.0
			x -= 1.0
		}
		// Handle fractional part
		result *= (1.0 + x*0.693147)
		return 1.0 / result
	}
	for x >= 1.0 {
		result *= 2.0
		x -= 1.0
	}
	result *= (1.0 + x*0.693147)
	return result
}

// waveshape applies soft clipping distortion for dark, aggressive tones
func waveshape(x float32) float32 {
	// Soft clipping using tanh approximation
	// Fast approximation of tanh for waveshaping
	if x > 3.0 {
		return 1.0
	} else if x < -3.0 {
		return -1.0
	}
	// tanh approximation: x / (1 + |x|)
	absX := x
	if absX < 0 {
		absX = -absX
	}
	return x / (1.0 + absX)
}

// Getter methods

func (d *DroneGen) GetMasterVolume() float32 {
	return d.masterVolume
}

func (d *DroneGen) GetMasterTune() int {
	return d.masterTune
}

func (d *DroneGen) GetOsc1Level() float32 {
	return d.osc1Level
}

func (d *DroneGen) GetOsc2Level() float32 {
	return d.osc2Level
}

func (d *DroneGen) GetOsc3Level() float32 {
	return d.osc3Level
}

func (d *DroneGen) GetOsc2Detune() float32 {
	return d.osc2Detune
}

func (d *DroneGen) GetOsc3Detune() float32 {
	return d.osc3Detune
}

func (d *DroneGen) GetSub1Level() float32 {
	return d.sub1Level
}

func (d *DroneGen) GetSub2Level() float32 {
	return d.sub2Level
}

func (d *DroneGen) GetNoiseLevel() float32 {
	return d.noiseLevel
}

func (d *DroneGen) GetRingModAmount() float32 {
	return d.ringModAmount
}

func (d *DroneGen) GetDriveAmount() float32 {
	return d.driveAmount
}

func (d *DroneGen) GetCutoff() float32 {
	return d.cutoff
}

func (d *DroneGen) GetResonance() float32 {
	return d.resonance
}

func (d *DroneGen) GetEnvMod() float32 {
	return d.envMod
}

func (d *DroneGen) GetAttack() float32 {
	return d.attack
}

func (d *DroneGen) GetDecay() float32 {
	return d.decay
}

func (d *DroneGen) GetSustain() float32 {
	return d.sustain
}

func (d *DroneGen) GetRelease() float32 {
	return d.release
}

func (d *DroneGen) GetLFO1Enabled() bool {
	return d.lfo1Enabled
}

func (d *DroneGen) GetLFO1Target() LFOTarget {
	return d.lfo1Target
}

func (d *DroneGen) GetLFO1Waveform() dsp.LFOWaveform {
	return d.lfo1.GetWaveform()
}

func (d *DroneGen) GetLFO1Rate() float32 {
	return d.lfo1.GetRate()
}

func (d *DroneGen) GetLFO1Depth() float32 {
	return d.lfo1.GetDepth()
}

func (d *DroneGen) GetLFO1Min() float32 {
	return d.lfo1.GetMin()
}

func (d *DroneGen) GetLFO1Max() float32 {
	return d.lfo1.GetMax()
}

func (d *DroneGen) GetLFO1() *dsp.LFO {
	return d.lfo1
}

func (d *DroneGen) GetLFO2Enabled() bool {
	return d.lfo2Enabled
}

func (d *DroneGen) GetLFO2Target() LFOTarget {
	return d.lfo2Target
}

func (d *DroneGen) GetLFO2Waveform() dsp.LFOWaveform {
	return d.lfo2.GetWaveform()
}

func (d *DroneGen) GetLFO2Rate() float32 {
	return d.lfo2.GetRate()
}

func (d *DroneGen) GetLFO2Depth() float32 {
	return d.lfo2.GetDepth()
}

func (d *DroneGen) GetLFO2Min() float32 {
	return d.lfo2.GetMin()
}

func (d *DroneGen) GetLFO2Max() float32 {
	return d.lfo2.GetMax()
}

func (d *DroneGen) GetLFO2() *dsp.LFO {
	return d.lfo2
}

// Setter methods

func (d *DroneGen) SetMasterVolume(vol float32) {
	d.masterVolume = util.Clamp(vol, 0.0, 1.0)
}

// SetBPM sets the current BPM for arpeggiator timing
func (d *DroneGen) SetBPM(bpm float32) {
	d.bpm = bpm
}

// UpdateLFOTempo updates the LFO tempo rates based on current BPM
func (d *DroneGen) UpdateLFOTempo(bpm float32) {
	d.lfo1.UpdateTempoRate(bpm)
	d.lfo2.UpdateTempoRate(bpm)
}

func (d *DroneGen) SetMasterTune(semitones int) {
	if semitones < -24 {
		semitones = -24
	} else if semitones > 24 {
		semitones = 24
	}
	d.masterTune = semitones
}

func (d *DroneGen) SetOsc1Level(level float32) {
	d.osc1Level = util.Clamp(level, 0.0, 1.0)
}

func (d *DroneGen) SetOsc2Level(level float32) {
	d.osc2Level = util.Clamp(level, 0.0, 1.0)
}

func (d *DroneGen) SetOsc3Level(level float32) {
	d.osc3Level = util.Clamp(level, 0.0, 1.0)
}

func (d *DroneGen) SetOsc2Detune(cents float32) {
	d.osc2Detune = util.Clamp(cents, -50.0, 50.0)
}

func (d *DroneGen) SetOsc3Detune(cents float32) {
	d.osc3Detune = util.Clamp(cents, -50.0, 50.0)
}

func (d *DroneGen) SetSub1Level(level float32) {
	d.sub1Level = util.Clamp(level, 0.0, 1.0)
}

func (d *DroneGen) SetSub2Level(level float32) {
	d.sub2Level = util.Clamp(level, 0.0, 1.0)
}

func (d *DroneGen) SetNoiseLevel(level float32) {
	d.noiseLevel = util.Clamp(level, 0.0, 1.0)
}

func (d *DroneGen) SetRingModAmount(amount float32) {
	d.ringModAmount = util.Clamp(amount, 0.0, 1.0)
}

func (d *DroneGen) SetDriveAmount(amount float32) {
	d.driveAmount = util.Clamp(amount, 0.0, 1.0)
}

func (d *DroneGen) SetCutoff(cutoff float32) {
	d.cutoff = cutoff
	d.filter.SetCutoff(cutoff)
}

func (d *DroneGen) SetResonance(res float32) {
	d.resonance = res
	d.filter.SetResonance(res)
}

func (d *DroneGen) SetEnvMod(mod float32) {
	d.envMod = mod
	d.filter.SetEnvMod(mod)
}

func (d *DroneGen) SetAttack(attack float32) {
	d.attack = util.Clamp(attack, 0.001, 2.0)
	d.env.SetAttack(d.attack)
}

func (d *DroneGen) SetDecay(decay float32) {
	d.decay = util.Clamp(decay, 0.1, 10.0)
	d.env.SetDecay(d.decay)
}

func (d *DroneGen) SetSustain(sustain float32) {
	d.sustain = util.Clamp(sustain, 0.0, 1.0)
	d.env.SetSustain(d.sustain)
}

func (d *DroneGen) SetRelease(release float32) {
	d.release = util.Clamp(release, 0.1, 5.0)
	d.env.SetRelease(d.release)
}

func (d *DroneGen) SetLFO1Enabled(enabled bool) {
	d.lfo1Enabled = enabled
}

func (d *DroneGen) SetLFO1Target(target LFOTarget) {
	d.lfo1Target = target
}

func (d *DroneGen) SetLFO1Waveform(wave dsp.LFOWaveform) {
	d.lfo1.SetWaveform(wave)
}

func (d *DroneGen) SetLFO1Rate(rate float32) {
	d.lfo1.SetRate(util.Clamp(rate, 0.01, 10.0))
}

func (d *DroneGen) SetLFO1Depth(depth float32) {
	d.lfo1.SetDepth(util.Clamp(depth, 0.0, 1.0))
}

func (d *DroneGen) SetLFO1Range(min, max float32) {
	d.lfo1.SetRange(min, max)
}

func (d *DroneGen) SetLFO2Enabled(enabled bool) {
	d.lfo2Enabled = enabled
}

func (d *DroneGen) SetLFO2Target(target LFOTarget) {
	d.lfo2Target = target
}

func (d *DroneGen) SetLFO2Waveform(wave dsp.LFOWaveform) {
	d.lfo2.SetWaveform(wave)
}

func (d *DroneGen) SetLFO2Rate(rate float32) {
	d.lfo2.SetRate(util.Clamp(rate, 0.01, 10.0))
}

func (d *DroneGen) SetLFO2Depth(depth float32) {
	d.lfo2.SetDepth(util.Clamp(depth, 0.0, 1.0))
}

func (d *DroneGen) SetLFO2Range(min, max float32) {
	d.lfo2.SetRange(min, max)
}

// Pulse Width getters/setters

func (d *DroneGen) GetOsc1PW() float32 {
	return d.osc1PW
}

func (d *DroneGen) GetOsc2PW() float32 {
	return d.osc2PW
}

func (d *DroneGen) GetOsc3PW() float32 {
	return d.osc3PW
}

func (d *DroneGen) SetOsc1PW(pw float32) {
	d.osc1PW = util.Clamp(pw, 0.01, 0.99)
}

func (d *DroneGen) SetOsc2PW(pw float32) {
	d.osc2PW = util.Clamp(pw, 0.01, 0.99)
}

func (d *DroneGen) SetOsc3PW(pw float32) {
	d.osc3PW = util.Clamp(pw, 0.01, 0.99)
}

// Unison getters/setters

func (d *DroneGen) GetUnisonVoices() int {
	return d.unisonVoices
}

func (d *DroneGen) GetUnisonDetune() float32 {
	return d.unisonDetune
}

func (d *DroneGen) GetUnisonWidth() float32 {
	return d.unisonWidth
}

func (d *DroneGen) SetUnisonVoices(voices int) {
	if voices < 1 {
		voices = 1
	} else if voices > 7 {
		voices = 7
	}
	d.unisonVoices = voices
}

func (d *DroneGen) SetUnisonDetune(detune float32) {
	d.unisonDetune = util.Clamp(detune, 0.0, 50.0)
}

func (d *DroneGen) SetUnisonWidth(width float32) {
	d.unisonWidth = util.Clamp(width, 0.0, 1.0)
}

// GetArpeggiator returns the arpeggiator instance
func (d *DroneGen) GetArpeggiator() *Arpeggiator {
	return d.arp
}

// GetWaveform returns the waveform for a specific oscillator
func (d *DroneGen) GetOsc1Waveform() Waveform {
	return d.osc1.waveform
}

func (d *DroneGen) GetOsc2Waveform() Waveform {
	return d.osc2.waveform
}

func (d *DroneGen) GetOsc3Waveform() Waveform {
	return d.osc3.waveform
}

// SetWaveform sets the waveform for a specific oscillator
func (d *DroneGen) SetOsc1Waveform(wf Waveform) {
	d.osc1.SetWaveform(wf)
}

func (d *DroneGen) SetOsc2Waveform(wf Waveform) {
	d.osc2.SetWaveform(wf)
}

func (d *DroneGen) SetOsc3Waveform(wf Waveform) {
	d.osc3.SetWaveform(wf)
}
