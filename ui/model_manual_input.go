package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/subbaan/gruvbox/audio"
	"github.com/subbaan/gruvbox/dsp"
)

// ============================================================================
// MANUAL VALUE ENTRY
// ============================================================================

// handleManualInputKeys handles keyboard input during manual value entry mode
func (m Model) handleManualInputKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel manual input
		m.manualInputMode = false
		m.manualInputValue = ""
		return m, nil

	case "enter":
		// Apply the value
		m.applyManualInputValue()
		m.manualInputMode = false
		m.manualInputValue = ""
		return m, nil

	case "backspace":
		// Delete last character
		if len(m.manualInputValue) > 0 {
			m.manualInputValue = m.manualInputValue[:len(m.manualInputValue)-1]
		}
		return m, nil

	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Add numeric character
		m.manualInputValue += msg.String()
		return m, nil

	case ".":
		// Add decimal point (only one allowed)
		if !strings.Contains(m.manualInputValue, ".") {
			m.manualInputValue += "."
		}
		return m, nil

	case "-":
		// Add minus sign (only at start)
		if len(m.manualInputValue) == 0 {
			m.manualInputValue = "-"
		} else if m.manualInputValue[0] != '-' {
			m.manualInputValue = "-" + m.manualInputValue
		} else {
			// Toggle minus sign
			m.manualInputValue = m.manualInputValue[1:]
		}
		return m, nil
	}

	return m, nil
}

// applyManualInputValue parses the input string and applies it to the current parameter
func (m *Model) applyManualInputValue() {
	if m.manualInputValue == "" {
		return
	}

	// Parse the input string to float32
	val64, err := strconv.ParseFloat(m.manualInputValue, 32)
	if err != nil {
		return // Invalid input, silently ignore
	}
	val := float32(val64)

	// Apply based on current view
	switch m.currentView {
	case ViewSynth:
		m.applySynthManualValue(val)
	case ViewDrums:
		m.applyDrumManualValue(val)
	}
}

