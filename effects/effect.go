package effects

// Effect is the interface for all audio effects processors
type Effect interface {
	// Process processes a buffer of audio samples in-place
	Process(buffer []float32)

	// Reset clears the effect state (useful when stopping/starting)
	Reset()

	// SetEnabled enables or disables the effect
	SetEnabled(enabled bool)

	// IsEnabled returns true if the effect is enabled
	IsEnabled() bool

	// SetMix sets the dry/wet mix (0.0 = dry, 1.0 = wet)
	SetMix(mix float32)
}

// BaseEffect provides common functionality for all effects
type BaseEffect struct {
	enabled bool
	mix     float32
}

// NewBaseEffect creates a new base effect
func NewBaseEffect() BaseEffect {
	return BaseEffect{
		enabled: true,
		mix:     1.0,
	}
}

// SetEnabled enables or disables the effect
func (e *BaseEffect) SetEnabled(enabled bool) {
	e.enabled = enabled
}

// IsEnabled returns true if the effect is enabled
func (e *BaseEffect) IsEnabled() bool {
	return e.enabled
}

// SetMix sets the dry/wet mix (0.0 = dry, 1.0 = wet)
func (e *BaseEffect) SetMix(mix float32) {
	if mix < 0.0 {
		mix = 0.0
	}
	if mix > 1.0 {
		mix = 1.0
	}
	e.mix = mix
}

// GetMix returns the current dry/wet mix
func (e *BaseEffect) GetMix() float32 {
	return e.mix
}

// ProcessMix applies dry/wet mixing
// dry is the original buffer, wet is the processed buffer
func (e *BaseEffect) ProcessMix(dry []float32, wet []float32) {
	for i := range dry {
		dry[i] = dry[i]*(1.0-e.mix) + wet[i]*e.mix
	}
}
