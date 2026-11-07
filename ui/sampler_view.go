package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/subbaan/gruvbox/config"
	"github.com/subbaan/gruvbox/sampler"
)

// renderSamplerView renders the sampler view with recording controls
func (m Model) renderSamplerView() string {
	// If file browser is open, show that instead
	if m.showFileBrowser {
		return m.renderWAVBrowser()
	}

	// If trim mode is active, show trim editor
	if m.trimMode {
		return m.renderTrimEditor()
	}

	var rows []string

	// Title
	samplerNum := m.selectedSampler + 1
	title := TitleStyle.Render(fmt.Sprintf("SAMPLER #%d", samplerNum))
	rows = append(rows, title)
	rows = append(rows, "")

	// Recording status
	isRecording := m.recorder.IsRecording()
	if isRecording {
		duration := m.recorder.GetDuration()
		status := lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true).
			Render(fmt.Sprintf("● RECORDING - %s", duration.Round(100)))
		rows = append(rows, status)
	} else {
		status := lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("○ Ready to record")
		rows = append(rows, status)
	}
	rows = append(rows, "")

	// Sample info
	var sample *sampler.Sample
	if m.selectedSampler == 0 {
		sample = m.engine.GetSampler1().GetSample()
	} else {
		sample = m.engine.GetSampler2().GetSample()
	}

	if sample != nil {
		rows = append(rows, HeaderStyle.Render("Loaded Sample:"))
		rows = append(rows, fmt.Sprintf("  Name:     %s", sample.Name))
		rows = append(rows, fmt.Sprintf("  Duration: %.2f seconds", sample.GetDuration()))
		rows = append(rows, fmt.Sprintf("  Trimmed:  %.2f seconds", sample.GetTrimmedDuration()))
		rows = append(rows, fmt.Sprintf("  Rate:     %d Hz", sample.SampleRate))
		rows = append(rows, fmt.Sprintf("  Channels: %d (%s)", sample.Channels,
			map[int]string{1: "Mono", 2: "Stereo"}[sample.Channels]))
		rows = append(rows, "")

		// Trim info
		rows = append(rows, HeaderStyle.Render("Trim Points:"))
		rows = append(rows, fmt.Sprintf("  Start: %d samples", sample.TrimStart))
		rows = append(rows, fmt.Sprintf("  End:   %d samples", sample.TrimEnd))
	} else {
		rows = append(rows, HelpStyle.Render("No sample loaded"))
	}
	rows = append(rows, "")
	rows = append(rows, "")

	// Controls
	rows = append(rows, HeaderStyle.Render("CONTROLS:"))
	rows = append(rows, "")

	controls := []string{
		KeyStyle.Render("[R]") + HelpStyle.Render(" - Start/Stop Recording (captures system audio)"),
		KeyStyle.Render("[L]") + HelpStyle.Render(" - Load WAV file"),
		KeyStyle.Render("[P]") + HelpStyle.Render(" - Preview/Play loaded sample"),
		KeyStyle.Render("[1/2]") + HelpStyle.Render(" - Switch between Sampler #1 / #2"),
		KeyStyle.Render("[T]") + HelpStyle.Render(" - Set trim points"),
		"",
		HelpStyle.Render("Tip: Play audio from YouTube, games, or any app,"),
		HelpStyle.Render("then press R to start recording system audio!"),
	}

	for _, ctrl := range controls {
		rows = append(rows, "  "+ctrl)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// handleSamplerKeys handles keys specific to sampler view
func (m Model) handleSamplerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If file browser is open, handle those keys instead
	if m.showFileBrowser {
		return m.handleFileBrowserKeys(msg)
	}

	// If trim mode is active, handle trim keys
	if m.trimMode {
		return m.handleTrimKeys(msg)
	}

	switch msg.String() {
	case "r", "R":
		// Toggle recording
		if m.recorder.IsRecording() {
			// Stop recording
			sample, err := m.recorder.StopRecording()
			if err != nil {
				m.statusMessage = fmt.Sprintf("Recording failed: %v", err)
				m.statusDuration = 120 // Show for 2 seconds
			} else {
				// Auto-save the recorded sample
				savePath := filepath.Join(config.RecordingsDir, sample.Name+".wav")
				if saveErr := sample.SaveWAV(savePath); saveErr == nil {
					m.statusMessage = fmt.Sprintf("Recorded %.2fs - saved to %s", sample.GetDuration(), savePath)
				} else {
					m.statusMessage = fmt.Sprintf("Recorded %.2fs (save failed: %v)", sample.GetDuration(), saveErr)
				}

				// Load sample into selected sampler
				if m.selectedSampler == 0 {
					m.engine.GetSampler1().LoadSample(sample)
				} else {
					m.engine.GetSampler2().LoadSample(sample)
				}
				m.recordedSample = sample
				m.statusDuration = 120
			}
		} else {
			// Start recording
			err := m.recorder.StartRecording()
			if err != nil {
				m.statusMessage = fmt.Sprintf("Failed to start recording: %v", err)
				m.statusDuration = 120
			} else {
				m.statusMessage = "Recording system audio..."
				m.statusDuration = 60
			}
		}
		return m, nil

	case "1":
		m.selectedSampler = 0
		return m, nil

	case "2":
		m.selectedSampler = 1
		return m, nil

	case "l", "L":
		// Open file browser for loading WAV files
		m.showFileBrowser = true
		m.availableWAVs = scanWAVFiles(config.SamplesDir)
		m.selectedWAVIndex = 0
		if len(m.availableWAVs) == 0 {
			m.statusMessage = "No WAV files found in samples/ directory"
			m.statusDuration = 120
			m.showFileBrowser = false
		}
		return m, nil

	case "p", "P":
		// Preview loaded sample
		var sample *sampler.Sample
		var samp *sampler.Sampler
		if m.selectedSampler == 0 {
			samp = m.engine.GetSampler1()
			sample = samp.GetSample()
		} else {
			samp = m.engine.GetSampler2()
			sample = samp.GetSample()
		}

		if sample == nil {
			m.statusMessage = "No sample loaded to preview"
			m.statusDuration = 120
		} else {
			samp.StopAll()    // Stop any currently playing voices first
			samp.Trigger(0.8) // Trigger at 80% velocity
			m.statusMessage = fmt.Sprintf("Previewing %s", sample.Name)
			m.statusDuration = 60
		}
		return m, nil

	case "t", "T":
		// Open trim editor
		var sample *sampler.Sample
		if m.selectedSampler == 0 {
			sample = m.engine.GetSampler1().GetSample()
		} else {
			sample = m.engine.GetSampler2().GetSample()
		}

		if sample == nil {
			m.statusMessage = "No sample loaded to trim"
			m.statusDuration = 120
		} else {
			m.trimMode = true
			m.trimEditingStart = true // Start by editing the start point
		}
		return m, nil
	}

	return m, nil
}

