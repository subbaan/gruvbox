package synth

import (
	"math"

	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/util"
)

// LFOTarget represents what parameter the LFO modulates
type LFOTarget int

const (
	LFOTargetNone LFOTarget = iota
	LFOTargetCutoff
	LFOTargetResonance
	LFOTargetPitch
	LFOTargetEnvMod // Filter envelope modulation depth
	LFOTargetPulseWidth // Pulse width for square wave
	LFOTargetVolume // Overall volume/tremolo
)

// TB303 is a complete TB-303 bass synthesizer voice
type TB303 struct {
	// Audio components
	oscillator *TB303Oscillator
	filter     *TB303Filter
	ampEnv     *DecayEnvelope
	lfo1       *dsp.LFO
	lfo2       *dsp.LFO

	// Voice parameters
	waveform   Waveform // Saw or Square
	tune       int      // Tuning in semitones (-12 to +12)
	accentAmt  float32  // Accent amount (0.0 to 1.0)
	slideTime  float32  // Slide time in seconds
	subLevel   float32  // Sub-oscillator level (0.0 to 1.0)

	// LFO parameters
	lfo1Target  LFOTarget // What LFO1 modulates
	lfo1Enabled bool      // Is LFO1 enabled
	lfo2Target  LFOTarget // What LFO2 modulates
	lfo2Enabled bool      // Is LFO2 enabled

	// Voice state
	currentNote int     // Current MIDI note
	targetNote  int     // Target note (for slide)
	noteActive  bool    // Is a note currently playing
	accent      bool    // Is current note accented
	sliding     bool    // Is currently sliding to target note
	slidePhase  float32 // Slide interpolation (0.0 to 1.0)

	// Slide state
	slideStartFreq  float32
	slideTargetFreq float32

	// Sub-oscillator state
	subPhase float32 // Phase for sub-oscillator sine wave (0.0 to 1.0)

	// Character controls (alpha.36). Their defaults leave the voice as it
	// always sounded.
	drive, driveGain, driveNorm float32 // pre-filter saturation
	postGain, postNorm          float32 // post-filter saturation
	gateCounter                 int     // samples until the note releases; 0 = ring
	releasing                   bool
	releaseGain, releaseCoeff   float32
	blend, noise                float32 // saw/square mix, white noise level
	noiseState                  uint32

	// Internal
	sampleRate float32

	// Pre-allocated buffer for processing
	oscBuffer []float32
}

// NewTB303 creates a new TB-303 synthesizer
func NewTB303(sampleRate float32, bufferSize int) *TB303 {
	tb := &TB303{
		oscillator:  NewTB303Oscillator(sampleRate),
		filter:      NewTB303Filter(sampleRate),
		ampEnv:      NewDecayEnvelope(sampleRate),
		lfo1:        dsp.NewLFO(sampleRate),
		lfo2:        dsp.NewLFO(sampleRate),
		waveform:    WaveformSaw,
		tune:        0,
		accentAmt:   0.5,
		slideTime:   0.06, // 60ms default slide
		subLevel:    0.3,  // 30% sub-oscillator level by default
		lfo1Target:  LFOTargetCutoff,
		lfo1Enabled: false,
		lfo2Target:  LFOTargetResonance,
		lfo2Enabled: false,
		currentNote: 36, // C2
		targetNote:  36,
		noteActive:  false,
		accent:      false,
		sliding:     false,
		slidePhase:  1.0,
		subPhase:    0.0,
		sampleRate:  sampleRate,
		oscBuffer:   make([]float32, bufferSize),
		releaseGain: 1,
		noiseState:  0x9e3779b9,
	}
	tb.releaseCoeff = float32(math.Exp(-1 / (0.004 * float64(sampleRate)))) // ~4 ms release

	// Set default envelope times
	tb.ampEnv.SetDecayTime(0.3) // 300ms amplitude decay
	tb.filter.SetDecay(0.2)     // 200ms filter decay

	// Set default filter parameters
	tb.filter.SetCutoff(0.65)
	tb.filter.SetResonance(0.4)
	tb.filter.SetEnvMod(0.75)

	// Configure LFO1 defaults (cutoff modulation)
	tb.lfo1.SetWaveform(dsp.LFOSine)
	tb.lfo1.SetDepth(0.5)         // 50% depth
	tb.lfo1.SetRange(0.3, 0.9)    // Modulate cutoff from 30% to 90%
	tb.lfo1.SetRetrigger(false)   // Free-running LFO
	// tempoSync defaults to true in NewLFO, syncRate 0.25 (1/4 note)

	// Configure LFO2 defaults (resonance modulation)
	tb.lfo2.SetWaveform(dsp.LFOTriangle)
	tb.lfo2.SetDepth(0.3)         // 30% depth
	tb.lfo2.SetRange(0.2, 0.8)    // Modulate resonance from 20% to 80%
	tb.lfo2.SetRetrigger(false)   // Free-running LFO
	// tempoSync defaults to true in NewLFO, syncRate 0.25 (1/4 note)

	return tb
}

