package ui

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/subbaan/gruvbox/audio"
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/effects"
	"github.com/subbaan/gruvbox/sequencer"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/synth/drums"
)

// Track names for display
var trackNames = []string{
	"TB-303 #1",
	"TB-303 #2",
	"Kick",
	"Snare",
	"HH Closed",
	"HH Open",
	"Clap",
	"Rimshot",
	"Crash",
	"Tom",
	"Sampler #1",
	"Sampler #2",
	"Drone",
}

// Instrument types corresponding to track names
var trackInstruments = []sequencer.InstrumentType{
	sequencer.InstrumentTB303_1,
	sequencer.InstrumentTB303_2,
	sequencer.InstrumentKick,
	sequencer.InstrumentSnare,
	sequencer.InstrumentHihatClosed,
	sequencer.InstrumentHihatOpen,
	sequencer.InstrumentClap,
	sequencer.InstrumentRimshot,
	sequencer.InstrumentCrash,
	sequencer.InstrumentTom,
	sequencer.InstrumentSampler1,
	sequencer.InstrumentSampler2,
	sequencer.InstrumentDrone,
}

// centerInWidth centers a string within a given width
func centerInWidth(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	leftPad := (width - len(s)) / 2
	rightPad := width - len(s) - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

// getBarBackground returns alternating background colors for bars
func getBarBackground(barIndex int) lipgloss.Color {
	if barIndex%2 == 0 {
		return ColorBg
	}
	return ColorPanel
}

// renderSequencerView renders the step sequencer grid
func (m Model) renderSequencerView() string {
	var rows []string

	// Get current track's pattern length
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	patternLength := 16
	if track != nil && track.Pattern != nil {
		patternLength = track.Pattern.Length
	}

	// View title with pattern info
	title := TitleStyle.Render("STEP SEQUENCER")
	patternInfo := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Render(fmt.Sprintf("  Pattern %d/16  Length: %d  [Use {/} to change length]", m.seq.GetCurrentPatternID()+1, patternLength))
	rows = append(rows, title+patternInfo)
	rows = append(rows, "")

	// Calculate how many steps fit on screen
	visibleSteps := m.calculateVisibleSteps()
	numBars := visibleSteps / 4

	// Step numbers header - show only bar numbers (every 4 steps)
	stepHeader := "              " // Track name spacing (14 chars: 10 name + 3 [M/S] + 1 space)
	dividerStyle := lipgloss.NewStyle().Foreground(ColorBorder)

	for bar := 0; bar < numBars; bar++ {
		stepIdx := m.stepOffset + (bar * 4)
		if stepIdx >= patternLength {
			// Beyond pattern length, show empty
			stepHeader += "     " // 4 spaces for bar + 1 for divider position
			if bar < numBars-1 {
				stepHeader += dividerStyle.Render("│")
			}
			continue
		}

		// Show bar number (1, 5, 9, 13, etc.) centered over 4 steps
		barNum := fmt.Sprintf("%d", stepIdx+1)
		stepHeader += centerInWidth(barNum, 4)

		// Add divider between bars (except after last bar)
		if bar < numBars-1 {
			stepHeader += dividerStyle.Render("│")
		}
	}

	// Add scroll indicators if pattern is longer than visible window
	scrollInfo := ""
	if patternLength > visibleSteps {
		endStep := m.stepOffset + visibleSteps
		if endStep > patternLength {
			endStep = patternLength
		}
		scrollInfo = lipgloss.NewStyle().
			Foreground(ColorDim).
			Render(fmt.Sprintf("  [Steps %d-%d/%d]", m.stepOffset+1, endStep, patternLength))
	}
	rows = append(rows, stepHeader+scrollInfo)
	rows = append(rows, "")

	// Render each track
	for trackIdx, trackName := range trackNames {
		rows = append(rows, m.renderTrackRow(trackIdx, trackName, visibleSteps))
	}

	return strings.Join(rows, "\n")
}

// renderTrackRow renders a single track row
func (m Model) renderTrackRow(trackIdx int, trackName string, visibleSteps int) string {
	instrument := trackInstruments[trackIdx]
	track := m.seq.GetTrack(instrument)
	if track == nil {
		return ""
	}

	// Track name (fixed width)
	nameStyle := lipgloss.NewStyle().
		Width(10).
		Foreground(ColorFg)

	if trackIdx == m.cursorY {
		nameStyle = nameStyle.
			Background(ColorPrimary).
			Foreground(ColorBg).
			Bold(true)
	}

	row := nameStyle.Render(trackName)

	// Add mute/solo indicators
	if track.Mute {
		row += lipgloss.NewStyle().Foreground(ColorDim).Bold(true).Render("[M]")
	} else if track.Solo {
		row += lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render("[S]")
	} else {
		row += lipgloss.NewStyle().Foreground(ColorDim).Render("[ ]")
	}

	// Add ARP indicator for Drone track (track 12) when enabled
	if trackIdx == 12 {
		drone := m.engine.GetDrone()
		arp := drone.GetArpeggiator()
		if arp.GetEnabled() {
			row += lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("[ARP]")
		}
	}

	row += " "

	// Render visible window of steps grouped into bars of 4
	patternLength := track.Pattern.Length
	numBars := visibleSteps / 4
	dividerStyle := lipgloss.NewStyle().Foreground(ColorBorder)
	boundaryStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true) // Pattern boundary marker

	for bar := 0; bar < numBars; bar++ {
		// Build content for this bar (4 steps)
		barContent := ""
		for stepInBar := 0; stepInBar < 4; stepInBar++ {
			stepIdx := m.stepOffset + (bar * 4) + stepInBar
			if stepIdx >= patternLength {
				// Beyond pattern length, show empty
				barContent += " "
				continue
			}
			step := track.Pattern.GetStep(stepIdx)
			barContent += m.renderStepSymbol(step, stepIdx, trackIdx)
		}

		// Apply alternating background color to the bar
		barBg := getBarBackground(bar)
		row += lipgloss.NewStyle().Background(barBg).Render(barContent)

		// Add divider between bars (except after last bar)
		if bar < numBars-1 {
			// Check if pattern boundary falls at this divider position
			dividerPos := m.stepOffset + (bar+1)*4
			if dividerPos == patternLength {
				// This is where the pattern loops - use special marker
				row += boundaryStyle.Render("║")
			} else {
				row += dividerStyle.Render("│")
			}
		}
	}

	return row
}

// renderStepSymbol renders a single step symbol with appropriate styling
func (m Model) renderStepSymbol(step *sequencer.Step, stepIdx, trackIdx int) string {
	instrument := trackInstruments[trackIdx]
	track := m.seq.GetTrack(instrument)

	isCursor := stepIdx == m.cursorX && trackIdx == m.cursorY
	// Use per-track last triggered step position (shows what's actually playing/audible)
	isPlaying := track != nil && stepIdx == track.GetLastTriggeredStep() && m.isPlaying

	// Determine step content
	content := "░"
	hasParamLocks := step != nil && step.HasParamLocks()

	if step != nil && step.Active {
		// Check if step has a condition (use diamond symbols)
		if step.Condition != sequencer.ConditionAlways {
			// Conditional step - use diamond symbols
			if step.Accent && step.Slide {
				content = "◈" // Diamond with both
			} else if step.Accent {
				content = "◆" // Filled diamond (accent)
			} else if step.Slide {
				content = "◇" // Empty diamond (slide)
			} else {
				content = "◆" // Diamond for conditional
			}
		} else {
			// Normal step - regular symbols
			if step.Accent && step.Slide {
				content = "▲" // Both accent and slide
			} else if step.Accent {
				content = "▪" // Accent only
			} else if step.Slide {
				content = "~" // Slide only
			} else {
				content = "█" // Just active
			}
		}
	} else if hasParamLocks {
		// Inactive but has p-locks - show subtle indicator
		content = "●"
	}

	// Apply styling
	style := lipgloss.NewStyle()

	if isPlaying && step != nil && step.Active {
		// Currently playing step - bright highlight
		style = style.
			Foreground(ColorBg).
			Background(ColorAccent).
			Bold(true)
	} else if isCursor {
		// Cursor position - use yellow for high visibility
		style = style.
			Foreground(ColorBg).
			Background(ColorWarning).
			Bold(true)
	} else if step != nil && step.Active {
		// Active step - use special color if it has p-locks
		if hasParamLocks {
			style = style.
				Foreground(ColorPrimary). // Use primary color for p-locked steps
				Bold(true)
		} else {
			style = style.
				Foreground(ColorSecondary).
				Bold(true)
		}
	} else if hasParamLocks {
		// Inactive step with p-locks - use distinct color
		style = style.
			Foreground(ColorPrimary).
			Bold(false)
	} else {
		// Inactive step
		style = style.Foreground(ColorDim)
	}

	return style.Render(content)
}

// toggleStep toggles the current step on/off
func (m *Model) toggleStep() {
	instrument := trackInstruments[m.cursorY]
	track := m.seq.GetTrack(instrument)
	if track == nil {
		return
	}

	step := track.Pattern.GetStep(m.cursorX)
	if step == nil {
		return
	}

	// Toggle active state
	if step.Active {
		step.Active = false
		step.Accent = false
		step.Slide = false
	} else {
		step.Active = true
		step.Velocity = 0.8
		step.Probability = 1.0

		// Set default note for melodic instruments (TB-303 and Drone)
		if instrument == sequencer.InstrumentTB303_1 || instrument == sequencer.InstrumentTB303_2 || instrument == sequencer.InstrumentDrone {
			step.Note = 36 // C2 default
		}
	}

	track.Pattern.SetStep(m.cursorX, *step)
}

// toggleAccent toggles accent on the current step
func (m *Model) toggleAccent() {
	instrument := trackInstruments[m.cursorY]
	track := m.seq.GetTrack(instrument)
	if track == nil {
		return
	}

	step := track.Pattern.GetStep(m.cursorX)
	if step == nil || !step.Active {
		return
	}

	step.Accent = !step.Accent
	track.Pattern.SetStep(m.cursorX, *step)
}

// toggleSlide toggles slide on the current step
func (m *Model) toggleSlide() {
	instrument := trackInstruments[m.cursorY]
	track := m.seq.GetTrack(instrument)
	if track == nil {
		return
	}

	step := track.Pattern.GetStep(m.cursorX)
	if step == nil || !step.Active {
		return
	}

	// Only TB-303 tracks support slide
	if instrument != sequencer.InstrumentTB303_1 && instrument != sequencer.InstrumentTB303_2 {
		return
	}

	step.Slide = !step.Slide
	track.Pattern.SetStep(m.cursorX, *step)
}

