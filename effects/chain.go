package effects

// Chain represents a chain of effects processors
type Chain struct {
	effects []Effect
	enabled bool
}

// NewChain creates a new effect chain
func NewChain() *Chain {
	return &Chain{
		effects: make([]Effect, 0, 8),
		enabled: true,
	}
}

// Add adds an effect to the end of the chain
func (c *Chain) Add(effect Effect) {
	c.effects = append(c.effects, effect)
}

// Process processes audio through the entire effect chain
func (c *Chain) Process(buffer []float32) {
	if !c.enabled {
		return
	}

	// Process through each effect in order
	for _, effect := range c.effects {
		if effect.IsEnabled() {
			effect.Process(buffer)
		}
	}
}

// Reset resets all effects in the chain
func (c *Chain) Reset() {
	for _, effect := range c.effects {
		effect.Reset()
	}
}

// SetEnabled enables or disables the entire chain
func (c *Chain) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// IsEnabled returns true if the chain is enabled
func (c *Chain) IsEnabled() bool {
	return c.enabled
}

// GetEffect returns the effect at the specified index
func (c *Chain) GetEffect(index int) Effect {
	if index < 0 || index >= len(c.effects) {
		return nil
	}
	return c.effects[index]
}

// Count returns the number of effects in the chain
func (c *Chain) Count() int {
	return len(c.effects)
}

// Clear removes all effects from the chain
func (c *Chain) Clear() {
	c.effects = c.effects[:0]
}
