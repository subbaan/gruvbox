package ui

import (
	"math/rand"

	"github.com/subbaan/gruvbox/sequencer"
)

// DrumPattern represents a pre-built drum pattern for a specific instrument
type DrumPattern struct {
	Name           string
	Description    string
	InstrumentType sequencer.InstrumentType // Which instrument this pattern is for
	Generate       func(pattern *sequencer.Pattern, patternLength int)
}

// DrumPatternRegistry holds all available drum patterns
var DrumPatternRegistry = []DrumPattern{
	// === KICK PATTERNS ===
	{
		Name:           "Four-On-Floor",
		Description:    "Kick on every beat (classic techno)",
		InstrumentType: sequencer.InstrumentKick,
		Generate:       generateKickFourOnFloor,
	},
	{
		Name:           "Broken Beat",
		Description:    "Syncopated kick pattern",
		InstrumentType: sequencer.InstrumentKick,
		Generate:       generateKickBroken,
	},
	{
		Name:           "Double Kick",
		Description:    "Extra kick hits for energy",
		InstrumentType: sequencer.InstrumentKick,
		Generate:       generateKickDouble,
	},
	{
		Name:           "Offbeat Kick",
		Description:    "Kick on offbeats",
		InstrumentType: sequencer.InstrumentKick,
		Generate:       generateKickOffbeat,
	},
	{
		Name:           "Sparse Kick",
		Description:    "Minimal kick pattern",
		InstrumentType: sequencer.InstrumentKick,
		Generate:       generateKickSparse,
	},
	{
		Name:           "Hard Techno Kick",
		Description:    "Driving hard techno kick",
		InstrumentType: sequencer.InstrumentKick,
		Generate:       generateKickHardTechno,
	},
	{
		Name:           "Tribal Kick",
		Description:    "Tribal rhythm pattern",
		InstrumentType: sequencer.InstrumentKick,
		Generate:       generateKickTribal,
	},

	// === SNARE/CLAP PATTERNS ===
	{
		Name:           "Backbeat",
		Description:    "Snare on 2 and 4",
		InstrumentType: sequencer.InstrumentSnare,
		Generate:       generateSnareBackbeat,
	},
	{
		Name:           "Backbeat",
		Description:    "Clap on 2 and 4",
		InstrumentType: sequencer.InstrumentClap,
		Generate:       generateSnareBackbeat, // Same pattern works for clap
	},
	{
		Name:           "Syncopated",
		Description:    "Off-grid snare hits",
		InstrumentType: sequencer.InstrumentSnare,
		Generate:       generateSnareSyncopated,
	},
	{
		Name:           "Syncopated",
		Description:    "Off-grid clap hits",
		InstrumentType: sequencer.InstrumentClap,
		Generate:       generateSnareSyncopated,
	},
	{
		Name:           "Ghost Notes",
		Description:    "Backbeat with quiet ghost hits",
		InstrumentType: sequencer.InstrumentSnare,
		Generate:       generateSnareGhost,
	},
	{
		Name:           "Ghost Notes",
		Description:    "Backbeat with quiet ghost claps",
		InstrumentType: sequencer.InstrumentClap,
		Generate:       generateSnareGhost,
	},
	{
		Name:           "Snare Roll",
		Description:    "Rapid snare fills",
		InstrumentType: sequencer.InstrumentSnare,
		Generate:       generateSnareRoll,
	},
	{
		Name:           "Clap Roll",
		Description:    "Rapid clap fills",
		InstrumentType: sequencer.InstrumentClap,
		Generate:       generateSnareRoll,
	},
	{
		Name:           "Minimal Snare",
		Description:    "Sparse placement",
		InstrumentType: sequencer.InstrumentSnare,
		Generate:       generateSnareMinimal,
	},
	{
		Name:           "Minimal Clap",
		Description:    "Sparse placement",
		InstrumentType: sequencer.InstrumentClap,
		Generate:       generateSnareMinimal,
	},

	// === HI-HAT CLOSED PATTERNS ===
	{
		Name:           "16th Notes",
		Description:    "Continuous 16th note hats",
		InstrumentType: sequencer.InstrumentHihatClosed,
		Generate:       generateHat16ths,
	},
	{
		Name:           "8th Notes",
		Description:    "Steady 8th note hats",
		InstrumentType: sequencer.InstrumentHihatClosed,
		Generate:       generateHat8ths,
	},
	{
		Name:           "Offbeat 8ths",
		Description:    "Hats on the '+' beats",
		InstrumentType: sequencer.InstrumentHihatClosed,
		Generate:       generateHatOffbeat,
	},
	{
		Name:           "Shuffle",
		Description:    "Swung hat groove",
		InstrumentType: sequencer.InstrumentHihatClosed,
		Generate:       generateHatShuffle,
	},
	{
		Name:           "Triplet Feel",
		Description:    "Triplet-based hat rhythm",
		InstrumentType: sequencer.InstrumentHihatClosed,
		Generate:       generateHatTriplets,
	},
	{
		Name:           "Broken Hats",
		Description:    "Skippy hat pattern",
		InstrumentType: sequencer.InstrumentHihatClosed,
		Generate:       generateHatBroken,
	},
	{
		Name:           "Minimal Hats",
		Description:    "Sparse hat hits",
		InstrumentType: sequencer.InstrumentHihatClosed,
		Generate:       generateHatMinimal,
	},

	// === HI-HAT OPEN PATTERNS ===
	{
		Name:           "Accent Open",
		Description:    "Open on offbeats",
		InstrumentType: sequencer.InstrumentHihatOpen,
		Generate:       generateOpenAccent,
	},
	{
		Name:           "8th Open",
		Description:    "Open hats on 8th notes",
		InstrumentType: sequencer.InstrumentHihatOpen,
		Generate:       generateOpen8ths,
	},
	{
		Name:           "Sparse Open",
		Description:    "Occasional open hits",
		InstrumentType: sequencer.InstrumentHihatOpen,
		Generate:       generateOpenSparse,
	},
	{
		Name:           "Groove Open",
		Description:    "Groovy open hat pattern",
		InstrumentType: sequencer.InstrumentHihatOpen,
		Generate:       generateOpenGroove,
	},

	// === PERCUSSION PATTERNS (Rimshot, Crash, Tom) ===
	{
		Name:           "Rimshot Accent",
		Description:    "Accent hits on key beats",
		InstrumentType: sequencer.InstrumentRimshot,
		Generate:       generateRimshotAccent,
	},
	{
		Name:           "Rimshot Groove",
		Description:    "Rhythmic rimshot pattern",
		InstrumentType: sequencer.InstrumentRimshot,
		Generate:       generateRimshotGroove,
	},
	{
		Name:           "Crash Accent",
		Description:    "Crash on downbeats",
		InstrumentType: sequencer.InstrumentCrash,
		Generate:       generateCrashAccent,
	},
	{
		Name:           "Crash Build",
		Description:    "Building crash hits",
		InstrumentType: sequencer.InstrumentCrash,
		Generate:       generateCrashBuild,
	},
	{
		Name:           "Tom Fill",
		Description:    "Tom fill pattern",
		InstrumentType: sequencer.InstrumentTom,
		Generate:       generateTomFill,
	},
	{
		Name:           "Tom Groove",
		Description:    "Rhythmic tom pattern",
		InstrumentType: sequencer.InstrumentTom,
		Generate:       generateTomGroove,
	},

	// === HUMANIZATION (works for all drums) ===
	{
		Name:           "Humanize (Mild)",
		Description:    "Slight velocity variation",
		InstrumentType: sequencer.InstrumentKick,
		Generate:       generateDrumHumanizeMild,
	},
	{
		Name:           "Humanize (Mild)",
		Description:    "Slight velocity variation",
		InstrumentType: sequencer.InstrumentSnare,
		Generate:       generateDrumHumanizeMild,
	},
	{
		Name:           "Humanize (Mild)",
		Description:    "Slight velocity variation",
		InstrumentType: sequencer.InstrumentHihatClosed,
		Generate:       generateDrumHumanizeMild,
	},
	{
		Name:           "Humanize (Mild)",
		Description:    "Slight velocity variation",
		InstrumentType: sequencer.InstrumentHihatOpen,
		Generate:       generateDrumHumanizeMild,
	},
	{
		Name:           "Humanize (Mild)",
		Description:    "Slight velocity variation",
		InstrumentType: sequencer.InstrumentClap,
		Generate:       generateDrumHumanizeMild,
	},
	{
		Name:           "Humanize (Mild)",
		Description:    "Slight velocity variation",
		InstrumentType: sequencer.InstrumentRimshot,
		Generate:       generateDrumHumanizeMild,
	},
	{
		Name:           "Humanize (Mild)",
		Description:    "Slight velocity variation",
		InstrumentType: sequencer.InstrumentCrash,
		Generate:       generateDrumHumanizeMild,
	},
	{
		Name:           "Humanize (Mild)",
		Description:    "Slight velocity variation",
		InstrumentType: sequencer.InstrumentTom,
		Generate:       generateDrumHumanizeMild,
	},
	{
		Name:           "Humanize (Strong)",
		Description:    "Strong velocity variation",
		InstrumentType: sequencer.InstrumentKick,
		Generate:       generateDrumHumanizeStrong,
	},
	{
		Name:           "Humanize (Strong)",
		Description:    "Strong velocity variation",
		InstrumentType: sequencer.InstrumentSnare,
		Generate:       generateDrumHumanizeStrong,
	},
	{
		Name:           "Humanize (Strong)",
		Description:    "Strong velocity variation",
		InstrumentType: sequencer.InstrumentHihatClosed,
		Generate:       generateDrumHumanizeStrong,
	},
	{
		Name:           "Humanize (Strong)",
		Description:    "Strong velocity variation",
		InstrumentType: sequencer.InstrumentHihatOpen,
		Generate:       generateDrumHumanizeStrong,
	},
	{
		Name:           "Humanize (Strong)",
		Description:    "Strong velocity variation",
		InstrumentType: sequencer.InstrumentClap,
		Generate:       generateDrumHumanizeStrong,
	},
	{
		Name:           "Humanize (Strong)",
		Description:    "Strong velocity variation",
		InstrumentType: sequencer.InstrumentRimshot,
		Generate:       generateDrumHumanizeStrong,
	},
	{
		Name:           "Humanize (Strong)",
		Description:    "Strong velocity variation",
		InstrumentType: sequencer.InstrumentCrash,
		Generate:       generateDrumHumanizeStrong,
	},
	{
		Name:           "Humanize (Strong)",
		Description:    "Strong velocity variation",
		InstrumentType: sequencer.InstrumentTom,
		Generate:       generateDrumHumanizeStrong,
	},
}

