package drums

// DrumVoice is the interface for all drum synthesizers
type DrumVoice interface {
	// Trigger starts the drum sound
	Trigger(velocity float32)

	// Process generates a buffer of audio samples
	Process(output []float32)

	// IsActive returns true if the voice is producing sound
	IsActive() bool

	// Reset resets the voice state
	Reset()
}