// applySynthManualValue applies a manually entered value to the current synth parameter
func (m *Model) applySynthManualValue(val float32) {
	var tb = m.engine.GetTB303()
	if m.selectedSynth == 1 {
		tb = m.engine.GetTB303_2()
	}

	// Get current value and set based on selected parameter
	switch m.selectedParam {
	case 0: // Waveform - can't be set manually
		return

	case 1: // Cutoff (0-1)
		tb.SetCutoff(clamp(val, 0.0, 1.0))

	case 2: // Resonance (0-1)
		tb.SetResonance(clamp(val, 0.0, 1.0))

	case 3: // Env Mod (0-1)
		tb.SetEnvMod(clamp(val, 0.0, 1.0))

	case 4: // Decay (0-2 seconds, displayed as 0-1 normalized)
		// User enters normalized value 0-1, we denormalize to 0-2 seconds
		tb.SetDecay(clamp(val, 0.0, 1.0) * 2.0)

	case 5: // Accent (0-1)
		tb.SetAccent(clamp(val, 0.0, 1.0))

	case 6: // Slide Time (0.01-0.5 seconds, displayed as 0-1 normalized)
		// User enters normalized value 0-1, we denormalize to 0.01-0.5 seconds
		tb.SetSlideTime(clamp(val, 0.0, 1.0)*0.49 + 0.01)

	case 7: // Sub Level (0-1)
		tb.SetSubLevel(clamp(val, 0.0, 1.0))

	case 8: // Volume (0-1)
		mixer := m.engine.GetMixer()
		var channel = mixer.GetChannel(0)
		if m.selectedSynth == 1 {
			channel = mixer.GetChannel(1)
		}
		channel.Level = clamp(val, 0.0, 1.0)

	case 10: // LFO1 Enable - toggle, can't set manually
		return

	case 11: // LFO1 Target - discrete, can't set manually
		return

	case 12: // LFO1 Waveform - discrete, can't set manually
		return

	case 13: // LFO1 Rate
		lfo := tb.GetLFO1()
		if lfo.GetTempoSync() {
			if lfo.GetSyncMode() == dsp.SyncModeSteps {
				// User enters steps directly (1-256)
				steps := int(val)
				if steps < 1 {
					steps = 1
				}
				if steps > 256 {
					steps = 256
				}
				lfo.SetStepsPerCycle(steps)
				tb.UpdateLFOTempo(m.bpm)
			} else {
				// Sync rate is discrete, can't set manually
				return
			}
		} else {
			// Hz rate (0.01-100)
			lfo.SetRate(clamp(val, 0.01, 100.0))
		}

	case 14: // LFO1 Depth (0-1)
		lfo := tb.GetLFO1()
		lfo.SetDepth(clamp(val, 0.0, 1.0))

	case 15: // LFO1 Min
		lfo := tb.GetLFO1()
		target := tb.GetLFO1Target()
		max := lfo.GetMax()

		if target == 2 { // LFOTargetPitch
			// Pitch: 0.95 to 1.05 range
			lfo.SetRange(clamp(val, 0.95, max-0.01), max)
		} else {
			// Cutoff/Resonance: 0 to 1 range
			lfo.SetRange(clamp(val, 0.0, max-0.05), max)
		}

	case 16: // LFO1 Max
		lfo := tb.GetLFO1()
		target := tb.GetLFO1Target()
		min := lfo.GetMin()

		if target == 2 { // LFOTargetPitch
			// Pitch: 0.95 to 1.05 range
			lfo.SetRange(min, clamp(val, min+0.01, 1.05))
		} else {
			// Cutoff/Resonance: 0 to 1 range
			lfo.SetRange(min, clamp(val, min+0.05, 1.0))
		}

	case 17: // LFO1 Tempo Sync - toggle, can't set manually
		return

	case 18: // LFO1 Mode - toggle, can't set manually
		return

	case 20: // LFO2 Enable - toggle, can't set manually
		return

	case 21: // LFO2 Target - discrete, can't set manually
		return

	case 22: // LFO2 Waveform - discrete, can't set manually
		return

	case 23: // LFO2 Rate
		lfo := tb.GetLFO2()
		if lfo.GetTempoSync() {
			if lfo.GetSyncMode() == dsp.SyncModeSteps {
				// User enters steps directly (1-256)
				steps := int(val)
				if steps < 1 {
					steps = 1
				}
				if steps > 256 {
					steps = 256
				}
				lfo.SetStepsPerCycle(steps)
				tb.UpdateLFOTempo(m.bpm)
			} else {
				// Sync rate is discrete, can't set manually
				return
			}
		} else {
			// Hz rate (0.01-100)
			lfo.SetRate(clamp(val, 0.01, 100.0))
		}

	case 24: // LFO2 Depth (0-1)
		lfo := tb.GetLFO2()
		lfo.SetDepth(clamp(val, 0.0, 1.0))

	case 25: // LFO2 Min
		lfo := tb.GetLFO2()
		target := tb.GetLFO2Target()
		max := lfo.GetMax()

		if target == 2 { // LFOTargetPitch
			// Pitch: 0.95 to 1.05 range
			lfo.SetRange(clamp(val, 0.95, max-0.01), max)
		} else {
			// Cutoff/Resonance: 0 to 1 range
			lfo.SetRange(clamp(val, 0.0, max-0.05), max)
		}

	case 26: // LFO2 Max
		lfo := tb.GetLFO2()
		target := tb.GetLFO2Target()
		min := lfo.GetMin()

		if target == 2 { // LFOTargetPitch
			// Pitch: 0.95 to 1.05 range
			lfo.SetRange(min, clamp(val, min+0.01, 1.05))
		} else {
			// Cutoff/Resonance: 0 to 1 range
			lfo.SetRange(min, clamp(val, min+0.05, 1.0))
		}

	case 27: // LFO2 Tempo Sync - toggle, can't set manually
		return

	case 28: // LFO2 Mode - toggle, can't set manually
		return
	}
}

// applyDrumManualValue applies a manually entered value to the current drum parameter
func (m *Model) applyDrumManualValue(val float32) {
	switch m.selectedDrum {
	case 0: // Kick
		m.applyKickManualValue(val)
	case 1: // Snare
		m.applySnareManualValue(val)
	case 2: // HH Closed
		m.applyHihatManualValue(m.engine.GetHihatClosed(), val)
	case 3: // HH Open
		m.applyHihatManualValue(m.engine.GetHihatOpen(), val)
	case 4: // Clap
		m.applyClapManualValue(val)
	case 5: // Rimshot
		m.applyRimshotManualValue(val)
	case 6: // Crash
		m.applyCrashManualValue(val)
	case 7: // Tom
		m.applyTomManualValue(val)
	}
}