// === KICK PATTERNS ===

func generateKickFourOnFloor(pattern *sequencer.Pattern, patternLength int) {
	// Kick on every quarter note (steps 0, 4, 8, 12, 16, 20, 24, 28...)
	for i := 0; i < patternLength; i += 4 {
		if step := pattern.GetStep(i); step != nil {
			step.Active = true
			step.Velocity = 0.9
		}
	}
}

func generateKickBroken(pattern *sequencer.Pattern, patternLength int) {
	// Syncopated kick pattern
	kickSteps := []int{0, 5, 8, 13, 16, 21, 24, 29}
	for _, stepNum := range kickSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.9
			}
		}
	}
}

func generateKickDouble(pattern *sequencer.Pattern, patternLength int) {
	// Four on floor with extra kicks
	kickSteps := []int{0, 3, 4, 7, 8, 11, 12, 16, 19, 20, 23, 24, 27, 28}
	for _, stepNum := range kickSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.85
			}
		}
	}
}

func generateKickOffbeat(pattern *sequencer.Pattern, patternLength int) {
	// Kicks on offbeats
	kickSteps := []int{2, 6, 10, 14, 18, 22, 26, 30}
	for _, stepNum := range kickSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.85
			}
		}
	}
}

func generateKickSparse(pattern *sequencer.Pattern, patternLength int) {
	// Minimal kick pattern
	kickSteps := []int{0, 8, 16, 24}
	for _, stepNum := range kickSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.9
			}
		}
	}
}

