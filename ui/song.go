package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// handleSongKeys handles keys specific to song view
func (m Model) handleSongKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	song := m.seq.GetSong()

	switch msg.String() {
	case "up":
		// Navigate song steps up
		if m.selectedSongStep > 0 {
			m.selectedSongStep--
		} else if song.GetStepCount() > 0 {
			m.selectedSongStep = song.GetStepCount() - 1 // Wrap to bottom
		}
		return m, nil

	case "down":
		// Navigate song steps down
		if m.selectedSongStep < song.GetStepCount()-1 {
			m.selectedSongStep++
		} else {
			m.selectedSongStep = 0 // Wrap to top
		}
		return m, nil

	case "a": // Add new step at end
		song.AddStep(0, 1) // Default: pattern 0, 1 repeat
		m.selectedSongStep = song.GetStepCount() - 1
		return m, nil

	case "i": // Insert step before current
		song.InsertStep(m.selectedSongStep, 0, 1)
		return m, nil

	case "d": // Duplicate current step
		if song.GetStepCount() > 0 && m.selectedSongStep < song.GetStepCount() {
			song.DuplicateStep(m.selectedSongStep)
		}
		return m, nil

	case "D": // Delete current step (Shift+d)
		if song.GetStepCount() > 0 {
			song.RemoveStep(m.selectedSongStep)
			if m.selectedSongStep >= song.GetStepCount() && song.GetStepCount() > 0 {
				m.selectedSongStep = song.GetStepCount() - 1
			}
		}
		return m, nil

	case "c": // Clear song
		song.Clear()
		m.selectedSongStep = 0
		return m, nil

	case "left":
		// Decrease pattern ID
		if song.GetStepCount() > 0 && m.selectedSongStep < song.GetStepCount() {
			step := song.GetStep(m.selectedSongStep)
			if step != nil && step.PatternID > 0 {
				song.SetStep(m.selectedSongStep, step.PatternID-1, step.Repeats)
			}
		}
		return m, nil

	case "right":
		// Increase pattern ID
		if song.GetStepCount() > 0 && m.selectedSongStep < song.GetStepCount() {
			step := song.GetStep(m.selectedSongStep)
			if step != nil && step.PatternID < 15 { // Max 16 patterns (0-15)
				song.SetStep(m.selectedSongStep, step.PatternID+1, step.Repeats)
			}
		}
		return m, nil

	case ",", "<":
		// Decrease repeats
		if song.GetStepCount() > 0 && m.selectedSongStep < song.GetStepCount() {
			step := song.GetStep(m.selectedSongStep)
			if step != nil && step.Repeats > 1 {
				song.SetStep(m.selectedSongStep, step.PatternID, step.Repeats-1)
			}
		}
		return m, nil

	case ".", ">":
		// Increase repeats
		if song.GetStepCount() > 0 && m.selectedSongStep < song.GetStepCount() {
			step := song.GetStep(m.selectedSongStep)
			if step != nil && step.Repeats < 99 {
				song.SetStep(m.selectedSongStep, step.PatternID, step.Repeats+1)
			}
		}
		return m, nil

	case "s": // Toggle song mode
		wasInSongMode := song.IsSongMode()
		song.SetSongMode(!wasInSongMode)

		// If we just enabled song mode, load the current pattern's sound settings
		if !wasInSongMode && song.IsSongMode() {
			currentPatternID := song.GetCurrentPatternID()
			if currentPatternID >= 0 {
				m.loadAllSoundsFromPattern()
				m.lastSongPatternID = currentPatternID
			}
		}

		return m, nil

	case "l": // Toggle loop
		song.SetLoopEnabled(!song.IsLoopEnabled())
		return m, nil
	}

	return m, nil
}