// applyKickManualValue applies a value to kick drum parameter
func (m *Model) applyKickManualValue(val float32) {
	kick := m.engine.GetKick()

	switch m.selectedDrumParam {
	case 0: // Tune (-12 to +12 semitones)
		kick.SetTune(clamp(val, -12.0, 12.0))
	case 1: // Decay (0.05-2.0 seconds)
		kick.SetDecay(clamp(val, 0.05, 2.0))
	case 2: // Pitch Env (0.001-0.1 seconds)
		kick.SetPitchDecay(clamp(val, 0.001, 0.1))
	case 3: // Drive (0-1)
		kick.SetDrive(clamp(val, 0.0, 1.0))
	case 4: // Click (0-1)
		kick.SetClickAmount(clamp(val, 0.0, 1.0))
	case 5: // Volume (0-1)
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(2)
		channel.Level = clamp(val, 0.0, 1.0)
	case 7: // LFO1 On - toggle, can't set manually
		return
	case 8: // LFO1 Target - discrete, can't set manually
		return
	case 9: // LFO1 Wave - discrete, can't set manually
		return
	case 10: // LFO1 Rate
		lfo := kick.GetLFO1()
		if lfo.GetTempoSync() {
			if lfo.GetSyncMode() == dsp.SyncModeSteps {
				steps := int(val)
				if steps < 1 {
					steps = 1
				}
				if steps > 256 {
					steps = 256
				}
				lfo.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			}
		} else {
			lfo.SetRate(clamp(val, 0.01, 100.0))
		}
	case 11: // LFO1 Depth (0-1)
		kick.GetLFO1().SetDepth(clamp(val, 0.0, 1.0))
	case 12, 13: // LFO1 Min/Max - complex, skip for now
		return
	case 14: // LFO1 Sync - toggle, can't set manually
		return
	case 15: // LFO1 Mode - toggle, can't set manually
		return
	}
}

// applySnareManualValue applies a value to snare drum parameter
func (m *Model) applySnareManualValue(val float32) {
	snare := m.engine.GetSnare()

	switch m.selectedDrumParam {
	case 0: // Tune (-12 to +12 semitones)
		snare.SetTune(clamp(val, -12.0, 12.0))
	case 1: // Snap (0-1)
		snare.SetSnap(clamp(val, 0.0, 1.0))
	case 2: // Tone Decay (0.01-0.3 seconds)
		snare.SetToneDecay(clamp(val, 0.01, 0.3))
	case 3: // Noise Decay (0.05-0.5 seconds)
		snare.SetNoiseDecay(clamp(val, 0.05, 0.5))
	case 4: // Tone Mix (0-1)
		snare.SetToneMix(clamp(val, 0.0, 1.0))
	case 5: // Volume (0-1)
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(3)
		channel.Level = clamp(val, 0.0, 1.0)
	case 7: // LFO1 On - toggle, can't set manually
		return
	case 8: // LFO1 Target - discrete, can't set manually
		return
	case 9: // LFO1 Wave - discrete, can't set manually
		return
	case 10: // LFO1 Rate
		lfo := snare.GetLFO1()
		if lfo.GetTempoSync() {
			if lfo.GetSyncMode() == dsp.SyncModeSteps {
				steps := int(val)
				if steps < 1 {
					steps = 1
				}
				if steps > 256 {
					steps = 256
				}
				lfo.SetStepsPerCycle(steps)
				m.engine.UpdateAllLFOTempo()
			}
		} else {
			lfo.SetRate(clamp(val, 0.01, 100.0))
		}
	case 11: // LFO1 Depth (0-1)
		snare.GetLFO1().SetDepth(clamp(val, 0.0, 1.0))
	}
}