func generateKickHardTechno(pattern *sequencer.Pattern, patternLength int) {
	// Aggressive hard techno kick
	for i := 0; i < patternLength; i += 4 {
		if step := pattern.GetStep(i); step != nil {
			step.Active = true
			step.Velocity = 1.0 // Full power
		}
	}
	// Extra accent kicks
	extraKicks := []int{3, 11, 19, 27}
	for _, stepNum := range extraKicks {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.75
			}
		}
	}
}

func generateKickTribal(pattern *sequencer.Pattern, patternLength int) {
	// Tribal rhythm
	kickSteps := []int{0, 3, 6, 8, 11, 14, 16, 19, 22, 24, 27, 30}
	for _, stepNum := range kickSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				if stepNum%8 == 0 {
					step.Velocity = 0.95 // Accent downbeats
				} else {
					step.Velocity = 0.75
				}
			}
		}
	}
}

// === SNARE/CLAP PATTERNS ===

func generateSnareBackbeat(pattern *sequencer.Pattern, patternLength int) {
	// Classic backbeat on 2 and 4
	snareSteps := []int{8, 24} // Steps 8 and 24 in a 32-step pattern
	for _, stepNum := range snareSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.85
			}
		}
	}
}

func generateSnareSyncopated(pattern *sequencer.Pattern, patternLength int) {
	// Syncopated snare pattern
	snareSteps := []int{6, 10, 14, 22, 26, 30}
	for _, stepNum := range snareSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.8
			}
		}
	}
}