// scanWAVFiles scans a directory recursively for WAV files
func scanWAVFiles(dir string) []string {
	var wavFiles []string

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return wavFiles
	}

	// Walk the directory tree recursively
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".wav") {
			// Store relative path from samples/ directory
			relPath, _ := filepath.Rel(dir, path)
			wavFiles = append(wavFiles, relPath)
		}
		return nil
	})

	return wavFiles
}

// handleFileBrowserKeys handles keyboard input for the file browser
func (m Model) handleFileBrowserKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check if renaming
	if m.renamingFile {
		switch msg.String() {
		case "esc":
			m.renamingFile = false
			m.renameInput = ""
			return m, nil

		case "enter":
			// Perform rename
			if m.renameInput != "" && m.selectedWAVIndex < len(m.availableWAVs) {
				oldPath := filepath.Join(config.SamplesDir, m.availableWAVs[m.selectedWAVIndex])
				newPath := filepath.Join(config.SamplesDir, m.renameInput+".wav")
				if err := os.Rename(oldPath, newPath); err == nil {
					m.availableWAVs[m.selectedWAVIndex] = m.renameInput + ".wav"
					m.statusMessage = fmt.Sprintf("Renamed to %s", m.renameInput+".wav")
					m.statusDuration = 60
				} else {
					m.statusMessage = fmt.Sprintf("Rename failed: %v", err)
					m.statusDuration = 120
				}
			}
			m.renamingFile = false
			m.renameInput = ""
			return m, nil

		case "backspace":
			if len(m.renameInput) > 0 {
				m.renameInput = m.renameInput[:len(m.renameInput)-1]
			}
			return m, nil

		default:
			// Add character to rename input
			if len(msg.String()) == 1 {
				m.renameInput += msg.String()
			}
			return m, nil
		}
	}

	// Normal file browser navigation
	switch msg.String() {
	case "esc":
		m.showFileBrowser = false
		return m, nil

	case "up":
		if m.selectedWAVIndex > 0 {
			m.selectedWAVIndex--
		}
		return m, nil

	case "down":
		if m.selectedWAVIndex < len(m.availableWAVs)-1 {
			m.selectedWAVIndex++
		}
		return m, nil

	case "pgup": // Page up (jump 10 files)
		m.selectedWAVIndex -= 10
		if m.selectedWAVIndex < 0 {
			m.selectedWAVIndex = 0
		}
		return m, nil

	case "pgdown": // Page down (jump 10 files)
		m.selectedWAVIndex += 10
		if m.selectedWAVIndex >= len(m.availableWAVs) {
			m.selectedWAVIndex = len(m.availableWAVs) - 1
		}
		return m, nil

	case "home", "g": // Jump to first file
		m.selectedWAVIndex = 0
		return m, nil

	case "end", "G": // Jump to last file
		if len(m.availableWAVs) > 0 {
			m.selectedWAVIndex = len(m.availableWAVs) - 1
		}
		return m, nil

	case "enter":
		// Load selected WAV file
		if m.selectedWAVIndex < len(m.availableWAVs) {
			filename := filepath.Join(config.SamplesDir, m.availableWAVs[m.selectedWAVIndex])
			sample := sampler.NewSample(m.availableWAVs[m.selectedWAVIndex])
			if err := sample.LoadWAV(filename); err == nil {
				// Load into selected sampler
				if m.selectedSampler == 0 {
					m.engine.GetSampler1().LoadSample(sample)
				} else {
					m.engine.GetSampler2().LoadSample(sample)
				}
				m.statusMessage = fmt.Sprintf("Loaded %s", m.availableWAVs[m.selectedWAVIndex])
				m.statusDuration = 60
				m.showFileBrowser = false
			} else {
				m.statusMessage = fmt.Sprintf("Failed to load: %v", err)
				m.statusDuration = 120
			}
		}
		return m, nil

	case "p", "P":
		// Preview selected WAV file
		if m.selectedWAVIndex < len(m.availableWAVs) {
			filename := filepath.Join(config.SamplesDir, m.availableWAVs[m.selectedWAVIndex])
			sample := sampler.NewSample(m.availableWAVs[m.selectedWAVIndex])
			if err := sample.LoadWAV(filename); err == nil {
				// Load into selected sampler temporarily and trigger
				var samp *sampler.Sampler
				if m.selectedSampler == 0 {
					samp = m.engine.GetSampler1()
				} else {
					samp = m.engine.GetSampler2()
				}

				// Store current sample for potential restore
				m.previewingSample = samp.GetSample()

				// Load and trigger preview
				samp.LoadSample(sample)
				samp.Trigger(0.8) // Trigger at 80% velocity

				m.statusMessage = fmt.Sprintf("Previewing %s", m.availableWAVs[m.selectedWAVIndex])
				m.statusDuration = 60
			} else {
				m.statusMessage = fmt.Sprintf("Failed to load preview: %v", err)
				m.statusDuration = 120
			}
		}
		return m, nil

	case "r", "R":
		// Start renaming
		if m.selectedWAVIndex < len(m.availableWAVs) {
			m.renamingFile = true
			// Get filename without extension for editing
			name := m.availableWAVs[m.selectedWAVIndex]
			m.renameInput = strings.TrimSuffix(name, filepath.Ext(name))
		}
		return m, nil

	case "d", "D":
		// Delete file (with confirmation)
		// TODO: Add confirmation dialog
		return m, nil
	}

	return m, nil
}

