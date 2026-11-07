package ui

import (
	"math"
	"math/rand"

	"github.com/subbaan/gruvbox/sequencer"
)

// PLockGenerator represents a parameter lock pattern generator
type PLockGenerator struct {
	Name        string
	Description string
	Category    string // "synth", "drum", "universal"
	Generate    func(pattern *sequencer.Pattern, paramName string, startStep, endStep int)
}

// PLockGeneratorRegistry holds all available generators
var PLockGeneratorRegistry = []PLockGenerator{
	// === MOST USED (TOP OF LIST) ===
	{
		Name:        "Random Chaos",
		Description: "Random values per step",
		Category:    "universal",
		Generate:    generateRandomChaos,
	},
	{
		Name:        "Humanize (Mild)",
		Description: "±5% variation around existing values",
		Category:    "universal",
		Generate:    generateHumanizeMild,
	},
	{
		Name:        "Humanize (Heavy)",
		Description: "±20% variation around existing values",
		Category:    "universal",
		Generate:    generateHumanizeHeavy,
	},
	{
		Name:        "Humanize (Super Heavy)",
		Description: "±35% variation around existing values",
		Category:    "universal",
		Generate:    generateHumanizeSuperHeavy,
	},

	// === UNIVERSAL GENERATORS ===
	{
		Name:        "Step Ladder",
		Description: "Stepped increments up then down",
		Category:    "universal",
		Generate:    generateStepLadder,
	},
	{
		Name:        "Smooth Wave",
		Description: "Sine wave modulation",
		Category:    "universal",
		Generate:    generateSmoothWave,
	},

	// === SYNTH GENERATORS (TB-303) ===
	{
		Name:        "Acid Sweep",
		Description: "Classic acid filter sweep upward",
		Category:    "synth",
		Generate:    generateAcidSweep,
	},
	{
		Name:        "Reverse Sweep",
		Description: "Filter sweep downward",
		Category:    "synth",
		Generate:    generateReverseSweep,
	},
	{
		Name:        "Filter Pulse",
		Description: "Alternating high/low values",
		Category:    "synth",
		Generate:    generateFilterPulse,
	},

	// === DRUM GENERATORS ===
	{
		Name:        "Accent Dance",
		Description: "Every 4th step accented",
		Category:    "drum",
		Generate:    generateAccentDance,
	},
	{
		Name:        "Decay Fade",
		Description: "Gradually reduce decay/sustain",
		Category:    "drum",
		Generate:    generateDecayFade,
	},
	{
		Name:        "Build Up",
		Description: "Gradually increase intensity",
		Category:    "drum",
		Generate:    generateBuildUp,
	},
	{
		Name:        "Break Down",
		Description: "Gradually decrease intensity",
		Category:    "drum",
		Generate:    generateBreakDown,
	},
	{
		Name:        "Hi-Hat Shuffle",
		Description: "Alternating decay for groove",
		Category:    "drum",
		Generate:    generateHiHatShuffle,
	},
	{
		Name:        "Kick Punch",
		Description: "Drive/pitch variation for punch",
		Category:    "drum",
		Generate:    generateKickPunch,
	},
}

// === GENERATOR IMPLEMENTATIONS ===

// generateAcidSweep creates classic acid filter sweep (low to high)
func generateAcidSweep(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	steps := endStep - startStep + 1
	for i := 0; i < steps; i++ {
		stepIdx := startStep + i
		step := pattern.GetStep(stepIdx)
		if step == nil {
			continue
		}

		// Linear sweep from 0 to 1
		value := float32(i) / float32(steps-1)
		step.SetParamLock(paramName, value)
	}
}

// generateReverseSweep creates reverse filter sweep (high to low)
func generateReverseSweep(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	steps := endStep - startStep + 1
	for i := 0; i < steps; i++ {
		stepIdx := startStep + i
		step := pattern.GetStep(stepIdx)
		if step == nil {
			continue
		}

		// Linear sweep from 1 to 0
		value := 1.0 - (float32(i) / float32(steps-1))
		step.SetParamLock(paramName, value)
	}
}