func generateSnareGhost(pattern *sequencer.Pattern, patternLength int) {
	// Backbeat with ghost notes
	// Main hits
	mainHits := []int{8, 24}
	for _, stepNum := range mainHits {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.85
			}
		}
	}
	// Ghost notes (quiet)
	ghostHits := []int{2, 6, 10, 14, 18, 22, 26, 30}
	for _, stepNum := range ghostHits {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.3 // Quiet
			}
		}
	}
}

func generateSnareRoll(pattern *sequencer.Pattern, patternLength int) {
	// Snare roll pattern (fill)
	// Every other step for rolls
	for i := 8; i < 16; i++ {
		if i < patternLength {
			if step := pattern.GetStep(i); step != nil {
				step.Active = true
				step.Velocity = 0.7
			}
		}
	}
	for i := 24; i < 32; i++ {
		if i < patternLength {
			if step := pattern.GetStep(i); step != nil {
				step.Active = true
				step.Velocity = 0.7
			}
		}
	}
}

func generateSnareMinimal(pattern *sequencer.Pattern, patternLength int) {
	// Minimal snare placement
	snareSteps := []int{8, 24}
	for _, stepNum := range snareSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.7
			}
		}
	}
}

// === HI-HAT CLOSED PATTERNS ===

func generateHat16ths(pattern *sequencer.Pattern, patternLength int) {
	// Continuous 16th notes
	for i := 0; i < patternLength; i++ {
		if step := pattern.GetStep(i); step != nil {
			step.Active = true
			// Accent every 4th
			if i%4 == 0 {
				step.Velocity = 0.75
			} else {
				step.Velocity = 0.5
			}
		}
	}
}

func generateHat8ths(pattern *sequencer.Pattern, patternLength int) {
	// 8th notes (every other step)
	for i := 0; i < patternLength; i += 2 {
		if step := pattern.GetStep(i); step != nil {
			step.Active = true
			step.Velocity = 0.65
		}
	}
}

func generateHatOffbeat(pattern *sequencer.Pattern, patternLength int) {
	// Offbeat 8ths (on the '+')
	for i := 1; i < patternLength; i += 2 {
		if step := pattern.GetStep(i); step != nil {
			step.Active = true
			step.Velocity = 0.6
		}
	}
}

func generateHatShuffle(pattern *sequencer.Pattern, patternLength int) {
	// Swung shuffle pattern
	shufflePattern := []int{0, 3, 4, 7, 8, 11, 12, 15, 16, 19, 20, 23, 24, 27, 28, 31}
	for _, stepNum := range shufflePattern {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.6
			}
		}
	}
}

func generateHatTriplets(pattern *sequencer.Pattern, patternLength int) {
	// Triplet feel (every 3 steps with adjustment)
	for i := 0; i < patternLength; i += 3 {
		if step := pattern.GetStep(i); step != nil {
			step.Active = true
			step.Velocity = 0.65
		}
	}
}

func generateHatBroken(pattern *sequencer.Pattern, patternLength int) {
	// Skippy broken hat pattern
	brokenPattern := []int{0, 2, 4, 7, 8, 10, 12, 15, 16, 18, 20, 23, 24, 26, 28, 31}
	for _, stepNum := range brokenPattern {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.55
			}
		}
	}
}