// SetWaveform sets the oscillator waveform (Saw or Square)
func (tb *TB303) SetWaveform(waveform Waveform) {
	tb.waveform = waveform
	tb.oscillator.SetWaveform(waveform)
}

// SetTune sets the tuning in semitones (-12 to +12)
func (tb *TB303) SetTune(semitones int) {
	if semitones < -12 {
		semitones = -12
	}
	if semitones > 12 {
		semitones = 12
	}
	tb.tune = semitones
}

// SetCutoff sets the filter cutoff (0.0 to 1.0)
func (tb *TB303) SetCutoff(cutoff float32) {
	tb.filter.SetCutoff(cutoff)
}

// SetResonance sets the filter resonance (0.0 to 1.0)
func (tb *TB303) SetResonance(resonance float32) {
	tb.filter.SetResonance(resonance)
}

// SetEnvMod sets the filter envelope modulation (0.0 to 1.0)
func (tb *TB303) SetEnvMod(envMod float32) {
	tb.filter.SetEnvMod(envMod)
}

// SetDecay sets the envelope decay time in seconds
func (tb *TB303) SetDecay(decay float32) {
	// Decay comes in normalized (0-1), scale to seconds (0-2)
	decaySeconds := decay * 2.0
	tb.filter.SetDecay(decaySeconds)
	tb.ampEnv.SetDecayTime(decaySeconds * 1.5) // Amp decay slightly longer
}

// SetAccent sets the accent amount (0.0 to 1.0)
func (tb *TB303) SetAccent(accent float32) {
	tb.accentAmt = util.Clamp(accent, 0.0, 1.0)
}

// SetSlideTime sets the slide time in seconds
func (tb *TB303) SetSlideTime(time float32) {
	tb.slideTime = util.Clamp(time, 0.01, 0.5)
}

// SetSubLevel sets the sub-oscillator level (0.0 to 1.0)
func (tb *TB303) SetSubLevel(level float32) {
	tb.subLevel = util.Clamp(level, 0.0, 1.0)
}

// SetDrive saturates the oscillator into the filter (0 = clean). The
// saturation is normalised so peaks stay put; the filter is driven harder,
// which squashes and roughens the resonance.
func (tb *TB303) SetDrive(drive float32) {
	tb.drive = util.Clamp(drive, 0, 1)
	tb.driveGain = 1 + tb.drive*tb.drive*12
	tb.driveNorm = 1 / util.FastTanh(tb.driveGain)
	tb.postGain = 1 + tb.drive*6
	// Quiet signals pass at about unity; loud ones flatten towards 1/postNorm.
	tb.postNorm = 1 / tb.postGain * (1 + tb.drive*.8)
}

// SetAccentSweep sets how much consecutive accents build up the cutoff
// (0 = off, the classic behaviour of this voice).
func (tb *TB303) SetAccentSweep(amount float32) { tb.filter.SetAccentSweep(amount) }

// SetBlend mixes in the other waveform: 0 is the selected wave alone, 1 an
// equal saw/square mix.
func (tb *TB303) SetBlend(amount float32) { tb.blend = util.Clamp(amount, 0, 1) }

// SetNoise adds white noise before the filter (0 = none).
func (tb *TB303) SetNoise(level float32) { tb.noise = util.Clamp(level, 0, 1) }

// SetFilterMode picks low-pass (0), band-pass (1) or high-pass (2).
func (tb *TB303) SetFilterMode(mode int) { tb.filter.SetMode(mode) }