// applyHihatManualValue applies a value to hi-hat parameter
func (m *Model) applyHihatManualValue(hh interface{}, val float32) {
	// Hi-hat implementation depends on the actual interface
	// Simplified for now - just handle volume
	switch m.selectedDrumParam {
	case 5: // Volume (0-1)
		mixer := m.engine.GetMixer()
		var channel *audio.Channel
		if m.selectedDrum == 2 { // HH Closed
			channel = mixer.GetChannel(4)
		} else { // HH Open
			channel = mixer.GetChannel(5)
		}
		channel.Level = clamp(val, 0.0, 1.0)
	}
}

// applyClapManualValue applies a value to clap parameter
func (m *Model) applyClapManualValue(val float32) {
	clap := m.engine.GetClap()

	switch m.selectedDrumParam {
	case 0: // Tune (-12 to +12 semitones)
		clap.SetTune(clamp(val, -12.0, 12.0))
	case 1: // Decay (0.05-0.5 seconds)
		clap.SetDecay(clamp(val, 0.05, 0.5))
	case 2: // Tone (0-1)
		clap.SetTone(clamp(val, 0.0, 1.0))
	case 3: // Drive (0-1)
		clap.SetDrive(clamp(val, 0.0, 1.0))
	case 4: // Volume (0-1)
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(6)
		channel.Level = clamp(val, 0.0, 1.0)
	}
}

// applyRimshotManualValue applies a value to rimshot parameter
func (m *Model) applyRimshotManualValue(val float32) {
	rimshot := m.engine.GetRimshot()

	switch m.selectedDrumParam {
	case 0: // Pitch (Hz, typically 100-1000)
		rimshot.SetPitch(clamp(val, 100.0, 1000.0))
	case 1: // Decay (0.01-0.3 seconds)
		rimshot.SetDecay(clamp(val, 0.01, 0.3))
	case 2: // Tone (0-1)
		rimshot.SetTone(clamp(val, 0.0, 1.0))
	case 3: // Snap (0-1)
		rimshot.SetSnap(clamp(val, 0.0, 1.0))
	case 4: // Volume (0-1)
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(7)
		channel.Level = clamp(val, 0.0, 1.0)
	}
}

// applyCrashManualValue applies a value to crash parameter
func (m *Model) applyCrashManualValue(val float32) {
	crash := m.engine.GetCrash()

	switch m.selectedDrumParam {
	case 0: // Decay (0.5-5.0 seconds)
		crash.SetDecay(clamp(val, 0.5, 5.0))
	case 1: // Tone (0-1)
		crash.SetTone(clamp(val, 0.0, 1.0))
	case 2: // Metallic (0-1)
		crash.SetMetallic(clamp(val, 0.0, 1.0))
	case 3: // Volume (0-1)
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(8)
		channel.Level = clamp(val, 0.0, 1.0)
	}
}

// applyTomManualValue applies a value to tom parameter
func (m *Model) applyTomManualValue(val float32) {
	tom := m.engine.GetTom()

	switch m.selectedDrumParam {
	case 0: // Tune (-12 to +12 semitones)
		tom.SetTune(clamp(val, -12.0, 12.0))
	case 1: // Decay (0.05-2.0 seconds)
		tom.SetDecay(clamp(val, 0.05, 2.0))
	case 2: // Tone (0-1)
		tom.SetTone(clamp(val, 0.0, 1.0))
	case 3: // Volume (0-1)
		mixer := m.engine.GetMixer()
		channel := mixer.GetChannel(9)
		channel.Level = clamp(val, 0.0, 1.0)
	}
}

// renderManualInputBox renders the manual input overlay box
func (m Model) renderManualInputBox() string {
	var rows []string

	rows = append(rows, TitleStyle.Render("ENTER VALUE"))
	rows = append(rows, "")

	// Show current input value (large and centered)
	inputStr := m.manualInputValue
	if inputStr == "" {
		inputStr = "_"
	} else {
		inputStr += "_"
	}

	inputStyle := lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true).
		Width(30).
		Align(lipgloss.Center)
	rows = append(rows, inputStyle.Render(inputStr))

	rows = append(rows, "")
	rows = append(rows, HelpStyle.Render("[0-9.-] Type value  [Enter] Apply  [Esc] Cancel"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(1, 2)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		boxStyle.Render(strings.Join(rows, "\n")),
	)
}
