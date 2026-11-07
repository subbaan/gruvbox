package synth

import "math"

// EnvelopeStage represents the current stage of an envelope
type EnvelopeStage int

const (
	StageIdle EnvelopeStage = iota
	StageAttack
	StageDecay
	StageSustain
	StageRelease
)

// Envelope is a general-purpose ADSR envelope generator
type Envelope struct {
	stage      EnvelopeStage
	value      float32
	sampleRate float32

	// ADSR parameters (in seconds)
	attackTime  float32
	decayTime   float32
	sustainLevel float32
	releaseTime float32

	// Calculated coefficients
	attackCoeff  float32
	decayCoeff   float32
	releaseCoeff float32
}

// NewEnvelope creates a new ADSR envelope
func NewEnvelope(sampleRate float32) *Envelope {
	env := &Envelope{
		stage:        StageIdle,
		value:        0.0,
		sampleRate:   sampleRate,
		attackTime:   0.001,  // 1ms
		decayTime:    0.3,    // 300ms
		sustainLevel: 0.7,    // 70%
		releaseTime:  0.1,    // 100ms
	}
	env.updateCoefficients()
	return env
}

// SetADSR sets all envelope parameters at once
func (env *Envelope) SetADSR(attack, decay, sustain, release float32) {
	env.attackTime = attack
	env.decayTime = decay
	env.sustainLevel = sustain
	env.releaseTime = release
	env.updateCoefficients()
}

// SetAttack sets the attack time in seconds
func (env *Envelope) SetAttack(attack float32) {
	env.attackTime = attack
	env.updateCoefficients()
}

// SetDecay sets the decay time in seconds
func (env *Envelope) SetDecay(decay float32) {
	env.decayTime = decay
	env.updateCoefficients()
}

// SetSustain sets the sustain level (0.0 to 1.0)
func (env *Envelope) SetSustain(sustain float32) {
	env.sustainLevel = sustain
}

// SetRelease sets the release time in seconds
func (env *Envelope) SetRelease(release float32) {
	env.releaseTime = release
	env.updateCoefficients()
}

// updateCoefficients calculates exponential coefficients from time parameters
func (env *Envelope) updateCoefficients() {
	// Exponential envelope coefficient: coeff = exp(-1 / (time * sampleRate))
	// This gives a smooth exponential curve
	if env.attackTime > 0 {
		env.attackCoeff = float32(math.Exp(-1.0 / float64(env.attackTime*env.sampleRate)))
	} else {
		env.attackCoeff = 0.0
	}

	if env.decayTime > 0 {
		env.decayCoeff = float32(math.Exp(-1.0 / float64(env.decayTime*env.sampleRate)))
	} else {
		env.decayCoeff = 0.0
	}

	if env.releaseTime > 0 {
		env.releaseCoeff = float32(math.Exp(-1.0 / float64(env.releaseTime*env.sampleRate)))
	} else {
		env.releaseCoeff = 0.0
	}
}

// Trigger starts the envelope (note on)
func (env *Envelope) Trigger() {
	env.stage = StageAttack
	// Don't reset value to 0 - allow retriggering from current level
}

// Release starts the release stage (note off)
func (env *Envelope) Release() {
	env.stage = StageRelease
}

// Reset resets the envelope to idle state
func (env *Envelope) Reset() {
	env.stage = StageIdle
	env.value = 0.0
}

// Process generates the next envelope sample
func (env *Envelope) Process() float32 {
	switch env.stage {
	case StageIdle:
		env.value = 0.0

	case StageAttack:
		// Exponential rise to 1.0
		env.value = env.attackCoeff*env.value + (1.0-env.attackCoeff)*1.0
		if env.value >= 0.999 {
			env.value = 1.0
			env.stage = StageDecay
		}

	case StageDecay:
		// Exponential decay to sustain level
		env.value = env.decayCoeff*env.value + (1.0-env.decayCoeff)*env.sustainLevel
		if env.value <= env.sustainLevel+0.001 {
			env.value = env.sustainLevel
			env.stage = StageSustain
		}

	case StageSustain:
		env.value = env.sustainLevel

	case StageRelease:
		// Exponential decay to 0
		env.value = env.releaseCoeff * env.value
		if env.value < 0.001 {
			env.value = 0.0
			env.stage = StageIdle
		}
	}

	return env.value
}

// GetValue returns the current envelope value without advancing
func (env *Envelope) GetValue() float32 {
	return env.value
}

// IsActive returns true if the envelope is not idle
func (env *Envelope) IsActive() bool {
	return env.stage != StageIdle
}

// DecayEnvelope is a simple decay-only envelope (like TB-303)
type DecayEnvelope struct {
	value      float32
	decayCoeff float32
	sampleRate float32
	active     bool
}

// NewDecayEnvelope creates a new decay-only envelope
func NewDecayEnvelope(sampleRate float32) *DecayEnvelope {
	env := &DecayEnvelope{
		value:      0.0,
		sampleRate: sampleRate,
		active:     false,
	}
	env.SetDecayTime(0.2) // 200ms default
	return env
}

// SetDecayTime sets the decay time in seconds
func (env *DecayEnvelope) SetDecayTime(timeSeconds float32) {
	// Exponential decay coefficient
	// After 'timeSeconds', the envelope should reach ~1% of initial value
	// coefficient = exp(-4.6 / (time * sampleRate))
	// 4.6 ≈ ln(100), so after 'time' seconds, value = 0.01
	if timeSeconds > 0.001 {
		env.decayCoeff = float32(math.Exp(-4.6 / float64(timeSeconds*env.sampleRate)))
	} else {
		env.decayCoeff = 0.0
	}
}

// Trigger starts the decay envelope
func (env *DecayEnvelope) Trigger() {
	env.value = 1.0
	env.active = true
}

// Reset resets the envelope to 0
func (env *DecayEnvelope) Reset() {
	env.value = 0.0
	env.active = false
}

// Process generates the next envelope sample
func (env *DecayEnvelope) Process() float32 {
	if env.active {
		// Exponential decay
		env.value *= env.decayCoeff

		// Stop when value is very small
		if env.value < 0.001 {
			env.value = 0.0
			env.active = false
		}
	}

	return env.value
}

// ProcessWithAccent processes envelope with accent (increases initial level)
func (env *DecayEnvelope) ProcessWithAccent(accent float32) float32 {
	value := env.Process()

	// Accent increases the envelope depth
	// With accent, the effective value can go above 1.0
	if accent > 0.0 {
		value *= (1.0 + accent)
	}

	return value
}

// GetValue returns the current envelope value
func (env *DecayEnvelope) GetValue() float32 {
	return env.value
}

// IsActive returns true if the envelope is decaying
func (env *DecayEnvelope) IsActive() bool {
	return env.active
}

// SetValue sets the envelope value directly (useful for retriggering)
func (env *DecayEnvelope) SetValue(value float32) {
	env.value = value
	env.active = value > 0.001
}