// SetPulseWidth sets the square wave's duty cycle (0.1–0.9).
func (tb *TB303) SetPulseWidth(width float32) {
	tb.oscillator.SetPulseWidth(util.Clamp(width, .1, .9))
}

// NoteOn triggers a note that rings until its envelope decays.
// note: MIDI note number (0-127)
// accent: whether this note is accented
// slide: whether to slide from previous note
func (tb *TB303) NoteOn(note int, accent bool, slide bool) {
	tb.NoteOnFor(note, accent, slide, 0)
}

// NoteOnFor triggers a note that releases quickly after gateSamples, like a
// real 303's gate; gateSamples <= 0 lets it ring.
func (tb *TB303) NoteOnFor(note int, accent bool, slide bool, gateSamples int) {
	tb.gateCounter = max(0, gateSamples)
	tb.releasing = false
	tb.releaseGain = 1
	tb.noteOn(note, accent, slide)
}

func (tb *TB303) noteOn(note int, accent bool, slide bool) {
	tb.targetNote = note
	tb.accent = accent

	if slide && tb.noteActive {
		// Slide from current note to target note
		tb.sliding = true
		tb.slidePhase = 0.0
		tb.slideStartFreq = util.MIDIToFrequency(tb.currentNote + tb.tune)
		tb.slideTargetFreq = util.MIDIToFrequency(tb.targetNote + tb.tune)

		// Retrigger envelopes on slide for more pronounced effect
		tb.filter.Trigger(accent)
		tb.ampEnv.Trigger()
		if accent {
			tb.ampEnv.SetValue(1.5)
		} else {
			tb.ampEnv.SetValue(1.0)
		}
	} else {
		// Normal note (no slide or first note)
		tb.currentNote = note
		tb.sliding = false
		tb.slidePhase = 1.0

		// Set oscillator frequency
		tb.oscillator.SetNoteTuned(tb.currentNote, float32(tb.tune))

		// Trigger envelopes
		tb.filter.Trigger(accent)
		tb.ampEnv.Trigger()

		// Accent affects amplitude envelope
		if accent {
			tb.ampEnv.SetValue(1.5) // Boost initial level
		} else {
			tb.ampEnv.SetValue(1.0)
		}
	}

	tb.noteActive = true
}

// NoteOff releases a note (TB-303 doesn't have traditional note off - decay only)
func (tb *TB303) NoteOff() {
	// TB-303 doesn't really have note off - it just lets the envelopes decay
	// We'll keep this for compatibility but it doesn't do much
	tb.noteActive = false
}

// Reset resets the synthesizer state
func (tb *TB303) Reset() {
	tb.gateCounter, tb.releasing, tb.releaseGain = 0, false, 1
	tb.oscillator.Reset()
	tb.filter.Reset()
	tb.ampEnv.Reset()
	tb.noteActive = false
	tb.sliding = false
}

