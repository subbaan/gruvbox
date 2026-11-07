package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/subbaan/gruvbox/sequencer"
)

// renderPianoRoll renders the piano roll view
func (m Model) renderPianoRoll() string {
	if !m.ready {
		return "Loading..."
	}

	// Get the current track (must be TB303)
	var instrument sequencer.InstrumentType
	if m.selectedTrack == 0 {
		instrument = sequencer.InstrumentTB303_1
	} else if m.selectedTrack == 1 {
		instrument = sequencer.InstrumentTB303_2
	} else {
		// Should not happen, but handle gracefully
		return "Piano roll only available for TB-303 tracks"
	}

	track := m.seq.GetTrack(instrument)
	pattern := track.Pattern
	patternID := m.seq.GetCurrentPatternID()

	// Calculate visible range
	noteNames := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	octaveStart := m.pianoRollOctaveOffset + 2 // Octave 2 is base (MIDI 24)

	// Build header
	title := fmt.Sprintf("Piano Roll - %s - Pattern %d", instrument.String(), patternID)
	header := TitleStyle.Render(title)

	info := fmt.Sprintf("Octave: %d-%d | BPM: %.1f | Steps: %d/%d",
		octaveStart, octaveStart+1, m.bpm, pattern.Length, 128)
	infoLine := lipgloss.NewStyle().Foreground(ColorDim).Render(info)

	// Build the grid
	var grid strings.Builder

	// Column headers (step numbers)
	grid.WriteString(lipgloss.NewStyle().Foreground(ColorSecondary).Render("    ")) // Left margin for note names
	visibleSteps := min(pattern.Length, 32) // Show up to 32 steps at once
	for step := 0; step < visibleSteps; step++ {
		if step%4 == 0 {
			grid.WriteString(lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render(fmt.Sprintf("%2d ", step)))
		} else {
			grid.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(fmt.Sprintf("%2d ", step)))
		}
	}
	grid.WriteString("\n")

	// Draw separator
	grid.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render("    "))
	for step := 0; step < visibleSteps; step++ {
		if step%4 == 0 {
			grid.WriteString(lipgloss.NewStyle().Foreground(ColorWarning).Render("│  "))
		} else {
			grid.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render("┊  "))
		}
	}
	grid.WriteString("\n")

	// Draw piano roll grid (top to bottom = high to low pitch)
	currentPlayStep := m.seq.GetCurrentStep()

	for row := 23; row >= 0; row-- { // 24 rows (2 octaves), reversed
		midiNote := (octaveStart * 12) + row
		noteName := noteNames[row%12]
		octave := octaveStart + (row / 12)

		// Note name column
		noteLabel := fmt.Sprintf("%2s%d", noteName, octave)
		isBlackKey := strings.Contains(noteName, "#")

		var noteLabelStyle lipgloss.Style
		if isBlackKey {
			noteLabelStyle = lipgloss.NewStyle().Foreground(ColorDim)
		} else {
			noteLabelStyle = lipgloss.NewStyle().Foreground(ColorFg)
		}

		// Highlight C notes
		if noteName == "C" {
			noteLabelStyle = noteLabelStyle.Bold(true).Foreground(ColorPrimary)
		}

		grid.WriteString(noteLabelStyle.Render(noteLabel + " "))

		// Draw cells for each step
		for step := 0; step < visibleSteps; step++ {
			cellContent := "  "
			var cellStyle lipgloss.Style

			// Check if this step has a note at this pitch
			stepData := pattern.GetStep(step)
			hasNote := stepData != nil && stepData.Active && stepData.Note == midiNote

			// Determine cell appearance
			isCursor := (step == m.pianoRollCursorX && row == m.pianoRollCursorY)
			isPlayhead := m.isPlaying && step == currentPlayStep
			isBeat := step%4 == 0

			if isCursor {
				// Cursor position - bright highlight
				if hasNote {
					cellContent = "██"
					cellStyle = lipgloss.NewStyle().
						Foreground(ColorBg).
						Background(ColorAccent)
				} else {
					cellContent = "┼┼"
					cellStyle = lipgloss.NewStyle().
						Foreground(ColorAccent).
						Bold(true)
				}
			} else if hasNote {
				// Note present
				velocity := stepData.Velocity

				// Show accent and slide indicators
				if stepData.Accent && stepData.Slide {
					cellContent = "▲~"
				} else if stepData.Accent {
					cellContent = "▲▲"
				} else if stepData.Slide {
					cellContent = "~~"
				} else {
					// Use brightness based on velocity
					if velocity > 0.8 {
						cellContent = "██"
					} else if velocity > 0.6 {
						cellContent = "▓▓"
					} else if velocity > 0.4 {
						cellContent = "▒▒"
					} else {
						cellContent = "░░"
					}
				}

				// Color based on accent
				if stepData.Accent {
					cellStyle = lipgloss.NewStyle().Foreground(ColorAccent)
				} else {
					cellStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
				}

				// Highlight playhead
				if isPlayhead {
					cellStyle = cellStyle.Background(ColorPrimary)
				}
			} else {
				// Empty cell
				if isPlayhead {
					cellContent = "┃┃"
					cellStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
				} else if isBeat {
					cellContent = "· "
					cellStyle = lipgloss.NewStyle().Foreground(ColorDim)
				} else {
					cellContent = "  "
					cellStyle = lipgloss.NewStyle().Foreground(ColorDim)
				}
			}

			grid.WriteString(cellStyle.Render(cellContent))

			// Add beat separator
			if step%4 == 3 && step < visibleSteps-1 {
				grid.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render("│"))
			} else {
				grid.WriteString(" ")
			}
		}
		grid.WriteString("\n")
	}

	// Help text
	help := strings.Builder{}
	help.WriteString("\n")
	help.WriteString(KeyStyle.Render("Arrows") + ": Move cursor  ")
	help.WriteString(KeyStyle.Render("Enter/Space") + ": Toggle note  ")
	help.WriteString(KeyStyle.Render("+/-") + ": Velocity  ")
	help.WriteString(KeyStyle.Render("a") + ": Accent  ")
	help.WriteString(KeyStyle.Render("s") + ": Slide  ")
	help.WriteString("\n")
	help.WriteString(KeyStyle.Render("PgUp/PgDn") + ": Change octave  ")
	help.WriteString(KeyStyle.Render("Del/Bksp") + ": Clear note  ")
	help.WriteString(KeyStyle.Render("Esc") + ": Exit piano roll  ")

	helpText := HelpStyle.Render(help.String())

	// Combine all parts
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		infoLine,
		"",
		grid.String(),
		helpText,
	)
}