// generateRandomChaos creates random values for each step
func generateRandomChaos(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	for i := startStep; i <= endStep; i++ {
		step := pattern.GetStep(i)
		if step == nil {
			continue
		}

		// Random value between 0 and 1
		value := rand.Float32()
		step.SetParamLock(paramName, value)
	}
}

// generateStepLadder creates stepped values (0, 0.25, 0.5, 0.75, 1.0, 0.75, 0.5, 0.25, repeat)
func generateStepLadder(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	levels := []float32{0.0, 0.25, 0.5, 0.75, 1.0, 0.75, 0.5, 0.25}

	for i := startStep; i <= endStep; i++ {
		step := pattern.GetStep(i)
		if step == nil {
			continue
		}

		// Cycle through levels
		value := levels[(i-startStep)%len(levels)]
		step.SetParamLock(paramName, value)
	}
}

// generateFilterPulse creates alternating high/low values
func generateFilterPulse(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	for i := startStep; i <= endStep; i++ {
		step := pattern.GetStep(i)
		if step == nil {
			continue
		}

		// Alternate between 0.2 and 0.8
		if (i-startStep)%2 == 0 {
			step.SetParamLock(paramName, 0.8)
		} else {
			step.SetParamLock(paramName, 0.2)
		}
	}
}

// generateSmoothWave creates sine wave modulation
func generateSmoothWave(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	steps := endStep - startStep + 1

	for i := 0; i < steps; i++ {
		stepIdx := startStep + i
		step := pattern.GetStep(stepIdx)
		if step == nil {
			continue
		}

		// Sine wave: one complete cycle over the range
		phase := float64(i) / float64(steps) * 2.0 * math.Pi
		value := float32((math.Sin(phase) + 1.0) / 2.0) // Normalize to 0-1
		step.SetParamLock(paramName, value)
	}
}

// generateHumanizeMild creates subtle random variation (±5%) around existing p-lock values
func generateHumanizeMild(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	// Get base value from pattern SoundParams (set by applyPLockGenerator)
	baseValue := float32(0.5) // Default fallback
	if pattern.SoundParams != nil {
		if val, ok := pattern.SoundParams["_base_"+paramName]; ok {
			baseValue = val
		}
	}

	for i := startStep; i <= endStep; i++ {
		step := pattern.GetStep(i)
		if step == nil {
			continue
		}

		// Use existing p-lock value if present, otherwise use base value from instrument
		existingValue, exists := step.GetParamLock(paramName)
		if !exists {
			existingValue = baseValue
		}

		// Add ±5% variation around the value
		variation := (rand.Float32() - 0.5) * 0.1 // ±5%
		value := existingValue + variation

		// Clamp to valid range
		if value < 0.0 {
			value = 0.0
		}
		if value > 1.0 {
			value = 1.0
		}
		step.SetParamLock(paramName, value)
	}
}

// generateHumanizeHeavy creates strong random variation (±20%) around existing p-lock values
func generateHumanizeHeavy(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	// Get base value from pattern SoundParams (set by applyPLockGenerator)
	baseValue := float32(0.5) // Default fallback
	if pattern.SoundParams != nil {
		if val, ok := pattern.SoundParams["_base_"+paramName]; ok {
			baseValue = val
		}
	}

	for i := startStep; i <= endStep; i++ {
		step := pattern.GetStep(i)
		if step == nil {
			continue
		}

		// Use existing p-lock value if present, otherwise use base value from instrument
		existingValue, exists := step.GetParamLock(paramName)
		if !exists {
			existingValue = baseValue
		}

		// Add ±20% variation around the value
		variation := (rand.Float32() - 0.5) * 0.4 // ±20%
		value := existingValue + variation

		// Clamp to valid range
		if value < 0.0 {
			value = 0.0
		}
		if value > 1.0 {
			value = 1.0
		}
		step.SetParamLock(paramName, value)
	}
}