// Process generates a buffer of audio samples
func (tb *TB303) Process(output []float32) {
	bufferSize := len(output)

	// If silent, don't touch the buffer - just return
	// (allows other sources like test tone to use this channel)
	if !tb.noteActive && !tb.ampEnv.IsActive() {
		return
	}

	// Not silent - generate our signal
	// (mixer already cleared buffer before calling us)

	// Calculate slide increment per sample
	slideIncrement := float32(0.0)
	if tb.sliding && tb.slideTime > 0 {
		samplesForSlide := tb.slideTime * tb.sampleRate
		slideIncrement = 1.0 / samplesForSlide
	}

	// Generate samples
	for i := 0; i < bufferSize; i++ {
		// Process both LFOs
		var lfo1Value, lfo2Value float32
		if tb.lfo1Enabled {
			lfo1Value = tb.lfo1.Process()
		}
		if tb.lfo2Enabled {
			lfo2Value = tb.lfo2.Process()
		}

		// Update slide
		if tb.sliding {
			tb.slidePhase += slideIncrement
			if tb.slidePhase >= 1.0 {
				tb.slidePhase = 1.0
				tb.sliding = false
				tb.currentNote = tb.targetNote
			}

			// Exponential slide curve (sounds more natural)
			t := tb.slidePhase
			t = t * t * (3.0 - 2.0*t) // Smoothstep

			// Interpolate frequency
			freq := tb.slideStartFreq + (tb.slideTargetFreq-tb.slideStartFreq)*t

			// Apply pitch LFO modulation if enabled
			if tb.lfo1Enabled && tb.lfo1Target == LFOTargetPitch {
				freq *= lfo1Value
			}
			if tb.lfo2Enabled && tb.lfo2Target == LFOTargetPitch {
				freq *= lfo2Value
			}

			tb.oscillator.SetFrequency(freq)
		}

		// Start with base filter values
		cutoffMod := tb.filter.cutoff
		resonanceMod := tb.filter.resonance

		// Apply LFO1 modulation
		if tb.lfo1Enabled {
			switch tb.lfo1Target {
			case LFOTargetCutoff:
				cutoffMod = lfo1Value
			case LFOTargetResonance:
				resonanceMod = lfo1Value
			case LFOTargetEnvMod:
				tb.filter.SetEnvModModulation(lfo1Value)
			case LFOTargetPulseWidth:
				if tb.waveform == WaveformSquare {
					tb.oscillator.SetPulseWidth(lfo1Value)
				}
			}
		}

		// Apply LFO2 modulation
		if tb.lfo2Enabled {
			switch tb.lfo2Target {
			case LFOTargetCutoff:
				cutoffMod = lfo2Value
			case LFOTargetResonance:
				resonanceMod = lfo2Value
			case LFOTargetEnvMod:
				tb.filter.SetEnvModModulation(lfo2Value)
			case LFOTargetPulseWidth:
				if tb.waveform == WaveformSquare {
					tb.oscillator.SetPulseWidth(lfo2Value)
				}
			}
		}

		// Set final filter modulation values
		tb.filter.SetCutoffModulation(cutoffMod)
		tb.filter.SetResonanceModulation(resonanceMod)

		// Generate oscillator sample
		var oscSample float32
		if tb.blend > 0 {
			oscSample = tb.oscillator.ProcessBlend(tb.blend)
		} else {
			oscSample = tb.oscillator.Process()
		}
		if tb.noise > 0 {
			// xorshift32 white noise
			tb.noiseState ^= tb.noiseState << 13
			tb.noiseState ^= tb.noiseState >> 17
			tb.noiseState ^= tb.noiseState << 5
			oscSample += (float32(tb.noiseState)/float32(1<<31) - 1) * tb.noise * .5
		}

		// Generate sub-oscillator (sine wave one octave below)
		if tb.subLevel > 0.0 {
			// Get current frequency from oscillator
			currentFreq := tb.oscillator.frequency

			// Sub-oscillator is one octave down (half frequency)
			subFreq := currentFreq * 0.5

			// Generate sine wave
			subSample := util.FastSine(tb.subPhase)

			// Advance sub-oscillator phase
			tb.subPhase += subFreq / tb.sampleRate
			if tb.subPhase >= 1.0 {
				tb.subPhase -= 1.0
			}

			// Mix sub-oscillator with main oscillator
			oscSample = oscSample*(1.0-tb.subLevel*0.5) + subSample*tb.subLevel
		}

		// Drive: normalised saturation, then a hotter signal into the
		// ladder, scaled back afterwards.
		var filtered float32
		if tb.drive > 0 {
			oscSample = util.FastTanh(oscSample*tb.driveGain) * tb.driveNorm
			hot := 1 + tb.drive*1.5
			filtered = tb.filter.Process(oscSample*hot) / hot
			// After the filter the resonance peak is what clips, which is
			// where a driven 303 gets its scream.
			filtered = util.FastTanh(filtered*tb.postGain) * tb.postNorm
		} else {
			filtered = tb.filter.Process(oscSample)
		}

		// Apply amplitude envelope
		ampEnvValue := tb.ampEnv.Process()

		// Gate: once the note's time is up it releases in a few ms.
		if tb.releasing {
			tb.releaseGain *= tb.releaseCoeff
			if tb.releaseGain < 1e-4 {
				tb.releasing = false
				tb.releaseGain = 1
				tb.ampEnv.Reset()
				ampEnvValue = 0
			}
			ampEnvValue *= tb.releaseGain
		}
		if tb.gateCounter > 0 {
			tb.gateCounter--
			if tb.gateCounter == 0 {
				tb.releasing = true
			}
		}

		// Apply accent to amplitude
		if tb.accent {
			ampEnvValue *= (1.0 + tb.accentAmt)
		}

		// Apply volume/tremolo LFO modulation if enabled
		volumeMod := float32(1.0)
		if tb.lfo1Enabled && tb.lfo1Target == LFOTargetVolume {
			volumeMod *= lfo1Value
		}
		if tb.lfo2Enabled && tb.lfo2Target == LFOTargetVolume {
			volumeMod *= lfo2Value
		}

		// Final output
		output[i] = filtered * ampEnvValue * volumeMod * 0.5 // Scale to prevent clipping
	}
}