func generateHatMinimal(pattern *sequencer.Pattern, patternLength int) {
	// Sparse minimal hats
	hatSteps := []int{0, 4, 8, 12, 16, 20, 24, 28}
	for _, stepNum := range hatSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.6
			}
		}
	}
}

// === HI-HAT OPEN PATTERNS ===

func generateOpenAccent(pattern *sequencer.Pattern, patternLength int) {
	// Open hat on offbeats for accent
	openSteps := []int{6, 14, 22, 30}
	for _, stepNum := range openSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.65
			}
		}
	}
}

func generateOpen8ths(pattern *sequencer.Pattern, patternLength int) {
	// Open hats on 8th notes
	for i := 0; i < patternLength; i += 2 {
		if step := pattern.GetStep(i); step != nil {
			step.Active = true
			step.Velocity = 0.6
		}
	}
}

func generateOpenSparse(pattern *sequencer.Pattern, patternLength int) {
	// Occasional open hits
	openSteps := []int{7, 15, 23, 31}
	for _, stepNum := range openSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.6
			}
		}
	}
}

func generateOpenGroove(pattern *sequencer.Pattern, patternLength int) {
	// Groovy open pattern
	openSteps := []int{3, 7, 11, 15, 19, 23, 27, 31}
	for _, stepNum := range openSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.65
			}
		}
	}
}

// === PERCUSSION PATTERNS ===

func generateRimshotAccent(pattern *sequencer.Pattern, patternLength int) {
	// Accent on key beats
	rimshotSteps := []int{0, 8, 16, 24}
	for _, stepNum := range rimshotSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.75
			}
		}
	}
}

func generateRimshotGroove(pattern *sequencer.Pattern, patternLength int) {
	// Rhythmic rimshot
	rimshotSteps := []int{4, 12, 20, 28}
	for _, stepNum := range rimshotSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.7
			}
		}
	}
}

func generateCrashAccent(pattern *sequencer.Pattern, patternLength int) {
	// Crash on downbeats
	crashSteps := []int{0, 16}
	for _, stepNum := range crashSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.8
			}
		}
	}
}

func generateCrashBuild(pattern *sequencer.Pattern, patternLength int) {
	// Building crash pattern
	crashSteps := []int{0, 8, 12, 16, 20, 24, 28}
	for _, stepNum := range crashSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				// Build up velocity
				step.Velocity = 0.5 + (float32(stepNum) / float32(patternLength) * 0.4)
			}
		}
	}
}

func generateTomFill(pattern *sequencer.Pattern, patternLength int) {
	// Tom fill pattern
	tomSteps := []int{8, 10, 12, 14, 24, 26, 28, 30}
	for _, stepNum := range tomSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.75
			}
		}
	}
}

func generateTomGroove(pattern *sequencer.Pattern, patternLength int) {
	// Rhythmic tom pattern
	tomSteps := []int{4, 12, 20, 28}
	for _, stepNum := range tomSteps {
		if stepNum < patternLength {
			if step := pattern.GetStep(stepNum); step != nil {
				step.Active = true
				step.Velocity = 0.7
			}
		}
	}
}

// === HUMANIZATION PATTERNS ===

func generateDrumHumanizeMild(pattern *sequencer.Pattern, patternLength int) {
	// Keep existing pattern but vary velocities slightly (±10%)
	for i := 0; i < patternLength; i++ {
		if step := pattern.GetStep(i); step != nil && step.Active {
			// Variation of ±10%
			variation := (rand.Float32() - 0.5) * 0.2 // ±10%
			newVelocity := step.Velocity + variation
			if newVelocity < 0.3 {
				newVelocity = 0.3
			}
			if newVelocity > 1.0 {
				newVelocity = 1.0
			}
			step.Velocity = newVelocity
		}
	}
}

func generateDrumHumanizeStrong(pattern *sequencer.Pattern, patternLength int) {
	// Keep existing pattern but vary velocities strongly (±25%)
	for i := 0; i < patternLength; i++ {
		if step := pattern.GetStep(i); step != nil && step.Active {
			// Variation of ±25%
			variation := (rand.Float32() - 0.5) * 0.5 // ±25%
			newVelocity := step.Velocity + variation
			if newVelocity < 0.2 {
				newVelocity = 0.2
			}
			if newVelocity > 1.0 {
				newVelocity = 1.0
			}
			step.Velocity = newVelocity
		}
	}
}