// generateHumanizeSuperHeavy creates extreme random variation (±35%) around existing p-lock values
func generateHumanizeSuperHeavy(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	// Get base value from pattern SoundParams (set by applyPLockGenerator)
	baseValue := float32(0.5) // Default fallback
	if pattern.SoundParams != nil {
		if val, ok := pattern.SoundParams["_base_"+paramName]; ok {
			baseValue = val
		}
	}

	for i := startStep; i <= endStep; i++ {
		step := pattern.GetStep(i)
		if step == nil {
			continue
		}

		// Use existing p-lock value if present, otherwise use base value from instrument
		existingValue, exists := step.GetParamLock(paramName)
		if !exists {
			existingValue = baseValue
		}

		// Add ±35% variation around the value
		variation := (rand.Float32() - 0.5) * 0.7 // ±35%
		value := existingValue + variation

		// Clamp to valid range
		if value < 0.0 {
			value = 0.0
		}
		if value > 1.0 {
			value = 1.0
		}
		step.SetParamLock(paramName, value)
	}
}

// generateAccentDance creates accent pattern (every 4th step higher)
func generateAccentDance(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	for i := startStep; i <= endStep; i++ {
		step := pattern.GetStep(i)
		if step == nil {
			continue
		}

		// Every 4th step is accented (higher value)
		if (i-startStep)%4 == 0 {
			step.SetParamLock(paramName, 0.9)
		} else {
			step.SetParamLock(paramName, 0.5)
		}
	}
}

// generateDecayFade gradually reduces value from 1.0 to 0.2
func generateDecayFade(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	steps := endStep - startStep + 1

	for i := 0; i < steps; i++ {
		stepIdx := startStep + i
		step := pattern.GetStep(stepIdx)
		if step == nil {
			continue
		}

		// Fade from 1.0 to 0.2
		value := 1.0 - (float32(i) / float32(steps-1) * 0.8)
		step.SetParamLock(paramName, value)
	}
}

// generateBuildUp gradually increases value from 0.2 to 1.0
func generateBuildUp(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	steps := endStep - startStep + 1

	for i := 0; i < steps; i++ {
		stepIdx := startStep + i
		step := pattern.GetStep(stepIdx)
		if step == nil {
			continue
		}

		// Build from 0.2 to 1.0
		value := 0.2 + (float32(i) / float32(steps-1) * 0.8)
		step.SetParamLock(paramName, value)
	}
}

// generateBreakDown gradually decreases value from 1.0 to 0.0
func generateBreakDown(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	steps := endStep - startStep + 1

	for i := 0; i < steps; i++ {
		stepIdx := startStep + i
		step := pattern.GetStep(stepIdx)
		if step == nil {
			continue
		}

		// Exponential fade out
		progress := float32(i) / float32(steps-1)
		value := float32(math.Pow(float64(1.0-progress), 2.0))
		step.SetParamLock(paramName, value)
	}
}

// generateHiHatShuffle creates alternating decay for groove (techno shuffle)
func generateHiHatShuffle(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	for i := startStep; i <= endStep; i++ {
		step := pattern.GetStep(i)
		if step == nil {
			continue
		}

		// Shuffle pattern: long-short-long-short
		if (i-startStep)%4 == 0 || (i-startStep)%4 == 2 {
			// Long decay (open feel)
			step.SetParamLock(paramName, 0.7)
		} else {
			// Short decay (tight)
			step.SetParamLock(paramName, 0.3)
		}
	}
}

// generateKickPunch creates drive/pitch variation for punch
func generateKickPunch(pattern *sequencer.Pattern, paramName string, startStep, endStep int) {
	for i := startStep; i <= endStep; i++ {
		step := pattern.GetStep(i)
		if step == nil {
			continue
		}

		// First kick of every 4 bars has extra punch
		if (i-startStep)%16 == 0 {
			step.SetParamLock(paramName, 1.0) // Max punch
		} else if (i-startStep)%4 == 0 {
			step.SetParamLock(paramName, 0.75) // Medium punch
		} else {
			step.SetParamLock(paramName, 0.5) // Normal
		}
	}
}