// IsActive returns true if the voice is producing sound
func (tb *TB303) IsActive() bool {
	return tb.ampEnv.IsActive() || tb.filter.IsEnvelopeActive()
}

// GetCurrentNote returns the current MIDI note
func (tb *TB303) GetCurrentNote() int {
	return tb.currentNote
}

// GetFilterCutoff returns the current filter cutoff with modulation
func (tb *TB303) GetFilterCutoff() float32 {
	return tb.filter.GetCurrentCutoff()
}

// Getter methods for UI

// GetCutoff returns the filter cutoff setting
func (tb *TB303) GetCutoff() float32 {
	return tb.filter.cutoff
}

// GetResonance returns the filter resonance setting
func (tb *TB303) GetResonance() float32 {
	return tb.filter.resonance
}

// GetEnvMod returns the envelope modulation amount
func (tb *TB303) GetEnvMod() float32 {
	return tb.filter.envMod
}

// GetDecay returns the decay time (normalized 0-1)
func (tb *TB303) GetDecay() float32 {
	// Return normalized decay time (0-1)
	return tb.filter.GetDecay() / 2.0 // Assuming max 2 seconds
}

// GetAccent returns the accent amount
func (tb *TB303) GetAccent() float32 {
	return tb.accentAmt
}

// GetSlideTime returns the slide time (normalized 0-1)
func (tb *TB303) GetSlideTime() float32 {
	// Normalize from 0.01-0.5s to 0-1
	return (tb.slideTime - 0.01) / 0.49
}

// GetSubLevel returns the sub-oscillator level (0-1)
func (tb *TB303) GetSubLevel() float32 {
	return tb.subLevel
}

// GetWaveform returns the current waveform
func (tb *TB303) GetWaveform() Waveform {
	return tb.waveform
}

// GetTune returns the current tuning in semitones
func (tb *TB303) GetTune() int {
	return tb.tune
}

// GetCurrentCutoff returns the current LFO-modulated cutoff value
func (tb *TB303) GetCurrentCutoff() float32 {
	// Check if either LFO is modulating cutoff
	if tb.lfo1Enabled && tb.lfo1Target == LFOTargetCutoff {
		return util.Clamp(tb.lfo1.GetValue(), 0.0, 1.0)
	}
	if tb.lfo2Enabled && tb.lfo2Target == LFOTargetCutoff {
		return util.Clamp(tb.lfo2.GetValue(), 0.0, 1.0)
	}
	return tb.filter.cutoff
}

// GetCurrentResonance returns the current LFO-modulated resonance value
func (tb *TB303) GetCurrentResonance() float32 {
	// Check if either LFO is modulating resonance
	if tb.lfo1Enabled && tb.lfo1Target == LFOTargetResonance {
		return util.Clamp(tb.lfo1.GetValue(), 0.0, 1.0)
	}
	if tb.lfo2Enabled && tb.lfo2Target == LFOTargetResonance {
		return util.Clamp(tb.lfo2.GetValue(), 0.0, 1.0)
	}
	return tb.filter.resonance
}

// GetCurrentEnvMod returns the current LFO-modulated envelope modulation value
func (tb *TB303) GetCurrentEnvMod() float32 {
	// Check if either LFO is modulating env mod
	if tb.lfo1Enabled && tb.lfo1Target == LFOTargetEnvMod {
		return util.Clamp(tb.lfo1.GetValue(), 0.0, 1.0)
	}
	if tb.lfo2Enabled && tb.lfo2Target == LFOTargetEnvMod {
		return util.Clamp(tb.lfo2.GetValue(), 0.0, 1.0)
	}
	return tb.filter.envMod
}