// renderSongView renders the song arrangement view
func (m Model) renderSongView() string {
	var rows []string

	song := m.seq.GetSong()

	// View title
	title := TitleStyle.Render("SONG MODE")
	rows = append(rows, title)
	rows = append(rows, "")

	// Song mode status
	statusText := "Song Mode: "
	if song.IsSongMode() {
		statusText += lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			Render("ON")
	} else {
		statusText += lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("OFF")
	}

	statusText += "  Loop: "
	if song.IsLoopEnabled() {
		statusText += lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Render("ON")
	} else {
		statusText += lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("OFF")
	}

	statusText += fmt.Sprintf("  Steps: %d", song.GetStepCount())

	rows = append(rows, statusText)
	rows = append(rows, "")

	// Song steps list
	if song.GetStepCount() == 0 {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("No steps in song. Press [a] to add a step."))
	} else {
		// Header
		header := lipgloss.JoinHorizontal(
			lipgloss.Left,
			lipgloss.NewStyle().Width(5).Render("#"),
			lipgloss.NewStyle().Width(12).Render("Pattern"),
			lipgloss.NewStyle().Width(10).Render("Repeats"),
			lipgloss.NewStyle().Width(15).Render("Status"),
		)
		rows = append(rows, lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(header))
		rows = append(rows, "")

		// Show song steps
		for i := 0; i < song.GetStepCount(); i++ {
			step := song.GetStep(i)
			if step == nil {
				continue
			}

			// Row number
			rowNumStr := fmt.Sprintf("%02d", i+1)
			patternStr := fmt.Sprintf("Pattern %02d", step.PatternID+1)
			repeatsStr := fmt.Sprintf("x%d", step.Repeats)

			// Status indicator
			statusStr := ""
			if song.IsSongMode() && i == song.GetCurrentStep() {
				if m.isPlaying {
					statusStr = lipgloss.NewStyle().
						Foreground(ColorAccent).
						Bold(true).
						Render("▶ PLAYING")
				} else {
					statusStr = lipgloss.NewStyle().
						Foreground(ColorSecondary).
						Render("→ NEXT")
				}
			}

			// Highlight selected step
			if i == m.selectedSongStep {
				rowLine := lipgloss.JoinHorizontal(
					lipgloss.Left,
					lipgloss.NewStyle().Width(5).Render(rowNumStr),
					lipgloss.NewStyle().Width(12).Render(patternStr),
					lipgloss.NewStyle().Width(10).Render(repeatsStr),
					lipgloss.NewStyle().Width(15).Render(statusStr),
				)
				rows = append(rows, lipgloss.NewStyle().
					Background(ColorWarning).
					Foreground(ColorBg).
					Bold(true).
					Render(rowLine))
			} else {
				rowLine := lipgloss.JoinHorizontal(
					lipgloss.Left,
					lipgloss.NewStyle().Width(5).Foreground(ColorDim).Render(rowNumStr),
					lipgloss.NewStyle().Width(12).Foreground(ColorFg).Render(patternStr),
					lipgloss.NewStyle().Width(10).Foreground(ColorSecondary).Render(repeatsStr),
					lipgloss.NewStyle().Width(15).Render(statusStr),
				)
				rows = append(rows, rowLine)
			}
		}
	}

	rows = append(rows, "")
	rows = append(rows, "")

	// Help text
	instructions := lipgloss.JoinHorizontal(
		lipgloss.Left,
		KeyStyle.Render("[↑/↓]"), HelpStyle.Render(" Navigate  "),
		KeyStyle.Render("[←/→]"), HelpStyle.Render(" Pattern  "),
		KeyStyle.Render("[</>]"), HelpStyle.Render(" Repeats  "),
	)
	rows = append(rows, instructions)

	instructions2 := lipgloss.JoinHorizontal(
		lipgloss.Left,
		KeyStyle.Render("[a]"), HelpStyle.Render(" Add  "),
		KeyStyle.Render("[i]"), HelpStyle.Render(" Insert  "),
		KeyStyle.Render("[d]"), HelpStyle.Render(" Duplicate  "),
		KeyStyle.Render("[D]"), HelpStyle.Render(" Delete  "),
	)
	rows = append(rows, instructions2)

	instructions3 := lipgloss.JoinHorizontal(
		lipgloss.Left,
		KeyStyle.Render("[s]"), HelpStyle.Render(" Toggle Song Mode  "),
		KeyStyle.Render("[l]"), HelpStyle.Render(" Toggle Loop  "),
		KeyStyle.Render("[c]"), HelpStyle.Render(" Clear  "),
	)
	rows = append(rows, instructions3)

	return strings.Join(rows, "\n")
}