// renderWAVBrowser renders the WAV file browser UI
func (m Model) renderWAVBrowser() string {
	var rows []string

	title := TitleStyle.Render("LOAD WAV FILE")
	rows = append(rows, title)
	rows = append(rows, "")

	if m.renamingFile {
		rows = append(rows, HeaderStyle.Render("RENAME FILE:"))
		rows = append(rows, "")
		rows = append(rows, fmt.Sprintf("  %s.wav", m.renameInput+"_"))
		rows = append(rows, "")
		rows = append(rows, HelpStyle.Render("  [Enter] Confirm  [Esc] Cancel"))
	} else {
		// Display file list
		rows = append(rows, HeaderStyle.Render("FILES:"))
		rows = append(rows, "")

		// Show up to 10 files at a time
		start := 0
		if m.selectedWAVIndex > 5 {
			start = m.selectedWAVIndex - 5
		}
		end := start + 10
		if end > len(m.availableWAVs) {
			end = len(m.availableWAVs)
		}

		for i := start; i < end; i++ {
			prefix := "  "
			style := lipgloss.NewStyle()
			if i == m.selectedWAVIndex {
				prefix = "> "
				style = style.Foreground(ColorAccent).Bold(true)
			}
			rows = append(rows, prefix+style.Render(m.availableWAVs[i]))
		}

		rows = append(rows, "")
		rows = append(rows, HeaderStyle.Render("CONTROLS:"))
		rows = append(rows, "")
		controls := []string{
			KeyStyle.Render("[↑/↓]") + HelpStyle.Render(" - Navigate"),
			KeyStyle.Render("[Enter]") + HelpStyle.Render(" - Load file"),
			KeyStyle.Render("[P]") + HelpStyle.Render(" - Preview sample"),
			KeyStyle.Render("[R]") + HelpStyle.Render(" - Rename"),
			KeyStyle.Render("[Esc]") + HelpStyle.Render(" - Cancel"),
		}
		for _, ctrl := range controls {
			rows = append(rows, "  "+ctrl)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderTrimEditor renders the sample trim editor
func (m Model) renderTrimEditor() string {
	var rows []string

	title := TitleStyle.Render("TRIM EDITOR")
	rows = append(rows, title)
	rows = append(rows, "")

	// Get current sample and sampler
	var sample *sampler.Sample
	var samp *sampler.Sampler
	if m.selectedSampler == 0 {
		samp = m.engine.GetSampler1()
		sample = samp.GetSample()
	} else {
		samp = m.engine.GetSampler2()
		sample = samp.GetSample()
	}

	if sample == nil {
		rows = append(rows, HelpStyle.Render("No sample loaded"))
		return lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	// Sample info
	rows = append(rows, HeaderStyle.Render("SAMPLE:"))
	rows = append(rows, fmt.Sprintf("  Name: %s", sample.Name))
	rows = append(rows, fmt.Sprintf("  Total Length: %d samples (%.2f seconds)", sample.Length, sample.GetDuration()))
	rows = append(rows, "")

	// Trim points
	rows = append(rows, HeaderStyle.Render("TRIM POINTS:"))
	rows = append(rows, "")

	startStyle := lipgloss.NewStyle()
	endStyle := lipgloss.NewStyle()
	if m.trimEditingStart {
		startStyle = startStyle.Foreground(ColorAccent).Bold(true)
	} else {
		endStyle = endStyle.Foreground(ColorAccent).Bold(true)
	}

	startTime := float64(sample.TrimStart) / float64(sample.SampleRate)
	endTime := float64(sample.TrimEnd) / float64(sample.SampleRate)
	trimmedDuration := sample.GetTrimmedDuration()

	rows = append(rows, "  "+startStyle.Render(fmt.Sprintf("Start: %d samples (%.3f s)", sample.TrimStart, startTime)))
	rows = append(rows, "  "+endStyle.Render(fmt.Sprintf("End:   %d samples (%.3f s)", sample.TrimEnd, endTime)))
	rows = append(rows, "")
	rows = append(rows, fmt.Sprintf("  Trimmed Duration: %.3f seconds", trimmedDuration))
	rows = append(rows, "")

	// Get playback position for playhead display
	playbackPos := samp.GetPlaybackPosition()

	// Visual waveform
	rows = append(rows, HeaderStyle.Render("WAVEFORM:"))
	rows = append(rows, "")
	waveform := m.renderWaveform(sample, playbackPos)
	for _, line := range waveform {
		rows = append(rows, "  "+line)
	}
	rows = append(rows, "")

	// Show playback position if playing
	if playbackPos >= 0 {
		posTime := float64(playbackPos) / float64(sample.SampleRate)
		rows = append(rows, fmt.Sprintf("  Playback: %.3f seconds (%d samples)", posTime, playbackPos))
		rows = append(rows, "")
	}

	// Controls
	rows = append(rows, HeaderStyle.Render("CONTROLS:"))
	rows = append(rows, "")
	controls := []string{
		KeyStyle.Render("[Space]") + HelpStyle.Render(" - Switch between Start/End"),
		KeyStyle.Render("[←/→]") + HelpStyle.Render(" - Fine adjust (10 samples)"),
		KeyStyle.Render("[,/.]") + HelpStyle.Render(" - Fine adjust (10 samples)"),
		KeyStyle.Render("[</>]") + HelpStyle.Render(" - Coarse adjust (1000 samples)"),
		KeyStyle.Render("[Home/End]") + HelpStyle.Render(" - Jump to beginning/end"),
		"",
		KeyStyle.Render("[P]") + HelpStyle.Render(" - Preview trimmed sample"),
		KeyStyle.Render("[ ]") + HelpStyle.Render(" - Set Start/End to playhead position (while playing)"),
		KeyStyle.Render("[S]") + HelpStyle.Render(" - Save trimmed sample as new file"),
		"",
		KeyStyle.Render("[Enter]") + HelpStyle.Render(" - Apply and close"),
		KeyStyle.Render("[Esc]") + HelpStyle.Render(" - Cancel (revert changes)"),
	}
	for _, ctrl := range controls {
		rows = append(rows, "  "+ctrl)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderTrimBar renders an ASCII bar showing the trim region
func (m Model) renderTrimBar(sample *sampler.Sample) string {
	const barWidth = 60

	if sample == nil || sample.Length == 0 {
		return strings.Repeat("░", barWidth)
	}

	// Calculate positions
	startPos := int(float64(sample.TrimStart) / float64(sample.Length) * float64(barWidth))
	endPos := int(float64(sample.TrimEnd) / float64(sample.Length) * float64(barWidth))

	if startPos < 0 {
		startPos = 0
	}
	if endPos > barWidth {
		endPos = barWidth
	}

	// Build bar
	var bar strings.Builder
	for i := 0; i < barWidth; i++ {
		if i < startPos {
			bar.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render("░"))
		} else if i >= endPos {
			bar.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render("░"))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Render("█"))
		}
	}

	return bar.String()
}

// renderWaveform renders an ASCII waveform visualization of the sample
// playbackPos is the current playback position in samples (-1 if not playing)
func (m Model) renderWaveform(sample *sampler.Sample, playbackPos int) []string {
	const width = 80
	const height = 11 // 5 above center, 1 center, 5 below

	lines := make([]string, height)

	if sample == nil || len(sample.Data) == 0 {
		// Return empty waveform
		for i := 0; i < height; i++ {
			lines[i] = strings.Repeat(" ", width)
		}
		return lines
	}

	// Sample the audio data across the width
	samplesPerColumn := len(sample.Data) / width
	if samplesPerColumn < 1 {
		samplesPerColumn = 1
	}

	// Calculate trim positions in screen coordinates
	trimStartPos := int(float64(sample.TrimStart) / float64(sample.Length) * float64(width))
	trimEndPos := int(float64(sample.TrimEnd) / float64(sample.Length) * float64(width))

	// Calculate playback position in screen coordinates
	playheadPos := -1
	if playbackPos >= 0 && playbackPos < sample.Length {
		playheadPos = int(float64(playbackPos) / float64(sample.Length) * float64(width))
	}

	// For stereo, we need to account for interleaved samples
	if sample.Channels == 2 {
		samplesPerColumn *= 2
	}

	// Build waveform data
	waveformData := make([]float32, width)
	var globalMax float32 = 0

	// First pass: collect peak amplitudes and find global maximum
	for x := 0; x < width; x++ {
		startIdx := x * samplesPerColumn
		endIdx := startIdx + samplesPerColumn

		if endIdx > len(sample.Data) {
			endIdx = len(sample.Data)
		}
		if startIdx >= len(sample.Data) {
			break
		}

		// Find peak amplitude in this window
		var maxAmp float32 = 0
		for i := startIdx; i < endIdx; i++ {
			amp := sample.Data[i]
			if amp < 0 {
				amp = -amp
			}
			if amp > maxAmp {
				maxAmp = amp
			}
		}
		waveformData[x] = maxAmp

		// Track global maximum
		if maxAmp > globalMax {
			globalMax = maxAmp
		}
	}

	// Normalize waveform data to make quiet parts more visible
	if globalMax > 0.001 { // Avoid division by zero
		for x := 0; x < width; x++ {
			waveformData[x] = waveformData[x] / globalMax
		}
	}

	// Initialize lines
	for i := 0; i < height; i++ {
		lines[i] = ""
	}

	// Build the visual representation
	centerLine := height / 2

	for x := 0; x < width; x++ {
		amp := waveformData[x]

		// Determine if this column is in the trimmed region
		isTrimmed := x >= trimStartPos && x < trimEndPos

		// Calculate how many lines this amplitude spans (from center)
		ampHeight := int(amp * float32(centerLine))
		if ampHeight > centerLine {
			ampHeight = centerLine
		}

		// Choose character and style based on amplitude and trim status
		var char string
		var style lipgloss.Style

		// Playhead takes priority over everything
		if x == playheadPos {
			style = lipgloss.NewStyle().Foreground(ColorPlayback).Bold(true)
			char = "▐"
		} else if !isTrimmed {
			// Trimmed out region - dim
			style = lipgloss.NewStyle().Foreground(ColorDim)
			char = "░"
		} else {
			// Active region - bright
			style = lipgloss.NewStyle().Foreground(ColorAccent)
			if amp < 0.05 {
				char = "·" // Very quiet
			} else if amp < 0.3 {
				char = "▪" // Quiet
			} else if amp < 0.7 {
				char = "▮" // Medium
			} else {
				char = "█" // Loud
			}
		}

		// Mark trim boundaries (but playhead takes priority)
		if x != playheadPos && (x == trimStartPos || x == trimEndPos) {
			style = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
			char = "|"
		}

		// Fill in the waveform vertically
		for y := 0; y < height; y++ {
			// Calculate distance from center
			distFromCenter := y - centerLine
			if distFromCenter < 0 {
				distFromCenter = -distFromCenter
			}

			// Draw if within amplitude range
			if distFromCenter <= ampHeight || (x == trimStartPos || x == trimEndPos) {
				lines[y] += style.Render(char)
			} else if y == centerLine && !isTrimmed {
				// Center line for trimmed-out regions
				lines[y] += style.Render("─")
			} else if y == centerLine && isTrimmed && amp < 0.05 {
				// Center line for very quiet regions
				lines[y] += lipgloss.NewStyle().Foreground(ColorDim).Render("─")
			} else {
				// Empty space
				lines[y] += " "
			}
		}
	}

	return lines
}

// handleTrimKeys handles keyboard input for the trim editor
func (m Model) handleTrimKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Get current sample
	var sample *sampler.Sample
	var samp *sampler.Sampler
	if m.selectedSampler == 0 {
		samp = m.engine.GetSampler1()
		sample = samp.GetSample()
	} else {
		samp = m.engine.GetSampler2()
		sample = samp.GetSample()
	}

	if sample == nil {
		m.trimMode = false
		return m, nil
	}

	switch msg.String() {
	case "esc":
		// Cancel - revert changes (would need to store original values)
		m.trimMode = false
		return m, nil

	case "enter":
		// Apply and close
		m.trimMode = false
		m.statusMessage = "Trim applied"
		m.statusDuration = 60
		return m, nil

	case " ":
		// Switch between start/end editing
		m.trimEditingStart = !m.trimEditingStart
		return m, nil

	case "left", ",":
		// Fine adjust left
		if m.trimEditingStart {
			sample.TrimStart -= 10
			if sample.TrimStart < 0 {
				sample.TrimStart = 0
			}
		} else {
			sample.TrimEnd -= 10
			if sample.TrimEnd < sample.TrimStart {
				sample.TrimEnd = sample.TrimStart
			}
		}
		return m, nil

	case "right", ".":
		// Fine adjust right
		if m.trimEditingStart {
			sample.TrimStart += 10
			if sample.TrimStart > sample.TrimEnd {
				sample.TrimStart = sample.TrimEnd
			}
		} else {
			sample.TrimEnd += 10
			if sample.TrimEnd > sample.Length {
				sample.TrimEnd = sample.Length
			}
		}
		return m, nil

	case "<":
		// Coarse adjust left
		if m.trimEditingStart {
			sample.TrimStart -= 1000
			if sample.TrimStart < 0 {
				sample.TrimStart = 0
			}
		} else {
			sample.TrimEnd -= 1000
			if sample.TrimEnd < sample.TrimStart {
				sample.TrimEnd = sample.TrimStart
			}
		}
		return m, nil

	case ">":
		// Coarse adjust right
		if m.trimEditingStart {
			sample.TrimStart += 1000
			if sample.TrimStart > sample.TrimEnd {
				sample.TrimStart = sample.TrimEnd
			}
		} else {
			sample.TrimEnd += 1000
			if sample.TrimEnd > sample.Length {
				sample.TrimEnd = sample.Length
			}
		}
		return m, nil

	case "home":
		// Jump to beginning
		if m.trimEditingStart {
			sample.TrimStart = 0
		} else {
			sample.TrimEnd = sample.TrimStart
		}
		return m, nil

	case "end":
		// Jump to end
		if m.trimEditingStart {
			sample.TrimStart = sample.TrimEnd
		} else {
			sample.TrimEnd = sample.Length
		}
		return m, nil

	case "p", "P":
		// Preview trimmed sample
		if sample == nil {
			m.statusMessage = "No sample to preview"
			m.statusDuration = 60
			return m, nil
		}

		// Show what will be played
		trimDuration := sample.GetTrimmedDuration()
		totalDuration := sample.GetDuration()

		samp.StopAll() // Stop any currently playing voices first
		samp.Trigger(0.8)
		m.statusMessage = fmt.Sprintf("Preview: %.2fs of %.2fs (trim: %d-%d samples)",
			trimDuration, totalDuration, sample.TrimStart, sample.TrimEnd)
		m.statusDuration = 120
		return m, nil

	case "[":
		// Set trim start to current playback position
		playbackPos := samp.GetPlaybackPosition()
		if playbackPos >= 0 {
			sample.TrimStart = playbackPos
			// Ensure TrimStart doesn't exceed TrimEnd
			if sample.TrimStart > sample.TrimEnd {
				sample.TrimStart = sample.TrimEnd
			}
			m.statusMessage = fmt.Sprintf("Trim start set to %.3f seconds", float64(sample.TrimStart)/float64(sample.SampleRate))
			m.statusDuration = 60
		} else {
			m.statusMessage = "No playback - press P to preview first"
			m.statusDuration = 60
		}
		return m, nil

	case "]":
		// Set trim end to current playback position
		playbackPos := samp.GetPlaybackPosition()
		if playbackPos >= 0 {
			sample.TrimEnd = playbackPos
			// Ensure TrimEnd doesn't go below TrimStart
			if sample.TrimEnd < sample.TrimStart {
				sample.TrimEnd = sample.TrimStart
			}
			m.statusMessage = fmt.Sprintf("Trim end set to %.3f seconds", float64(sample.TrimEnd)/float64(sample.SampleRate))
			m.statusDuration = 60
		} else {
			m.statusMessage = "No playback - press P to preview first"
			m.statusDuration = 60
		}
		return m, nil

	case "s", "S":
		// Save trimmed sample as new file
		trimmedData := sample.GetTrimmedData()
		if len(trimmedData) == 0 {
			m.statusMessage = "No data to save (check trim points)"
			m.statusDuration = 120
			return m, nil
		}

		// Create new sample with trimmed data
		trimmedSample := sampler.NewSample(sample.Name + "_trimmed")
		trimmedSample.Data = trimmedData
		trimmedSample.Channels = sample.Channels
		trimmedSample.SampleRate = sample.SampleRate
		trimmedSample.Length = len(trimmedData) / sample.Channels
		trimmedSample.TrimStart = 0
		trimmedSample.TrimEnd = trimmedSample.Length

		// Ensure trimmed directory exists
		if err := os.MkdirAll(config.TrimmedDir, 0755); err != nil {
			m.statusMessage = fmt.Sprintf("Failed to create directory: %v", err)
			m.statusDuration = 120
			return m, nil
		}

		// Save to trimmed directory
		savePath := filepath.Join(config.TrimmedDir, trimmedSample.Name+".wav")
		if err := trimmedSample.SaveWAV(savePath); err != nil {
			m.statusMessage = fmt.Sprintf("Save failed: %v", err)
			m.statusDuration = 120
		} else {
			m.statusMessage = fmt.Sprintf("Saved: samples/trimmed/%s.wav", trimmedSample.Name)
			m.statusDuration = 120
		}
		return m, nil
	}

	return m, nil
}