// GetCurrentPulseWidth returns the current LFO-modulated pulse width value
func (tb *TB303) GetCurrentPulseWidth() float32 {
	// Check if either LFO is modulating pulse width
	if tb.lfo1Enabled && tb.lfo1Target == LFOTargetPulseWidth {
		return util.Clamp(tb.lfo1.GetValue(), 0.0, 1.0)
	}
	if tb.lfo2Enabled && tb.lfo2Target == LFOTargetPulseWidth {
		return util.Clamp(tb.lfo2.GetValue(), 0.0, 1.0)
	}
	// Return base pulse width from oscillator
	return tb.oscillator.GetPulseWidth()
}

// LFO control methods

// GetLFO1 returns a pointer to LFO1 for direct access
func (tb *TB303) GetLFO1() *dsp.LFO {
	return tb.lfo1
}

// GetLFO2 returns a pointer to LFO2 for direct access
func (tb *TB303) GetLFO2() *dsp.LFO {
	return tb.lfo2
}

// SetLFO1Enabled enables or disables LFO1 modulation
func (tb *TB303) SetLFO1Enabled(enabled bool) {
	tb.lfo1Enabled = enabled
}

// GetLFO1Enabled returns whether LFO1 is enabled
func (tb *TB303) GetLFO1Enabled() bool {
	return tb.lfo1Enabled
}

// SetLFO2Enabled enables or disables LFO2 modulation
func (tb *TB303) SetLFO2Enabled(enabled bool) {
	tb.lfo2Enabled = enabled
}

// GetLFO2Enabled returns whether LFO2 is enabled
func (tb *TB303) GetLFO2Enabled() bool {
	return tb.lfo2Enabled
}

// SetLFO1Target sets what parameter LFO1 modulates
func (tb *TB303) SetLFO1Target(target LFOTarget) {
	tb.lfo1Target = target

	// Update LFO range based on target
	switch target {
	case LFOTargetCutoff:
		tb.lfo1.SetRange(0.3, 0.9) // Cutoff range 30-90%
	case LFOTargetResonance:
		tb.lfo1.SetRange(0.2, 0.8) // Resonance range 20-80%
	case LFOTargetPitch:
		tb.lfo1.SetRange(0.98, 1.02) // Pitch ±2% for vibrato
	case LFOTargetEnvMod:
		tb.lfo1.SetRange(0.3, 1.0) // Env mod 30-100%
	case LFOTargetPulseWidth:
		tb.lfo1.SetRange(0.1, 0.9) // Pulse width 10-90%
	case LFOTargetVolume:
		tb.lfo1.SetRange(0.5, 1.0) // Volume 50-100% for tremolo
	}
}

// GetLFO1Target returns the current LFO1 modulation target
func (tb *TB303) GetLFO1Target() LFOTarget {
	return tb.lfo1Target
}

// SetLFO2Target sets what parameter LFO2 modulates
func (tb *TB303) SetLFO2Target(target LFOTarget) {
	tb.lfo2Target = target

	// Update LFO range based on target
	switch target {
	case LFOTargetCutoff:
		tb.lfo2.SetRange(0.3, 0.9) // Cutoff range 30-90%
	case LFOTargetResonance:
		tb.lfo2.SetRange(0.2, 0.8) // Resonance range 20-80%
	case LFOTargetPitch:
		tb.lfo2.SetRange(0.98, 1.02) // Pitch ±2% for vibrato
	case LFOTargetEnvMod:
		tb.lfo2.SetRange(0.3, 1.0) // Env mod 30-100%
	case LFOTargetPulseWidth:
		tb.lfo2.SetRange(0.1, 0.9) // Pulse width 10-90%
	case LFOTargetVolume:
		tb.lfo2.SetRange(0.5, 1.0) // Volume 50-100% for tremolo
	}
}

// GetLFO2Target returns the current LFO2 modulation target
func (tb *TB303) GetLFO2Target() LFOTarget {
	return tb.lfo2Target
}

// UpdateLFOTempo updates both LFO rates based on current BPM
func (tb *TB303) UpdateLFOTempo(bpm float32) {
	tb.lfo1.UpdateTempoRate(bpm)
	tb.lfo2.UpdateTempoRate(bpm)
}