// adjustSynthParam adjusts the currently selected synth parameter
func (m *Model) adjustSynthParam(delta float32) {
	var tb *synth.TB303
	if m.selectedSynth == 0 {
		tb = m.engine.GetTB303()
	} else {
		tb = m.engine.GetTB303_2()
	}

	// Get current value and adjust based on selected parameter
	switch m.selectedParam {
	case 0: // Waveform - cycle through waveforms
		current := tb.GetWaveform()
		if delta > 0 {
			// Cycle forward: Saw -> Square -> Triangle -> Sine -> Saw
			switch current {
			case synth.WaveformSaw:
				tb.SetWaveform(synth.WaveformSquare)
			case synth.WaveformSquare:
				tb.SetWaveform(synth.WaveformTriangle)
			case synth.WaveformTriangle:
				tb.SetWaveform(synth.WaveformSine)
			case synth.WaveformSine:
				tb.SetWaveform(synth.WaveformSaw)
			}
		} else {
			// Cycle backward: Saw -> Sine -> Triangle -> Square -> Saw
			switch current {
			case synth.WaveformSaw:
				tb.SetWaveform(synth.WaveformSine)
			case synth.WaveformSine:
				tb.SetWaveform(synth.WaveformTriangle)
			case synth.WaveformTriangle:
				tb.SetWaveform(synth.WaveformSquare)
			case synth.WaveformSquare:
				tb.SetWaveform(synth.WaveformSaw)
			}
		}
	case 1: // Cutoff
		val := tb.GetCutoff() + delta
		if val < 0 {
			val = 0
		}
		if val > 1 {
			val = 1
		}
		tb.SetCutoff(val)
	case 2: // Resonance
		val := tb.GetResonance() + delta
		if val < 0 {
			val = 0
		}
		if val > 1 {
			val = 1
		}
		tb.SetResonance(val)
	case 3: // Env Mod
		val := tb.GetEnvMod() + delta
		if val < 0 {
			val = 0
		}
		if val > 1 {
			val = 1
		}
		tb.SetEnvMod(val)
	case 4: // Decay
		val := tb.GetDecay() + delta
		if val < 0 {
			val = 0
		}
		if val > 1 {
			val = 1
		}
		// SetDecay expects normalized (0-1) and will scale internally
		tb.SetDecay(val)
	case 5: // Accent
		val := tb.GetAccent() + delta
		if val < 0 {
			val = 0
		}
		if val > 1 {
			val = 1
		}
		tb.SetAccent(val)
	case 6: // Slide Time
		val := tb.GetSlideTime() + delta
		if val < 0 {
			val = 0
		}
		if val > 1 {
			val = 1
		}
		// Denormalize from 0-1 to seconds (0.01-0.5) before setting
		tb.SetSlideTime(val*0.49 + 0.01)
	case 7: // Sub Level
		val := tb.GetSubLevel() + delta
		if val < 0 {
			val = 0
		}
		if val > 1 {
			val = 1
		}
		tb.SetSubLevel(val)
	case 8: // Volume
		mixer := m.engine.GetMixer()
		var channel *audio.Channel
		if m.selectedSynth == 0 {
			channel = mixer.GetChannel(0)
		} else {
			channel = mixer.GetChannel(1)
		}
		val := channel.Level + delta
		channel.Level = clamp(val, 0.0, 1.0)

	// Case 9 is separator, skipped

	case 10: // LFO1 Enable (toggle)
		tb.SetLFO1Enabled(!tb.GetLFO1Enabled())

	case 11: // LFO1 Target
		direction := 1
		if delta < 0 {
			direction = -1
		}
		m.cycleLFO1Target(tb, direction)

	case 12: // LFO1 Waveform
		direction := 1
		if delta < 0 {
			direction = -1
		}
		m.cycleLFO1Waveform(tb, direction)

	case 13: // LFO1 Rate
		lfo := tb.GetLFO1()
		if lfo.GetTempoSync() {
			if lfo.GetSyncMode() == dsp.SyncModeSteps {
				// Adjust steps per cycle
				steps := lfo.GetStepsPerCycle()
				if delta > 0 {
					steps++
					if steps > 256 {
						steps = 256
					}
				} else if delta < 0 {
					steps--
					if steps < 1 {
						steps = 1
					}
				}
				lfo.SetStepsPerCycle(steps)
				tb.UpdateLFOTempo(m.bpm)
			} else {
				// Adjust sync rate (musical divisions)
				m.adjustLFOSyncRate(lfo, delta)
			}
		} else {
			// Adjust Hz rate
			val := lfo.GetRate() + delta*10.0 // Scale for rate (0.01-100 Hz)
			lfo.SetRate(clamp(val, 0.01, 100.0))
		}

	case 14: // LFO1 Depth
		lfo := tb.GetLFO1()
		val := lfo.GetDepth() + delta
		lfo.SetDepth(clamp(val, 0.0, 1.0))

	case 15: // LFO1 Min
		lfo := tb.GetLFO1()
		target := tb.GetLFO1Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		if target == synth.LFOTargetPitch {
			// Pitch: 0.95 to 1.05 range
			min = clamp(min+delta*0.01, 0.95, max-0.01)
		} else {
			// Cutoff/Resonance: 0 to 1 range
			min = clamp(min+delta, 0.0, max-0.05)
		}
		lfo.SetRange(min, max)

	case 16: // LFO1 Max
		lfo := tb.GetLFO1()
		target := tb.GetLFO1Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		if target == synth.LFOTargetPitch {
			// Pitch: 0.95 to 1.05 range
			max = clamp(max+delta*0.01, min+0.01, 1.05)
		} else {
			// Cutoff/Resonance: 0 to 1 range
			max = clamp(max+delta, min+0.05, 1.0)
		}
		lfo.SetRange(min, max)

	case 17: // LFO1 Tempo Sync (toggle)
		lfo := tb.GetLFO1()
		lfo.SetTempoSync(!lfo.GetTempoSync())
		if lfo.GetTempoSync() {
			// Update rate based on current BPM
			tb.UpdateLFOTempo(m.bpm)
		}

	case 18: // LFO1 Mode (toggle Time/Steps)
		lfo := tb.GetLFO1()
		if lfo.GetSyncMode() == dsp.SyncModeTime {
			lfo.SetSyncMode(dsp.SyncModeSteps)
			if lfo.GetStepsPerCycle() == 0 {
				lfo.SetStepsPerCycle(16) // Default to 16 steps
			}
		} else {
			lfo.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo() // Recalculate rate

	// Case 19 is separator, skipped

	case 20: // LFO2 Enable (toggle)
		tb.SetLFO2Enabled(!tb.GetLFO2Enabled())

	case 21: // LFO2 Target
		direction := 1
		if delta < 0 {
			direction = -1
		}
		m.cycleLFO2Target(tb, direction)

	case 22: // LFO2 Waveform
		direction := 1
		if delta < 0 {
			direction = -1
		}
		m.cycleLFO2Waveform(tb, direction)

	case 23: // LFO2 Rate
		lfo := tb.GetLFO2()
		if lfo.GetTempoSync() {
			if lfo.GetSyncMode() == dsp.SyncModeSteps {
				// Adjust steps per cycle
				steps := lfo.GetStepsPerCycle()
				if delta > 0 {
					steps++
					if steps > 256 {
						steps = 256
					}
				} else if delta < 0 {
					steps--
					if steps < 1 {
						steps = 1
					}
				}
				lfo.SetStepsPerCycle(steps)
				tb.UpdateLFOTempo(m.bpm)
			} else {
				// Adjust sync rate (musical divisions)
				m.adjustLFOSyncRate(lfo, delta)
			}
		} else {
			// Adjust Hz rate
			val := lfo.GetRate() + delta*10.0 // Scale for rate (0.01-100 Hz)
			lfo.SetRate(clamp(val, 0.01, 100.0))
		}

	case 24: // LFO2 Depth
		lfo := tb.GetLFO2()
		val := lfo.GetDepth() + delta
		lfo.SetDepth(clamp(val, 0.0, 1.0))

	case 25: // LFO2 Min
		lfo := tb.GetLFO2()
		target := tb.GetLFO2Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		if target == synth.LFOTargetPitch {
			// Pitch: 0.95 to 1.05 range
			min = clamp(min+delta*0.01, 0.95, max-0.01)
		} else {
			// Cutoff/Resonance: 0 to 1 range
			min = clamp(min+delta, 0.0, max-0.05)
		}
		lfo.SetRange(min, max)

	case 26: // LFO2 Max
		lfo := tb.GetLFO2()
		target := tb.GetLFO2Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		if target == synth.LFOTargetPitch {
			// Pitch: 0.95 to 1.05 range
			max = clamp(max+delta*0.01, min+0.01, 1.05)
		} else {
			// Cutoff/Resonance: 0 to 1 range
			max = clamp(max+delta, min+0.05, 1.0)
		}
		lfo.SetRange(min, max)

	case 27: // LFO2 Tempo Sync (toggle)
		lfo := tb.GetLFO2()
		lfo.SetTempoSync(!lfo.GetTempoSync())
		if lfo.GetTempoSync() {
			// Update rate based on current BPM
			tb.UpdateLFOTempo(m.bpm)
		}

	case 28: // LFO2 Mode (toggle Time/Steps)
		lfo := tb.GetLFO2()
		if lfo.GetSyncMode() == dsp.SyncModeTime {
			lfo.SetSyncMode(dsp.SyncModeSteps)
			if lfo.GetStepsPerCycle() == 0 {
				lfo.SetStepsPerCycle(16) // Default to 16 steps
			}
		} else {
			lfo.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo() // Recalculate rate
	}

	// Mark sound parameters as dirty
	m.soundParamsDirty = true
}

// toggleWaveform toggles the waveform of the selected synth
func (m *Model) toggleWaveform() {
	var tb *synth.TB303
	if m.selectedSynth == 0 {
		tb = m.engine.GetTB303()
	} else {
		tb = m.engine.GetTB303_2()
	}

	// Toggle between Saw and Square (the two main TB-303 waveforms)
	current := tb.GetWaveform()
	if current == synth.WaveformSaw {
		tb.SetWaveform(synth.WaveformSquare)
	} else {
		tb.SetWaveform(synth.WaveformSaw)
	}
}

// cycleLFO1Target cycles the LFO1 modulation target
func (m *Model) cycleLFO1Target(tb *synth.TB303, direction int) {
	current := tb.GetLFO1Target()
	targets := []synth.LFOTarget{
		synth.LFOTargetNone,
		synth.LFOTargetCutoff,
		synth.LFOTargetResonance,
		synth.LFOTargetPitch,
		synth.LFOTargetEnvMod,
		synth.LFOTargetPulseWidth,
		synth.LFOTargetVolume,
	}

	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	tb.SetLFO1Target(targets[nextIdx])
}

// cycleLFO1Waveform cycles the LFO1 waveform
func (m *Model) cycleLFO1Waveform(tb *synth.TB303, direction int) {
	lfo := tb.GetLFO1()
	m.cycleLFOWaveform(lfo, direction)
}

// cycleLFO2Target cycles the LFO2 modulation target
func (m *Model) cycleLFO2Target(tb *synth.TB303, direction int) {
	current := tb.GetLFO2Target()
	targets := []synth.LFOTarget{
		synth.LFOTargetNone,
		synth.LFOTargetCutoff,
		synth.LFOTargetResonance,
		synth.LFOTargetPitch,
		synth.LFOTargetEnvMod,
		synth.LFOTargetPulseWidth,
		synth.LFOTargetVolume,
	}

	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	tb.SetLFO2Target(targets[nextIdx])
}

// cycleLFO2Waveform cycles the LFO2 waveform
func (m *Model) cycleLFO2Waveform(tb *synth.TB303, direction int) {
	lfo := tb.GetLFO2()
	m.cycleLFOWaveform(lfo, direction)
}

// cycleKickLFO1Target cycles the kick LFO1 target
func (m *Model) cycleKickLFO1Target(kick *drums.KickDrum, direction int) {
	current := kick.GetLFO1Target()
	targets := []drums.KickLFOTarget{
		drums.KickLFOTargetNone,
		drums.KickLFOTargetPitch,
		drums.KickLFOTargetDecay,
		drums.KickLFOTargetPitchDecay,
		drums.KickLFOTargetDrive,
	}

	// Find current index
	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	kick.SetLFO1Target(targets[nextIdx])
}

// cycleSnareLFO1Target cycles the snare LFO1 target
func (m *Model) cycleSnareLFO1Target(snare *drums.SnareDrum, direction int) {
	current := snare.GetLFO1Target()
	targets := []drums.SnareLFOTarget{
		drums.SnareLFOTargetNone,
		drums.SnareLFOTargetToneMix,
		drums.SnareLFOTargetSnap,
		drums.SnareLFOTargetNoiseDecay,
		drums.SnareLFOTargetToneDecay,
	}

	// Find current index
	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	snare.SetLFO1Target(targets[nextIdx])
}

// cycleHiHatLFO1Target cycles the hi-hat LFO1 target
func (m *Model) cycleHiHatLFO1Target(hh *drums.HiHat, direction int) {
	current := hh.GetLFO1Target()
	targets := []drums.HiHatLFOTarget{
		drums.HiHatLFOTargetNone,
		drums.HiHatLFOTargetTone,
		drums.HiHatLFOTargetNoiseMix,
		drums.HiHatLFOTargetDecay,
	}

	// Find current index
	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	hh.SetLFO1Target(targets[nextIdx])
}

// cycleClapLFO1Target cycles the clap LFO1 target
func (m *Model) cycleClapLFO1Target(clap *drums.ClapDrum, direction int) {
	current := clap.GetLFO1Target()
	targets := []drums.ClapLFOTarget{
		drums.ClapLFOTargetNone,
		drums.ClapLFOTargetTune,
		drums.ClapLFOTargetDecay,
		drums.ClapLFOTargetTone,
		drums.ClapLFOTargetDrive,
	}

	// Find current index
	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	clap.SetLFO1Target(targets[nextIdx])
}

// cycleClapLFO2Target cycles the clap LFO2 target
func (m *Model) cycleClapLFO2Target(clap *drums.ClapDrum, direction int) {
	current := clap.GetLFO2Target()
	targets := []drums.ClapLFOTarget{
		drums.ClapLFOTargetNone,
		drums.ClapLFOTargetTune,
		drums.ClapLFOTargetDecay,
		drums.ClapLFOTargetTone,
		drums.ClapLFOTargetDrive,
	}

	// Find current index
	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	clap.SetLFO2Target(targets[nextIdx])
}

// cycleRimshotLFO1Target cycles the rimshot LFO1 target
func (m *Model) cycleRimshotLFO1Target(rimshot *drums.RimshotDrum, direction int) {
	current := rimshot.GetLFO1Target()
	targets := []drums.RimshotLFOTarget{
		drums.RimshotLFOTargetNone,
		drums.RimshotLFOTargetPitch,
		drums.RimshotLFOTargetDecay,
		drums.RimshotLFOTargetTone,
		drums.RimshotLFOTargetSnap,
	}

	// Find current index
	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	rimshot.SetLFO1Target(targets[nextIdx])
}

// cycleRimshotLFO2Target cycles the rimshot LFO2 target
func (m *Model) cycleRimshotLFO2Target(rimshot *drums.RimshotDrum, direction int) {
	current := rimshot.GetLFO2Target()
	targets := []drums.RimshotLFOTarget{
		drums.RimshotLFOTargetNone,
		drums.RimshotLFOTargetPitch,
		drums.RimshotLFOTargetDecay,
		drums.RimshotLFOTargetTone,
		drums.RimshotLFOTargetSnap,
	}

	// Find current index
	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	rimshot.SetLFO2Target(targets[nextIdx])
}

// cycleCrashLFO1Target cycles the crash LFO1 target
func (m *Model) cycleCrashLFO1Target(crash *drums.CrashDrum, direction int) {
	current := crash.GetLFO1Target()
	targets := []drums.CrashLFOTarget{
		drums.CrashLFOTargetNone,
		drums.CrashLFOTargetDecay,
		drums.CrashLFOTargetTone,
		drums.CrashLFOTargetMetallic,
	}

	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	crash.SetLFO1Target(targets[nextIdx])
}

// cycleCrashLFO2Target cycles the crash LFO2 target
func (m *Model) cycleCrashLFO2Target(crash *drums.CrashDrum, direction int) {
	current := crash.GetLFO2Target()
	targets := []drums.CrashLFOTarget{
		drums.CrashLFOTargetNone,
		drums.CrashLFOTargetDecay,
		drums.CrashLFOTargetTone,
		drums.CrashLFOTargetMetallic,
	}

	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	crash.SetLFO2Target(targets[nextIdx])
}

// cycleTomLFO1Target cycles the tom LFO1 target
func (m *Model) cycleTomLFO1Target(tom *drums.TomDrum, direction int) {
	current := tom.GetLFO1Target()
	targets := []drums.TomLFOTarget{
		drums.TomLFOTargetNone,
		drums.TomLFOTargetPitch,
		drums.TomLFOTargetDecay,
		drums.TomLFOTargetTune,
		drums.TomLFOTargetPitchDecay,
		drums.TomLFOTargetTone,
		drums.TomLFOTargetAttack,
	}

	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	tom.SetLFO1Target(targets[nextIdx])
}

// cycleTomLFO2Target cycles the tom LFO2 target
func (m *Model) cycleTomLFO2Target(tom *drums.TomDrum, direction int) {
	current := tom.GetLFO2Target()
	targets := []drums.TomLFOTarget{
		drums.TomLFOTargetNone,
		drums.TomLFOTargetPitch,
		drums.TomLFOTargetDecay,
		drums.TomLFOTargetTune,
		drums.TomLFOTargetPitchDecay,
		drums.TomLFOTargetTone,
		drums.TomLFOTargetAttack,
	}

	currentIdx := 0
	for i, t := range targets {
		if t == current {
			currentIdx = i
			break
		}
	}

	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(targets) - 1
	} else if nextIdx >= len(targets) {
		nextIdx = 0
	}

	tom.SetLFO2Target(targets[nextIdx])
}

// cycleLFOWaveform cycles the LFO waveform
func (m *Model) cycleLFOWaveform(lfo *dsp.LFO, direction int) {
	current := lfo.GetWaveform()
	waveforms := []dsp.LFOWaveform{
		dsp.LFOSine,
		dsp.LFOTriangle,
		dsp.LFOSquare,
		dsp.LFOSawUp,
		dsp.LFOSawDown,
		dsp.LFORandom,
	}

	// Find current index
	currentIdx := 0
	for i, w := range waveforms {
		if w == current {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(waveforms) - 1
	} else if nextIdx >= len(waveforms) {
		nextIdx = 0
	}

	lfo.SetWaveform(waveforms[nextIdx])
}

// cycleLFOTempoRate cycles the LFO tempo sync rate
func (m *Model) cycleLFOTempoRate(lfo *dsp.LFO, direction int) {
	// Musical divisions from fast to slow: 1/1, 1/2, 1/4, 1/8, 1/16, 1/32, 1/64, 1/128, 1/256
	// At 120 BPM: 1/256 = one cycle every ~128 seconds (ultra-slow sweeps)
	divisions := []float32{1.0, 0.5, 0.25, 0.125, 0.0625, 0.03125, 0.015625, 0.0078125, 0.00390625}
	current := lfo.GetSyncRate()

	// Find current index
	currentIdx := 0
	for i, div := range divisions {
		if div == current {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(divisions) - 1
	} else if nextIdx >= len(divisions) {
		nextIdx = 0
	}

	lfo.SetSyncRate(divisions[nextIdx])
	// Update actual rate based on BPM
	lfo.UpdateTempoRate(m.bpm)
}

// adjustLFOSyncRate adjusts the tempo sync rate (musical divisions)
func (m *Model) adjustLFOSyncRate(lfo *dsp.LFO, delta float32) {
	// Musical divisions from fast to slow: 1/1, 1/2, 1/4, 1/8, 1/16, 1/32, 1/64, 1/128, 1/256
	// Smaller numbers = slower modulation (longer period)
	// At 120 BPM: 1/256 = one cycle every ~128 seconds (ultra-slow sweeps)
	divisions := []float32{1.0, 0.5, 0.25, 0.125, 0.0625, 0.03125, 0.015625, 0.0078125, 0.00390625}
	current := lfo.GetSyncRate()

	// Find current index
	currentIdx := 0
	for i, div := range divisions {
		if current == div {
			currentIdx = i
			break
		}
	}

	// Adjust index
	if delta > 0 && currentIdx < len(divisions)-1 {
		currentIdx++
	} else if delta < 0 && currentIdx > 0 {
		currentIdx--
	}

	lfo.SetSyncRate(divisions[currentIdx])
	// Update actual rate based on BPM
	lfo.UpdateTempoRate(m.bpm)
}

// adjustDroneParam adjusts the currently selected drone parameter
func (m *Model) adjustDroneParam(delta float32) {
	drone := m.engine.GetDrone()

	switch m.selectedDroneParam {
	case 0: // Master Volume
		val := drone.GetMasterVolume() + delta
		drone.SetMasterVolume(clamp(val, 0.0, 1.0))

	case 1: // Master Tune (semitones, -24 to +24)
		tune := drone.GetMasterTune()
		if delta > 0 {
			tune++
			if tune > 24 {
				tune = 24
			}
		} else if delta < 0 {
			tune--
			if tune < -24 {
				tune = -24
			}
		}
		drone.SetMasterTune(tune)

	// Case 2 is separator, skipped

	case 3: // Osc1 Level
		val := drone.GetOsc1Level() + delta
		drone.SetOsc1Level(clamp(val, 0.0, 1.0))

	case 4: // Osc1 Waveform
		m.cycleDroneOscWaveform(1)

	case 5: // Osc1 PW
		val := drone.GetOsc1PW() + delta
		drone.SetOsc1PW(clamp(val, 0.01, 0.99))

	case 6: // Osc2 Level
		val := drone.GetOsc2Level() + delta
		drone.SetOsc2Level(clamp(val, 0.0, 1.0))

	case 7: // Osc2 Waveform
		m.cycleDroneOscWaveform(2)

	case 8: // Osc2 PW
		val := drone.GetOsc2PW() + delta
		drone.SetOsc2PW(clamp(val, 0.01, 0.99))

	case 9: // Osc2 Detune (cents, -50 to +50)
		val := drone.GetOsc2Detune() + delta*10.0 // Scale for cents
		drone.SetOsc2Detune(clamp(val, -50.0, 50.0))

	case 10: // Osc3 Level
		val := drone.GetOsc3Level() + delta
		drone.SetOsc3Level(clamp(val, 0.0, 1.0))

	case 11: // Osc3 Waveform
		m.cycleDroneOscWaveform(3)

	case 12: // Osc3 PW
		val := drone.GetOsc3PW() + delta
		drone.SetOsc3PW(clamp(val, 0.01, 0.99))

	case 13: // Osc3 Detune (cents, -50 to +50)
		val := drone.GetOsc3Detune() + delta*10.0 // Scale for cents
		drone.SetOsc3Detune(clamp(val, -50.0, 50.0))

	// Case 14 is separator, skipped

	case 15: // Sub1 Level
		val := drone.GetSub1Level() + delta
		drone.SetSub1Level(clamp(val, 0.0, 1.0))

	case 16: // Sub2 Level
		val := drone.GetSub2Level() + delta
		drone.SetSub2Level(clamp(val, 0.0, 1.0))

	case 17: // Noise Level
		val := drone.GetNoiseLevel() + delta
		drone.SetNoiseLevel(clamp(val, 0.0, 1.0))

	// Case 18 is separator, skipped

	case 19: // Unison Voices (1-7)
		voices := drone.GetUnisonVoices()
		if delta > 0 {
			voices++
			if voices > 7 {
				voices = 7
			}
		} else if delta < 0 {
			voices--
			if voices < 1 {
				voices = 1
			}
		}
		drone.SetUnisonVoices(voices)

	case 20: // Unison Detune (0-50 cents)
		val := drone.GetUnisonDetune() + delta*10.0 // Scale for cents
		drone.SetUnisonDetune(clamp(val, 0.0, 50.0))

	case 21: // Unison Width (0.0-1.0)
		val := drone.GetUnisonWidth() + delta
		drone.SetUnisonWidth(clamp(val, 0.0, 1.0))

	// Case 22 is separator, skipped

	case 23: // Ring Mod
		val := drone.GetRingModAmount() + delta
		drone.SetRingModAmount(clamp(val, 0.0, 1.0))

	case 24: // Drive
		val := drone.GetDriveAmount() + delta
		drone.SetDriveAmount(clamp(val, 0.0, 1.0))

	// Case 25 is separator, skipped

	case 26: // Cutoff
		val := drone.GetCutoff() + delta
		drone.SetCutoff(clamp(val, 0.0, 1.0))

	case 27: // Resonance
		val := drone.GetResonance() + delta
		drone.SetResonance(clamp(val, 0.0, 0.95))

	case 28: // Env Mod
		val := drone.GetEnvMod() + delta
		drone.SetEnvMod(clamp(val, 0.0, 1.0))

	case 29: // Attack (0.001-2.0 sec)
		val := drone.GetAttack() + delta*2.0 // Scale for seconds
		drone.SetAttack(clamp(val, 0.001, 2.0))

	case 30: // Decay (0.1-10.0 sec)
		val := drone.GetDecay() + delta*10.0 // Scale for seconds
		drone.SetDecay(clamp(val, 0.1, 10.0))

	case 31: // Sustain (0.0-1.0)
		val := drone.GetSustain() + delta
		drone.SetSustain(clamp(val, 0.0, 1.0))

	case 32: // Release (0.1-5.0 sec)
		val := drone.GetRelease() + delta*5.0 // Scale for seconds
		drone.SetRelease(clamp(val, 0.1, 5.0))

	// Case 33 is separator, skipped

	case 34: // LFO1 Enable (toggle)
		drone.SetLFO1Enabled(!drone.GetLFO1Enabled())

	case 35: // LFO1 Target
		m.cycleDroneLFOTarget(1)

	case 36: // LFO1 Waveform
		m.cycleDroneLFOWaveform(1)

	case 37: // LFO1 Sync (toggle)
		lfo := drone.GetLFO1()
		lfo.SetTempoSync(!lfo.GetTempoSync())
		if lfo.GetTempoSync() {
			// Update rate based on current BPM
			drone.UpdateLFOTempo(m.bpm)
		}

	case 38: // LFO1 Steps
		lfo := drone.GetLFO1()
		steps := lfo.GetStepsPerCycle()
		if delta > 0 {
			steps++
			if steps > 256 {
				steps = 256
			}
		} else if delta < 0 {
			steps--
			if steps < 1 {
				steps = 1
			}
		}
		lfo.SetStepsPerCycle(steps)
		// Ensure we're in step mode when adjusting steps
		lfo.SetSyncMode(dsp.SyncModeSteps)
		// Update rate based on new step count
		drone.UpdateLFOTempo(m.bpm)

	case 39: // LFO1 Depth
		lfo := drone.GetLFO1()
		val := lfo.GetDepth() + delta
		lfo.SetDepth(clamp(val, 0.0, 1.0))

	case 40: // LFO1 Min
		lfo := drone.GetLFO1()
		min := lfo.GetMin()
		max := lfo.GetMax()
		min = clamp(min+delta, 0.0, max-0.05)
		lfo.SetRange(min, max)

	case 41: // LFO1 Max
		lfo := drone.GetLFO1()
		min := lfo.GetMin()
		max := lfo.GetMax()
		max = clamp(max+delta, min+0.05, 1.0)
		lfo.SetRange(min, max)

	// Case 42 is separator, skipped

	case 43: // LFO2 Enable (toggle)
		drone.SetLFO2Enabled(!drone.GetLFO2Enabled())

	case 44: // LFO2 Target
		m.cycleDroneLFOTarget(2)

	case 45: // LFO2 Waveform
		m.cycleDroneLFOWaveform(2)

	case 46: // LFO2 Sync (toggle)
		lfo := drone.GetLFO2()
		lfo.SetTempoSync(!lfo.GetTempoSync())
		if lfo.GetTempoSync() {
			// Update rate based on current BPM
			drone.UpdateLFOTempo(m.bpm)
		}

	case 47: // LFO2 Steps
		lfo := drone.GetLFO2()
		steps := lfo.GetStepsPerCycle()
		if delta > 0 {
			steps++
			if steps > 256 {
				steps = 256
			}
		} else if delta < 0 {
			steps--
			if steps < 1 {
				steps = 1
			}
		}
		lfo.SetStepsPerCycle(steps)
		// Ensure we're in step mode when adjusting steps
		lfo.SetSyncMode(dsp.SyncModeSteps)
		// Update rate based on new step count
		drone.UpdateLFOTempo(m.bpm)

	case 48: // LFO2 Depth
		lfo := drone.GetLFO2()
		val := lfo.GetDepth() + delta
		lfo.SetDepth(clamp(val, 0.0, 1.0))

	case 49: // LFO2 Min
		lfo := drone.GetLFO2()
		min := lfo.GetMin()
		max := lfo.GetMax()
		min = clamp(min+delta, 0.0, max-0.05)
		lfo.SetRange(min, max)

	case 50: // LFO2 Max
		lfo := drone.GetLFO2()
		min := lfo.GetMin()
		max := lfo.GetMax()
		max = clamp(max+delta, min+0.05, 1.0)
		lfo.SetRange(min, max)
	}

	m.soundParamsDirty = true
}

// cycleDroneLFOTarget cycles through LFO targets for drone
func (m *Model) cycleDroneLFOTarget(lfoNum int) {
	drone := m.engine.GetDrone()
	var target synth.LFOTarget

	if lfoNum == 1 {
		target = drone.GetLFO1Target()
	} else {
		target = drone.GetLFO2Target()
	}

	// Cycle through: Cutoff -> Resonance -> Pitch -> Volume -> Cutoff
	var newTarget synth.LFOTarget
	switch target {
	case synth.LFOTargetCutoff:
		newTarget = synth.LFOTargetResonance
	case synth.LFOTargetResonance:
		newTarget = synth.LFOTargetPitch
	case synth.LFOTargetPitch:
		newTarget = synth.LFOTargetVolume
	case synth.LFOTargetVolume:
		newTarget = synth.LFOTargetCutoff
	default:
		newTarget = synth.LFOTargetCutoff
	}

	if lfoNum == 1 {
		drone.SetLFO1Target(newTarget)
	} else {
		drone.SetLFO2Target(newTarget)
	}
}

// cycleDroneLFOWaveform cycles through LFO waveforms for drone
func (m *Model) cycleDroneLFOWaveform(lfoNum int) {
	drone := m.engine.GetDrone()
	var lfo *dsp.LFO

	if lfoNum == 1 {
		lfo = drone.GetLFO1()
	} else {
		lfo = drone.GetLFO2()
	}

	waveform := lfo.GetWaveform()

	// Cycle through: Sine -> Triangle -> Square -> SawUp -> SawDown -> Random -> Sine
	switch waveform {
	case dsp.LFOSine:
		lfo.SetWaveform(dsp.LFOTriangle)
	case dsp.LFOTriangle:
		lfo.SetWaveform(dsp.LFOSquare)
	case dsp.LFOSquare:
		lfo.SetWaveform(dsp.LFOSawUp)
	case dsp.LFOSawUp:
		lfo.SetWaveform(dsp.LFOSawDown)
	case dsp.LFOSawDown:
		lfo.SetWaveform(dsp.LFORandom)
	case dsp.LFORandom:
		lfo.SetWaveform(dsp.LFOSine)
	default:
		lfo.SetWaveform(dsp.LFOSine)
	}
}

// adjustDrumParam adjusts the currently selected drum parameter
func (m *Model) adjustDrumParam(delta float32) {
	switch m.selectedDrum {
	case 0: // Kick
		m.adjustKickParam(delta)
	case 1: // Snare
		m.adjustSnareParam(delta)
	case 2: // HH Closed
		m.adjustHihatParam(m.engine.GetHihatClosed(), delta)
	case 3: // HH Open
		m.adjustHihatParam(m.engine.GetHihatOpen(), delta)
	case 4: // Clap
		m.adjustClapParam(delta)
	case 5: // Rimshot
		m.adjustRimshotParam(delta)
	case 6: // Crash
		m.adjustCrashParam(delta)
	case 7: // Tom
		m.adjustTomParam(delta)
	}

	// Mark sound parameters as dirty
	m.soundParamsDirty = true
}

// adjustKickParam adjusts kick drum parameters
func (m *Model) adjustKickParam(delta float32) {
	kick := m.engine.GetKick()

	switch m.selectedDrumParam {
	case 0: // Tune
		val := kick.GetTune() + delta*24.0 // Scale for semitones (-12 to +12)
		kick.SetTune(clamp(val, -12.0, 12.0))
	case 1: // Decay
		val := kick.GetDecay() + delta*2.0 // Scale for seconds
		kick.SetDecay(clamp(val, 0.05, 2.0))
	case 2: // Pitch Env
		val := kick.GetPitchDecay() + delta*0.1 // Scale for ms
		kick.SetPitchDecay(clamp(val, 0.001, 0.1))
	case 3: // Drive
		val := kick.GetDrive() + delta
		kick.SetDrive(clamp(val, 0.0, 1.0))
	case 4: // Click
		val := kick.GetClickAmount() + delta
		kick.SetClickAmount(clamp(val, 0.0, 1.0))
	case 5: // Volume
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(2) // Kick is on channel 2
		channel.Level = clamp(channel.Level+delta, 0.0, 1.0)
	// Index 6 is separator
	case 7: // LFO1 On
		kick.SetLFO1Enabled(!kick.GetLFO1Enabled())
	case 8: // LFO1 Target
		if delta > 0 {
			m.cycleKickLFO1Target(kick, 1)
		} else {
			m.cycleKickLFO1Target(kick, -1)
		}
	case 9: // LFO1 Wave
		lfo1 := kick.GetLFO1()
		if delta > 0 {
			m.cycleLFOWaveform(lfo1, 1)
		} else {
			m.cycleLFOWaveform(lfo1, -1)
		}
	case 10: // LFO1 Rate
		lfo := kick.GetLFO1()
		if lfo.GetTempoSync() {
			if lfo.GetSyncMode() == dsp.SyncModeSteps {
				// Adjust steps per cycle
				steps := lfo.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
				} else {
					steps -= 4
				}
				lfo.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo() // Update rate
			} else {
				// Adjust musical division
				m.adjustLFOSyncRate(lfo, delta)
			}
		} else {
			val := lfo.GetRate() + delta*10.0
			lfo.SetRate(clamp(val, 0.01, 100.0))
		}
	case 11: // LFO1 Depth
		lfo := kick.GetLFO1()
		val := lfo.GetDepth() + delta
		lfo.SetDepth(clamp(val, 0.0, 1.0))
	case 12: // LFO1 Min
		lfo := kick.GetLFO1()
		target := kick.GetLFO1Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		switch target {
		case drums.KickLFOTargetPitch:
			min = clamp(min+delta*5.0, 30.0, max-5.0)
		case drums.KickLFOTargetDecay:
			min = clamp(min+delta*0.1, 0.05, max-0.05)
		case drums.KickLFOTargetPitchDecay:
			min = clamp(min+delta*0.01, 0.001, max-0.01)
		case drums.KickLFOTargetDrive:
			min = clamp(min+delta, 0.0, max-0.05)
		default:
			min = clamp(min+delta, 0.0, max-0.05)
		}
		lfo.SetRange(min, max)
	case 13: // LFO1 Max
		lfo := kick.GetLFO1()
		target := kick.GetLFO1Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		switch target {
		case drums.KickLFOTargetPitch:
			max = clamp(max+delta*5.0, min+5.0, 100.0)
		case drums.KickLFOTargetDecay:
			max = clamp(max+delta*0.1, min+0.05, 2.0)
		case drums.KickLFOTargetPitchDecay:
			max = clamp(max+delta*0.01, min+0.01, 0.1)
		case drums.KickLFOTargetDrive:
			max = clamp(max+delta, min+0.05, 1.0)
		default:
			max = clamp(max+delta, min+0.05, 1.0)
		}
		lfo.SetRange(min, max)
	case 14: // LFO1 Sync
		lfo := kick.GetLFO1()
		lfo.SetTempoSync(!lfo.GetTempoSync())
	case 15: // LFO1 Mode
		lfo := kick.GetLFO1()
		if lfo.GetSyncMode() == dsp.SyncModeTime {
			lfo.SetSyncMode(dsp.SyncModeSteps)
		} else {
			lfo.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo() // Recalculate rate
	}
}

// adjustSnareParam adjusts snare drum parameters
func (m *Model) adjustSnareParam(delta float32) {
	snare := m.engine.GetSnare()

	switch m.selectedDrumParam {
	case 0: // Tune
		val := snare.GetTune() + delta*24.0
		snare.SetTune(clamp(val, -12.0, 12.0))
	case 1: // Snap
		val := snare.GetSnap() + delta
		snare.SetSnap(clamp(val, 0.0, 1.0))
	case 2: // Tone Decay
		val := snare.GetToneDecay() + delta*0.3 // Scale for 0.01-0.3s range
		snare.SetToneDecay(clamp(val, 0.01, 0.3))
	case 3: // Noise Decay
		val := snare.GetNoiseDecay() + delta*0.5 // Scale for 0.05-0.5s range
		snare.SetNoiseDecay(clamp(val, 0.05, 0.5))
	case 4: // Tone Mix
		val := snare.GetToneMix() + delta
		snare.SetToneMix(clamp(val, 0.0, 1.0))
	case 5: // Volume
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(3) // Snare is on channel 3
		channel.Level = clamp(channel.Level+delta, 0.0, 1.0)
	// Index 6 is separator
	case 7: // LFO1 On
		snare.SetLFO1Enabled(!snare.GetLFO1Enabled())
	case 8: // LFO1 Target
		if delta > 0 {
			m.cycleSnareLFO1Target(snare, 1)
		} else {
			m.cycleSnareLFO1Target(snare, -1)
		}
	case 9: // LFO1 Wave
		lfo1 := snare.GetLFO1()
		if delta > 0 {
			m.cycleLFOWaveform(lfo1, 1)
		} else {
			m.cycleLFOWaveform(lfo1, -1)
		}
	case 10: // LFO1 Rate
		lfo := snare.GetLFO1()
		if lfo.GetTempoSync() {
			if lfo.GetSyncMode() == dsp.SyncModeSteps {
				steps := lfo.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
				} else {
					steps -= 4
				}
				lfo.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			} else {
				m.adjustLFOSyncRate(lfo, delta)
			}
		} else {
			val := lfo.GetRate() + delta*10.0
			lfo.SetRate(clamp(val, 0.01, 100.0))
		}
	case 11: // LFO1 Depth
		lfo := snare.GetLFO1()
		val := lfo.GetDepth() + delta
		lfo.SetDepth(clamp(val, 0.0, 1.0))
	case 12: // LFO1 Min
		lfo := snare.GetLFO1()
		target := snare.GetLFO1Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		switch target {
		case drums.SnareLFOTargetToneMix:
			min = clamp(min+delta, 0.0, max-0.05)
		case drums.SnareLFOTargetSnap:
			min = clamp(min+delta, 0.0, max-0.05)
		case drums.SnareLFOTargetNoiseDecay:
			min = clamp(min+delta*0.05, 0.05, max-0.05)
		case drums.SnareLFOTargetToneDecay:
			min = clamp(min+delta*0.05, 0.01, max-0.01)
		default:
			min = clamp(min+delta, 0.0, max-0.05)
		}
		lfo.SetRange(min, max)
	case 13: // LFO1 Max
		lfo := snare.GetLFO1()
		target := snare.GetLFO1Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		switch target {
		case drums.SnareLFOTargetToneMix:
			max = clamp(max+delta, min+0.05, 1.0)
		case drums.SnareLFOTargetSnap:
			max = clamp(max+delta, min+0.05, 1.0)
		case drums.SnareLFOTargetNoiseDecay:
			max = clamp(max+delta*0.05, min+0.05, 0.5)
		case drums.SnareLFOTargetToneDecay:
			max = clamp(max+delta*0.05, min+0.01, 0.3)
		default:
			max = clamp(max+delta, min+0.05, 1.0)
		}
		lfo.SetRange(min, max)
	case 14: // LFO1 Sync
		lfo := snare.GetLFO1()
		lfo.SetTempoSync(!lfo.GetTempoSync())
	case 15: // LFO1 Mode
		lfo := snare.GetLFO1()
		if lfo.GetSyncMode() == dsp.SyncModeTime {
			lfo.SetSyncMode(dsp.SyncModeSteps)
		} else {
			lfo.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo()
	}
}

// adjustHihatParam adjusts hi-hat parameters (same for closed and open)
func (m *Model) adjustHihatParam(hihat interface{}, delta float32) {
	// Type assert to get the actual hihat
	hh, ok := hihat.(*drums.HiHat)
	if !ok {
		return
	}

	switch m.selectedDrumParam {
	case 0: // Decay
		val := hh.GetDecay() + delta*1.0 // Scale for seconds
		hh.SetDecay(clamp(val, 0.01, 1.0))
	case 1: // Tone
		val := hh.GetTone() + delta
		hh.SetTone(clamp(val, 0.0, 1.0))
	case 2: // Noise Mix
		val := hh.GetNoiseMix() + delta
		hh.SetNoiseMix(clamp(val, 0.0, 1.0))
	case 3: // Volume
		mixer := m.engine.GetMixer()
		var channelIdx int
		if m.selectedDrum == 2 {
			channelIdx = 4 // Closed hi-hat is on channel 4
		} else {
			channelIdx = 5 // Open hi-hat is on channel 5
		}
		channel := mixer.GetChannel(channelIdx)
		channel.Level = clamp(channel.Level+delta, 0.0, 1.0)
	case 5: // LFO1 On
		hh.SetLFO1Enabled(!hh.GetLFO1Enabled())
	case 6: // LFO1 Target
		if delta > 0 {
			m.cycleHiHatLFO1Target(hh, 1)
		} else {
			m.cycleHiHatLFO1Target(hh, -1)
		}
	case 7: // LFO1 Wave
		lfo1 := hh.GetLFO1()
		if delta > 0 {
			m.cycleLFOWaveform(lfo1, 1)
		} else {
			m.cycleLFOWaveform(lfo1, -1)
		}
	case 8: // LFO1 Rate
		lfo1 := hh.GetLFO1()
		if lfo1.GetTempoSync() {
			if lfo1.GetSyncMode() == dsp.SyncModeSteps {
				steps := lfo1.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
				} else {
					steps -= 4
				}
				lfo1.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			} else {
				if delta > 0 {
					m.cycleLFOTempoRate(lfo1, 1)
				} else {
					m.cycleLFOTempoRate(lfo1, -1)
				}
			}
		} else {
			lfo1.SetRate(clamp(lfo1.GetRate()+delta*2.0, 0.1, 20.0))
		}
	case 9: // LFO1 Depth
		lfo1 := hh.GetLFO1()
		lfo1.SetDepth(clamp(lfo1.GetDepth()+delta, 0.0, 1.0))
	case 10: // LFO1 Min
		lfo1 := hh.GetLFO1()
		target := hh.GetLFO1Target()
		min := lfo1.GetMin()
		max := lfo1.GetMax()
		switch target {
		case drums.HiHatLFOTargetTone, drums.HiHatLFOTargetNoiseMix:
			min = clamp(min+delta, 0.0, max-0.05)
		case drums.HiHatLFOTargetDecay:
			min = clamp(min+delta*0.05, 0.01, max-0.01)
		}
		lfo1.SetRange(min, max)
	case 11: // LFO1 Max
		lfo1 := hh.GetLFO1()
		target := hh.GetLFO1Target()
		min := lfo1.GetMin()
		max := lfo1.GetMax()
		switch target {
		case drums.HiHatLFOTargetTone, drums.HiHatLFOTargetNoiseMix:
			max = clamp(max+delta, min+0.05, 1.0)
		case drums.HiHatLFOTargetDecay:
			max = clamp(max+delta*0.05, min+0.01, 1.0)
		}
		lfo1.SetRange(min, max)
	case 12: // LFO1 Sync
		lfo1 := hh.GetLFO1()
		lfo1.SetTempoSync(!lfo1.GetTempoSync())
	case 13: // LFO1 Mode
		lfo1 := hh.GetLFO1()
		if lfo1.GetSyncMode() == dsp.SyncModeTime {
			lfo1.SetSyncMode(dsp.SyncModeSteps)
		} else {
			lfo1.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo()
	}
}

// adjustClapParam adjusts clap drum parameters
func (m *Model) adjustClapParam(delta float32) {
	clap := m.engine.GetClap()

	switch m.selectedDrumParam {
	case 0: // Tune
		val := clap.GetTune() + delta*24.0 // Scale for semitones (-12 to +12)
		clap.SetTune(clamp(val, -12.0, 12.0))
	case 1: // Decay
		val := clap.GetDecay() + delta*0.3 // Scale for seconds
		clap.SetDecay(clamp(val, 0.01, 0.3))
	case 2: // Tone
		val := clap.GetTone() + delta
		clap.SetTone(clamp(val, 0.0, 1.0))
	case 3: // Drive
		val := clap.GetDrive() + delta
		clap.SetDrive(clamp(val, 0.0, 1.0))
	case 4: // Volume
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(6) // Clap is on channel 6
		channel.Level = clamp(channel.Level+delta, 0.0, 1.0)
	// Index 5 is separator
	case 6: // LFO1 On
		clap.SetLFO1Enabled(!clap.GetLFO1Enabled())
	case 7: // LFO1 Target
		if delta > 0 {
			m.cycleClapLFO1Target(clap, 1)
		} else {
			m.cycleClapLFO1Target(clap, -1)
		}
	case 8: // LFO1 Wave
		lfo1 := clap.GetLFO1()
		if delta > 0 {
			m.cycleLFOWaveform(lfo1, 1)
		} else {
			m.cycleLFOWaveform(lfo1, -1)
		}
	case 9: // LFO1 Rate
		lfo1 := clap.GetLFO1()
		if lfo1.GetTempoSync() {
			if lfo1.GetSyncMode() == dsp.SyncModeSteps {
				steps := lfo1.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
				} else {
					steps -= 4
				}
				lfo1.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			} else {
				if delta > 0 {
					m.cycleLFOTempoRate(lfo1, 1)
				} else {
					m.cycleLFOTempoRate(lfo1, -1)
				}
			}
		} else {
			lfo1.SetRate(clamp(lfo1.GetRate()+delta*2.0, 0.1, 20.0))
		}
	case 10: // LFO1 Depth
		lfo1 := clap.GetLFO1()
		lfo1.SetDepth(clamp(lfo1.GetDepth()+delta, 0.0, 1.0))
	case 11: // LFO1 Min
		lfo1 := clap.GetLFO1()
		target := clap.GetLFO1Target()
		var val float32
		switch target {
		case drums.ClapLFOTargetTune:
			val = clamp(lfo1.GetMin()+delta*2.0, -12.0, 12.0)
		case drums.ClapLFOTargetDecay:
			val = clamp(lfo1.GetMin()+delta*0.05, 0.01, 0.3)
		case drums.ClapLFOTargetTone, drums.ClapLFOTargetDrive:
			val = clamp(lfo1.GetMin()+delta, 0.0, 1.0)
		}
		lfo1.SetRange(val, lfo1.GetMax())
	case 12: // LFO1 Max
		lfo1 := clap.GetLFO1()
		target := clap.GetLFO1Target()
		var val float32
		switch target {
		case drums.ClapLFOTargetTune:
			val = clamp(lfo1.GetMax()+delta*2.0, -12.0, 12.0)
		case drums.ClapLFOTargetDecay:
			val = clamp(lfo1.GetMax()+delta*0.05, 0.01, 0.3)
		case drums.ClapLFOTargetTone, drums.ClapLFOTargetDrive:
			val = clamp(lfo1.GetMax()+delta, 0.0, 1.0)
		}
		lfo1.SetRange(lfo1.GetMin(), val)
	case 13: // LFO1 Sync
		lfo1 := clap.GetLFO1()
		lfo1.SetTempoSync(!lfo1.GetTempoSync())
	case 14: // LFO1 Mode
		lfo1 := clap.GetLFO1()
		if lfo1.GetSyncMode() == dsp.SyncModeTime {
			lfo1.SetSyncMode(dsp.SyncModeSteps)
		} else {
			lfo1.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo()
	// Index 15 is separator
	case 16: // LFO2 On
		clap.SetLFO2Enabled(!clap.GetLFO2Enabled())
	case 17: // LFO2 Target
		if delta > 0 {
			m.cycleClapLFO2Target(clap, 1)
		} else {
			m.cycleClapLFO2Target(clap, -1)
		}
	case 18: // LFO2 Wave
		lfo2 := clap.GetLFO2()
		if delta > 0 {
			m.cycleLFOWaveform(lfo2, 1)
		} else {
			m.cycleLFOWaveform(lfo2, -1)
		}
	case 19: // LFO2 Rate
		lfo2 := clap.GetLFO2()
		if lfo2.GetTempoSync() {
			if lfo2.GetSyncMode() == dsp.SyncModeSteps {
				steps := lfo2.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
				} else {
					steps -= 4
				}
				lfo2.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			} else {
				if delta > 0 {
					m.cycleLFOTempoRate(lfo2, 1)
				} else {
					m.cycleLFOTempoRate(lfo2, -1)
				}
			}
		} else {
			lfo2.SetRate(clamp(lfo2.GetRate()+delta*2.0, 0.1, 20.0))
		}
	case 20: // LFO2 Depth
		lfo2 := clap.GetLFO2()
		lfo2.SetDepth(clamp(lfo2.GetDepth()+delta, 0.0, 1.0))
	case 21: // LFO2 Min
		lfo2 := clap.GetLFO2()
		target := clap.GetLFO2Target()
		var val float32
		switch target {
		case drums.ClapLFOTargetTune:
			val = clamp(lfo2.GetMin()+delta*2.0, -12.0, 12.0)
		case drums.ClapLFOTargetDecay:
			val = clamp(lfo2.GetMin()+delta*0.05, 0.01, 0.3)
		case drums.ClapLFOTargetTone, drums.ClapLFOTargetDrive:
			val = clamp(lfo2.GetMin()+delta, 0.0, 1.0)
		}
		lfo2.SetRange(val, lfo2.GetMax())
	case 22: // LFO2 Max
		lfo2 := clap.GetLFO2()
		target := clap.GetLFO2Target()
		var val float32
		switch target {
		case drums.ClapLFOTargetTune:
			val = clamp(lfo2.GetMax()+delta*2.0, -12.0, 12.0)
		case drums.ClapLFOTargetDecay:
			val = clamp(lfo2.GetMax()+delta*0.05, 0.01, 0.3)
		case drums.ClapLFOTargetTone, drums.ClapLFOTargetDrive:
			val = clamp(lfo2.GetMax()+delta, 0.0, 1.0)
		}
		lfo2.SetRange(lfo2.GetMin(), val)
	case 23: // LFO2 Sync
		lfo2 := clap.GetLFO2()
		lfo2.SetTempoSync(!lfo2.GetTempoSync())
	case 24: // LFO2 Mode
		lfo2 := clap.GetLFO2()
		if lfo2.GetSyncMode() == dsp.SyncModeTime {
			lfo2.SetSyncMode(dsp.SyncModeSteps)
		} else {
			lfo2.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo()
	}
}

// adjustRimshotParam adjusts rimshot drum parameters
func (m *Model) adjustRimshotParam(delta float32) {
	rimshot := m.engine.GetRimshot()

	switch m.selectedDrumParam {
	case 0: // Pitch
		val := rimshot.GetPitch() + delta*1000.0 // Scale for Hz (200-800)
		rimshot.SetPitch(clamp(val, 200.0, 800.0))
	case 1: // Decay
		val := rimshot.GetDecay() + delta*0.3 // Scale for seconds
		rimshot.SetDecay(clamp(val, 0.01, 0.15))
	case 2: // Tone
		val := rimshot.GetTone() + delta
		rimshot.SetTone(clamp(val, 0.0, 1.0))
	case 3: // Snap
		val := rimshot.GetSnap() + delta
		rimshot.SetSnap(clamp(val, 0.0, 1.0))
	case 4: // Volume
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(7) // Rimshot is on channel 7
		channel.Level = clamp(channel.Level+delta, 0.0, 1.0)
	// Index 5 is separator
	case 6: // LFO1 On
		rimshot.SetLFO1Enabled(!rimshot.GetLFO1Enabled())
	case 7: // LFO1 Target
		if delta > 0 {
			m.cycleRimshotLFO1Target(rimshot, 1)
		} else {
			m.cycleRimshotLFO1Target(rimshot, -1)
		}
	case 8: // LFO1 Wave
		lfo1 := rimshot.GetLFO1()
		if delta > 0 {
			m.cycleLFOWaveform(lfo1, 1)
		} else {
			m.cycleLFOWaveform(lfo1, -1)
		}
	case 9: // LFO1 Rate
		lfo1 := rimshot.GetLFO1()
		if lfo1.GetTempoSync() {
			if lfo1.GetSyncMode() == dsp.SyncModeSteps {
				steps := lfo1.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
				} else {
					steps -= 4
				}
				lfo1.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			} else {
				if delta > 0 {
					m.cycleLFOTempoRate(lfo1, 1)
				} else {
					m.cycleLFOTempoRate(lfo1, -1)
				}
			}
		} else {
			lfo1.SetRate(clamp(lfo1.GetRate()+delta*2.0, 0.1, 20.0))
		}
	case 10: // LFO1 Depth
		lfo1 := rimshot.GetLFO1()
		lfo1.SetDepth(clamp(lfo1.GetDepth()+delta, 0.0, 1.0))
	case 11: // LFO1 Min
		lfo1 := rimshot.GetLFO1()
		target := rimshot.GetLFO1Target()
		min := lfo1.GetMin()
		max := lfo1.GetMax()
		switch target {
		case drums.RimshotLFOTargetPitch:
			min = clamp(min+delta*20.0, 200.0, max-10.0)
		case drums.RimshotLFOTargetDecay:
			min = clamp(min+delta*0.01, 0.01, max-0.01)
		case drums.RimshotLFOTargetTone:
			min = clamp(min+delta, 0.0, max-0.05)
		case drums.RimshotLFOTargetSnap:
			min = clamp(min+delta, 0.0, max-0.05)
		}
		lfo1.SetRange(min, max)
	case 12: // LFO1 Max
		lfo1 := rimshot.GetLFO1()
		target := rimshot.GetLFO1Target()
		min := lfo1.GetMin()
		max := lfo1.GetMax()
		switch target {
		case drums.RimshotLFOTargetPitch:
			max = clamp(max+delta*20.0, min+10.0, 800.0)
		case drums.RimshotLFOTargetDecay:
			max = clamp(max+delta*0.01, min+0.01, 0.15)
		case drums.RimshotLFOTargetTone:
			max = clamp(max+delta, min+0.05, 1.0)
		case drums.RimshotLFOTargetSnap:
			max = clamp(max+delta, min+0.05, 1.0)
		}
		lfo1.SetRange(min, max)
	case 13: // LFO1 Sync
		lfo1 := rimshot.GetLFO1()
		lfo1.SetTempoSync(!lfo1.GetTempoSync())
	case 14: // LFO1 Mode
		lfo1 := rimshot.GetLFO1()
		if lfo1.GetSyncMode() == dsp.SyncModeTime {
			lfo1.SetSyncMode(dsp.SyncModeSteps)
		} else {
			lfo1.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo()
	// Index 15 is separator
	case 16: // LFO2 On
		rimshot.SetLFO2Enabled(!rimshot.GetLFO2Enabled())
	case 17: // LFO2 Target
		if delta > 0 {
			m.cycleRimshotLFO2Target(rimshot, 1)
		} else {
			m.cycleRimshotLFO2Target(rimshot, -1)
		}
	case 18: // LFO2 Wave
		lfo2 := rimshot.GetLFO2()
		if delta > 0 {
			m.cycleLFOWaveform(lfo2, 1)
		} else {
			m.cycleLFOWaveform(lfo2, -1)
		}
	case 19: // LFO2 Rate
		lfo2 := rimshot.GetLFO2()
		if lfo2.GetTempoSync() {
			if lfo2.GetSyncMode() == dsp.SyncModeSteps {
				steps := lfo2.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
				} else {
					steps -= 4
				}
				lfo2.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			} else {
				if delta > 0 {
					m.cycleLFOTempoRate(lfo2, 1)
				} else {
					m.cycleLFOTempoRate(lfo2, -1)
				}
			}
		} else {
			lfo2.SetRate(clamp(lfo2.GetRate()+delta*2.0, 0.1, 20.0))
		}
	case 20: // LFO2 Depth
		lfo2 := rimshot.GetLFO2()
		lfo2.SetDepth(clamp(lfo2.GetDepth()+delta, 0.0, 1.0))
	case 21: // LFO2 Min
		lfo2 := rimshot.GetLFO2()
		target := rimshot.GetLFO2Target()
		min := lfo2.GetMin()
		max := lfo2.GetMax()
		switch target {
		case drums.RimshotLFOTargetPitch:
			min = clamp(min+delta*20.0, 200.0, max-10.0)
		case drums.RimshotLFOTargetDecay:
			min = clamp(min+delta*0.01, 0.01, max-0.01)
		case drums.RimshotLFOTargetTone:
			min = clamp(min+delta, 0.0, max-0.05)
		case drums.RimshotLFOTargetSnap:
			min = clamp(min+delta, 0.0, max-0.05)
		}
		lfo2.SetRange(min, max)
	case 22: // LFO2 Max
		lfo2 := rimshot.GetLFO2()
		target := rimshot.GetLFO2Target()
		min := lfo2.GetMin()
		max := lfo2.GetMax()
		switch target {
		case drums.RimshotLFOTargetPitch:
			max = clamp(max+delta*20.0, min+10.0, 800.0)
		case drums.RimshotLFOTargetDecay:
			max = clamp(max+delta*0.01, min+0.01, 0.15)
		case drums.RimshotLFOTargetTone:
			max = clamp(max+delta, min+0.05, 1.0)
		case drums.RimshotLFOTargetSnap:
			max = clamp(max+delta, min+0.05, 1.0)
		}
		lfo2.SetRange(min, max)
	case 23: // LFO2 Sync
		lfo2 := rimshot.GetLFO2()
		lfo2.SetTempoSync(!lfo2.GetTempoSync())
	case 24: // LFO2 Mode
		lfo2 := rimshot.GetLFO2()
		if lfo2.GetSyncMode() == dsp.SyncModeTime {
			lfo2.SetSyncMode(dsp.SyncModeSteps)
		} else {
			lfo2.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo()
	}
}

// adjustCrashParam adjusts crash cymbal parameters
func (m *Model) adjustCrashParam(delta float32) {
	crash := m.engine.GetCrash()

	switch m.selectedDrumParam {
	case 0: // Decay
		val := crash.GetDecay() + delta*5.0 // Scale for seconds (0.3-3.0)
		crash.SetDecay(clamp(val, 0.3, 3.0))
	case 1: // Tone
		val := crash.GetTone() + delta
		crash.SetTone(clamp(val, 0.0, 1.0))
	case 2: // Metallic
		val := crash.GetMetallic() + delta
		crash.SetMetallic(clamp(val, 0.0, 1.0))
	case 3: // Volume
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(8) // Crash is on channel 8
		channel.Level = clamp(channel.Level+delta, 0.0, 1.0)
	// Index 4 is separator
	case 5: // LFO1 On
		crash.SetLFO1Enabled(!crash.GetLFO1Enabled())
	case 6: // LFO1 Target
		if delta > 0 {
			m.cycleCrashLFO1Target(crash, 1)
		} else {
			m.cycleCrashLFO1Target(crash, -1)
		}
	case 7: // LFO1 Wave
		lfo1 := crash.GetLFO1()
		if delta > 0 {
			m.cycleLFOWaveform(lfo1, 1)
		} else {
			m.cycleLFOWaveform(lfo1, -1)
		}
	case 8: // LFO1 Rate
		lfo1 := crash.GetLFO1()
		if lfo1.GetTempoSync() {
			if lfo1.GetSyncMode() == dsp.SyncModeSteps {
				steps := lfo1.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
				} else {
					steps -= 4
				}
				lfo1.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			} else {
				if delta > 0 {
					m.cycleLFOTempoRate(lfo1, 1)
				} else {
					m.cycleLFOTempoRate(lfo1, -1)
				}
			}
		} else {
			lfo1.SetRate(clamp(lfo1.GetRate()+delta*2.0, 0.1, 20.0))
		}
	case 9: // LFO1 Depth
		lfo1 := crash.GetLFO1()
		lfo1.SetDepth(clamp(lfo1.GetDepth()+delta, 0.0, 1.0))
	case 10: // LFO1 Min
		lfo1 := crash.GetLFO1()
		target := crash.GetLFO1Target()
		min := lfo1.GetMin()
		max := lfo1.GetMax()

		switch target {
		case drums.CrashLFOTargetDecay:
			min = clamp(min+delta*0.2, 0.3, max-0.05)
		case drums.CrashLFOTargetTone, drums.CrashLFOTargetMetallic:
			min = clamp(min+delta, 0.0, max-0.05)
		default:
			min = clamp(min+delta, 0.0, max-0.05)
		}
		lfo1.SetRange(min, max)
	case 11: // LFO1 Max
		lfo1 := crash.GetLFO1()
		target := crash.GetLFO1Target()
		min := lfo1.GetMin()
		max := lfo1.GetMax()

		switch target {
		case drums.CrashLFOTargetDecay:
			max = clamp(max+delta*0.2, min+0.05, 3.0)
		case drums.CrashLFOTargetTone, drums.CrashLFOTargetMetallic:
			max = clamp(max+delta, min+0.05, 1.0)
		default:
			max = clamp(max+delta, min+0.05, 1.0)
		}
		lfo1.SetRange(min, max)
	case 12: // LFO1 Sync
		lfo1 := crash.GetLFO1()
		lfo1.SetTempoSync(!lfo1.GetTempoSync())
	case 13: // LFO1 Mode
		lfo1 := crash.GetLFO1()
		if lfo1.GetSyncMode() == dsp.SyncModeTime {
			lfo1.SetSyncMode(dsp.SyncModeSteps)
		} else {
			lfo1.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo()
	// Index 14 is separator
	case 15: // LFO2 On
		crash.SetLFO2Enabled(!crash.GetLFO2Enabled())
	case 16: // LFO2 Target
		if delta > 0 {
			m.cycleCrashLFO2Target(crash, 1)
		} else {
			m.cycleCrashLFO2Target(crash, -1)
		}
	case 17: // LFO2 Wave
		lfo2 := crash.GetLFO2()
		if delta > 0 {
			m.cycleLFOWaveform(lfo2, 1)
		} else {
			m.cycleLFOWaveform(lfo2, -1)
		}
	case 18: // LFO2 Rate
		lfo2 := crash.GetLFO2()
		if lfo2.GetTempoSync() {
			if lfo2.GetSyncMode() == dsp.SyncModeSteps {
				steps := lfo2.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
				} else {
					steps -= 4
				}
				lfo2.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			} else {
				if delta > 0 {
					m.cycleLFOTempoRate(lfo2, 1)
				} else {
					m.cycleLFOTempoRate(lfo2, -1)
				}
			}
		} else {
			lfo2.SetRate(clamp(lfo2.GetRate()+delta*2.0, 0.1, 20.0))
		}
	case 19: // LFO2 Depth
		lfo2 := crash.GetLFO2()
		lfo2.SetDepth(clamp(lfo2.GetDepth()+delta, 0.0, 1.0))
	case 20: // LFO2 Min
		lfo2 := crash.GetLFO2()
		target := crash.GetLFO2Target()
		min := lfo2.GetMin()
		max := lfo2.GetMax()

		switch target {
		case drums.CrashLFOTargetDecay:
			min = clamp(min+delta*0.2, 0.3, max-0.05)
		case drums.CrashLFOTargetTone, drums.CrashLFOTargetMetallic:
			min = clamp(min+delta, 0.0, max-0.05)
		default:
			min = clamp(min+delta, 0.0, max-0.05)
		}
		lfo2.SetRange(min, max)
	case 21: // LFO2 Max
		lfo2 := crash.GetLFO2()
		target := crash.GetLFO2Target()
		min := lfo2.GetMin()
		max := lfo2.GetMax()

		switch target {
		case drums.CrashLFOTargetDecay:
			max = clamp(max+delta*0.2, min+0.05, 3.0)
		case drums.CrashLFOTargetTone, drums.CrashLFOTargetMetallic:
			max = clamp(max+delta, min+0.05, 1.0)
		default:
			max = clamp(max+delta, min+0.05, 1.0)
		}
		lfo2.SetRange(min, max)
	case 22: // LFO2 Sync
		lfo2 := crash.GetLFO2()
		lfo2.SetTempoSync(!lfo2.GetTempoSync())
	case 23: // LFO2 Mode
		lfo2 := crash.GetLFO2()
		if lfo2.GetSyncMode() == dsp.SyncModeTime {
			lfo2.SetSyncMode(dsp.SyncModeSteps)
		} else {
			lfo2.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo()
	}
}

// adjustTomParam adjusts tom drum parameters
func (m *Model) adjustTomParam(delta float32) {
	tom := m.engine.GetTom()

	switch m.selectedDrumParam {
	case 0: // Pitch
		val := tom.GetPitch() + delta*500.0 // Scale for Hz (40-300)
		tom.SetPitch(clamp(val, 40.0, 300.0))
	case 1: // Decay
		val := tom.GetDecay() + delta*3.0 // Scale for seconds
		tom.SetDecay(clamp(val, 0.1, 2.0))
	case 2: // Tune
		val := tom.GetTune() + delta*24.0 // Scale for semitones (-12 to +12)
		tom.SetTune(clamp(val, -12.0, 12.0))
	case 3: // Pitch Decay
		val := tom.GetPitchDecay() + delta
		tom.SetPitchDecay(clamp(val, 0.0, 1.0))
	case 4: // Tone
		val := tom.GetTone() + delta
		tom.SetTone(clamp(val, 0.0, 1.0))
	case 5: // Attack
		val := tom.GetAttack() + delta
		tom.SetAttack(clamp(val, 0.0, 1.0))
	case 6: // Volume
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(9) // Tom is on channel 9
		channel.Level = clamp(channel.Level+delta, 0.0, 1.0)
	// Index 7 is separator
	case 8: // LFO1 On
		tom.SetLFO1Enabled(!tom.GetLFO1Enabled())
	case 9: // LFO1 Target
		if delta > 0 {
			m.cycleTomLFO1Target(tom, 1)
		} else {
			m.cycleTomLFO1Target(tom, -1)
		}
	case 10: // LFO1 Wave
		lfo1 := tom.GetLFO1()
		if delta > 0 {
			m.cycleLFOWaveform(lfo1, 1)
		} else {
			m.cycleLFOWaveform(lfo1, -1)
		}
	case 11: // LFO1 Rate
		lfo1 := tom.GetLFO1()
		if lfo1.GetTempoSync() {
			if lfo1.GetSyncMode() == dsp.SyncModeSteps {
				// Adjust steps per cycle
				steps := lfo1.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
					if steps > 64 {
						steps = 64
					}
				} else {
					steps -= 4
					if steps < 1 {
						steps = 1
					}
				}
				lfo1.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			} else {
				// Cycle musical division
				m.adjustLFOSyncRate(lfo1, delta)
			}
		} else {
			// Adjust free rate
			lfo1.SetRate(clamp(lfo1.GetRate()+delta*2.0, 0.1, 20.0))
		}
	case 12: // LFO1 Depth
		lfo1 := tom.GetLFO1()
		lfo1.SetDepth(clamp(lfo1.GetDepth()+delta, 0.0, 1.0))
	case 13: // LFO1 Min
		lfo := tom.GetLFO1()
		target := tom.GetLFO1Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		switch target {
		case drums.TomLFOTargetPitch:
			min = clamp(min+delta*10.0, 40.0, max-5.0)
		case drums.TomLFOTargetTune:
			min = clamp(min+delta*2.0, -12.0, max-1.0)
		case drums.TomLFOTargetDecay:
			min = clamp(min+delta*0.1, 0.1, max-0.05)
		case drums.TomLFOTargetPitchDecay, drums.TomLFOTargetTone, drums.TomLFOTargetAttack:
			min = clamp(min+delta, 0.0, max-0.05)
		default:
			min = clamp(min+delta, 0.0, max-0.05)
		}
		lfo.SetRange(min, max)
	case 14: // LFO1 Max
		lfo := tom.GetLFO1()
		target := tom.GetLFO1Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		switch target {
		case drums.TomLFOTargetPitch:
			max = clamp(max+delta*10.0, min+5.0, 300.0)
		case drums.TomLFOTargetTune:
			max = clamp(max+delta*2.0, min+1.0, 12.0)
		case drums.TomLFOTargetDecay:
			max = clamp(max+delta*0.1, min+0.05, 2.0)
		case drums.TomLFOTargetPitchDecay, drums.TomLFOTargetTone, drums.TomLFOTargetAttack:
			max = clamp(max+delta, min+0.05, 1.0)
		default:
			max = clamp(max+delta, min+0.05, 1.0)
		}
		lfo.SetRange(min, max)
	case 15: // LFO1 Sync
		lfo1 := tom.GetLFO1()
		lfo1.SetTempoSync(!lfo1.GetTempoSync())
	case 16: // LFO1 Mode
		lfo1 := tom.GetLFO1()
		if lfo1.GetSyncMode() == dsp.SyncModeTime {
			lfo1.SetSyncMode(dsp.SyncModeSteps)
			if lfo1.GetStepsPerCycle() == 0 {
				lfo1.SetStepsPerCycle(16) // Default to 16 steps
			}
		} else {
			lfo1.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo()
	// Index 17 is separator
	case 18: // LFO2 On
		tom.SetLFO2Enabled(!tom.GetLFO2Enabled())
	case 19: // LFO2 Target
		if delta > 0 {
			m.cycleTomLFO2Target(tom, 1)
		} else {
			m.cycleTomLFO2Target(tom, -1)
		}
	case 20: // LFO2 Wave
		lfo2 := tom.GetLFO2()
		if delta > 0 {
			m.cycleLFOWaveform(lfo2, 1)
		} else {
			m.cycleLFOWaveform(lfo2, -1)
		}
	case 21: // LFO2 Rate
		lfo2 := tom.GetLFO2()
		if lfo2.GetTempoSync() {
			if lfo2.GetSyncMode() == dsp.SyncModeSteps {
				// Adjust steps per cycle
				steps := lfo2.GetStepsPerCycle()
				if delta > 0 {
					steps += 4
					if steps > 64 {
						steps = 64
					}
				} else {
					steps -= 4
					if steps < 1 {
						steps = 1
					}
				}
				lfo2.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			} else {
				// Cycle musical division
				m.adjustLFOSyncRate(lfo2, delta)
			}
		} else {
			// Adjust free rate
			lfo2.SetRate(clamp(lfo2.GetRate()+delta*2.0, 0.1, 20.0))
		}
	case 22: // LFO2 Depth
		lfo2 := tom.GetLFO2()
		lfo2.SetDepth(clamp(lfo2.GetDepth()+delta, 0.0, 1.0))
	case 23: // LFO2 Min
		lfo := tom.GetLFO2()
		target := tom.GetLFO2Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		switch target {
		case drums.TomLFOTargetPitch:
			min = clamp(min+delta*10.0, 40.0, max-5.0)
		case drums.TomLFOTargetTune:
			min = clamp(min+delta*2.0, -12.0, max-1.0)
		case drums.TomLFOTargetDecay:
			min = clamp(min+delta*0.1, 0.1, max-0.05)
		case drums.TomLFOTargetPitchDecay, drums.TomLFOTargetTone, drums.TomLFOTargetAttack:
			min = clamp(min+delta, 0.0, max-0.05)
		default:
			min = clamp(min+delta, 0.0, max-0.05)
		}
		lfo.SetRange(min, max)
	case 24: // LFO2 Max
		lfo := tom.GetLFO2()
		target := tom.GetLFO2Target()
		min := lfo.GetMin()
		max := lfo.GetMax()

		switch target {
		case drums.TomLFOTargetPitch:
			max = clamp(max+delta*10.0, min+5.0, 300.0)
		case drums.TomLFOTargetTune:
			max = clamp(max+delta*2.0, min+1.0, 12.0)
		case drums.TomLFOTargetDecay:
			max = clamp(max+delta*0.1, min+0.05, 2.0)
		case drums.TomLFOTargetPitchDecay, drums.TomLFOTargetTone, drums.TomLFOTargetAttack:
			max = clamp(max+delta, min+0.05, 1.0)
		default:
			max = clamp(max+delta, min+0.05, 1.0)
		}
		lfo.SetRange(min, max)
	case 25: // LFO2 Sync
		lfo2 := tom.GetLFO2()
		lfo2.SetTempoSync(!lfo2.GetTempoSync())
	case 26: // LFO2 Mode
		lfo2 := tom.GetLFO2()
		if lfo2.GetSyncMode() == dsp.SyncModeTime {
			lfo2.SetSyncMode(dsp.SyncModeSteps)
			if lfo2.GetStepsPerCycle() == 0 {
				lfo2.SetStepsPerCycle(16) // Default to 16 steps
			}
		} else {
			lfo2.SetSyncMode(dsp.SyncModeTime)
		}
		m.engine.UpdateAllLFOTempo()
	}
}

// adjustEffectParam adjusts the currently selected effect parameter
func (m *Model) adjustEffectParam(delta float32) {
	mixer := m.engine.GetMixer()
	channel := mixer.GetChannel(m.selectedChannel)

	switch m.selectedEffect {
	case 0: // Distortion
		m.adjustDistortionParam(channel, delta)
	case 1: // Delay
		m.adjustDelayParam(channel, delta)
	case 2: // Reverb
		m.adjustReverbParam(channel, delta)
	case 3: // Pitch Shifter
		m.adjustPitchShifterParam(channel, delta)
	case 4: // Multimode Filter
		m.adjustMultimodeFilterParam(channel, delta)
	case 5: // Bit Crusher
		m.adjustBitCrusherParam(channel, delta)
	}
}

// adjustDistortionParam adjusts distortion parameters
func (m *Model) adjustDistortionParam(channel *audio.Channel, delta float32) {
	dist := m.findEffectInChain(channel, 0) // 0 = distortion index
	if dist == nil {
		return
	}

	distortion, ok := dist.(*effects.Distortion)
	if !ok {
		return
	}

	switch m.selectedEffectParam {
	case 0: // Enabled - toggle on/off
		distortion.SetEnabled(!distortion.IsEnabled())
	case 1: // Drive
		val := distortion.GetDrive() + delta*10.0 // Scale for drive (1-20 range)
		distortion.SetDrive(clamp(val, 1.0, 20.0))
	case 2: // Tone
		val := distortion.GetTone() + delta
		distortion.SetTone(clamp(val, 0.0, 1.0))
	case 3: // Mix
		val := distortion.GetMix() + delta
		distortion.SetMix(clamp(val, 0.0, 1.0))
	}
}

// adjustDelayParam adjusts delay parameters
func (m *Model) adjustDelayParam(channel *audio.Channel, delta float32) {
	del := m.findEffectInChain(channel, 1) // 1 = delay index
	if del == nil {
		return
	}

	delay, ok := del.(*effects.Delay)
	if !ok {
		return
	}

	switch m.selectedEffectParam {
	case 0: // Enabled - toggle on/off
		delay.SetEnabled(!delay.IsEnabled())
	case 1: // Delay Time
		val := delay.GetDelayTime() + delta*2.0 // Scale for delay time (0-2s range)
		delay.SetDelayTime(clamp(val, 0.001, 2.0))
	case 2: // Feedback
		val := delay.GetFeedback() + delta
		delay.SetFeedback(clamp(val, 0.0, 0.95))
	case 3: // Mix
		val := delay.GetMix() + delta
		delay.SetMix(clamp(val, 0.0, 1.0))
	}
}

// adjustReverbParam adjusts reverb parameters
func (m *Model) adjustReverbParam(channel *audio.Channel, delta float32) {
	rev := m.findEffectInChain(channel, 2) // 2 = reverb index
	if rev == nil {
		return
	}

	reverb, ok := rev.(*effects.Reverb)
	if !ok {
		return
	}

	switch m.selectedEffectParam {
	case 0: // Enabled - toggle on/off
		reverb.SetEnabled(!reverb.IsEnabled())
	case 1: // Room Size
		val := reverb.GetRoomSize() + delta
		reverb.SetRoomSize(clamp(val, 0.0, 1.0))
	case 2: // Damping
		val := reverb.GetDamping() + delta
		reverb.SetDamping(clamp(val, 0.0, 1.0))
	case 3: // Mix
		val := reverb.GetMix() + delta
		reverb.SetMix(clamp(val, 0.0, 1.0))
	}
}

// adjustPitchShifterParam adjusts pitch shifter parameters
func (m *Model) adjustPitchShifterParam(channel *audio.Channel, delta float32) {
	ps := m.findEffectInChain(channel, 3) // 3 = pitch shifter index
	if ps == nil {
		return
	}

	pitchShifter, ok := ps.(*effects.PitchShifter)
	if !ok {
		return
	}

	switch m.selectedEffectParam {
	case 0: // Enabled - toggle on/off
		pitchShifter.SetEnabled(!pitchShifter.IsEnabled())
	case 1: // Pitch
		val := pitchShifter.GetPitch() + delta*12.0 // Scale for semitones (-12 to +12 range)
		pitchShifter.SetPitch(clamp(val, -12.0, 12.0))
	case 2: // Ratio (read-only, skip)
		return
	case 3: // Mix
		val := pitchShifter.GetMix() + delta
		pitchShifter.SetMix(clamp(val, 0.0, 1.0))
	}
}

// adjustMultimodeFilterParam adjusts multimode filter parameters
func (m *Model) adjustMultimodeFilterParam(channel *audio.Channel, delta float32) {
	mf := m.findEffectInChain(channel, 4) // 4 = multimode filter index
	if mf == nil {
		return
	}

	filter, ok := mf.(*effects.MultimodeFilter)
	if !ok {
		return
	}

	switch m.selectedEffectParam {
	case 0: // Enabled - toggle on/off
		filter.SetEnabled(!filter.IsEnabled())
	case 1: // Cutoff (logarithmic scale)
		// Use logarithmic scaling for cutoff (20 Hz to 20 kHz)
		currentCutoff := filter.GetCutoff()
		// Scale delta exponentially
		scaleFactor := 1.0 + delta*2.0 // Small deltas translate to multipliers
		newCutoff := currentCutoff * scaleFactor
		filter.SetCutoff(clamp(newCutoff, 20.0, 20000.0))
	case 2: // Resonance
		val := filter.GetResonance() + delta
		filter.SetResonance(clamp(val, 0.0, 0.95))
	case 3: // Mode
		// Cycle through filter modes
		if delta > 0 {
			// Move forward
			currentMode := filter.GetFilterMode()
			newMode := (int(currentMode) + 1) % 3
			filter.SetFilterMode(effects.FilterMode(newMode))
		} else if delta < 0 {
			// Move backward
			currentMode := filter.GetFilterMode()
			newMode := (int(currentMode) + 2) % 3 // +2 is same as -1 with mod 3
			filter.SetFilterMode(effects.FilterMode(newMode))
		}
	case 4: // Mix
		val := filter.GetMix() + delta
		filter.SetMix(clamp(val, 0.0, 1.0))
	}
}

// adjustBitCrusherParam adjusts bit crusher parameters
func (m *Model) adjustBitCrusherParam(channel *audio.Channel, delta float32) {
	bc := m.findEffectInChain(channel, 5) // 5 = bit crusher index
	if bc == nil {
		return
	}

	crusher, ok := bc.(*effects.BitCrusher)
	if !ok {
		return
	}

	switch m.selectedEffectParam {
	case 0: // Enabled - toggle on/off
		crusher.SetEnabled(!crusher.IsEnabled())
	case 1: // Bit Depth
		val := crusher.GetBitDepth() + delta*8.0 // Scale for bit depth (1-16 range)
		crusher.SetBitDepth(clamp(val, 1.0, 16.0))
	case 2: // Crush Rate
		val := crusher.GetCrushRate() + delta*16.0 // Scale for crush rate (1-32 range)
		crusher.SetCrushRate(clamp(val, 1.0, 32.0))
	case 3: // Effective SR (read-only, skip)
		return
	case 4: // Mix
		val := crusher.GetMix() + delta
		crusher.SetMix(clamp(val, 0.0, 1.0))
	}
}

// toggleEffect toggles the currently selected effect on/off
func (m *Model) toggleEffect() {
	mixer := m.engine.GetMixer()
	channel := mixer.GetChannel(m.selectedChannel)

	effect := m.findEffectInChain(channel, m.selectedEffect)
	if effect != nil {
		effect.SetEnabled(!effect.IsEnabled())
	}
}

// adjustMixerParam adjusts the currently selected mixer parameter
func (m *Model) adjustMixerParam(delta float32) {
	mixer := m.engine.GetMixer()
	channel := mixer.GetChannel(m.selectedMixerChannel)
	if channel == nil {
		return
	}

	switch m.selectedMixerParam {
	case 0: // Level
		channel.Level = clamp(channel.Level+delta, 0.0, 1.0)
	case 1: // Pan
		channel.Pan = clamp(channel.Pan+delta, -1.0, 1.0)
	case 2: // Mute (no-op, use M key)
		return
	case 3: // Solo (no-op, use S key)
		return
	}
}

// queueMuteToggle queues a mute toggle for quantized execution
func (m *Model) queueMuteToggle(quantizeToBar bool) {
	m.pendingMuteChannel = m.selectedMixerChannel
	m.quantizeToBar = quantizeToBar
}

// queueSoloToggle queues a solo toggle for quantized execution
func (m *Model) queueSoloToggle(quantizeToBar bool) {
	m.pendingSoloChannel = m.selectedMixerChannel
	m.quantizeToBar = quantizeToBar
}

// applyPendingMixerChanges applies queued mute/solo changes at quantization boundaries
func (m *Model) applyPendingMixerChanges() {
	currentStep := m.seq.GetCurrentStep()

	// Determine if we're at a quantization boundary
	atPatternEnd := currentStep == 0         // Just wrapped from step 15 to 0
	atBarBoundary := (currentStep % 64) == 0 // Every 4 bars (64 steps)

	mixer := m.engine.GetMixer()

	// Apply pending mute toggle
	if m.pendingMuteChannel >= 0 {
		shouldApply := (!m.quantizeToBar && atPatternEnd) || (m.quantizeToBar && atBarBoundary)
		if shouldApply {
			channel := mixer.GetChannel(m.pendingMuteChannel)
			if channel != nil {
				channel.Mute = !channel.Mute
			}
			m.pendingMuteChannel = -1 // Clear pending
		}
	}

	// Apply pending solo toggle
	if m.pendingSoloChannel >= 0 {
		shouldApply := (!m.quantizeToBar && atPatternEnd) || (m.quantizeToBar && atBarBoundary)
		if shouldApply {
			channel := mixer.GetChannel(m.pendingSoloChannel)
			if channel != nil {
				channel.Solo = !channel.Solo
			}
			m.pendingSoloChannel = -1 // Clear pending
		}
	}
}

// toggleMute toggles mute on the selected mixer channel (immediate, for backwards compat)
func (m *Model) toggleMute() {
	mixer := m.engine.GetMixer()
	channel := mixer.GetChannel(m.selectedMixerChannel)
	if channel != nil {
		channel.Mute = !channel.Mute
	}
}

// toggleSolo toggles solo on the selected mixer channel (immediate, for backwards compat)
func (m *Model) toggleSolo() {
	mixer := m.engine.GetMixer()
	channel := mixer.GetChannel(m.selectedMixerChannel)
	if channel != nil {
		channel.Solo = !channel.Solo
	}
}

// getPianoKeyNote maps keyboard keys to MIDI note offsets within an octave
// Returns -1 if key is not a piano key
func (m *Model) getPianoKeyNote(key string) int {
	// Map keyboard to piano keys (one octave)
	// z x c v b n m = C D E F G A B (white keys)
	// s d   g h j   = C# D# F# G# A# (black keys)
	keyMap := map[string]int{
		"z": 0,  // C
		"s": 1,  // C#
		"x": 2,  // D
		"d": 3,  // D#
		"c": 4,  // E
		"v": 5,  // F
		"g": 6,  // F#
		"b": 7,  // G
		"h": 8,  // G#
		"n": 9,  // A
		"j": 10, // A#
		"m": 11, // B
	}

	offset, exists := keyMap[key]
	if !exists {
		return -1
	}

	// Calculate MIDI note: octave * 12 + offset
	// Octave 0 starts at MIDI note 12 (C0)
	return (m.currentOctave * 12) + offset
}

// setStepNote sets the note for the currently selected step
func (m *Model) setStepNote(note int) {
	// Only for TB-303 tracks
	if m.selectedTrack != 0 && m.selectedTrack != 1 {
		return
	}

	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil {
		return
	}

	step := track.Pattern.GetStep(m.selectedStep)
	if step == nil {
		// Create a new step if it doesn't exist
		step = &sequencer.Step{
			Active:      true,
			Note:        note,
			Velocity:    0.8,
			Probability: 1.0,
		}
	} else {
		// Update existing step
		step.Active = true
		step.Note = note
	}

	track.Pattern.SetStep(m.selectedStep, *step)
}

// generateAcidLine generates an algorithmic acid bassline
func (m *Model) generateAcidLine(sparse bool) {
	// Only for TB-303 tracks
	if m.selectedTrack != 0 && m.selectedTrack != 1 {
		return
	}

	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil {
		return
	}

	// === GRAMMAR + CONSTRAINTS APPROACH ===
	// Based on "most acid per effort" from bassline_generation.txt

	// Scale: C minor (very acid-friendly)
	rootNote := 36                           // C2
	scale := []int{0, 2, 3, 5, 7, 8, 10, 12} // C D Eb F G Ab Bb C

	// Get pattern length
	patternLength := 16
	if track.Pattern != nil {
		patternLength = track.Pattern.Length
	}

	// DENSITY RULE: Target number of hits per 16 steps
	var targetDensity int
	if sparse {
		targetDensity = 4 + rand.Intn(4) // 4-7 hits (minimal)
	} else {
		targetDensity = 7 + rand.Intn(5) // 7-11 hits (acid)
	}

	// Generate gate pattern (which steps are active)
	gates := make([]bool, 16)
	activeSteps := make([]int, 0, 16)

	// Fill to target density
	for len(activeSteps) < targetDensity && len(activeSteps) < 16 {
		step := rand.Intn(16)
		if !gates[step] {
			gates[step] = true
			activeSteps = append(activeSteps, step)
		}
	}

	// PITCH RULES: Generate notes with musical constraints
	notes := make([]int, 16)
	currentNote := rootNote + scale[rand.Intn(len(scale))] // Start on random scale note

	for i := 0; i < 16; i++ {
		if !gates[i] {
			continue
		}

		// Decide note movement
		choice := rand.Float32()

		if choice < 0.4 {
			// REPEAT: Same note (40% - very TB-303)
			notes[i] = currentNote
		} else if choice < 0.75 {
			// SMALL INTERVAL: ±1 or ±2 scale degrees (35%)
			direction := 1
			if rand.Float32() < 0.5 {
				direction = -1
			}
			steps := 1 + rand.Intn(2) // 1 or 2 scale degrees
			currentNote += direction * scale[steps]
			notes[i] = currentNote
		} else if choice < 0.95 {
			// MEDIUM INTERVAL: ±3 to ±5 semitones (20%)
			direction := 1
			if rand.Float32() < 0.5 {
				direction = -1
			}
			currentNote += direction * (3 + rand.Intn(3))
			notes[i] = currentNote
		} else {
			// FEATURE JUMP: Octave jump (5%)
			if rand.Float32() < 0.5 {
				currentNote += 12 // Up octave
			} else {
				currentNote -= 12 // Down octave
			}
			notes[i] = currentNote
		}

		// RANGE RULE: Keep within 2 octaves of root
		if currentNote < rootNote {
			currentNote = rootNote
		}
		if currentNote > rootNote+24 {
			currentNote = rootNote + 24
		}
		notes[i] = currentNote
	}

	// ACCENT DISTRIBUTION: Bias toward downbeats
	accents := make([]bool, 16)
	for i := 0; i < 16; i++ {
		if !gates[i] {
			continue
		}

		// Strong beats (0, 4, 8, 12) have higher accent chance
		accentChance := float32(0.15) // Base 15%
		if i%4 == 0 {
			accentChance = 0.5 // 50% on downbeats
		} else if i%2 == 0 {
			accentChance = 0.3 // 30% on even beats
		}

		accents[i] = rand.Float32() < accentChance
	}

	// SLIDE FREQUENCY: 15-35% of consecutive gated transitions
	slides := make([]bool, 16)
	slideChance := 0.15 + rand.Float32()*0.2 // 15-35%

	for i := 0; i < 15; i++ {
		// Slide only valid if current AND next step are gated
		if gates[i] && gates[i+1] {
			slides[i] = rand.Float32() < slideChance
		}
	}

	// Apply to pattern
	for i := 0; i < 16 && i < patternLength; i++ {
		if gates[i] {
			step := sequencer.Step{
				Active:      true,
				Note:        notes[i],
				Velocity:    0.75 + rand.Float32()*0.25, // 0.75-1.0
				Accent:      accents[i],
				Slide:       slides[i],
				Probability: 1.0,
			}
			track.Pattern.SetStep(i, step)
		} else {
			track.Pattern.SetStep(i, sequencer.Step{Active: false})
		}
	}

	// If pattern is longer than 16 steps, create variations of first 16 steps
	if patternLength > 16 {
		for i := 16; i < patternLength; i++ {
			sourceStep := track.Pattern.GetStep(i % 16) // Get corresponding step from first 16
			if sourceStep == nil {
				continue
			}

			if sourceStep.Active {
				// Create variation: 80% chance to keep the note, 20% to skip
				if rand.Float32() < 0.8 {
					variedStep := *sourceStep // Copy the source step

					// Vary the note slightly (50% same, 30% transpose by one scale degree, 20% skip)
					choice := rand.Float32()
					if choice < 0.5 {
						// Keep same note
					} else if choice < 0.8 {
						// Transpose by one scale degree up or down
						if rand.Float32() < 0.5 {
							variedStep.Note += scale[1] // Up
						} else {
							variedStep.Note -= scale[1] // Down
						}
					} else {
						// Skip this note in the variation
						variedStep.Active = false
					}

					// Vary accent (30% chance to flip)
					if rand.Float32() < 0.3 {
						variedStep.Accent = !variedStep.Accent
					}

					// Vary slide (20% chance to flip)
					if rand.Float32() < 0.2 {
						variedStep.Slide = !variedStep.Slide
					}

					track.Pattern.SetStep(i, variedStep)
				} else {
					// Skip this step in variation
					track.Pattern.SetStep(i, sequencer.Step{Active: false})
				}
			} else {
				// Source step was empty, keep it empty (or 10% chance to add a note)
				if rand.Float32() < 0.1 {
					// Add a ghost note
					track.Pattern.SetStep(i, sequencer.Step{
						Active:      true,
						Note:        currentNote,
						Velocity:    0.5 + rand.Float32()*0.2, // Quieter ghost notes
						Accent:      false,
						Slide:       false,
						Probability: 1.0,
					})
				} else {
					track.Pattern.SetStep(i, sequencer.Step{Active: false})
				}
			}
		}
	}

	// Emphasize beat 1 (first step) with accent
	step := track.Pattern.GetStep(0)
	if step != nil && step.Active {
		step.Accent = true
		track.Pattern.SetStep(0, *step)
	}
}

// randomizeTrackSound randomizes the currently selected track's sound parameters
func (m *Model) randomizeTrackSound() {
	switch m.selectedTrack {
	case 0, 1: // TB-303
		m.randomizeTB303Sound(m.selectedTrack)
	case 2: // Kick
		m.randomizeKick()
	case 3: // Snare
		m.randomizeSnare()
	case 4, 5: // Hi-hats
		m.randomizeHihat(m.selectedTrack) // Pass channel index (4 or 5)
	case 6: // Clap
		m.randomizeClap()
	case 7: // Rimshot
		m.randomizeRimshot()
	case 8: // Crash
		m.randomizeCrash()
	case 9: // Tom
		m.randomizeTom()
	case 10, 11: // Samplers (no randomization implemented yet)
		// TODO: Implement sampler randomization
	case 12: // Drone
		m.randomizeDroneSound()
	}
	m.soundParamsDirty = true // Mark as unsaved
}

// randomizeSynthSound randomizes the currently selected TB-303 sound
func (m *Model) randomizeSynthSound() {
	m.randomizeTB303Sound(m.selectedSynth)
	m.soundParamsDirty = true // Mark as unsaved
}

// randomizeDrumSound randomizes the currently selected drum sound
func (m *Model) randomizeDrumSound() {
	switch m.selectedDrum {
	case 0: // Kick
		m.randomizeKick()
	case 1: // Snare
		m.randomizeSnare()
	case 2: // HH Closed
		m.randomizeHihat(4) // Track 4
	case 3: // HH Open
		m.randomizeHihat(5) // Track 5
	case 4: // Clap
		m.randomizeClap()
	case 5: // Rimshot
		m.randomizeRimshot()
	case 6: // Crash
		m.randomizeCrash()
	case 7: // Tom
		m.randomizeTom()
	}
	m.soundParamsDirty = true // Mark as unsaved
}

// randomizeDroneSound randomizes all drone synth parameters within musical ranges
func (m *Model) randomizeDroneSound() {
	drone := m.engine.GetDrone()

	// Randomize master parameters
	drone.SetMasterVolume(0.6 + rand.Float32()*0.3) // 60-90%

	// Master tune: -12 to +12 semitones
	drone.SetMasterTune(-12 + rand.Intn(25))

	// Randomize oscillator levels (always audible)
	drone.SetOsc1Level(0.5 + rand.Float32()*0.5) // 50-100%
	drone.SetOsc2Level(0.5 + rand.Float32()*0.5) // 50-100%
	drone.SetOsc3Level(0.5 + rand.Float32()*0.5) // 50-100%

	// Randomize detuning (-30 to +30 cents)
	drone.SetOsc2Detune(-30.0 + rand.Float32()*60.0)
	drone.SetOsc3Detune(-30.0 + rand.Float32()*60.0)

	// Randomize sub-oscillator levels (strong but not overpowering)
	drone.SetSub1Level(0.3 + rand.Float32()*0.5) // 30-80%
	drone.SetSub2Level(0.3 + rand.Float32()*0.5) // 30-80%

	// Randomize filter parameters
	drone.SetCutoff(0.2 + rand.Float32()*0.6)    // 20-80%
	drone.SetResonance(0.3 + rand.Float32()*0.6) // 30-90% (favor high resonance)
	drone.SetEnvMod(0.4 + rand.Float32()*0.6)    // 40-100%

	// Randomize ADSR envelope (long values for drone character)
	drone.SetAttack(0.01 + rand.Float32()*0.49) // 10-500ms
	drone.SetDecay(1.0 + rand.Float32()*7.0)    // 1-8 sec (long decay)
	drone.SetSustain(0.6 + rand.Float32()*0.4)  // 60-100% (high sustain)
	drone.SetRelease(0.5 + rand.Float32()*2.5)  // 0.5-3.0 sec

	// LFO1: 50% chance to enable
	if rand.Float32() < 0.5 {
		drone.SetLFO1Enabled(true)

		lfo := drone.GetLFO1()

		// Slow modulation rate (0.1-2.0 Hz)
		lfo.SetRate(0.1 + rand.Float32()*1.9)

		// Moderate to strong depth
		lfo.SetDepth(0.3 + rand.Float32()*0.5) // 30-80%

		// Randomize target (favor Cutoff)
		targetRoll := rand.Float32()
		if targetRoll < 0.5 {
			drone.SetLFO1Target(synth.LFOTargetCutoff)
		} else if targetRoll < 0.75 {
			drone.SetLFO1Target(synth.LFOTargetResonance)
		} else if targetRoll < 0.9 {
			drone.SetLFO1Target(synth.LFOTargetPitch)
		} else {
			drone.SetLFO1Target(synth.LFOTargetVolume)
		}

		// Randomize waveform (favor Sine/Triangle for smooth modulation)
		waveRoll := rand.Float32()
		if waveRoll < 0.35 {
			lfo.SetWaveform(dsp.LFOSine)
		} else if waveRoll < 0.7 {
			lfo.SetWaveform(dsp.LFOTriangle)
		} else if waveRoll < 0.85 {
			lfo.SetWaveform(dsp.LFOSawUp)
		} else if waveRoll < 0.93 {
			lfo.SetWaveform(dsp.LFOSquare)
		} else {
			lfo.SetWaveform(dsp.LFORandom)
		}

		// Set min/max range (0.0-1.0 for most targets)
		minVal := rand.Float32() * 0.3     // 0-30%
		maxVal := 0.6 + rand.Float32()*0.4 // 60-100%
		lfo.SetRange(minVal, maxVal)
	} else {
		drone.SetLFO1Enabled(false)
	}
	m.soundParamsDirty = true // Mark as unsaved
}

// lockCurrentParameter locks the currently selected parameter value to the selected step
func (m *Model) lockCurrentParameter() {
	// Determine which instrument/track to lock to
	var instrument sequencer.InstrumentType
	var paramName string
	var paramValue float32
	var ok bool

	switch m.currentView {
	case ViewSynth:
		// TB-303 parameter locking
		if m.selectedSynth == 0 {
			instrument = sequencer.InstrumentTB303_1
		} else {
			instrument = sequencer.InstrumentTB303_2
		}
		paramName, paramValue, ok = m.getSynthParamLock()
		if !ok {
			return // Invalid parameter (separator, toggle, etc.)
		}

	case ViewDrone:
		// Drone parameter locking
		instrument = sequencer.InstrumentDrone
		paramName, paramValue, ok = m.getDroneParamLock()
		if !ok {
			return
		}

	case ViewDrums:
		// Drum parameter locking
		// Map selectedDrum (0-7) to instrument type
		drumInstruments := []sequencer.InstrumentType{
			sequencer.InstrumentKick,
			sequencer.InstrumentSnare,
			sequencer.InstrumentHihatClosed,
			sequencer.InstrumentHihatOpen,
			sequencer.InstrumentClap,
			sequencer.InstrumentRimshot,
			sequencer.InstrumentCrash,
			sequencer.InstrumentTom,
		}
		instrument = drumInstruments[m.selectedDrum]
		paramName, paramValue, ok = m.getDrumParamLock()
		if !ok {
			return
		}

	default:
		return // Not in a sound editing view
	}

	// Get the current pattern for this instrument
	patternID := m.seq.GetCurrentPatternID()
	pattern := m.seq.GetPattern(instrument, patternID)
	if pattern == nil {
		return
	}

	// Get the selected step
	step := pattern.GetStep(m.selectedStep)
	if step == nil {
		return
	}

	// Lock the parameter
	step.SetParamLock(paramName, paramValue)
	m.statusMessage = fmt.Sprintf("Locked %s = %.2f to step %d", paramName, paramValue, m.selectedStep+1)
	m.statusDuration = 60 // Show for 1 second (assuming 60fps ticks)
}

// getSynthParamLock returns the parameter name and normalized value (0-1) for TB-303
func (m *Model) getSynthParamLock() (string, float32, bool) {
	var tb *synth.TB303
	if m.selectedSynth == 0 {
		tb = m.engine.GetTB303()
	} else {
		tb = m.engine.GetTB303_2()
	}

	mixer := m.engine.GetMixer()
	var channel *audio.Channel
	if m.selectedSynth == 0 {
		channel = mixer.GetChannel(0)
	} else {
		channel = mixer.GetChannel(1)
	}

	// Map parameter index to name and value
	switch m.selectedParam {
	case 0: // Waveform - enum, use as-is
		wf := tb.GetWaveform()
		return "tb303_waveform", float32(wf), true
	case 1: // Cutoff
		return "tb303_cutoff", tb.GetCutoff(), true
	case 2: // Resonance
		return "tb303_resonance", tb.GetResonance(), true
	case 3: // Env Mod
		return "tb303_envmod", tb.GetEnvMod(), true
	case 4: // Decay (normalized)
		return "tb303_decay", tb.GetDecay() / 2.0, true
	case 5: // Accent
		return "tb303_accent", tb.GetAccent(), true
	case 6: // Slide Time (normalized)
		val := (tb.GetSlideTime() - 0.01) / 0.49
		return "tb303_slidetime", val, true
	case 7: // Sub Level
		return "tb303_sublevel", tb.GetSubLevel(), true
	case 8: // Volume
		return "tb303_volume", channel.Level, true
	case 9: // Separator
		return "", 0, false
	// LFO1 parameters (10-18) - skip for now
	// LFO2 parameters (20-28) - skip for now
	default:
		// For LFO params, we could add them later
		return "", 0, false
	}
}

// getDroneParamLock returns the parameter name and normalized value for Drone
func (m *Model) getDroneParamLock() (string, float32, bool) {
	drone := m.engine.GetDrone()

	switch m.selectedDroneParam {
	case 0: // Master Volume
		return "drone_volume", drone.GetMasterVolume(), true
	case 1: // Master Tune - semitones, not normalized
		return "drone_tune", float32(drone.GetMasterTune()), true
	case 2: // Osc1 Level
		return "drone_osc1level", drone.GetOsc1Level(), true
	case 3: // Osc2 Level
		return "drone_osc2level", drone.GetOsc2Level(), true
	case 4: // Osc3 Level
		return "drone_osc3level", drone.GetOsc3Level(), true
	case 5: // Osc2 Detune - cents, not normalized
		return "drone_osc2detune", drone.GetOsc2Detune(), true
	case 6: // Osc3 Detune - cents, not normalized
		return "drone_osc3detune", drone.GetOsc3Detune(), true
	case 7: // Sub1 Level
		return "drone_sub1level", drone.GetSub1Level(), true
	case 8: // Sub2 Level
		return "drone_sub2level", drone.GetSub2Level(), true
	case 9: // Separator
		return "", 0, false
	case 10: // Cutoff
		return "drone_cutoff", drone.GetCutoff(), true
	case 11: // Resonance
		return "drone_resonance", drone.GetResonance(), true
	case 12: // Env Mod
		return "drone_envmod", drone.GetEnvMod(), true
	case 13: // Attack - seconds, not normalized
		return "drone_attack", drone.GetAttack(), true
	case 14: // Decay - seconds, not normalized
		return "drone_decay", drone.GetDecay(), true
	case 15: // Sustain
		return "drone_sustain", drone.GetSustain(), true
	case 16: // Release - seconds, not normalized
		return "drone_release", drone.GetRelease(), true
	case 17: // Separator
		return "", 0, false
	// LFO parameters - skip for now
	default:
		return "", 0, false
	}
}

// getDrumParamLock returns the parameter name and normalized value for the selected drum
func (m *Model) getDrumParamLock() (string, float32, bool) {
	mixer := m.engine.GetMixer()

	// Each drum has different parameters, handle them separately
	switch m.selectedDrum {
	case 0: // Kick
		kick := m.engine.GetKick()
		switch m.selectedDrumParam {
		case 0: // Tune - semitones
			return "kick_tune", kick.GetTune(), true
		case 1: // Decay
			return "kick_decay", kick.GetCurrentDecay(), true
		case 2: // Pitch Env
			return "kick_pitchdecay", kick.GetCurrentPitchDecay(), true
		case 3: // Drive
			return "kick_drive", kick.GetCurrentDrive(), true
		case 4: // Click
			return "kick_click", kick.GetClickAmount(), true
		case 5: // Volume
			return "kick_volume", mixer.GetChannel(2).Level, true
		default:
			return "", 0, false
		}

	case 1: // Snare
		snare := m.engine.GetSnare()
		switch m.selectedDrumParam {
		case 0: // Tune
			return "snare_tune", snare.GetTune(), true
		case 1: // Snap
			return "snare_snap", snare.GetCurrentSnap(), true
		case 2: // Tone Decay
			return "snare_tonedecay", snare.GetCurrentToneDecay(), true
		case 3: // Noise Decay
			return "snare_noisedecay", snare.GetCurrentNoiseDecay(), true
		case 4: // Tone Mix
			return "snare_tonemix", snare.GetCurrentToneMix(), true
		case 5: // Volume
			return "snare_volume", mixer.GetChannel(3).Level, true
		default:
			return "", 0, false
		}

	case 2: // HH Closed
		hh := m.engine.GetHihatClosed()
		switch m.selectedDrumParam {
		case 0: // Decay
			return "hhclosed_decay", hh.GetCurrentDecay(), true
		case 1: // Tone
			return "hhclosed_tone", hh.GetCurrentTone(), true
		case 2: // Noise Mix
			return "hhclosed_noisemix", hh.GetCurrentNoiseMix(), true
		case 3: // Volume
			return "hhclosed_volume", mixer.GetChannel(4).Level, true
		default:
			return "", 0, false
		}

	case 3: // HH Open
		hh := m.engine.GetHihatOpen()
		switch m.selectedDrumParam {
		case 0: // Decay
			return "hhopen_decay", hh.GetCurrentDecay(), true
		case 1: // Tone
			return "hhopen_tone", hh.GetCurrentTone(), true
		case 2: // Noise Mix
			return "hhopen_noisemix", hh.GetCurrentNoiseMix(), true
		case 3: // Volume
			return "hhopen_volume", mixer.GetChannel(5).Level, true
		default:
			return "", 0, false
		}

	case 4: // Clap
		clap := m.engine.GetClap()
		switch m.selectedDrumParam {
		case 0: // Tune
			return "clap_tune", clap.GetCurrentTune(), true
		case 1: // Decay
			return "clap_decay", clap.GetCurrentDecay(), true
		case 2: // Tone
			return "clap_tone", clap.GetCurrentTone(), true
		case 3: // Drive
			return "clap_drive", clap.GetCurrentDrive(), true
		case 4: // Volume
			return "clap_volume", mixer.GetChannel(6).Level, true
		default:
			return "", 0, false
		}

	case 5: // Rimshot
		rimshot := m.engine.GetRimshot()
		switch m.selectedDrumParam {
		case 0: // Pitch
			return "rimshot_pitch", rimshot.GetCurrentPitch(), true
		case 1: // Decay
			return "rimshot_decay", rimshot.GetCurrentDecay(), true
		case 2: // Tone
			return "rimshot_tone", rimshot.GetCurrentTone(), true
		case 3: // Snap
			return "rimshot_snap", rimshot.GetCurrentSnap(), true
		case 4: // Volume
			return "rimshot_volume", mixer.GetChannel(7).Level, true
		default:
			return "", 0, false
		}

	case 6: // Crash
		crash := m.engine.GetCrash()
		switch m.selectedDrumParam {
		case 0: // Decay
			return "crash_decay", crash.GetCurrentDecay(), true
		case 1: // Tone
			return "crash_tone", crash.GetCurrentTone(), true
		case 2: // Metallic
			return "crash_metallic", crash.GetCurrentMetallic(), true
		case 3: // Volume
			return "crash_volume", mixer.GetChannel(8).Level, true
		default:
			return "", 0, false
		}

	case 7: // Tom
		tom := m.engine.GetTom()
		switch m.selectedDrumParam {
		case 0: // Tune
			return "tom_tune", tom.GetTune(), true
		case 1: // Decay
			return "tom_decay", tom.GetCurrentDecay(), true
		case 2: // Pitch Env
			return "tom_pitchdecay", tom.GetCurrentPitchDecay(), true
		case 3: // Volume
			return "tom_volume", mixer.GetChannel(9).Level, true
		default:
			return "", 0, false
		}
	}

	return "", 0, false
}

// randomizeTB303Sound randomizes TB-303 parameters within musical ranges
func (m *Model) randomizeTB303Sound(synthIdx int) {
	var tb *synth.TB303
	if synthIdx == 0 {
		tb = m.engine.GetTB303()
	} else {
		tb = m.engine.GetTB303_2()
	}

	// Randomize filter parameters
	tb.SetCutoff(0.3 + rand.Float32()*0.6)      // 30-90%
	tb.SetResonance(0.2 + rand.Float32()*0.7)   // 20-90%
	tb.SetEnvMod(0.4 + rand.Float32()*0.6)      // 40-100%
	tb.SetDecay(0.1 + rand.Float32()*0.9)       // 0.2-2.0 seconds (will be scaled)
	tb.SetAccent(0.3 + rand.Float32()*0.6)      // 30-90%
	tb.SetSlideTime(0.03 + rand.Float32()*0.15) // 30-180ms

	// Randomly choose waveform
	if rand.Float32() < 0.5 {
		tb.SetWaveform(synth.WaveformSaw)
	} else {
		tb.SetWaveform(synth.WaveformSquare)
	}

	// Randomize LFO1 if enabled
	lfo := tb.GetLFO1()
	if tb.GetLFO1Enabled() {
		lfo.SetRate(1.0 + rand.Float32()*8.0)  // 1-9 Hz
		lfo.SetDepth(0.3 + rand.Float32()*0.6) // 30-90%

		// Randomize LFO waveform
		waveforms := []dsp.LFOWaveform{
			dsp.LFOSine, dsp.LFOTriangle, dsp.LFOSquare,
			dsp.LFOSawUp, dsp.LFOSawDown, dsp.LFORandom,
		}
		lfo.SetWaveform(waveforms[rand.Intn(len(waveforms))])
	}
}

// randomizeKick randomizes kick drum parameters
func (m *Model) randomizeKick() {
	kick := m.engine.GetKick()

	kick.SetTune(-6.0 + rand.Float32()*12.0)         // -6 to +6 semitones
	kick.SetDecay(0.15 + rand.Float32()*0.35)        // 150-500ms
	kick.SetPitchDecay(0.005 + rand.Float32()*0.045) // 5-50ms
	kick.SetDrive(0.2 + rand.Float32()*0.6)          // 20-80%
	kick.SetClickAmount(rand.Float32() * 0.3)        // 0-30%
}

// randomizeSnare randomizes snare drum parameters
func (m *Model) randomizeSnare() {
	snare := m.engine.GetSnare()

	snare.SetTune(-6.0 + rand.Float32()*12.0)       // -6 to +6 semitones
	snare.SetToneDecay(0.04 + rand.Float32()*0.08)  // 40-120ms for tone
	snare.SetNoiseDecay(0.08 + rand.Float32()*0.17) // 80-250ms for noise
	snare.SetToneMix(0.3 + rand.Float32()*0.6)      // 30-90% tone/noise balance
	snare.SetSnap(0.3 + rand.Float32()*0.6)         // 30-90%
}

// randomizeClap randomizes clap drum parameters
func (m *Model) randomizeClap() {
	clap := m.engine.GetClap()

	clap.SetTune(-8.0 + rand.Float32()*16.0)  // -8 to +8 semitones
	clap.SetDecay(0.04 + rand.Float32()*0.14) // 40-180ms
	clap.SetTone(0.4 + rand.Float32()*0.5)    // 40-90% (favor brighter)
	clap.SetDrive(0.2 + rand.Float32()*0.6)   // 20-80%
}

// randomizeRimshot randomizes rimshot drum parameters
func (m *Model) randomizeRimshot() {
	rimshot := m.engine.GetRimshot()

	rimshot.SetPitch(350.0 + rand.Float32()*300.0) // 350-650 Hz
	rimshot.SetDecay(0.02 + rand.Float32()*0.08)   // 20-100ms
	rimshot.SetTone(0.3 + rand.Float32()*0.6)      // 30-90%
	rimshot.SetSnap(0.5 + rand.Float32()*0.5)      // 50-100%
}

// randomizeCrash randomizes crash cymbal parameters
func (m *Model) randomizeCrash() {
	crash := m.engine.GetCrash()

	crash.SetDecay(0.5 + rand.Float32()*2.0)    // 0.5-2.5s
	crash.SetTone(0.4 + rand.Float32()*0.5)     // 40-90%
	crash.SetMetallic(0.5 + rand.Float32()*0.4) // 50-90%
}

// randomizeTom randomizes tom drum parameters
func (m *Model) randomizeTom() {
	tom := m.engine.GetTom()

	tom.SetPitch(60.0 + rand.Float32()*180.0)   // 60-240 Hz
	tom.SetDecay(0.2 + rand.Float32()*1.0)      // 200-1200ms
	tom.SetTune(-6.0 + rand.Float32()*12.0)     // -6 to +6 semitones
	tom.SetPitchDecay(0.3 + rand.Float32()*0.6) // 30-90%
	tom.SetTone(0.5 + rand.Float32()*0.4)       // 50-90%
	tom.SetAttack(0.3 + rand.Float32()*0.6)     // 30-90%
}

// randomizeHihat randomizes hi-hat parameters
func (m *Model) randomizeHihat(trackIdx int) {
	var hihat *drums.HiHat

	if trackIdx == 4 {
		hihat = m.engine.GetHihatClosed()
	} else {
		hihat = m.engine.GetHihatOpen()
	}

	// Different decay ranges for closed vs open
	if trackIdx == 4 {
		hihat.SetDecay(0.02 + rand.Float32()*0.08) // 20-100ms for closed
	} else {
		hihat.SetDecay(0.1 + rand.Float32()*0.3) // 100-400ms for open
	}

	hihat.SetTone(0.3 + rand.Float32()*0.6)     // 30-90%
	hihat.SetNoiseMix(0.5 + rand.Float32()*0.4) // 50-90% noise mix
}

// generateHiHatPattern generates various hi-hat patterns (cycles through styles)
func (m *Model) generateHiHatPattern() {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil || track.Pattern == nil {
		return
	}

	// Only for hi-hat tracks
	if instrument != sequencer.InstrumentHihatClosed && instrument != sequencer.InstrumentHihatOpen {
		return
	}

	// Cycle through 6 hi-hat variations
	switch m.hihatVariation {
	case 0:
		track.Pattern.GenerateHiHat8ths() // Straight 8ths
	case 1:
		track.Pattern.GenerateHiHat16ths() // Straight 16ths
	case 2:
		track.Pattern.GenerateHiHatSwing() // Swing groove
	case 3:
		track.Pattern.GenerateHiHatOffbeat() // Offbeat shuffle
	case 4:
		track.Pattern.GenerateHiHatTriplet() // Triplet groove
	case 5:
		track.Pattern.GenerateHiHatSparse() // Minimal sparse
	}
	m.hihatVariation = (m.hihatVariation + 1) % 6
}

// generateFillPattern generates a drum fill pattern (cycles through variations)
// and sets up auto-return to previous pattern after one loop
func (m *Model) generateFillPattern() {
	// Set pending state - will be applied at next pattern loop
	m.fillPending = true
	m.fillPendingType = "fill"
	m.fillReturnTrack = m.selectedTrack
}

// generateBuildupPattern generates a buildup pattern (cycles through variations)
// and sets up auto-return to previous pattern after one loop
func (m *Model) generateBuildupPattern() {
	// Set pending state - will be applied at next pattern loop
	m.fillPending = true
	m.fillPendingType = "buildup"
	m.fillReturnTrack = m.selectedTrack
}

// generateDefaultPattern creates a default pattern for the given instrument
func (m *Model) generateDefaultPattern(instrument sequencer.InstrumentType) *sequencer.Pattern {
	pattern := sequencer.NewPattern16()

	switch instrument {
	case sequencer.InstrumentKick:
		// 4-on-the-floor (steps 0, 4, 8, 12)
		for i := 0; i < 16; i += 4 {
			pattern.SetStep(i, sequencer.Step{
				Active:      true,
				Velocity:    0.9,
				Probability: 1.0,
			})
		}

	case sequencer.InstrumentSnare:
		// Backbeat (steps 4 and 12)
		pattern.SetStep(4, sequencer.Step{Active: true, Velocity: 0.85, Probability: 1.0})
		pattern.SetStep(12, sequencer.Step{Active: true, Velocity: 0.85, Probability: 1.0})

	case sequencer.InstrumentHihatClosed:
		// 8th notes (every other step)
		for i := 0; i < 16; i += 2 {
			pattern.SetStep(i, sequencer.Step{
				Active:      true,
				Velocity:    0.6 + rand.Float32()*0.2, // Slight velocity variation
				Probability: 1.0,
			})
		}

	case sequencer.InstrumentHihatOpen:
		// Offbeat open hats (steps 6 and 14)
		pattern.SetStep(6, sequencer.Step{Active: true, Velocity: 0.7, Probability: 1.0})
		pattern.SetStep(14, sequencer.Step{Active: true, Velocity: 0.7, Probability: 1.0})
	}

	return pattern
}

// isPatternEmpty checks if a pattern has no active steps
func isPatternEmpty(pattern *sequencer.Pattern) bool {
	if pattern == nil {
		return true
	}
	for i := 0; i < pattern.Length; i++ {
		step := pattern.GetStep(i)
		if step != nil && step.Active {
			return false
		}
	}
	return true
}

// applyPendingFill applies a pending fill/buildup at pattern boundary
func (m *Model) applyPendingFill() {
	if !m.fillPending {
		return
	}

	instrument := trackInstruments[m.fillReturnTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil || track.Pattern == nil {
		m.fillPending = false
		return
	}

	// Save current pattern for auto-return (clone it)
	m.fillReturnPattern = track.Pattern.Clone()
	m.fillReturnActive = true
	m.fillStartStep = 0         // Fill starts at step 0
	m.fillHasProgressed = false // Reset progression tracker

	// Apply the fill based on type
	if m.fillPendingType == "fill" {
		// Cycle through fill variations
		if instrument == sequencer.InstrumentSnare {
			track.Pattern.GenerateSnareRoll()
		} else {
			switch m.fillVariation {
			case 0:
				track.Pattern.GenerateDrumFill()
			case 1:
				track.Pattern.GenerateDrumFillRoll()
			case 2:
				track.Pattern.GenerateDrumFillTriplet()
			case 3:
				track.Pattern.GenerateDrumFillSparse()
			}
			m.fillVariation = (m.fillVariation + 1) % 4
		}
	} else if m.fillPendingType == "buildup" {
		// Apply buildup
		if instrument == sequencer.InstrumentHihatClosed || instrument == sequencer.InstrumentHihatOpen {
			switch m.buildupVariation {
			case 0:
				track.Pattern.GenerateHiHatBuildup()
			case 1:
				track.Pattern.GenerateHiHatBuildupRoll()
			case 2:
				track.Pattern.GenerateHiHatBuildupStutter()
			}
			m.buildupVariation = (m.buildupVariation + 1) % 3
		} else {
			track.Pattern.GenerateDrumFill()
		}
	}

	// Clear pending state
	m.fillPending = false
	m.fillPendingType = ""
}

// generateKickPattern cycles through kick pattern variations
func (m *Model) generateKickPattern() {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil || track.Pattern == nil {
		return
	}

	// Only for kick
	if instrument != sequencer.InstrumentKick {
		return
	}

	// Cycle through 4 kick variations
	switch m.kickVariation {
	case 0:
		track.Pattern.GenerateKickPattern4OnFloor() // Classic 4-on-the-floor
	case 1:
		track.Pattern.GenerateKickOffbeat() // Offbeat kicks
	case 2:
		track.Pattern.GenerateKickSyncopated() // Syncopated groove
	case 3:
		track.Pattern.GenerateKickBroken() // Broken beat
	}
	m.kickVariation = (m.kickVariation + 1) % 4
}

// generateSnarePattern cycles through snare pattern variations
func (m *Model) generateSnarePattern() {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil || track.Pattern == nil {
		return
	}

	// Only for snare
	if instrument != sequencer.InstrumentSnare {
		return
	}

	// Cycle through 4 snare variations
	switch m.snareVariation {
	case 0:
		track.Pattern.GenerateSnareBackbeat() // Classic 2 & 4
	case 1:
		track.Pattern.GenerateSnareClap() // Double clap style
	case 2:
		track.Pattern.GenerateSnareSyncopated() // Syncopated hits
	case 3:
		track.Pattern.GenerateSnareRoll() // Roll/fill
	}
	m.snareVariation = (m.snareVariation + 1) % 4
}

// clearCurrentPattern clears all steps on the current track
func (m *Model) clearCurrentPattern() {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil || track.Pattern == nil {
		return
	}

	track.Pattern.ClearAll()
}

// toggleMuteSelectedTrack toggles mute on the selected track
func (m *Model) toggleMuteSelectedTrack() {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track != nil {
		track.Mute = !track.Mute
		m.soundParamsDirty = true // Mark as unsaved
	}
}

// toggleSoloSelectedTrack toggles solo on the selected track
func (m *Model) toggleSoloSelectedTrack() {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track != nil {
		track.Solo = !track.Solo
		m.soundParamsDirty = true // Mark as unsaved
	}
}

// clamp helper function
func clamp(val, min, max float32) float32 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// saveCurrentSoundToPattern saves current sound settings to the active pattern
func (m *Model) saveCurrentSoundToPattern() {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil || track.Pattern == nil {
		return
	}

	pattern := track.Pattern

	// Save mute/solo state to pattern (works for all instruments)
	if track.Mute {
		pattern.SoundParams["mute"] = 1.0
	} else {
		pattern.SoundParams["mute"] = 0.0
	}
	if track.Solo {
		pattern.SoundParams["solo"] = 1.0
	} else {
		pattern.SoundParams["solo"] = 0.0
	}

	switch instrument {
	case sequencer.InstrumentTB303_1, sequencer.InstrumentTB303_2:
		var tb *synth.TB303
		if instrument == sequencer.InstrumentTB303_1 {
			tb = m.engine.GetTB303()
		} else {
			tb = m.engine.GetTB303_2()
		}

		// Save basic parameters
		pattern.SoundParams["waveform"] = float32(tb.GetWaveform())
		pattern.SoundParams["tune"] = float32(tb.GetTune())
		pattern.SoundParams["cutoff"] = tb.GetCutoff()
		pattern.SoundParams["resonance"] = tb.GetResonance()
		pattern.SoundParams["envmod"] = tb.GetEnvMod()
		pattern.SoundParams["decay"] = tb.GetDecay()
		pattern.SoundParams["accent"] = tb.GetAccent()
		pattern.SoundParams["slidetime"] = tb.GetSlideTime()
		pattern.SoundParams["sublevel"] = tb.GetSubLevel()

		// Save LFO1 parameters
		lfo1 := tb.GetLFO1()
		if tb.GetLFO1Enabled() {
			pattern.SoundParams["lfo1_enabled"] = 1.0
		} else {
			pattern.SoundParams["lfo1_enabled"] = 0.0
		}
		pattern.SoundParams["lfo1_target"] = float32(tb.GetLFO1Target())
		pattern.SoundParams["lfo1_waveform"] = float32(lfo1.GetWaveform())
		pattern.SoundParams["lfo1_rate"] = lfo1.GetRate()
		pattern.SoundParams["lfo1_depth"] = lfo1.GetDepth()
		pattern.SoundParams["lfo1_min"] = lfo1.GetMin()
		pattern.SoundParams["lfo1_max"] = lfo1.GetMax()
		if lfo1.GetTempoSync() {
			pattern.SoundParams["lfo1_temposync"] = 1.0
		} else {
			pattern.SoundParams["lfo1_temposync"] = 0.0
		}
		pattern.SoundParams["lfo1_syncrate"] = lfo1.GetSyncRate()
		pattern.SoundParams["lfo1_syncmode"] = float32(lfo1.GetSyncMode())
		pattern.SoundParams["lfo1_stepspercycle"] = float32(lfo1.GetStepsPerCycle())

		// Save LFO2 parameters
		lfo2 := tb.GetLFO2()
		if tb.GetLFO2Enabled() {
			pattern.SoundParams["lfo2_enabled"] = 1.0
		} else {
			pattern.SoundParams["lfo2_enabled"] = 0.0
		}
		pattern.SoundParams["lfo2_target"] = float32(tb.GetLFO2Target())
		pattern.SoundParams["lfo2_waveform"] = float32(lfo2.GetWaveform())
		pattern.SoundParams["lfo2_rate"] = lfo2.GetRate()
		pattern.SoundParams["lfo2_depth"] = lfo2.GetDepth()
		pattern.SoundParams["lfo2_min"] = lfo2.GetMin()
		pattern.SoundParams["lfo2_max"] = lfo2.GetMax()
		if lfo2.GetTempoSync() {
			pattern.SoundParams["lfo2_temposync"] = 1.0
		} else {
			pattern.SoundParams["lfo2_temposync"] = 0.0
		}
		pattern.SoundParams["lfo2_syncrate"] = lfo2.GetSyncRate()
		pattern.SoundParams["lfo2_syncmode"] = float32(lfo2.GetSyncMode())
		pattern.SoundParams["lfo2_stepspercycle"] = float32(lfo2.GetStepsPerCycle())

	case sequencer.InstrumentKick:
		kick := m.engine.GetKick()
		pattern.SoundParams["tune"] = kick.GetTune()
		pattern.SoundParams["decay"] = kick.GetDecay()
		pattern.SoundParams["pitchdecay"] = kick.GetPitchDecay()
		pattern.SoundParams["drive"] = kick.GetDrive()
		pattern.SoundParams["click"] = kick.GetClickAmount()

		// Save LFO1 parameters
		lfo1 := kick.GetLFO1()
		if kick.GetLFO1Enabled() {
			pattern.SoundParams["lfo1_enabled"] = 1.0
		} else {
			pattern.SoundParams["lfo1_enabled"] = 0.0
		}
		pattern.SoundParams["lfo1_target"] = float32(kick.GetLFO1Target())
		pattern.SoundParams["lfo1_waveform"] = float32(lfo1.GetWaveform())
		pattern.SoundParams["lfo1_rate"] = lfo1.GetRate()
		pattern.SoundParams["lfo1_depth"] = lfo1.GetDepth()
		pattern.SoundParams["lfo1_min"] = lfo1.GetMin()
		pattern.SoundParams["lfo1_max"] = lfo1.GetMax()
		if lfo1.GetTempoSync() {
			pattern.SoundParams["lfo1_temposync"] = 1.0
		} else {
			pattern.SoundParams["lfo1_temposync"] = 0.0
		}
		pattern.SoundParams["lfo1_syncrate"] = lfo1.GetSyncRate()
		pattern.SoundParams["lfo1_syncmode"] = float32(lfo1.GetSyncMode())
		pattern.SoundParams["lfo1_stepspercycle"] = float32(lfo1.GetStepsPerCycle())

	case sequencer.InstrumentSnare:
		snare := m.engine.GetSnare()
		pattern.SoundParams["tune"] = snare.GetTune()
		pattern.SoundParams["snap"] = snare.GetSnap()
		pattern.SoundParams["tonedecay"] = snare.GetToneDecay()
		pattern.SoundParams["noisedecay"] = snare.GetNoiseDecay()
		pattern.SoundParams["tonemix"] = snare.GetToneMix()

		// Save LFO1 parameters
		lfo1 := snare.GetLFO1()
		if snare.GetLFO1Enabled() {
			pattern.SoundParams["lfo1_enabled"] = 1.0
		} else {
			pattern.SoundParams["lfo1_enabled"] = 0.0
		}
		pattern.SoundParams["lfo1_target"] = float32(snare.GetLFO1Target())
		pattern.SoundParams["lfo1_waveform"] = float32(lfo1.GetWaveform())
		pattern.SoundParams["lfo1_rate"] = lfo1.GetRate()
		pattern.SoundParams["lfo1_depth"] = lfo1.GetDepth()
		pattern.SoundParams["lfo1_min"] = lfo1.GetMin()
		pattern.SoundParams["lfo1_max"] = lfo1.GetMax()
		if lfo1.GetTempoSync() {
			pattern.SoundParams["lfo1_temposync"] = 1.0
		} else {
			pattern.SoundParams["lfo1_temposync"] = 0.0
		}
		pattern.SoundParams["lfo1_syncrate"] = lfo1.GetSyncRate()
		pattern.SoundParams["lfo1_syncmode"] = float32(lfo1.GetSyncMode())
		pattern.SoundParams["lfo1_stepspercycle"] = float32(lfo1.GetStepsPerCycle())

	case sequencer.InstrumentHihatClosed:
		hhc := m.engine.GetHihatClosed()
		pattern.SoundParams["decay"] = hhc.GetDecay()
		pattern.SoundParams["tone"] = hhc.GetTone()
		pattern.SoundParams["noisemix"] = hhc.GetNoiseMix()

		// Save LFO1 parameters
		lfo1 := hhc.GetLFO1()
		if hhc.GetLFO1Enabled() {
			pattern.SoundParams["lfo1_enabled"] = 1.0
		} else {
			pattern.SoundParams["lfo1_enabled"] = 0.0
		}
		pattern.SoundParams["lfo1_target"] = float32(hhc.GetLFO1Target())
		pattern.SoundParams["lfo1_waveform"] = float32(lfo1.GetWaveform())
		pattern.SoundParams["lfo1_rate"] = lfo1.GetRate()
		pattern.SoundParams["lfo1_depth"] = lfo1.GetDepth()
		pattern.SoundParams["lfo1_min"] = lfo1.GetMin()
		pattern.SoundParams["lfo1_max"] = lfo1.GetMax()
		if lfo1.GetTempoSync() {
			pattern.SoundParams["lfo1_temposync"] = 1.0
		} else {
			pattern.SoundParams["lfo1_temposync"] = 0.0
		}
		pattern.SoundParams["lfo1_syncrate"] = lfo1.GetSyncRate()
		pattern.SoundParams["lfo1_syncmode"] = float32(lfo1.GetSyncMode())
		pattern.SoundParams["lfo1_stepspercycle"] = float32(lfo1.GetStepsPerCycle())

	case sequencer.InstrumentHihatOpen:
		hho := m.engine.GetHihatOpen()
		pattern.SoundParams["decay"] = hho.GetDecay()
		pattern.SoundParams["tone"] = hho.GetTone()
		pattern.SoundParams["noisemix"] = hho.GetNoiseMix()

		// Save LFO1 parameters
		lfo1 := hho.GetLFO1()
		if hho.GetLFO1Enabled() {
			pattern.SoundParams["lfo1_enabled"] = 1.0
		} else {
			pattern.SoundParams["lfo1_enabled"] = 0.0
		}
		pattern.SoundParams["lfo1_target"] = float32(hho.GetLFO1Target())
		pattern.SoundParams["lfo1_waveform"] = float32(lfo1.GetWaveform())
		pattern.SoundParams["lfo1_rate"] = lfo1.GetRate()
		pattern.SoundParams["lfo1_depth"] = lfo1.GetDepth()
		pattern.SoundParams["lfo1_min"] = lfo1.GetMin()
		pattern.SoundParams["lfo1_max"] = lfo1.GetMax()
		if lfo1.GetTempoSync() {
			pattern.SoundParams["lfo1_temposync"] = 1.0
		} else {
			pattern.SoundParams["lfo1_temposync"] = 0.0
		}
		pattern.SoundParams["lfo1_syncrate"] = lfo1.GetSyncRate()
		pattern.SoundParams["lfo1_syncmode"] = float32(lfo1.GetSyncMode())
		pattern.SoundParams["lfo1_stepspercycle"] = float32(lfo1.GetStepsPerCycle())

	case sequencer.InstrumentClap:
		clap := m.engine.GetClap()
		pattern.SoundParams["tune"] = clap.GetTune()
		pattern.SoundParams["decay"] = clap.GetDecay()
		pattern.SoundParams["tone"] = clap.GetTone()
		pattern.SoundParams["drive"] = clap.GetDrive()

		// Save LFO1 parameters
		lfo1 := clap.GetLFO1()
		if clap.GetLFO1Enabled() {
			pattern.SoundParams["lfo1_enabled"] = 1.0
		} else {
			pattern.SoundParams["lfo1_enabled"] = 0.0
		}
		pattern.SoundParams["lfo1_target"] = float32(clap.GetLFO1Target())
		pattern.SoundParams["lfo1_waveform"] = float32(lfo1.GetWaveform())
		pattern.SoundParams["lfo1_rate"] = lfo1.GetRate()
		pattern.SoundParams["lfo1_depth"] = lfo1.GetDepth()
		pattern.SoundParams["lfo1_min"] = lfo1.GetMin()
		pattern.SoundParams["lfo1_max"] = lfo1.GetMax()
		if lfo1.GetTempoSync() {
			pattern.SoundParams["lfo1_temposync"] = 1.0
		} else {
			pattern.SoundParams["lfo1_temposync"] = 0.0
		}
		pattern.SoundParams["lfo1_syncrate"] = lfo1.GetSyncRate()
		pattern.SoundParams["lfo1_syncmode"] = float32(lfo1.GetSyncMode())
		pattern.SoundParams["lfo1_stepspercycle"] = float32(lfo1.GetStepsPerCycle())
	}

	// Clear dirty flag after save
	m.soundParamsDirty = false
}

// loadSoundFromPattern loads sound settings from the active pattern for the selected track
func (m *Model) loadSoundFromPattern() {
	m.loadSoundForTrack(m.selectedTrack)
}

// loadAllSoundsFromPattern loads sound settings for ALL tracks from their patterns
func (m *Model) loadAllSoundsFromPattern() {
	for i := range trackInstruments {
		m.loadSoundForTrack(i)
	}
}

// loadSoundForTrack loads sound settings from the pattern for a specific track
func (m *Model) loadSoundForTrack(trackIndex int) {
	instrument := trackInstruments[trackIndex]
	track := m.seq.GetTrack(instrument)
	if track == nil || track.Pattern == nil {
		return
	}

	pattern := track.Pattern

	// Load mute/solo state from pattern (default to false if not saved)
	if mute, ok := pattern.SoundParams["mute"]; ok {
		track.Mute = mute > 0.5
	} else {
		track.Mute = false // Default: unmuted
	}
	if solo, ok := pattern.SoundParams["solo"]; ok {
		track.Solo = solo > 0.5
	} else {
		track.Solo = false // Default: not solo'd
	}

	if len(pattern.SoundParams) == 0 {
		return // No saved sound settings (but mute/solo already set to defaults)
	}

	switch instrument {
	case sequencer.InstrumentTB303_1, sequencer.InstrumentTB303_2:
		var tb *synth.TB303
		if instrument == sequencer.InstrumentTB303_1 {
			tb = m.engine.GetTB303()
		} else {
			tb = m.engine.GetTB303_2()
		}

		// Load basic parameters
		if waveform, ok := pattern.SoundParams["waveform"]; ok {
			tb.SetWaveform(synth.Waveform(int(waveform)))
		}
		if tune, ok := pattern.SoundParams["tune"]; ok {
			tb.SetTune(int(tune))
		}
		if cutoff, ok := pattern.SoundParams["cutoff"]; ok {
			tb.SetCutoff(cutoff)
		}
		if resonance, ok := pattern.SoundParams["resonance"]; ok {
			tb.SetResonance(resonance)
		}
		if envmod, ok := pattern.SoundParams["envmod"]; ok {
			tb.SetEnvMod(envmod)
		}
		if decay, ok := pattern.SoundParams["decay"]; ok {
			tb.SetDecay(decay)
		}
		if accent, ok := pattern.SoundParams["accent"]; ok {
			tb.SetAccent(accent)
		}
		if slidetime, ok := pattern.SoundParams["slidetime"]; ok {
			tb.SetSlideTime(slidetime)
		}
		if sublevel, ok := pattern.SoundParams["sublevel"]; ok {
			tb.SetSubLevel(sublevel)
		}

		// Load LFO1 parameters
		lfo1 := tb.GetLFO1()
		if lfo1Enabled, ok := pattern.SoundParams["lfo1_enabled"]; ok {
			tb.SetLFO1Enabled(lfo1Enabled > 0.5)
		}
		if lfo1Target, ok := pattern.SoundParams["lfo1_target"]; ok {
			tb.SetLFO1Target(synth.LFOTarget(int(lfo1Target)))
		}
		if lfo1Waveform, ok := pattern.SoundParams["lfo1_waveform"]; ok {
			lfo1.SetWaveform(dsp.LFOWaveform(int(lfo1Waveform)))
		}
		if lfo1Rate, ok := pattern.SoundParams["lfo1_rate"]; ok {
			lfo1.SetRate(lfo1Rate)
		}
		if lfo1Depth, ok := pattern.SoundParams["lfo1_depth"]; ok {
			lfo1.SetDepth(lfo1Depth)
		}
		lfo1Min, okMin := pattern.SoundParams["lfo1_min"]
		lfo1Max, okMax := pattern.SoundParams["lfo1_max"]
		if okMin && okMax {
			lfo1.SetRange(lfo1Min, lfo1Max)
		}
		if lfo1TempoSync, ok := pattern.SoundParams["lfo1_temposync"]; ok {
			lfo1.SetTempoSync(lfo1TempoSync > 0.5)
		}
		if lfo1SyncRate, ok := pattern.SoundParams["lfo1_syncrate"]; ok {
			lfo1.SetSyncRate(lfo1SyncRate)
		}
		if lfo1SyncMode, ok := pattern.SoundParams["lfo1_syncmode"]; ok {
			lfo1.SetSyncMode(dsp.LFOSyncMode(int(lfo1SyncMode)))
		}
		if lfo1StepsPerCycle, ok := pattern.SoundParams["lfo1_stepspercycle"]; ok {
			lfo1.SetStepsPerCycle(int(lfo1StepsPerCycle))
		}

		// Load LFO2 parameters
		lfo2 := tb.GetLFO2()
		if lfo2Enabled, ok := pattern.SoundParams["lfo2_enabled"]; ok {
			tb.SetLFO2Enabled(lfo2Enabled > 0.5)
		}
		if lfo2Target, ok := pattern.SoundParams["lfo2_target"]; ok {
			tb.SetLFO2Target(synth.LFOTarget(int(lfo2Target)))
		}
		if lfo2Waveform, ok := pattern.SoundParams["lfo2_waveform"]; ok {
			lfo2.SetWaveform(dsp.LFOWaveform(int(lfo2Waveform)))
		}
		if lfo2Rate, ok := pattern.SoundParams["lfo2_rate"]; ok {
			lfo2.SetRate(lfo2Rate)
		}
		if lfo2Depth, ok := pattern.SoundParams["lfo2_depth"]; ok {
			lfo2.SetDepth(lfo2Depth)
		}
		lfo2Min, okMin2 := pattern.SoundParams["lfo2_min"]
		lfo2Max, okMax2 := pattern.SoundParams["lfo2_max"]
		if okMin2 && okMax2 {
			lfo2.SetRange(lfo2Min, lfo2Max)
		}
		if lfo2TempoSync, ok := pattern.SoundParams["lfo2_temposync"]; ok {
			lfo2.SetTempoSync(lfo2TempoSync > 0.5)
		}
		if lfo2SyncRate, ok := pattern.SoundParams["lfo2_syncrate"]; ok {
			lfo2.SetSyncRate(lfo2SyncRate)
		}
		if lfo2SyncMode, ok := pattern.SoundParams["lfo2_syncmode"]; ok {
			lfo2.SetSyncMode(dsp.LFOSyncMode(int(lfo2SyncMode)))
		}
		if lfo2StepsPerCycle, ok := pattern.SoundParams["lfo2_stepspercycle"]; ok {
			lfo2.SetStepsPerCycle(int(lfo2StepsPerCycle))
		}

	case sequencer.InstrumentKick:
		kick := m.engine.GetKick()
		if tune, ok := pattern.SoundParams["tune"]; ok {
			kick.SetTune(tune)
		}
		if decay, ok := pattern.SoundParams["decay"]; ok {
			kick.SetDecay(decay)
		}
		if pitchDecay, ok := pattern.SoundParams["pitchdecay"]; ok {
			kick.SetPitchDecay(pitchDecay)
		}
		if drive, ok := pattern.SoundParams["drive"]; ok {
			kick.SetDrive(drive)
		}
		if click, ok := pattern.SoundParams["click"]; ok {
			kick.SetClickAmount(click)
		}

		// Load LFO1 parameters
		lfo1 := kick.GetLFO1()
		if lfo1Enabled, ok := pattern.SoundParams["lfo1_enabled"]; ok {
			kick.SetLFO1Enabled(lfo1Enabled > 0.5)
		}
		if lfo1Target, ok := pattern.SoundParams["lfo1_target"]; ok {
			kick.SetLFO1Target(drums.KickLFOTarget(int(lfo1Target)))
		}
		if lfo1Waveform, ok := pattern.SoundParams["lfo1_waveform"]; ok {
			lfo1.SetWaveform(dsp.LFOWaveform(int(lfo1Waveform)))
		}
		if lfo1Rate, ok := pattern.SoundParams["lfo1_rate"]; ok {
			lfo1.SetRate(lfo1Rate)
		}
		if lfo1Depth, ok := pattern.SoundParams["lfo1_depth"]; ok {
			lfo1.SetDepth(lfo1Depth)
		}
		lfo1Min, okMin := pattern.SoundParams["lfo1_min"]
		lfo1Max, okMax := pattern.SoundParams["lfo1_max"]
		if okMin && okMax {
			lfo1.SetRange(lfo1Min, lfo1Max)
		}
		if lfo1TempoSync, ok := pattern.SoundParams["lfo1_temposync"]; ok {
			lfo1.SetTempoSync(lfo1TempoSync > 0.5)
		}
		if lfo1SyncRate, ok := pattern.SoundParams["lfo1_syncrate"]; ok {
			lfo1.SetSyncRate(lfo1SyncRate)
		}
		if lfo1SyncMode, ok := pattern.SoundParams["lfo1_syncmode"]; ok {
			lfo1.SetSyncMode(dsp.LFOSyncMode(int(lfo1SyncMode)))
		}
		if lfo1StepsPerCycle, ok := pattern.SoundParams["lfo1_stepspercycle"]; ok {
			lfo1.SetStepsPerCycle(int(lfo1StepsPerCycle))
		}

	case sequencer.InstrumentSnare:
		snare := m.engine.GetSnare()
		if tune, ok := pattern.SoundParams["tune"]; ok {
			snare.SetTune(tune)
		}
		if snap, ok := pattern.SoundParams["snap"]; ok {
			snare.SetSnap(snap)
		}
		if toneDecay, ok := pattern.SoundParams["tonedecay"]; ok {
			snare.SetToneDecay(toneDecay)
		}
		if noiseDecay, ok := pattern.SoundParams["noisedecay"]; ok {
			snare.SetNoiseDecay(noiseDecay)
		}
		if toneMix, ok := pattern.SoundParams["tonemix"]; ok {
			snare.SetToneMix(toneMix)
		}

		// Load LFO1 parameters
		lfo1 := snare.GetLFO1()
		if lfo1Enabled, ok := pattern.SoundParams["lfo1_enabled"]; ok {
			snare.SetLFO1Enabled(lfo1Enabled > 0.5)
		}
		if lfo1Target, ok := pattern.SoundParams["lfo1_target"]; ok {
			snare.SetLFO1Target(drums.SnareLFOTarget(int(lfo1Target)))
		}
		if lfo1Waveform, ok := pattern.SoundParams["lfo1_waveform"]; ok {
			lfo1.SetWaveform(dsp.LFOWaveform(int(lfo1Waveform)))
		}
		if lfo1Rate, ok := pattern.SoundParams["lfo1_rate"]; ok {
			lfo1.SetRate(lfo1Rate)
		}
		if lfo1Depth, ok := pattern.SoundParams["lfo1_depth"]; ok {
			lfo1.SetDepth(lfo1Depth)
		}
		lfo1Min, okMin := pattern.SoundParams["lfo1_min"]
		lfo1Max, okMax := pattern.SoundParams["lfo1_max"]
		if okMin && okMax {
			lfo1.SetRange(lfo1Min, lfo1Max)
		}
		if lfo1TempoSync, ok := pattern.SoundParams["lfo1_temposync"]; ok {
			lfo1.SetTempoSync(lfo1TempoSync > 0.5)
		}
		if lfo1SyncRate, ok := pattern.SoundParams["lfo1_syncrate"]; ok {
			lfo1.SetSyncRate(lfo1SyncRate)
		}
		if lfo1SyncMode, ok := pattern.SoundParams["lfo1_syncmode"]; ok {
			lfo1.SetSyncMode(dsp.LFOSyncMode(int(lfo1SyncMode)))
		}
		if lfo1StepsPerCycle, ok := pattern.SoundParams["lfo1_stepspercycle"]; ok {
			lfo1.SetStepsPerCycle(int(lfo1StepsPerCycle))
		}

	case sequencer.InstrumentHihatClosed:
		hhc := m.engine.GetHihatClosed()
		if decay, ok := pattern.SoundParams["decay"]; ok {
			hhc.SetDecay(decay)
		}
		if tone, ok := pattern.SoundParams["tone"]; ok {
			hhc.SetTone(tone)
		}
		if noiseMix, ok := pattern.SoundParams["noisemix"]; ok {
			hhc.SetNoiseMix(noiseMix)
		}

		// Load LFO1 parameters
		lfo1 := hhc.GetLFO1()
		if lfo1Enabled, ok := pattern.SoundParams["lfo1_enabled"]; ok {
			hhc.SetLFO1Enabled(lfo1Enabled > 0.5)
		}
		if lfo1Target, ok := pattern.SoundParams["lfo1_target"]; ok {
			hhc.SetLFO1Target(drums.HiHatLFOTarget(int(lfo1Target)))
		}
		if lfo1Waveform, ok := pattern.SoundParams["lfo1_waveform"]; ok {
			lfo1.SetWaveform(dsp.LFOWaveform(int(lfo1Waveform)))
		}
		if lfo1Rate, ok := pattern.SoundParams["lfo1_rate"]; ok {
			lfo1.SetRate(lfo1Rate)
		}
		if lfo1Depth, ok := pattern.SoundParams["lfo1_depth"]; ok {
			lfo1.SetDepth(lfo1Depth)
		}
		lfo1Min, okMin := pattern.SoundParams["lfo1_min"]
		lfo1Max, okMax := pattern.SoundParams["lfo1_max"]
		if okMin && okMax {
			lfo1.SetRange(lfo1Min, lfo1Max)
		}
		if lfo1TempoSync, ok := pattern.SoundParams["lfo1_temposync"]; ok {
			lfo1.SetTempoSync(lfo1TempoSync > 0.5)
		}
		if lfo1SyncRate, ok := pattern.SoundParams["lfo1_syncrate"]; ok {
			lfo1.SetSyncRate(lfo1SyncRate)
		}
		if lfo1SyncMode, ok := pattern.SoundParams["lfo1_syncmode"]; ok {
			lfo1.SetSyncMode(dsp.LFOSyncMode(int(lfo1SyncMode)))
		}
		if lfo1StepsPerCycle, ok := pattern.SoundParams["lfo1_stepspercycle"]; ok {
			lfo1.SetStepsPerCycle(int(lfo1StepsPerCycle))
		}

	case sequencer.InstrumentHihatOpen:
		hho := m.engine.GetHihatOpen()
		if decay, ok := pattern.SoundParams["decay"]; ok {
			hho.SetDecay(decay)
		}
		if tone, ok := pattern.SoundParams["tone"]; ok {
			hho.SetTone(tone)
		}
		if noiseMix, ok := pattern.SoundParams["noisemix"]; ok {
			hho.SetNoiseMix(noiseMix)
		}

		// Load LFO1 parameters
		lfo1 := hho.GetLFO1()
		if lfo1Enabled, ok := pattern.SoundParams["lfo1_enabled"]; ok {
			hho.SetLFO1Enabled(lfo1Enabled > 0.5)
		}
		if lfo1Target, ok := pattern.SoundParams["lfo1_target"]; ok {
			hho.SetLFO1Target(drums.HiHatLFOTarget(int(lfo1Target)))
		}
		if lfo1Waveform, ok := pattern.SoundParams["lfo1_waveform"]; ok {
			lfo1.SetWaveform(dsp.LFOWaveform(int(lfo1Waveform)))
		}
		if lfo1Rate, ok := pattern.SoundParams["lfo1_rate"]; ok {
			lfo1.SetRate(lfo1Rate)
		}
		if lfo1Depth, ok := pattern.SoundParams["lfo1_depth"]; ok {
			lfo1.SetDepth(lfo1Depth)
		}
		lfo1Min, okMin := pattern.SoundParams["lfo1_min"]
		lfo1Max, okMax := pattern.SoundParams["lfo1_max"]
		if okMin && okMax {
			lfo1.SetRange(lfo1Min, lfo1Max)
		}
		if lfo1TempoSync, ok := pattern.SoundParams["lfo1_temposync"]; ok {
			lfo1.SetTempoSync(lfo1TempoSync > 0.5)
		}
		if lfo1SyncRate, ok := pattern.SoundParams["lfo1_syncrate"]; ok {
			lfo1.SetSyncRate(lfo1SyncRate)
		}
		if lfo1SyncMode, ok := pattern.SoundParams["lfo1_syncmode"]; ok {
			lfo1.SetSyncMode(dsp.LFOSyncMode(int(lfo1SyncMode)))
		}
		if lfo1StepsPerCycle, ok := pattern.SoundParams["lfo1_stepspercycle"]; ok {
			lfo1.SetStepsPerCycle(int(lfo1StepsPerCycle))
		}

	case sequencer.InstrumentClap:
		clap := m.engine.GetClap()
		if tune, ok := pattern.SoundParams["tune"]; ok {
			clap.SetTune(tune)
		}
		if decay, ok := pattern.SoundParams["decay"]; ok {
			clap.SetDecay(decay)
		}
		if tone, ok := pattern.SoundParams["tone"]; ok {
			clap.SetTone(tone)
		}
		if drive, ok := pattern.SoundParams["drive"]; ok {
			clap.SetDrive(drive)
		}

		// Load LFO1 parameters
		lfo1 := clap.GetLFO1()
		if lfo1Enabled, ok := pattern.SoundParams["lfo1_enabled"]; ok {
			clap.SetLFO1Enabled(lfo1Enabled > 0.5)
		}
		if lfo1Target, ok := pattern.SoundParams["lfo1_target"]; ok {
			clap.SetLFO1Target(drums.ClapLFOTarget(int(lfo1Target)))
		}
		if lfo1Waveform, ok := pattern.SoundParams["lfo1_waveform"]; ok {
			lfo1.SetWaveform(dsp.LFOWaveform(int(lfo1Waveform)))
		}
		if lfo1Rate, ok := pattern.SoundParams["lfo1_rate"]; ok {
			lfo1.SetRate(lfo1Rate)
		}
		if lfo1Depth, ok := pattern.SoundParams["lfo1_depth"]; ok {
			lfo1.SetDepth(lfo1Depth)
		}
		lfo1Min, okMin := pattern.SoundParams["lfo1_min"]
		lfo1Max, okMax := pattern.SoundParams["lfo1_max"]
		if okMin && okMax {
			lfo1.SetRange(lfo1Min, lfo1Max)
		}
		if lfo1TempoSync, ok := pattern.SoundParams["lfo1_temposync"]; ok {
			lfo1.SetTempoSync(lfo1TempoSync > 0.5)
		}
		if lfo1SyncRate, ok := pattern.SoundParams["lfo1_syncrate"]; ok {
			lfo1.SetSyncRate(lfo1SyncRate)
		}
		if lfo1SyncMode, ok := pattern.SoundParams["lfo1_syncmode"]; ok {
			lfo1.SetSyncMode(dsp.LFOSyncMode(int(lfo1SyncMode)))
		}
		if lfo1StepsPerCycle, ok := pattern.SoundParams["lfo1_stepspercycle"]; ok {
			lfo1.SetStepsPerCycle(int(lfo1StepsPerCycle))
		}
	}

	// Clear dirty flag after load
	m.soundParamsDirty = false
}

// loadMuteSoloForAllTracks loads mute/solo states for ALL tracks from their patterns
func (m *Model) loadMuteSoloForAllTracks() {
	// Load mute/solo for all instruments
	for i, instrument := range trackInstruments {
		if i >= len(trackInstruments) {
			break
		}

		track := m.seq.GetTrack(instrument)
		if track == nil || track.Pattern == nil {
			continue
		}

		pattern := track.Pattern

		// Load mute/solo state from pattern (default to false if not saved)
		if mute, ok := pattern.SoundParams["mute"]; ok {
			track.Mute = mute > 0.5
		} else {
			track.Mute = false // Default: unmuted
		}

		if solo, ok := pattern.SoundParams["solo"]; ok {
			track.Solo = solo > 0.5
		} else {
			track.Solo = false // Default: not solo'd
		}
	}
}

// saveAllMuteSoloToPatterns saves mute/solo states for ALL tracks to their patterns
func (m *Model) saveAllMuteSoloToPatterns() {
	// Save mute/solo for all instruments
	for i, instrument := range trackInstruments {
		if i >= len(trackInstruments) {
			break
		}

		track := m.seq.GetTrack(instrument)
		if track == nil || track.Pattern == nil {
			continue
		}

		pattern := track.Pattern
		if pattern.SoundParams == nil {
			pattern.SoundParams = make(map[string]float32)
		}

		// Save mute/solo state to pattern
		if track.Mute {
			pattern.SoundParams["mute"] = 1.0
		} else {
			pattern.SoundParams["mute"] = 0.0
		}

		if track.Solo {
			pattern.SoundParams["solo"] = 1.0
		} else {
			pattern.SoundParams["solo"] = 0.0
		}
	}
}

// changePatternLength changes the pattern length for the current track
func (m *Model) changePatternLength(direction int) {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil || track.Pattern == nil {
		return
	}

	currentLength := track.Pattern.Length
	var newLength int

	if direction > 0 {
		// Increase: 16 → 32 → 64 → 128 → 16
		switch currentLength {
		case 16:
			newLength = 32
		case 32:
			newLength = 64
		case 64:
			newLength = 128
		default:
			newLength = 16
		}
	} else {
		// Decrease: 16 → 128 → 64 → 32 → 16
		switch currentLength {
		case 128:
			newLength = 64
		case 64:
			newLength = 32
		case 32:
			newLength = 16
		default:
			newLength = 128
		}
	}

	track.Pattern.SetLength(newLength)

	// Update the sequencer clock to use the maximum pattern length across all tracks
	// This enables polyrhythms where different tracks loop at different rates
	m.seq.UpdateClockLength()

	// Adjust stepOffset if it's now beyond the visible range
	if m.stepOffset >= newLength {
		m.stepOffset = max(0, newLength-16)
	}

	// Adjust cursor position if it's beyond the new length
	if m.cursorX >= newLength {
		m.cursorX = newLength - 1
	}
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// calculateVisibleSteps determines how many steps can fit on screen
// Returns the number of steps (always a multiple of 4, capped at 64)
func (m Model) calculateVisibleSteps() int {
	const trackNameWidth = 14 // Track name + mute/solo indicators
	const charsPerBar = 5     // 4 steps + 1 divider

	// Calculate available width for steps
	availableWidth := m.width - trackNameWidth - 20 // Reserve 20 chars for margins/scroll info

	if availableWidth < charsPerBar {
		return 4 // Minimum 4 steps
	}

	// Calculate how many complete bars fit
	numBars := availableWidth / charsPerBar

	// Cap at 16 bars (64 steps)
	if numBars > 16 {
		numBars = 16
	}

	// Ensure at least 1 bar (4 steps)
	if numBars < 1 {
		numBars = 1
	}

	return numBars * 4
}

// cycleDroneOscWaveform cycles through waveforms for drone oscillators
func (m *Model) cycleDroneOscWaveform(oscNum int) {
	drone := m.engine.GetDrone()
	var currentWaveform synth.Waveform

	switch oscNum {
	case 1:
		currentWaveform = drone.GetOsc1Waveform()
	case 2:
		currentWaveform = drone.GetOsc2Waveform()
	case 3:
		currentWaveform = drone.GetOsc3Waveform()
	default:
		return
	}

	// Cycle through: Saw -> Square -> Triangle -> Sine -> Saw
	var newWaveform synth.Waveform
	switch currentWaveform {
	case synth.WaveformSaw:
		newWaveform = synth.WaveformSquare
	case synth.WaveformSquare:
		newWaveform = synth.WaveformTriangle
	case synth.WaveformTriangle:
		newWaveform = synth.WaveformSine
	case synth.WaveformSine:
		newWaveform = synth.WaveformSaw
	default:
		newWaveform = synth.WaveformSaw
	}

	switch oscNum {
	case 1:
		drone.SetOsc1Waveform(newWaveform)
	case 2:
		drone.SetOsc2Waveform(newWaveform)
	case 3:
		drone.SetOsc3Waveform(newWaveform)
	}

	m.soundParamsDirty = true
}

// adjustArpParam adjusts arpeggiator parameters
func (m *Model) adjustArpParam(delta int) {
	drone := m.engine.GetDrone()
	arp := drone.GetArpeggiator()

	switch m.selectedArpParam {
	case 0: // Enabled (toggle)
		arp.SetEnabled(!arp.GetEnabled())

	case 1: // Pattern
		currentPattern := arp.GetPattern()
		newPattern := int(currentPattern) + delta
		if newPattern < 0 {
			newPattern = 5 // Wrap to last pattern
		} else if newPattern > 5 {
			newPattern = 0 // Wrap to first pattern
		}
		arp.SetPattern(synth.ArpPattern(newPattern))

	case 2: // Rate
		currentRate := arp.GetRate()
		newRate := int(currentRate) + delta
		if newRate < 0 {
			newRate = 5 // Wrap to last rate
		} else if newRate > 5 {
			newRate = 0 // Wrap to first rate
		}
		arp.SetRate(synth.ArpRate(newRate))

	case 3: // Gate Length
		val := arp.GetGateLength()
		if delta > 0 {
			val += 0.05
		} else {
			val -= 0.05
		}
		arp.SetGateLength(clamp(val, 0.1, 1.0))

	case 4: // Octave Range
		octaves := arp.GetOctaveRange()
		octaves += delta
		arp.SetOctaveRange(octaves) // SetOctaveRange clamps to 1-4

	case 5: // Swing
		val := arp.GetSwing()
		if delta > 0 {
			val += 0.05
		} else {
			val -= 0.05
		}
		arp.SetSwing(clamp(val, 0.0, 1.0))

	case 6: // Latch Mode (toggle)
		arp.SetLatchMode(!arp.GetLatchMode())
	}
}
