package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/subbaan/gruvbox/audio"
	"github.com/subbaan/gruvbox/config"
	"github.com/subbaan/gruvbox/dsp"
	"github.com/subbaan/gruvbox/effects"
	"github.com/subbaan/gruvbox/project"
	"github.com/subbaan/gruvbox/sampler"
	"github.com/subbaan/gruvbox/sequencer"
	"github.com/subbaan/gruvbox/synth"
	"github.com/subbaan/gruvbox/synth/drums"
)

// tickMsg is sent periodically to check for quantization boundaries
type tickMsg time.Time

// ViewMode represents the current view
type ViewMode int

const (
	ViewSequencer ViewMode = iota
	ViewSynth
	ViewDrums
	ViewEffects
	ViewMixer
	ViewSong
	ViewSampler
	ViewDrone
	ViewArp
)

// Model is the main Bubble Tea model
type Model struct {
	// Audio engine reference
	engine *audio.Engine
	seq    *sequencer.Sequencer

	// UI state
	currentView       ViewMode
	previousSoundView ViewMode // Tracks which sound view (Synth/Drums/Sampler) to return to from Effects
	width             int
	height            int
	ready             bool
	version           string // Application version

	// Sequencer state
	selectedTrack int
	selectedStep  int
	cursorX       int
	cursorY       int
	stepOffset    int  // Scroll offset for viewing patterns longer than 16 steps
	noteEntryMode bool // If true, keyboard acts as piano for note entry
	currentOctave int  // Current octave for note entry (2-6)
	fillVariation int  // Current fill pattern variation (0-3)
	buildupVariation int  // Current buildup pattern variation (0-2)
	kickVariation int  // Current kick pattern variation (0-3)
	snareVariation int  // Current snare pattern variation (0-3)
	hihatVariation int  // Current hi-hat pattern variation (0-6)

	// Piano roll state
	pianoRollMode         bool // Whether piano roll is active
	pianoRollOctaveOffset int  // Which octave to display (0=C2-C4, 1=C3-C5, etc.)
	pianoRollCursorX      int  // Step position (0 to pattern length)
	pianoRollCursorY      int  // Pitch position (0-23 for 2 octaves, 0=lowest note)

	// Fill auto-return state
	fillPending          bool                        // True if a fill is requested and waiting for quantization
	fillPendingType      string                      // "fill" or "buildup"
	fillReturnActive     bool                        // True if a fill is playing that should auto-return
	fillReturnTrack      int                         // Which track has the active fill (0-7)
	fillReturnPattern    *sequencer.Pattern          // Saved pattern to return to after fill
	fillStartStep        int                         // Step number when fill was triggered
	fillHasProgressed    bool                        // True once we've progressed past step 0 in the fill

	// Synth view state
	selectedSynth int // 0 = TB-303 #1, 1 = TB-303 #2
	selectedParam int // Index of selected parameter

	// Drum view state
	selectedDrum      int // 0 = Kick, 1 = Snare, 2 = HH Closed, 3 = HH Open, 4 = Clap, 5 = Rimshot, 6 = Crash, 7 = Tom
	selectedDrumParam int // Index of selected parameter

	// Drone view state
	selectedDroneSection int // 0=Oscillators, 1=Filter&Env, 2=Modulation
	selectedDroneParam   int // Index of selected parameter within current section

	// Arpeggiator view state
	selectedArpParam int  // Index of selected arp parameter
	arpChordEditMode bool // Whether in chord editing mode
	arpChordBuffer   []int // Temporary chord being edited

	// Effects view state
	selectedChannel    int // 0-7 for the 8 mixer channels
	selectedEffect     int // 0-5: Distortion, Delay, Reverb, PitchShifter, Filter, BitCrusher
	selectedEffectParam int // Index of selected effect parameter

	// Mixer view state
	selectedMixerChannel int // 0-5 for the 6 channels
	selectedMixerParam   int // 0 = Level, 1 = Pan, 2 = Mute, 3 = Solo

	// Pending mixer changes (for quantized mute/solo)
	pendingMuteChannel  int  // -1 = none, 0-5 = channel to toggle
	pendingSoloChannel  int  // -1 = none, 0-5 = channel to toggle
	quantizeToBar       bool // false = quantize to pattern end, true = quantize to 4-bar phrase

	// Song view state
	selectedSongStep int // Currently selected step in song arrangement
	songEditMode     int // 0 = navigate, 1 = edit pattern ID, 2 = edit repeats
	lastSongPatternID int // Last pattern ID in song mode (for detecting changes)

	// Save/Load state
	saveLoadMode         int      // 0 = none, 1 = save, 2 = load (file browser)
	filenameInput        string   // Filename being typed (for save mode)
	availableFiles       []string // List of .gruvbox files in projects/ directory
	selectedFileIndex    int      // Selected file in browser
	sortMode             int      // 0 = name, 1 = date modified
	currentProjectName   string   // Name of currently loaded/saved project
	deleteConfirmMode    bool     // Whether in delete confirmation mode
	deleteConfirmProject string   // Project name pending deletion
	statusMessage        string   // Status/feedback message
	statusDuration       int      // Frames to display status (0 = hide)

	// Autosave state
	autosaveEnabled    bool   // Whether autosave is enabled
	autosavePath       string // Path to autosave file
	autosaveCounter    int    // Frame counter for autosave
	autosaveInterval   int    // Frames between autosaves (300 = ~5 seconds at 60fps)

	// Transport state
	isPlaying bool
	bpm       float32
	swing     float32 // Swing amount 0.0-1.0

	// Help screen state
	showHelp bool

	// Sampler view state
	selectedSampler    int // 0 = Sampler #1, 1 = Sampler #2
	recorder           *sampler.Recorder
	recordedSample     *sampler.Sample // Last recorded sample
	showFileBrowser    bool            // Whether file browser is open
	availableWAVs      []string        // List of WAV files
	selectedWAVIndex   int             // Selected file in browser
	renamingFile       bool            // Whether currently renaming
	renameInput        string          // Rename input buffer
	previewingSample   *sampler.Sample // Currently previewing sample
	trimMode           bool            // Whether in trim editor
	trimEditingStart   bool            // true = editing start, false = editing end

	// Step condition editor
	conditionEditMode      bool
	conditionEditStep      *sequencer.Step

	// Sound parameter dirty tracking
	soundParamsDirty bool // True when sound params changed but not saved to pattern
	conditionEditSelection int // Which condition is selected
	conditionEditParameter int // For EVERY_NTH parameter

	// Euclidean generator
	euclideanMode       bool
	euclideanPulses     int
	euclideanRotation   int
	euclideanEditParam  int  // 0=pulses, 1=rotation
	euclideanReplace    bool // true=replace pattern, false=layer

	// Pattern length editor
	lengthEditMode  bool

	// P-lock generator menu
	plockGenMenuOpen       bool   // Whether the p-lock generator menu is open
	plockGenSelectedIndex  int    // Selected generator in the menu
	paramSelectMenuOpen    bool   // Whether parameter selection menu is open (for sequencer view)
	paramSelectIndex       int    // Selected parameter in the menu
	paramSelectForGenParam string // Stored parameter name when selected from sequencer

	// Drum pattern menu
	drumPatternMenuOpen    bool // Whether drum pattern menu is open
	drumPatternSelectedIdx int  // Selected pattern in the menu

	lengthEditValue        int

	// Manual value entry
	manualInputMode  bool   // Whether in manual input mode
	manualInputValue string // Value being typed
}


// NewModel creates a new TUI model
func NewModel(engine *audio.Engine, version string) Model {
	return Model{
		engine:             engine,
		seq:                engine.GetSequencer(),
		currentView:        ViewSequencer,
		previousSoundView:  ViewSynth, // Default to Synth view for Tab toggle
		version:            version,
		selectedTrack:      0,
		selectedStep:       0,
		cursorX:            0,
		cursorY:            0,
		noteEntryMode:      false,
		currentOctave:      3, // Middle octave (C3 = MIDI 48)
		pendingMuteChannel: -1,
		pendingSoloChannel: -1,
		quantizeToBar:      false,
		selectedSongStep:   0,
		songEditMode:       0,
		lastSongPatternID:  -1,
		bpm:                128.0,
		swing:              0.0,
		selectedSampler:    0,
		recorder:           sampler.NewRecorder(),
		recordedSample:     nil,
		autosaveEnabled:    false, // Disabled per user request
		autosavePath:       filepath.Join(config.ProjectsDir, "autosave.gruvbox"),
		autosaveCounter:    0,
		autosaveInterval:   1800, // ~30 seconds at 60fps
		fillPending:        false,
		fillPendingType:    "",
		fillReturnActive:   false,
		fillReturnTrack:    -1,
		fillReturnPattern:  nil,
		fillStartStep:      -1,
		fillHasProgressed:  false,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tickCmd()
}

// tickCmd returns a command that sends a tick message every 16ms
func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tickMsg:
		// Apply pending mixer changes at quantization boundaries
		if m.isPlaying {
			m.applyPendingMixerChanges()
		}

		// Apply pending fills at pattern start (quantize to loop)
		if m.fillPending && m.isPlaying {
			currentStep := m.seq.GetCurrentStep()

			// Get the track that will receive the fill
			instrument := trackInstruments[m.fillReturnTrack]
			track := m.seq.GetTrack(instrument)

			// Calculate local step for this track's pattern (enables polyrhythm support)
			if track != nil && track.Pattern != nil {
				localStep := currentStep % track.Pattern.Length
				if localStep == 0 {
					m.applyPendingFill()
				}
			}
		}

		// Handle fill auto-return (check if fill has played once and should return)
		if m.fillReturnActive && m.isPlaying {
			currentStep := m.seq.GetCurrentStep()

			// Get the track that has the fill
			instrument := trackInstruments[m.fillReturnTrack]
			track := m.seq.GetTrack(instrument)

			if track != nil && track.Pattern != nil {
				// Calculate local step for this track's pattern
				localStep := currentStep % track.Pattern.Length

				// Track that we've progressed past step 0
				if localStep > 0 {
					m.fillHasProgressed = true
				}

				// Check if we've completed a full loop (back to step 0 after progressing)
				if localStep == 0 && m.fillHasProgressed {
					// Check if saved pattern was empty - if so, generate default pattern
					var patternToRestore *sequencer.Pattern
					if isPatternEmpty(m.fillReturnPattern) {
						// Generate default pattern for this instrument
						patternToRestore = m.generateDefaultPattern(instrument)
					} else {
						// Use saved pattern
						patternToRestore = m.fillReturnPattern
					}

					if patternToRestore != nil {
						track.SetPattern(patternToRestore)
					}

					// Reset fill return state
					m.fillReturnActive = false
					m.fillReturnPattern = nil
					m.fillReturnTrack = -1
					m.fillStartStep = -1
					m.fillHasProgressed = false
				}
			}
		}

		// Decrement status message duration
		if m.statusDuration > 0 {
			m.statusDuration--
			if m.statusDuration == 0 {
				m.statusMessage = ""
			}
		}

		// Autosave logic
		if m.autosaveEnabled {
			m.autosaveCounter++
			if m.autosaveCounter >= m.autosaveInterval {
				m.autosaveCounter = 0
				// Perform autosave silently in background
				go m.performAutosave()
			}
		}

		// Check for song mode pattern changes and load sound settings
		song := m.seq.GetSong()
		if song.IsSongMode() && m.isPlaying {
			currentPatternID := song.GetCurrentPatternID()
			if currentPatternID >= 0 && currentPatternID != m.lastSongPatternID {
				// Pattern has changed - load sound settings for all tracks
				m.loadAllSoundsFromPattern()
				m.lastSongPatternID = currentPatternID
			}
		} else if !song.IsSongMode() {
			// Reset tracking when not in song mode
			m.lastSongPatternID = -1
		}

		return m, tickCmd() // Schedule next tick

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle special input modes FIRST before any global keybinds
	// This prevents global keys from interfering with text input

	// Manual value entry
	if m.manualInputMode {
		return m.handleManualInputKeys(msg)
	}

	// Step condition editor
	if m.conditionEditMode {
		return m.handleConditionEditorKeys(msg)
	}

	// Euclidean generator
	if m.euclideanMode {
		return m.handleEuclideanGeneratorKeys(msg)
	}

	// Pattern length editor
	if m.lengthEditMode {
		return m.handleLengthEditorKeys(msg)
	}

	// Parameter selection menu (for sequencer -> p-lock generator)
	if m.paramSelectMenuOpen {
		return m.handleParamSelectMenuKeys(msg)
	}

	// P-lock generator menu
	if m.plockGenMenuOpen {
		return m.handlePLockGenMenuKeys(msg)
	}

	// Drum pattern menu
	if m.drumPatternMenuOpen {
		return m.handleDrumPatternMenuKeys(msg)
	}

	// Save/load mode
	if m.saveLoadMode > 0 {
		return m.handleSaveLoadInput(msg)
	}

	// File renaming mode (in file browser)
	if m.renamingFile {
		return m.handleSamplerKeys(msg)
	}

	// Trim mode (has its own special key handling)
	if m.trimMode {
		return m.handleSamplerKeys(msg)
	}

	// Piano roll mode
	if m.pianoRollMode {
		return m.handlePianoRollKeys(msg)
	}

	switch msg.String() {
	case "ctrl+q", "ctrl+c":
		return m, tea.Quit

	case "?": // Toggle help screen
		m.showHelp = !m.showHelp
		return m, nil

	case "ctrl+s": // Save project
		m.saveLoadMode = 1
		// Default to current project name if we have one, otherwise empty
		if m.currentProjectName != "" {
			m.filenameInput = m.currentProjectName
		} else {
			m.filenameInput = ""
		}
		m.scanProjectFiles()
		// Select the current project in the file list if it exists
		m.selectedFileIndex = 0
		if m.currentProjectName != "" {
			for i, filename := range m.availableFiles {
				if filename == m.currentProjectName {
					m.selectedFileIndex = i
					break
				}
			}
		}
		return m, nil

	case "ctrl+l": // Load project
		m.saveLoadMode = 2
		m.filenameInput = ""
		m.scanProjectFiles()
		m.selectedFileIndex = 0
		return m, nil

	case "`": // Backtick = play/stop
		if m.isPlaying {
			m.seq.Stop()
			m.isPlaying = false
			// Release drone note when stopping
			m.engine.GetDrone().NoteOff()
		} else {
			m.seq.Start()
			m.isPlaying = true

			// If starting playback in song mode, load the current pattern's sound settings
			song := m.seq.GetSong()
			if song.IsSongMode() {
				currentPatternID := song.GetCurrentPatternID()
				if currentPatternID >= 0 {
					m.loadAllSoundsFromPattern()
					m.lastSongPatternID = currentPatternID
				}
			}
		}
		return m, nil

	case "esc": // Return to sequencer from any view
		// Don't switch views if we're in a special mode (trim mode, file browser, etc.)
		if m.trimMode || m.showFileBrowser || m.saveLoadMode > 0 {
			// Let the view-specific handler deal with it
			break
		}
		if m.currentView != ViewSequencer {
			m.currentView = ViewSequencer
			return m, nil
		}

	case "tab": // Smart Tab navigation
		switch m.currentView {
		case ViewSequencer:
			// Smart tab based on selected track
			instrument := trackInstruments[m.selectedTrack]

			switch instrument {
			case sequencer.InstrumentTB303_1:
				m.selectedSynth = 0
				m.currentView = ViewSynth
				m.previousSoundView = ViewSynth
			case sequencer.InstrumentTB303_2:
				m.selectedSynth = 1
				m.currentView = ViewSynth
				m.previousSoundView = ViewSynth
			case sequencer.InstrumentKick:
				m.selectedDrum = 0
				m.currentView = ViewDrums
				m.previousSoundView = ViewDrums
			case sequencer.InstrumentSnare:
				m.selectedDrum = 1
				m.currentView = ViewDrums
				m.previousSoundView = ViewDrums
			case sequencer.InstrumentHihatClosed:
				m.selectedDrum = 2
				m.currentView = ViewDrums
				m.previousSoundView = ViewDrums
			case sequencer.InstrumentHihatOpen:
				m.selectedDrum = 3
				m.currentView = ViewDrums
				m.previousSoundView = ViewDrums
			case sequencer.InstrumentClap:
				m.selectedDrum = 4
				m.currentView = ViewDrums
				m.previousSoundView = ViewDrums
			case sequencer.InstrumentRimshot:
				m.selectedDrum = 5
				m.currentView = ViewDrums
				m.previousSoundView = ViewDrums
			case sequencer.InstrumentCrash:
				m.selectedDrum = 6
				m.currentView = ViewDrums
				m.previousSoundView = ViewDrums
			case sequencer.InstrumentTom:
				m.selectedDrum = 7
				m.currentView = ViewDrums
				m.previousSoundView = ViewDrums
			case sequencer.InstrumentSampler1:
				m.selectedSampler = 0
				m.currentView = ViewSampler
				m.previousSoundView = ViewSampler
			case sequencer.InstrumentSampler2:
				m.selectedSampler = 1
				m.currentView = ViewSampler
				m.previousSoundView = ViewSampler
			case sequencer.InstrumentDrone:
				m.currentView = ViewDrone
				m.previousSoundView = ViewDrone
			}

		case ViewSynth, ViewDrums, ViewSampler, ViewDrone:
			// Toggle to Effects with correct channel selected
			m.previousSoundView = m.currentView
			m.currentView = ViewEffects

			// Set the correct channel based on current instrument
			switch m.previousSoundView {
			case ViewSynth:
				// TB-303 #1 or #2
				m.selectedChannel = m.selectedSynth // 0 or 1
			case ViewDrums:
				// Kick, Snare, HH Closed, HH Open, Clap, Rimshot, Crash, Tom
				m.selectedChannel = 2 + m.selectedDrum // 2-9 (drums on channels 2-9)
			case ViewSampler:
				// Sampler #1 or #2
				m.selectedChannel = 10 + m.selectedSampler // 10 or 11
			case ViewDrone:
				// Drone
				m.selectedChannel = 12
			}
			m.selectedEffect = 0
			m.selectedEffectParam = 0

		case ViewEffects:
			// Return to previous sound view
			m.currentView = m.previousSoundView

		default:
			// Tab does nothing from Mixer, Song, or other views
			return m, nil
		}
		return m, nil

	case "+": // Increase BPM
		m.bpm += 1
		if m.bpm > 300 {
			m.bpm = 300
		}
		m.seq.SetBPM(m.bpm)
		return m, nil

	case "-": // Decrease BPM
		m.bpm -= 1
		if m.bpm < 40 {
			m.bpm = 40
		}
		m.seq.SetBPM(m.bpm)
		return m, nil
	}

	// View-specific controls
	switch m.currentView {
	case ViewSequencer:
		return m.handleSequencerKeys(msg)
	case ViewSynth:
		return m.handleSynthKeys(msg)
	case ViewDrums:
		return m.handleDrumKeys(msg)
	case ViewEffects:
		return m.handleEffectsKeys(msg)
	case ViewMixer:
		return m.handleMixerKeys(msg)
	case ViewSong:
		return m.handleSongKeys(msg)
	case ViewSampler:
		return m.handleSamplerKeys(msg)
	case ViewDrone:
		return m.handleDroneKeys(msg)
	case ViewArp:
		return m.handleArpKeys(msg)
	}

	return m, nil
}

// handleSequencerKeys handles keys specific to sequencer view
func (m Model) handleSequencerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.cursorY > 0 {
			m.cursorY--
		} else {
			m.cursorY = 12 // Wrap to bottom (13 tracks: 0-12)
		}
		m.selectedTrack = m.cursorY
		return m, nil

	case "down":
		if m.cursorY < 12 { // 13 tracks total
			m.cursorY++
		} else {
			m.cursorY = 0 // Wrap to top
		}
		m.selectedTrack = m.cursorY
		return m, nil

	case "left":
		// Get current pattern length
		instrument := trackInstruments[m.cursorY]
		track := m.seq.GetTrack(instrument)
		patternLength := 16
		if track != nil && track.Pattern != nil {
			patternLength = track.Pattern.Length
		}

		if m.cursorX > 0 {
			m.cursorX--
		} else {
			m.cursorX = patternLength - 1 // Wrap to end
		}

		// Auto-scroll if cursor moves outside visible window
		visibleSteps := m.calculateVisibleSteps()
		if m.cursorX < m.stepOffset {
			m.stepOffset = m.cursorX
		} else if m.cursorX >= m.stepOffset+visibleSteps {
			m.stepOffset = m.cursorX - (visibleSteps - 1)
		}

		m.selectedStep = m.cursorX
		return m, nil

	case "right":
		// Get current pattern length
		instrument := trackInstruments[m.cursorY]
		track := m.seq.GetTrack(instrument)
		patternLength := 16
		if track != nil && track.Pattern != nil {
			patternLength = track.Pattern.Length
		}

		if m.cursorX < patternLength-1 {
			m.cursorX++
		} else {
			m.cursorX = 0 // Wrap to start
			m.stepOffset = 0
		}

		// Auto-scroll if cursor moves outside visible window
		visibleSteps := m.calculateVisibleSteps()
		if m.cursorX < m.stepOffset {
			m.stepOffset = m.cursorX
		} else if m.cursorX >= m.stepOffset+visibleSteps {
			m.stepOffset = m.cursorX - (visibleSteps - 1)
		}

		m.selectedStep = m.cursorX
		return m, nil

	case " ": // Space = toggle step
		m.toggleStep()
		return m, nil

	case "A": // Toggle accent (Shift+a)
		m.toggleAccent()
		return m, nil

	case "L": // Toggle slide (Shift+l, only for TB-303 tracks)
		m.toggleSlide()
		return m, nil

	case "t": // Transpose down (only for TB-303 tracks)
		if m.selectedTrack == 0 || m.selectedTrack == 1 {
			instrument := trackInstruments[m.selectedTrack]
			track := m.seq.GetTrack(instrument)
			if track != nil {
				track.Transpose -= 12 // Down one octave
			}
		}
		return m, nil

	case "T": // Transpose up (Shift+t, only for TB-303 tracks)
		if m.selectedTrack == 0 || m.selectedTrack == 1 {
			instrument := trackInstruments[m.selectedTrack]
			track := m.seq.GetTrack(instrument)
			if track != nil {
				track.Transpose += 12 // Up one octave
			}
		}
		return m, nil

	case "n": // Toggle note entry mode
		m.noteEntryMode = !m.noteEntryMode
		return m, nil

	case "p": // Open piano roll (TB-303 tracks only)
		// Only allow on TB-303 tracks (track 0 or 1)
		if m.selectedTrack == 0 || m.selectedTrack == 1 {
			m.pianoRollMode = true
			// Initialize piano roll cursor to a sensible position
			m.pianoRollCursorX = 0
			m.pianoRollCursorY = 12 // Middle of 2 octave range (C note)
			m.pianoRollOctaveOffset = 1 // Start at C3-C5 range (MIDI 48-71)
		}
		return m, nil

	case "g": // Generate random acid line (dense)
		m.generateAcidLine(false) // false = not sparse
		return m, nil

	case "G": // Generate sparse acid line (Shift+G)
		m.generateAcidLine(true) // true = sparse
		return m, nil

	case "R": // Randomize sound (Shift+R)
		m.randomizeTrackSound()
		return m, nil

	case "D": // Shift+D - Open drum pattern menu
		m.drumPatternMenuOpen = true
		m.drumPatternSelectedIdx = 0
		return m, nil

	case "F": // Generate drum fill - Shift+F
		m.generateFillPattern()
		return m, nil

	case "B": // Generate buildup - Shift+B
		m.generateBuildupPattern()
		return m, nil

	case "x": // X - Open parameter selection for p-lock generator
		m.paramSelectMenuOpen = true
		m.paramSelectIndex = 0
		m.paramSelectForGenParam = ""
		return m, nil

	case "[": // Previous pattern
		currentPattern := m.seq.GetCurrentPatternID()
		if currentPattern > 0 {
			m.seq.SetCurrentPatternID(currentPattern - 1)
		} else {
			m.seq.SetCurrentPatternID(15) // Wrap to last pattern
		}
		m.loadAllSoundsFromPattern()  // Load sound settings for ALL tracks
		m.loadMuteSoloForAllTracks()  // Load mute/solo for all tracks
		return m, nil

	case "]": // Next pattern
		currentPattern := m.seq.GetCurrentPatternID()
		if currentPattern < 15 {
			m.seq.SetCurrentPatternID(currentPattern + 1)
		} else {
			m.seq.SetCurrentPatternID(0) // Wrap to first pattern
		}
		m.loadAllSoundsFromPattern()  // Load sound settings for ALL tracks
		m.loadMuteSoloForAllTracks()  // Load mute/solo for all tracks
		return m, nil

	case "{": // Decrease pattern length (Shift+[)
		m.changePatternLength(-1)
		return m, nil

	case "}": // Increase pattern length (Shift+])
		m.changePatternLength(1)
		return m, nil

	case "C": // Copy current pattern to next slot (Shift+C)
		currentPattern := m.seq.GetCurrentPatternID()
		targetPattern := (currentPattern + 1) % 16
		m.seq.CopyCurrentPattern(targetPattern)
		return m, nil

	case "P": // Save current sound to pattern (Shift+P)
		m.saveCurrentSoundToPattern()   // Save sound settings for selected track
		m.saveAllMuteSoloToPatterns()   // Save mute/solo for all tracks
		return m, nil

	case "backspace": // Clear all steps on current track
		m.clearCurrentPattern()
		return m, nil

	case "m": // Mute selected track
		m.toggleMuteSelectedTrack()
		return m, nil

	case "o": // Solo selected track (o for sOlo)
		m.toggleSoloSelectedTrack()
		return m, nil

	case "<", "shift+,": // Decrease swing
		m.swing -= 0.05
		if m.swing < 0.0 {
			m.swing = 0.0
		}
		m.seq.SetSwing(m.swing)
		return m, nil

	case ">", "shift+.": // Increase swing
		m.swing += 0.05
		if m.swing > 1.0 {
			m.swing = 1.0
		}
		m.seq.SetSwing(m.swing)
		return m, nil

	case ",": // Nudge pattern left
		instrument := trackInstruments[m.selectedTrack]
		track := m.seq.GetTrack(instrument)
		if track != nil && track.Pattern != nil {
			track.Pattern.RotateLeft()
			m.statusMessage = "Pattern nudged left"
			m.statusDuration = 30
		}
		return m, nil

	case ".": // Nudge pattern right
		instrument := trackInstruments[m.selectedTrack]
		track := m.seq.GetTrack(instrument)
		if track != nil && track.Pattern != nil {
			track.Pattern.RotateRight()
			m.statusMessage = "Pattern nudged right"
			m.statusDuration = 30
		}
		return m, nil

	// Octave controls (when in note entry mode)
	case "q":
		if m.noteEntryMode && m.currentOctave > 2 {
			m.currentOctave--
			return m, nil
		}
		// Fall through to view-specific handlers if not in note entry mode

	case "w":
		if m.noteEntryMode && m.currentOctave < 6 {
			m.currentOctave++
			return m, nil
		}
		// Fall through to view-specific handlers if not in note entry mode

	case "c": // Edit step condition
		return m.openConditionEditor(), nil

	case "e": // Euclidean rhythm generator (takes priority over effects view switch)
		return m.openEuclideanGenerator(), nil

	case "N": // Pattern length editor (Shift+n for Number of steps)
		return m.openLengthEditor(), nil

	case "pageup": // Scroll left one page
		if m.stepOffset > 0 {
			m.stepOffset -= 16 // Scroll by 16 steps (one page)
			if m.stepOffset < 0 {
				m.stepOffset = 0
			}
		}
		return m, nil

	case "pagedown": // Scroll right one page
		instrument := trackInstruments[m.selectedTrack]
		track := m.seq.GetTrack(instrument)
		if track != nil && track.Pattern != nil {
			maxOffset := track.Pattern.Length - 16
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.stepOffset < maxOffset {
				m.stepOffset += 16 // Scroll by 16 steps (one page)
				if m.stepOffset > maxOffset {
					m.stepOffset = maxOffset
				}
			}
		}
		return m, nil
	}

	// Piano keyboard note entry (for melodic tracks: TB-303s and Drone/MegaSynth)
	if m.noteEntryMode && (m.selectedTrack == 0 || m.selectedTrack == 1 || m.selectedTrack == 12) {
		note := m.getPianoKeyNote(msg.String())
		if note >= 0 {
			m.setStepNote(note)

			// Auto-advance to next step
			instrument := trackInstruments[m.selectedTrack]
			track := m.seq.GetTrack(instrument)
			patternLength := 16
			if track != nil && track.Pattern != nil {
				patternLength = track.Pattern.Length
			}

			if m.cursorX < patternLength-1 {
				m.cursorX++
				// Auto-scroll if cursor moves outside visible window
				visibleSteps := m.calculateVisibleSteps()
				if m.cursorX >= m.stepOffset+visibleSteps {
					m.stepOffset = m.cursorX - (visibleSteps - 1)
				}
				m.selectedStep = m.cursorX
			}
			return m, nil
		}
	}

	// View switching keys (only work in sequencer)
	switch msg.String() {
	case "M": // Shift+M to mixer view
		m.currentView = ViewMixer
		return m, nil
	case "s":
		m.currentView = ViewSong
		return m, nil
	case "e":
		m.currentView = ViewEffects
		return m, nil
	case "a":
		m.currentView = ViewSampler
		return m, nil
	case "d":
		m.currentView = ViewDrone
		return m, nil
	}

	return m, nil
}

// handlePianoRollKeys handles keyboard input in piano roll mode
func (m Model) handlePianoRollKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Get current track and pattern
	var instrument sequencer.InstrumentType
	if m.selectedTrack == 0 {
		instrument = sequencer.InstrumentTB303_1
	} else {
		instrument = sequencer.InstrumentTB303_2
	}
	track := m.seq.GetTrack(instrument)
	pattern := track.Pattern

	switch msg.String() {
	case "esc":
		// Exit piano roll mode
		m.pianoRollMode = false
		return m, nil

	case "up":
		// Move cursor up (higher pitch)
		if m.pianoRollCursorY < 23 {
			m.pianoRollCursorY++
		}
		return m, nil

	case "down":
		// Move cursor down (lower pitch)
		if m.pianoRollCursorY > 0 {
			m.pianoRollCursorY--
		}
		return m, nil

	case "left":
		// Move cursor left
		if m.pianoRollCursorX > 0 {
			m.pianoRollCursorX--
		} else {
			m.pianoRollCursorX = pattern.Length - 1
		}
		return m, nil

	case "right":
		// Move cursor right
		if m.pianoRollCursorX < pattern.Length-1 {
			m.pianoRollCursorX++
		} else {
			m.pianoRollCursorX = 0
		}
		return m, nil

	case "pgup":
		// Scroll octave up
		if m.pianoRollOctaveOffset < 7 { // Max offset to keep in MIDI range
			m.pianoRollOctaveOffset++
		}
		return m, nil

	case "pgdown":
		// Scroll octave down
		if m.pianoRollOctaveOffset > 0 {
			m.pianoRollOctaveOffset--
		}
		return m, nil

	case "enter", " ":
		// Toggle note at cursor position
		step := pattern.GetStep(m.pianoRollCursorX)
		if step != nil {
			// Calculate MIDI note from cursor position
			midiNote := ((m.pianoRollOctaveOffset + 2) * 12) + m.pianoRollCursorY

			// Check if this note is already active
			if step.Active && step.Note == midiNote {
				// Clear the note
				step.Clear()
			} else {
				// Set the note
				step.Active = true
				step.Note = midiNote
				if step.Velocity == 0 {
					step.Velocity = 0.8 // Default velocity
				}
			}
		}
		return m, nil

	case "+", "=":
		// Increase velocity
		step := pattern.GetStep(m.pianoRollCursorX)
		if step != nil && step.Active {
			midiNote := ((m.pianoRollOctaveOffset + 2) * 12) + m.pianoRollCursorY
			if step.Note == midiNote {
				step.Velocity += 0.1
				if step.Velocity > 1.0 {
					step.Velocity = 1.0
				}
			}
		}
		return m, nil

	case "-", "_":
		// Decrease velocity
		step := pattern.GetStep(m.pianoRollCursorX)
		if step != nil && step.Active {
			midiNote := ((m.pianoRollOctaveOffset + 2) * 12) + m.pianoRollCursorY
			if step.Note == midiNote {
				step.Velocity -= 0.1
				if step.Velocity < 0.1 {
					step.Velocity = 0.1
				}
			}
		}
		return m, nil

	case "a":
		// Toggle accent
		step := pattern.GetStep(m.pianoRollCursorX)
		if step != nil && step.Active {
			midiNote := ((m.pianoRollOctaveOffset + 2) * 12) + m.pianoRollCursorY
			if step.Note == midiNote {
				step.Accent = !step.Accent
			}
		}
		return m, nil

	case "s":
		// Toggle slide
		step := pattern.GetStep(m.pianoRollCursorX)
		if step != nil && step.Active {
			midiNote := ((m.pianoRollOctaveOffset + 2) * 12) + m.pianoRollCursorY
			if step.Note == midiNote {
				step.Slide = !step.Slide
			}
		}
		return m, nil

	case "delete", "backspace":
		// Clear note at cursor
		step := pattern.GetStep(m.pianoRollCursorX)
		if step != nil {
			midiNote := ((m.pianoRollOctaveOffset + 2) * 12) + m.pianoRollCursorY
			if step.Active && step.Note == midiNote {
				step.Clear()
			}
		}
		return m, nil
	}

	return m, nil
}

// handleSynthKeys handles keys specific to synth view
func (m Model) handleSynthKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left":
		// Switch to TB-303 #1
		m.selectedSynth = 0
		return m, nil

	case "right":
		// Switch to TB-303 #2
		m.selectedSynth = 1
		return m, nil

	case "up":
		// Navigate parameters up
		if m.selectedParam > 0 {
			m.selectedParam--
			// Skip separator lines
			if m.selectedParam == 19 || m.selectedParam == 9 {
				m.selectedParam--
			}
		}
		return m, nil

	case "down":
		// Navigate parameters down
		if m.selectedParam < 28 { // 29 parameters (0-28, with separators at 9 and 19)
			m.selectedParam++
			// Skip separator lines
			if m.selectedParam == 9 || m.selectedParam == 19 {
				m.selectedParam++
			}
		}
		return m, nil

	case ",", "<":
		// Decrease parameter value
		m.adjustSynthParam(-0.05)
		return m, nil

	case ".", ">":
		// Increase parameter value
		m.adjustSynthParam(0.05)
		return m, nil

	case "r": // Randomize synth sound
		m.randomizeSynthSound()
		return m, nil

	case "l": // Lock current parameter to selected step
		m.lockCurrentParameter()
		return m, nil

	case "P": // Shift+P - Open p-lock generator menu
		m.plockGenMenuOpen = true
		m.plockGenSelectedIndex = 0
		return m, nil

	case "enter":
		// Open manual value entry for current parameter
		m.manualInputMode = true
		m.manualInputValue = ""
		return m, nil
	}

	return m, nil
}

// handleDroneKeys handles keys specific to drone view
func (m Model) handleDroneKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left":
		// Jump to left column (Sound Generation)
		if m.selectedDroneParam >= 26 { // Currently in right column
			m.selectedDroneParam = 0 // Jump to first param in left column
		}
		return m, nil

	case "right":
		// Jump to right column (Filter & Modulation)
		if m.selectedDroneParam <= 24 { // Currently in left column
			m.selectedDroneParam = 26 // Jump to first param in right column (skip separator at 25)
		}
		return m, nil

	case "up":
		// Navigate parameters up
		if m.selectedDroneParam > 0 {
			m.selectedDroneParam--
			// Skip separator lines at indices 2, 14, 18, 22, 25, 33, 42
			for m.selectedDroneParam == 2 || m.selectedDroneParam == 14 || m.selectedDroneParam == 18 ||
			    m.selectedDroneParam == 22 || m.selectedDroneParam == 25 || m.selectedDroneParam == 33 ||
			    m.selectedDroneParam == 42 {
				if m.selectedDroneParam > 0 {
					m.selectedDroneParam--
				} else {
					break
				}
			}
		}
		return m, nil

	case "down":
		// Navigate parameters down
		if m.selectedDroneParam < 50 { // 51 parameters (0-50)
			m.selectedDroneParam++
			// Skip separator lines at indices 2, 14, 18, 22, 25, 33, 42
			for m.selectedDroneParam == 2 || m.selectedDroneParam == 14 || m.selectedDroneParam == 18 ||
			    m.selectedDroneParam == 22 || m.selectedDroneParam == 25 || m.selectedDroneParam == 33 ||
			    m.selectedDroneParam == 42 {
				if m.selectedDroneParam < 50 {
					m.selectedDroneParam++
				} else {
					break
				}
			}
		}
		return m, nil

	case ",", "<":
		// Decrease parameter value
		m.adjustDroneParam(-0.05)
		return m, nil

	case ".", ">":
		// Increase parameter value
		m.adjustDroneParam(0.05)
		return m, nil

	case "r": // Randomize drone sound
		m.randomizeDroneSound()
		return m, nil

	case "l": // Lock current parameter to selected step
		m.lockCurrentParameter()
		return m, nil

	case "P": // Shift+P - Open p-lock generator menu
		m.plockGenMenuOpen = true
		m.plockGenSelectedIndex = 0
		return m, nil

	case "a", "A": // Open arpeggiator view
		m.currentView = ViewArp
		m.selectedArpParam = 0
		return m, nil
	}

	return m, nil
}

// handleArpKeys handles keys specific to arpeggiator view
func (m Model) handleArpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	drone := m.engine.GetDrone()
	arp := drone.GetArpeggiator()

	// Chord edit mode handling
	if m.arpChordEditMode {
		switch msg.String() {
		case "enter":
			// Save chord buffer to arpeggiator
			arp.SetInputNotes(m.arpChordBuffer)
			m.arpChordEditMode = false
			return m, nil

		case "esc":
			// Cancel editing
			m.arpChordEditMode = false
			m.arpChordBuffer = nil
			return m, nil

		case "backspace":
			// Remove last note from buffer
			if len(m.arpChordBuffer) > 0 {
				m.arpChordBuffer = m.arpChordBuffer[:len(m.arpChordBuffer)-1]
			}
			return m, nil

		default:
			// Try to add piano key note to buffer
			note := m.getPianoKeyNote(msg.String())
			if note >= 0 {
				// Add note to buffer if not already present
				noteExists := false
				for _, n := range m.arpChordBuffer {
					if n == note {
						noteExists = true
						break
					}
				}
				if !noteExists {
					m.arpChordBuffer = append(m.arpChordBuffer, note)
				}
			}
			return m, nil
		}
	}

	// Normal parameter editing mode
	switch msg.String() {
	case "e":
		// Enter chord edit mode
		m.arpChordEditMode = true
		// Initialize buffer with current chord or empty
		currentNotes := arp.GetInputNotes()
		m.arpChordBuffer = make([]int, len(currentNotes))
		copy(m.arpChordBuffer, currentNotes)
		return m, nil

	case "up":
		// Navigate parameters up
		if m.selectedArpParam > 0 {
			m.selectedArpParam--
		}
		return m, nil

	case "down":
		// Navigate parameters down
		if m.selectedArpParam < 6 { // 7 parameters (0-6)
			m.selectedArpParam++
		}
		return m, nil

	case ",", "<":
		// Decrease parameter value
		m.adjustArpParam(-1)
		return m, nil

	case ".", ">":
		// Increase parameter value
		m.adjustArpParam(1)
		return m, nil

	case " ": // Space - Toggle enabled
		if m.selectedArpParam == 0 { // Enabled parameter
			arp.SetEnabled(!arp.GetEnabled())
		} else if m.selectedArpParam == 6 { // Latch mode parameter
			arp.SetLatchMode(!arp.GetLatchMode())
		}
		return m, nil
	}

	return m, nil
}

// handleDrumKeys handles keys specific to drum view
func (m Model) handleDrumKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1":
		m.selectedDrum = 0 // Kick
		m.selectedDrumParam = 0
		return m, nil

	case "2":
		m.selectedDrum = 1 // Snare
		m.selectedDrumParam = 0
		return m, nil

	case "3":
		m.selectedDrum = 2 // HH Closed
		m.selectedDrumParam = 0
		return m, nil

	case "4":
		m.selectedDrum = 3 // HH Open
		m.selectedDrumParam = 0
		return m, nil

	case "5":
		m.selectedDrum = 4 // Clap
		m.selectedDrumParam = 0
		return m, nil

	case "6":
		m.selectedDrum = 5 // Rimshot
		m.selectedDrumParam = 0
		return m, nil

	case "7":
		m.selectedDrum = 6 // Crash
		m.selectedDrumParam = 0
		return m, nil

	case "8":
		m.selectedDrum = 7 // Tom
		m.selectedDrumParam = 0
		return m, nil

	case "up":
		// Navigate parameters up
		if m.selectedDrumParam > 0 {
			m.selectedDrumParam--
		}
		return m, nil

	case "down":
		// Navigate parameters down
		maxParam := m.getMaxDrumParam()
		if m.selectedDrumParam < maxParam {
			m.selectedDrumParam++
		}
		return m, nil

	case ",", "<":
		// Decrease parameter value
		m.adjustDrumParam(-0.05)
		return m, nil

	case ".", ">":
		// Increase parameter value
		m.adjustDrumParam(0.05)
		return m, nil

	case "r": // Randomize drum sound
		m.randomizeDrumSound()
		return m, nil

	case "l": // Lock current parameter to selected step
		m.lockCurrentParameter()
		return m, nil

	case "P": // Shift+P - Open p-lock generator menu
		m.plockGenMenuOpen = true
		m.plockGenSelectedIndex = 0
		return m, nil

	case "enter":
		// Open manual value entry for current parameter
		m.manualInputMode = true
		m.manualInputValue = ""
		return m, nil
	}

	return m, nil
}

// handleEffectsKeys handles keys specific to effects view
func (m Model) handleEffectsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1":
		m.selectedChannel = 0 // TB-303 #1
		m.selectedEffect = 0
		m.selectedEffectParam = 0
		return m, nil

	case "2":
		m.selectedChannel = 1 // TB-303 #2
		m.selectedEffect = 0
		m.selectedEffectParam = 0
		return m, nil

	case "3":
		m.selectedChannel = 2 // Kick
		m.selectedEffect = 0
		m.selectedEffectParam = 0
		return m, nil

	case "4":
		m.selectedChannel = 3 // Snare
		m.selectedEffect = 0
		m.selectedEffectParam = 0
		return m, nil

	case "5":
		m.selectedChannel = 4 // HH Closed
		m.selectedEffect = 0
		m.selectedEffectParam = 0
		return m, nil

	case "6":
		m.selectedChannel = 5 // HH Open
		m.selectedEffect = 0
		m.selectedEffectParam = 0
		return m, nil

	case "7":
		m.selectedChannel = 6 // Clap
		m.selectedEffect = 0
		m.selectedEffectParam = 0
		return m, nil

	case "8":
		m.selectedChannel = 7 // Rimshot
		m.selectedEffect = 0
		m.selectedEffectParam = 0
		return m, nil

	case "9":
		m.selectedChannel = 8 // Crash
		m.selectedEffect = 0
		m.selectedEffectParam = 0
		return m, nil

	case "0":
		m.selectedChannel = 9 // Tom
		m.selectedEffect = 0
		m.selectedEffectParam = 0
		return m, nil

	// Note: Channels 10-12 (Sampler #1, Sampler #2, Drone) are accessed via Tab from sequencer

	case "left":
		// Navigate effects left (they're laid out horizontally)
		if m.selectedEffect > 0 {
			m.selectedEffect--
			m.selectedEffectParam = 0
		}
		return m, nil

	case "right":
		// Navigate effects right (they're laid out horizontally)
		if m.selectedEffect < 5 { // 6 effects (0-5: Dist, Delay, Reverb, Pitch, Filter, Crusher)
			m.selectedEffect++
			m.selectedEffectParam = 0
		}
		return m, nil

	case "up":
		// Navigate parameters up (within effect)
		if m.selectedEffectParam > 0 {
			m.selectedEffectParam--
		}
		return m, nil

	case "down":
		// Navigate parameters down (within effect)
		maxParam := m.getMaxEffectParam()
		if m.selectedEffectParam < maxParam {
			m.selectedEffectParam++
		}
		return m, nil

	case ",", "<":
		// Decrease parameter value
		m.adjustEffectParam(-0.05)
		return m, nil

	case ".", ">":
		// Increase parameter value
		m.adjustEffectParam(0.05)
		return m, nil
	}

	return m, nil
}

// getMaxEffectParam returns the max parameter index for the selected effect
func (m Model) getMaxEffectParam() int {
	switch m.selectedEffect {
	case 0: // Distortion - 4 params (Enable, Drive, Tone, Mix)
		return 3
	case 1: // Delay - 4 params (Enable, Time, Feedback, Mix)
		return 3
	case 2: // Reverb - 4 params (Enable, Room Size, Damping, Mix)
		return 3
	default:
		return 3
	}
}

// handleMixerKeys handles keys specific to mixer view
func (m Model) handleMixerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1":
		m.selectedMixerChannel = 0 // TB-303 #1
		return m, nil

	case "2":
		m.selectedMixerChannel = 1 // TB-303 #2
		return m, nil

	case "3":
		m.selectedMixerChannel = 2 // Kick
		return m, nil

	case "4":
		m.selectedMixerChannel = 3 // Snare
		return m, nil

	case "5":
		m.selectedMixerChannel = 4 // HH Closed
		return m, nil

	case "6":
		m.selectedMixerChannel = 5 // HH Open
		return m, nil

	case "up":
		// Navigate parameters up
		if m.selectedMixerParam > 0 {
			m.selectedMixerParam--
		}
		return m, nil

	case "down":
		// Navigate parameters down
		if m.selectedMixerParam < 3 { // 4 params (0-3: Level, Pan, Mute, Solo)
			m.selectedMixerParam++
		}
		return m, nil

	case ",", "<":
		// Decrease parameter value
		m.adjustMixerParam(-0.05)
		return m, nil

	case ".", ">":
		// Increase parameter value
		m.adjustMixerParam(0.05)
		return m, nil

	case "m":
		// Toggle mute (quantized to pattern end)
		m.queueMuteToggle(false)
		return m, nil

	case "M":
		// Toggle mute (quantized to 4-bar phrase with Shift)
		m.queueMuteToggle(true)
		return m, nil

	case "s":
		// Toggle solo (quantized to pattern end)
		m.queueSoloToggle(false)
		return m, nil

	case "S":
		// Toggle solo (quantized to 4-bar phrase with Shift)
		m.queueSoloToggle(true)
		return m, nil
	}

	return m, nil
}

// getMaxDrumParam returns the max parameter index for the selected drum
func (m Model) getMaxDrumParam() int {
	switch m.selectedDrum {
	case 0: // Kick - 6 base params + LFO1 controls with Min/Max (0-15)
		return 15
	case 1: // Snare - 6 base params + LFO1 controls with Min/Max (0-15)
		return 15
	case 2, 3: // Hi-hats - 4 base params + LFO1 controls with Min/Max (0-13)
		return 13
	case 4: // Clap - 5 base params + LFO1 + LFO2 controls with Min/Max (0-24)
		return 24
	case 5: // Rimshot - 5 base params + LFO1 + LFO2 controls with Min/Max (0-24)
		return 24
	case 6: // Crash - 4 base params + LFO1 + LFO2 controls with Min/Max (0-23)
		return 23
	case 7: // Tom - 7 base params + LFO1 + LFO2 controls with Min/Max (0-26)
		return 26
	default:
		return 0
	}
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Piano roll mode overrides normal view rendering
	if m.pianoRollMode {
		return m.renderPianoRoll()
	}

	var content string

	switch m.currentView {
	case ViewSequencer:
		content = m.renderSequencerView()
	case ViewSynth:
		content = m.renderSynthView()
	case ViewDrums:
		content = m.renderDrumsView()
	case ViewEffects:
		content = m.renderEffectsView()
	case ViewMixer:
		content = m.renderMixerView()
	case ViewSong:
		content = m.renderSongView()
	case ViewSampler:
		content = m.renderSamplerView()
	case ViewDrone:
		content = m.renderDroneView()
	case ViewArp:
		content = m.renderArpView()
	}

	// Build complete view
	header := m.renderHeader()
	footer := m.renderFooter()

	// Override content with help screen if active
	if m.showHelp {
		content = m.renderHelp()
	}

	// Override content with condition editor if active
	if m.conditionEditMode {
		content = m.renderConditionEditor()
	}

	// Override content with Euclidean generator if active
	if m.euclideanMode {
		content = m.renderEuclideanGenerator()
	}

	// Override content with pattern length editor if active
	if m.lengthEditMode {
		content = m.renderLengthEditor()
	}

	// Override content with parameter selection menu if active
	if m.paramSelectMenuOpen {
		content = m.renderParamSelectMenu()
	}

	// Override content with p-lock generator menu if active
	if m.plockGenMenuOpen {
		content = m.renderPLockGenMenu()
	}

	// Override content with drum pattern menu if active
	if m.drumPatternMenuOpen {
		content = m.renderDrumPatternMenu()
	}

	// Override content with manual input box if active
	if m.manualInputMode {
		content = m.renderManualInputBox()
	}

	// Add file browser if active (not shown if help is active)
	fileBrowser := ""
	if m.saveLoadMode > 0 && !m.showHelp {
		fileBrowser = m.renderFileBrowser()
	}

	// Add status message if active
	statusMsg := m.renderStatusMessage()

	parts := []string{
		header,
		"",
		content,
		fileBrowser,
		statusMsg,
		"",
		footer,
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderHeader renders the header
func (m Model) renderHeader() string {
	// Title with project name
	titleText := fmt.Sprintf("♪ GRUVBOX v%s", m.version)
	if m.currentProjectName != "" {
		titleText += " - " + m.currentProjectName
	} else {
		titleText += " - [Unsaved]"
	}
	title := HeaderStyle.Render(titleText)

	status := ""
	if m.isPlaying {
		status = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			Render("▶ PLAYING")
	} else {
		status = lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("⏸ STOPPED")
	}

	bpm := lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true).
		Render(fmt.Sprintf("♩ %d BPM", int(m.bpm)))

	step := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Render(fmt.Sprintf("Step %d/16", m.seq.GetCurrentStep()+1))

	// Add unsaved sound indicator if dirty
	parts := []string{status, "  ", bpm}

	// Add swing display when active
	if m.swing > 0.0 {
		swing := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Render(fmt.Sprintf("⇲ %d%% SWING", int(m.swing*100)))
		parts = append(parts, "  ", swing)
	}

	parts = append(parts, "  ", step)
	if m.soundParamsDirty {
		unsaved := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Render("⚠ UNSAVED")
		parts = append(parts, "  ", unsaved)
	}

	statusLine := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		StatusStyle.Width(m.width).Render(statusLine),
	)
}

// renderFooter renders the footer with help
func (m Model) renderFooter() string {
	var help string

	switch m.currentView {
	case ViewSequencer:
		if m.noteEntryMode {
			// Note entry mode help
			octaveInfo := lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Render(fmt.Sprintf("(Octave %d)", m.currentOctave))
			help = lipgloss.JoinHorizontal(
				lipgloss.Left,
				KeyStyle.Render("[N]"),
				HelpStyle.Render(" Exit Note Mode  "),
				KeyStyle.Render("[Z-M]"),
				HelpStyle.Render(" Play Notes  "),
				KeyStyle.Render("[Q/W]"),
				HelpStyle.Render(" Oct -/+  "),
				octaveInfo,
				"  ",
				KeyStyle.Render("[G]"),
				HelpStyle.Render(" Generate  "),
			)
		} else {
			// Normal sequencer help - four rows for all commands
			row1 := lipgloss.JoinHorizontal(
				lipgloss.Left,
				KeyStyle.Render("[`]"),
				HelpStyle.Render(" Play/Stop  "),
				KeyStyle.Render("[R]"),
				HelpStyle.Render(" Reset  "),
				KeyStyle.Render("[+/-]"),
				HelpStyle.Render(" BPM  "),
				KeyStyle.Render("[</>]"),
				HelpStyle.Render(" Swing  "),
				KeyStyle.Render("[Space]"),
				HelpStyle.Render(" Toggle  "),
				KeyStyle.Render("[X]"),
				HelpStyle.Render(" Clear  "),
				KeyStyle.Render("[V]"),
				HelpStyle.Render(" Velocity  "),
				KeyStyle.Render("[A/S]"),
				HelpStyle.Render(" Accent/Slide  "),
				KeyStyle.Render("[M/O]"),
				HelpStyle.Render(" Mute/Solo  "),
			)
			row2 := lipgloss.JoinHorizontal(
				lipgloss.Left,
				KeyStyle.Render("[[ / ]]"),
				HelpStyle.Render(" Pattern  "),
				KeyStyle.Render("[{ / }]"),
				HelpStyle.Render(" Length  "),
				KeyStyle.Render("[Shift+N]"),
				HelpStyle.Render(" Set Length  "),
				KeyStyle.Render("[PgUp/Dn]"),
				HelpStyle.Render(" Scroll  "),
				KeyStyle.Render("[, / .]"),
				HelpStyle.Render(" Nudge  "),
				KeyStyle.Render("[Shift+C]"),
				HelpStyle.Render(" Copy  "),
			)
			row3 := lipgloss.JoinHorizontal(
				lipgloss.Left,
				KeyStyle.Render("[N]"),
				HelpStyle.Render(" Note Entry  "),
				KeyStyle.Render("[C]"),
				HelpStyle.Render(" Condition  "),
				KeyStyle.Render("[E]"),
				HelpStyle.Render(" Euclidean  "),
				KeyStyle.Render("[G]"),
				HelpStyle.Render(" Generate  "),
				KeyStyle.Render("[Shift+B]"),
				HelpStyle.Render(" Buildup  "),
				KeyStyle.Render("[x]"),
				HelpStyle.Render(" P-Lock Gen  "),
				KeyStyle.Render("[Shift+D]"),
				HelpStyle.Render(" Drum Patterns  "),
			)
			row4 := lipgloss.JoinHorizontal(
				lipgloss.Left,
				KeyStyle.Render("[Shift+P]"),
				HelpStyle.Render(" Save Sound  "),
				KeyStyle.Render("[Ctrl+S]"),
				HelpStyle.Render(" Save Project  "),
				KeyStyle.Render("[Ctrl+O]"),
				HelpStyle.Render(" Load Project  "),
				KeyStyle.Render("[1-8/m/s/e/a/d]"),
				HelpStyle.Render(" Switch View  "),
				KeyStyle.Render("[?]"),
				HelpStyle.Render(" Help  "),
			)
			help = lipgloss.JoinVertical(
				lipgloss.Left,
				row1,
				row2,
				row3,
				row4,
			)
		}
	case ViewSynth:
		row1 := lipgloss.JoinHorizontal(
			lipgloss.Left,
			KeyStyle.Render("[Left/Right]"),
			HelpStyle.Render(" Switch TB-303  "),
			KeyStyle.Render("[Up/Down]"),
			HelpStyle.Render(" Navigate  "),
			KeyStyle.Render("[</>]"),
			HelpStyle.Render(" Adjust  "),
			KeyStyle.Render("[W]"),
			HelpStyle.Render(" Waveform  "),
			KeyStyle.Render("[R]"),
			HelpStyle.Render(" Randomize  "),
		)
		row2 := lipgloss.JoinHorizontal(
			lipgloss.Left,
			KeyStyle.Render("[L]"),
			HelpStyle.Render(" Lock to Step  "),
			KeyStyle.Render("[Shift+L]"),
			HelpStyle.Render(" Lock to Pattern  "),
			KeyStyle.Render("[Shift+P]"),
			HelpStyle.Render(" P-Lock Gen  "),
			KeyStyle.Render("[1-8/s/m/e]"),
			HelpStyle.Render(" Views  "),
			KeyStyle.Render("[Esc]"),
			HelpStyle.Render(" Sequencer  "),
			KeyStyle.Render("[?]"),
			HelpStyle.Render(" Help  "),
		)
		help = lipgloss.JoinVertical(lipgloss.Left, row1, row2)
	case ViewDrums:
		row1 := lipgloss.JoinHorizontal(
			lipgloss.Left,
			KeyStyle.Render("[1-4]"),
			HelpStyle.Render(" Select Drum  "),
			KeyStyle.Render("[Up/Down]"),
			HelpStyle.Render(" Navigate  "),
			KeyStyle.Render("[</>]"),
			HelpStyle.Render(" Adjust  "),
			KeyStyle.Render("[R]"),
			HelpStyle.Render(" Randomize  "),
		)
		row2 := lipgloss.JoinHorizontal(
			lipgloss.Left,
			KeyStyle.Render("[L]"),
			HelpStyle.Render(" Lock to Step  "),
			KeyStyle.Render("[Shift+L]"),
			HelpStyle.Render(" Lock to Pattern  "),
			KeyStyle.Render("[Shift+P]"),
			HelpStyle.Render(" P-Lock Gen  "),
			KeyStyle.Render("[1-8/s/m/e]"),
			HelpStyle.Render(" Views  "),
			KeyStyle.Render("[Esc]"),
			HelpStyle.Render(" Sequencer  "),
			KeyStyle.Render("[?]"),
			HelpStyle.Render(" Help  "),
		)
		help = lipgloss.JoinVertical(lipgloss.Left, row1, row2)
	case ViewSong:
		help = lipgloss.JoinHorizontal(
			lipgloss.Left,
			KeyStyle.Render("[↑/↓]"),
			HelpStyle.Render(" Navigate  "),
			KeyStyle.Render("[Shift+A]"),
			HelpStyle.Render(" Add  "),
			KeyStyle.Render("[D]"),
			HelpStyle.Render(" Delete  "),
			KeyStyle.Render("[Shift+S]"),
			HelpStyle.Render(" Song Mode  "),
		)
	case ViewDrone:
		row1 := lipgloss.JoinHorizontal(
			lipgloss.Left,
			KeyStyle.Render("[Up/Down]"),
			HelpStyle.Render(" Navigate  "),
			KeyStyle.Render("[</>]"),
			HelpStyle.Render(" Adjust  "),
			KeyStyle.Render("[R]"),
			HelpStyle.Render(" Randomize  "),
		)
		row2 := lipgloss.JoinHorizontal(
			lipgloss.Left,
			KeyStyle.Render("[L]"),
			HelpStyle.Render(" Lock to Step  "),
			KeyStyle.Render("[Shift+L]"),
			HelpStyle.Render(" Lock to Pattern  "),
			KeyStyle.Render("[Shift+P]"),
			HelpStyle.Render(" P-Lock Gen  "),
			KeyStyle.Render("[1-8/s/m/e]"),
			HelpStyle.Render(" Views  "),
			KeyStyle.Render("[Esc]"),
			HelpStyle.Render(" Sequencer  "),
			KeyStyle.Render("[?]"),
			HelpStyle.Render(" Help  "),
		)
		help = lipgloss.JoinVertical(lipgloss.Left, row1, row2)
	default:
		help = HelpStyle.Render("[Tab] Smart nav  [m/s/e/a/d] Views  [Esc] Sequencer  [Ctrl+S] Save  [Ctrl+L] Load  [Ctrl+Q] Quit")
	}

	return StatusStyle.Width(m.width).Render(help)
}

// renderSynthView renders the TB-303 parameter controls
func (m Model) renderSynthView() string {
	tb303_1 := m.engine.GetTB303()
	tb303_2 := m.engine.GetTB303_2()

	// Parameter names
	params := []string{
		"Waveform",
		"Cutoff",
		"Resonance",
		"Env Mod",
		"Decay",
		"Accent",
		"Slide Time",
		"Sub Level",
		"Volume",
		"", // Separator
		"LFO1 Enable",
		"LFO1 Target",
		"LFO1 Wave",
		"LFO1 Rate",
		"LFO1 Depth",
		"LFO1 Min",
		"LFO1 Max",
		"LFO1 Sync",
		"LFO1 Mode",
		"", // Separator
		"LFO2 Enable",
		"LFO2 Target",
		"LFO2 Wave",
		"LFO2 Rate",
		"LFO2 Depth",
		"LFO2 Min",
		"LFO2 Max",
		"LFO2 Sync",
		"LFO2 Mode",
	}

	// Get current values for each TB-303
	mixer := m.engine.GetMixer()
	lfo1_1 := tb303_1.GetLFO1() // TB-303 #1's LFO1
	lfo1_2 := tb303_2.GetLFO1() // TB-303 #2's LFO1
	lfo2_1 := tb303_1.GetLFO2() // TB-303 #1's LFO2
	lfo2_2 := tb303_2.GetLFO2() // TB-303 #2's LFO2

	values1 := []string{
		m.getWaveformName(tb303_1),
		m.formatPercent(tb303_1.GetCurrentCutoff()),
		m.formatPercent(tb303_1.GetCurrentResonance()),
		m.formatPercent(tb303_1.GetCurrentEnvMod()),
		m.formatPercent(tb303_1.GetDecay()),
		m.formatPercent(tb303_1.GetAccent()),
		m.formatPercent(tb303_1.GetSlideTime()),
		m.formatPercent(tb303_1.GetSubLevel()),
		m.formatPercent(mixer.GetChannel(0).Level),
		"", // Separator
		m.formatBool(tb303_1.GetLFO1Enabled()),
		m.getLFOTargetName(tb303_1.GetLFO1Target()),
		m.getLFOWaveformName(lfo1_1.GetWaveform()),
		m.formatLFORate(lfo1_1),
		m.formatPercent(lfo1_1.GetDepth()),
		m.formatLFOValue(lfo1_1.GetMin(), tb303_1.GetLFO1Target()),
		m.formatLFOValue(lfo1_1.GetMax(), tb303_1.GetLFO1Target()),
		m.formatBool(lfo1_1.GetTempoSync()),
		m.formatLFOSyncMode(lfo1_1.GetSyncMode()),
		"", // Separator
		m.formatBool(tb303_1.GetLFO2Enabled()),
		m.getLFOTargetName(tb303_1.GetLFO2Target()),
		m.getLFOWaveformName(lfo2_1.GetWaveform()),
		m.formatLFORate(lfo2_1),
		m.formatPercent(lfo2_1.GetDepth()),
		m.formatLFOValue(lfo2_1.GetMin(), tb303_1.GetLFO2Target()),
		m.formatLFOValue(lfo2_1.GetMax(), tb303_1.GetLFO2Target()),
		m.formatBool(lfo2_1.GetTempoSync()),
		m.formatLFOSyncMode(lfo2_1.GetSyncMode()),
	}

	values2 := []string{
		m.getWaveformName(tb303_2),
		m.formatPercent(tb303_2.GetCurrentCutoff()),
		m.formatPercent(tb303_2.GetCurrentResonance()),
		m.formatPercent(tb303_2.GetCurrentEnvMod()),
		m.formatPercent(tb303_2.GetDecay()),
		m.formatPercent(tb303_2.GetAccent()),
		m.formatPercent(tb303_2.GetSlideTime()),
		m.formatPercent(tb303_2.GetSubLevel()),
		m.formatPercent(mixer.GetChannel(1).Level),
		"", // Separator
		m.formatBool(tb303_2.GetLFO1Enabled()),
		m.getLFOTargetName(tb303_2.GetLFO1Target()),
		m.getLFOWaveformName(lfo1_2.GetWaveform()),
		m.formatLFORate(lfo1_2),
		m.formatPercent(lfo1_2.GetDepth()),
		m.formatLFOValue(lfo1_2.GetMin(), tb303_2.GetLFO1Target()),
		m.formatLFOValue(lfo1_2.GetMax(), tb303_2.GetLFO1Target()),
		m.formatBool(lfo1_2.GetTempoSync()),
		m.formatLFOSyncMode(lfo1_2.GetSyncMode()),
		"", // Separator
		m.formatBool(tb303_2.GetLFO2Enabled()),
		m.getLFOTargetName(tb303_2.GetLFO2Target()),
		m.getLFOWaveformName(lfo2_2.GetWaveform()),
		m.formatLFORate(lfo2_2),
		m.formatPercent(lfo2_2.GetDepth()),
		m.formatLFOValue(lfo2_2.GetMin(), tb303_2.GetLFO2Target()),
		m.formatLFOValue(lfo2_2.GetMax(), tb303_2.GetLFO2Target()),
		m.formatBool(lfo2_2.GetTempoSync()),
		m.formatLFOSyncMode(lfo2_2.GetSyncMode()),
	}

	// Build the view
	var rows []string
	rows = append(rows, TitleStyle.Render("TB-303 SYNTHESIZERS"))
	rows = append(rows, "")

	// Instructions
	instructions := HelpStyle.Render("[←/→] Switch Synth  [↑/↓] Select Parameter  [</>] Adjust Value  [W] Waveform")
	rows = append(rows, instructions)
	rows = append(rows, "")

	// Render both TB-303 panels side by side
	panel1 := m.renderSynthPanel(1, params, values1, m.selectedSynth == 0)
	panel2 := m.renderSynthPanel(2, params, values2, m.selectedSynth == 1)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, panel1, "  ", panel2)
	rows = append(rows, panels)

	return strings.Join(rows, "\n")
}

// renderSynthPanel renders a single TB-303 panel
func (m Model) renderSynthPanel(synthNum int, params []string, values []string, focused bool) string {
	var rows []string

	// Title
	title := fmt.Sprintf("TB-303 #%d", synthNum)
	if focused {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			Render("▶ "+title))
	} else {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("  "+title))
	}
	rows = append(rows, "")

	// Parameters
	for i, param := range params {
		isSelected := focused && i == m.selectedParam

		// Check if this parameter is locked on the current step
		paramName := param
		if m.isParamLockedOnCurrentStep(synthNum, i) {
			paramName += "*" // Add asterisk for p-locked params
		}

		var line string
		if isSelected {
			line = lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(ColorPrimary).
				Bold(true).
				Width(30).
				Render(fmt.Sprintf(" %-12s %s", paramName+":", values[i]))
		} else {
			paramStyle := lipgloss.NewStyle().Foreground(ColorFg)
			valueStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			line = fmt.Sprintf(" %-12s %s",
				paramStyle.Render(paramName+":"),
				valueStyle.Render(values[i]))
		}
		rows = append(rows, line)
	}

	// Apply panel style
	content := strings.Join(rows, "\n")
	if focused {
		return FocusedPanelStyle.Width(34).Render(content)
	}
	return PanelStyle.Width(34).Render(content)
}

// isParamLockedOnCurrentStep checks if a parameter is locked on the currently selected step
// synthNum: 1 or 2 for TB-303 synths (0 for other views)
// paramIdx: the parameter index in the view
func (m Model) isParamLockedOnCurrentStep(synthNum int, paramIdx int) bool {
	// Get the current step
	var instrument sequencer.InstrumentType
	var paramName string

	switch m.currentView {
	case ViewSynth:
		// TB-303 parameter check
		if synthNum == 1 {
			instrument = sequencer.InstrumentTB303_1
		} else {
			instrument = sequencer.InstrumentTB303_2
		}
		paramName, _, _ = m.getSynthParamNameFromIndex(paramIdx)

	case ViewDrone:
		instrument = sequencer.InstrumentDrone
		paramName, _, _ = m.getDroneParamNameFromIndex(paramIdx)

	case ViewDrums:
		// Map selected drum to instrument
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
		paramName, _, _ = m.getDrumParamNameFromIndex(m.selectedDrum, paramIdx)

	default:
		return false
	}

	if paramName == "" {
		return false
	}

	// Get the pattern for this instrument
	patternID := m.seq.GetCurrentPatternID()
	pattern := m.seq.GetPattern(instrument, patternID)
	if pattern == nil {
		return false
	}

	// Get the selected step
	step := pattern.GetStep(m.selectedStep)
	if step == nil {
		return false
	}

	// Check if this parameter is locked
	_, locked := step.GetParamLock(paramName)
	return locked
}

// getSynthParamNameFromIndex returns the parameter name for a given synth param index
func (m Model) getSynthParamNameFromIndex(paramIdx int) (string, float32, bool) {
	// This mirrors the logic from getSynthParamLock but returns name only
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

	switch paramIdx {
	case 0:
		return "tb303_waveform", float32(tb.GetWaveform()), true
	case 1:
		return "tb303_cutoff", tb.GetCutoff(), true
	case 2:
		return "tb303_resonance", tb.GetResonance(), true
	case 3:
		return "tb303_envmod", tb.GetEnvMod(), true
	case 4:
		return "tb303_decay", tb.GetDecay() / 2.0, true
	case 5:
		return "tb303_accent", tb.GetAccent(), true
	case 6:
		val := (tb.GetSlideTime() - 0.01) / 0.49
		return "tb303_slidetime", val, true
	case 7:
		return "tb303_sublevel", tb.GetSubLevel(), true
	case 8:
		return "tb303_volume", channel.Level, true
	case 9:
		return "", 0, false // Separator
	default:
		return "", 0, false
	}
}

// getDroneParamNameFromIndex returns the parameter name for a given drone param index
func (m Model) getDroneParamNameFromIndex(paramIdx int) (string, float32, bool) {
	drone := m.engine.GetDrone()

	switch paramIdx {
	case 0:
		return "drone_volume", drone.GetMasterVolume(), true
	case 1:
		return "drone_tune", float32(drone.GetMasterTune()), true
	case 2:
		return "drone_osc1level", drone.GetOsc1Level(), true
	case 3:
		return "drone_osc2level", drone.GetOsc2Level(), true
	case 4:
		return "drone_osc3level", drone.GetOsc3Level(), true
	case 5:
		return "drone_osc2detune", drone.GetOsc2Detune(), true
	case 6:
		return "drone_osc3detune", drone.GetOsc3Detune(), true
	case 7:
		return "drone_sub1level", drone.GetSub1Level(), true
	case 8:
		return "drone_sub2level", drone.GetSub2Level(), true
	case 9:
		return "", 0, false // Separator
	case 10:
		return "drone_cutoff", drone.GetCutoff(), true
	case 11:
		return "drone_resonance", drone.GetResonance(), true
	case 12:
		return "drone_envmod", drone.GetEnvMod(), true
	case 13:
		return "drone_attack", drone.GetAttack(), true
	case 14:
		return "drone_decay", drone.GetDecay(), true
	case 15:
		return "drone_sustain", drone.GetSustain(), true
	case 16:
		return "drone_release", drone.GetRelease(), true
	case 17:
		return "", 0, false // Separator
	default:
		return "", 0, false
	}
}

// getDrumParamNameFromIndex returns the parameter name for a given drum param index
func (m Model) getDrumParamNameFromIndex(drumIdx int, paramIdx int) (string, float32, bool) {
	mixer := m.engine.GetMixer()

	switch drumIdx {
	case 0: // Kick
		kick := m.engine.GetKick()
		switch paramIdx {
		case 0:
			return "kick_tune", kick.GetTune(), true
		case 1:
			return "kick_decay", kick.GetCurrentDecay(), true
		case 2:
			return "kick_pitchdecay", kick.GetCurrentPitchDecay(), true
		case 3:
			return "kick_drive", kick.GetCurrentDrive(), true
		case 4:
			return "kick_click", kick.GetClickAmount(), true
		case 5:
			return "kick_volume", mixer.GetChannel(2).Level, true
		default:
			return "", 0, false
		}

	case 1: // Snare
		snare := m.engine.GetSnare()
		switch paramIdx {
		case 0:
			return "snare_tune", snare.GetTune(), true
		case 1:
			return "snare_snap", snare.GetCurrentSnap(), true
		case 2:
			return "snare_tonedecay", snare.GetCurrentToneDecay(), true
		case 3:
			return "snare_noisedecay", snare.GetCurrentNoiseDecay(), true
		case 4:
			return "snare_tonemix", snare.GetCurrentToneMix(), true
		case 5:
			return "snare_volume", mixer.GetChannel(3).Level, true
		default:
			return "", 0, false
		}

	case 2: // HH Closed
		hh := m.engine.GetHihatClosed()
		switch paramIdx {
		case 0:
			return "hhclosed_decay", hh.GetCurrentDecay(), true
		case 1:
			return "hhclosed_tone", hh.GetCurrentTone(), true
		case 2:
			return "hhclosed_noisemix", hh.GetCurrentNoiseMix(), true
		case 3:
			return "hhclosed_volume", mixer.GetChannel(4).Level, true
		default:
			return "", 0, false
		}

	case 3: // HH Open
		hh := m.engine.GetHihatOpen()
		switch paramIdx {
		case 0:
			return "hhopen_decay", hh.GetCurrentDecay(), true
		case 1:
			return "hhopen_tone", hh.GetCurrentTone(), true
		case 2:
			return "hhopen_noisemix", hh.GetCurrentNoiseMix(), true
		case 3:
			return "hhopen_volume", mixer.GetChannel(5).Level, true
		default:
			return "", 0, false
		}

	case 4: // Clap
		clap := m.engine.GetClap()
		switch paramIdx {
		case 0:
			return "clap_tune", clap.GetCurrentTune(), true
		case 1:
			return "clap_decay", clap.GetCurrentDecay(), true
		case 2:
			return "clap_tone", clap.GetCurrentTone(), true
		case 3:
			return "clap_drive", clap.GetCurrentDrive(), true
		case 4:
			return "clap_volume", mixer.GetChannel(6).Level, true
		default:
			return "", 0, false
		}

	case 5: // Rimshot
		rimshot := m.engine.GetRimshot()
		switch paramIdx {
		case 0:
			return "rimshot_pitch", rimshot.GetCurrentPitch(), true
		case 1:
			return "rimshot_decay", rimshot.GetCurrentDecay(), true
		case 2:
			return "rimshot_tone", rimshot.GetCurrentTone(), true
		case 3:
			return "rimshot_snap", rimshot.GetCurrentSnap(), true
		case 4:
			return "rimshot_volume", mixer.GetChannel(7).Level, true
		default:
			return "", 0, false
		}

	case 6: // Crash
		crash := m.engine.GetCrash()
		switch paramIdx {
		case 0:
			return "crash_decay", crash.GetCurrentDecay(), true
		case 1:
			return "crash_tone", crash.GetCurrentTone(), true
		case 2:
			return "crash_metallic", crash.GetCurrentMetallic(), true
		case 3:
			return "crash_volume", mixer.GetChannel(8).Level, true
		default:
			return "", 0, false
		}

	case 7: // Tom
		tom := m.engine.GetTom()
		switch paramIdx {
		case 0:
			return "tom_tune", tom.GetTune(), true
		case 1:
			return "tom_decay", tom.GetCurrentDecay(), true
		case 2:
			return "tom_pitchdecay", tom.GetCurrentPitchDecay(), true
		case 3:
			return "tom_volume", mixer.GetChannel(9).Level, true
		default:
			return "", 0, false
		}
	}

	return "", 0, false
}

// Helper functions for formatting
func (m Model) formatPercent(val float32) string {
	// Clamp value to 0.0-1.0 range to prevent negative repeat counts
	if val < 0.0 {
		val = 0.0
	}
	if val > 1.0 {
		val = 1.0
	}
	percent := int(val * 100)
	bars := percent / 10
	spaces := 10 - bars
	return fmt.Sprintf("[%s%s] %3d%%",
		strings.Repeat("▓", bars),
		strings.Repeat("░", spaces),
		percent)
}

func (m Model) getWaveformName(tb *synth.TB303) string {
	waveform := tb.GetWaveform()
	switch waveform {
	case synth.WaveformSaw:
		return "Saw"
	case synth.WaveformSquare:
		return "Square"
	case synth.WaveformTriangle:
		return "Triangle"
	case synth.WaveformSine:
		return "Sine"
	default:
		return "Unknown"
	}
}

func (m Model) getWaveformNameDrone(waveform synth.Waveform) string {
	switch waveform {
	case synth.WaveformSaw:
		return "Sawtooth"
	case synth.WaveformSquare:
		return "Square"
	case synth.WaveformTriangle:
		return "Triangle"
	case synth.WaveformSine:
		return "Sine"
	default:
		return "Unknown"
	}
}

func (m Model) renderDroneView() string {
	drone := m.engine.GetDrone()

	// Left column: Sound Generation
	leftParams := []string{
		"Master Volume",
		"Master Tune",
		"", // Separator
		"Osc1 Level",
		"Osc1 Waveform",
		"Osc1 PW",
		"Osc2 Level",
		"Osc2 Waveform",
		"Osc2 PW",
		"Osc2 Detune",
		"Osc3 Level",
		"Osc3 Waveform",
		"Osc3 PW",
		"Osc3 Detune",
		"", // Separator
		"Sub1 Level",
		"Sub2 Level",
		"Noise Level",
		"", // Separator
		"Unison Voices",
		"Unison Detune",
		"Unison Width",
		"", // Separator
		"Ring Mod",
		"Drive",
	}

	leftValues := []string{
		m.formatPercent(drone.GetMasterVolume()),
		fmt.Sprintf("%+d semitones", drone.GetMasterTune()),
		"", // Separator
		m.formatPercent(drone.GetOsc1Level()),
		m.getWaveformNameDrone(drone.GetOsc1Waveform()),
		m.formatPercent(drone.GetOsc1PW()),
		m.formatPercent(drone.GetOsc2Level()),
		m.getWaveformNameDrone(drone.GetOsc2Waveform()),
		m.formatPercent(drone.GetOsc2PW()),
		fmt.Sprintf("%+.1f cents", drone.GetOsc2Detune()),
		m.formatPercent(drone.GetOsc3Level()),
		m.getWaveformNameDrone(drone.GetOsc3Waveform()),
		m.formatPercent(drone.GetOsc3PW()),
		fmt.Sprintf("%+.1f cents", drone.GetOsc3Detune()),
		"", // Separator
		m.formatPercent(drone.GetSub1Level()),
		m.formatPercent(drone.GetSub2Level()),
		m.formatPercent(drone.GetNoiseLevel()),
		"", // Separator
		fmt.Sprintf("%d", drone.GetUnisonVoices()),
		fmt.Sprintf("%.1f cents", drone.GetUnisonDetune()),
		m.formatPercent(drone.GetUnisonWidth()),
		"", // Separator
		m.formatPercent(drone.GetRingModAmount()),
		m.formatPercent(drone.GetDriveAmount()),
	}

	// Right column: Filter & Modulation
	rightParams := []string{
		"", // Separator (maintains index compatibility with old layout)
		"Cutoff",
		"Resonance",
		"Env Mod",
		"Attack",
		"Decay",
		"Sustain",
		"Release",
		"", // Separator
		"LFO1 Enable",
		"LFO1 Target",
		"LFO1 Wave",
		"LFO1 Sync",
		"LFO1 Steps",
		"LFO1 Depth",
		"LFO1 Min",
		"LFO1 Max",
		"", // Separator
		"LFO2 Enable",
		"LFO2 Target",
		"LFO2 Wave",
		"LFO2 Sync",
		"LFO2 Steps",
		"LFO2 Depth",
		"LFO2 Min",
		"LFO2 Max",
	}

	rightValues := []string{
		"", // Separator (maintains index compatibility with old layout)
		m.formatPercent(drone.GetCutoff()),
		m.formatPercent(drone.GetResonance()),
		m.formatPercent(drone.GetEnvMod()),
		fmt.Sprintf("%.2f sec", drone.GetAttack()),
		fmt.Sprintf("%.2f sec", drone.GetDecay()),
		m.formatPercent(drone.GetSustain()),
		fmt.Sprintf("%.2f sec", drone.GetRelease()),
		"", // Separator
		m.formatBool(drone.GetLFO1Enabled()),
		m.getLFOTargetName(drone.GetLFO1Target()),
		m.getLFOWaveformName(drone.GetLFO1Waveform()),
		m.formatBool(drone.GetLFO1().GetTempoSync()),
		fmt.Sprintf("%d steps", drone.GetLFO1().GetStepsPerCycle()),
		m.formatPercent(drone.GetLFO1Depth()),
		m.formatLFOValue(drone.GetLFO1Min(), drone.GetLFO1Target()),
		m.formatLFOValue(drone.GetLFO1Max(), drone.GetLFO1Target()),
		"", // Separator
		m.formatBool(drone.GetLFO2Enabled()),
		m.getLFOTargetName(drone.GetLFO2Target()),
		m.getLFOWaveformName(drone.GetLFO2Waveform()),
		m.formatBool(drone.GetLFO2().GetTempoSync()),
		fmt.Sprintf("%d steps", drone.GetLFO2().GetStepsPerCycle()),
		m.formatPercent(drone.GetLFO2Depth()),
		m.formatLFOValue(drone.GetLFO2Min(), drone.GetLFO2Target()),
		m.formatLFOValue(drone.GetLFO2Max(), drone.GetLFO2Target()),
	}

	// Build the view
	var rows []string
	rows = append(rows, TitleStyle.Render("🎛️  MEGASYNTH"))
	rows = append(rows, "")

	// Instructions
	instructions := HelpStyle.Render("[↑/↓] Navigate  [</>] Adjust  [W] Cycle Waveform  [R] Randomize  [A] Arpeggiator")
	rows = append(rows, instructions)
	rows = append(rows, "")

	// Build left panel (Sound Generation)
	var leftRows []string
	leftRows = append(leftRows, lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render("▶ SOUND GENERATION"))
	leftRows = append(leftRows, "")

	leftParamCount := len(leftParams)
	for i, param := range leftParams {
		isSelected := i == m.selectedDroneParam

		// Check if this parameter is locked on the current step
		paramName := param
		if param != "" && m.isParamLockedOnCurrentStep(0, i) {
			paramName += "*" // Add asterisk for p-locked params
		}

		var line string
		if isSelected {
			line = lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(ColorPrimary).
				Bold(true).
				Width(40).
				Render(fmt.Sprintf(" ▶ %-15s %s", paramName+":", leftValues[i]))
		} else if param == "" {
			line = "" // Separator
		} else {
			paramStyle := lipgloss.NewStyle().Foreground(ColorFg)
			valueStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			line = fmt.Sprintf("   %-15s %s",
				paramStyle.Render(paramName+":"),
				valueStyle.Render(leftValues[i]))
		}
		leftRows = append(leftRows, line)
	}

	leftPanel := FocusedPanelStyle.Width(45).Render(strings.Join(leftRows, "\n"))

	// Build right panel (Filter & Modulation)
	var rightRows []string
	rightRows = append(rightRows, lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render("▶ FILTER & MODULATION"))
	rightRows = append(rightRows, "")

	for i, param := range rightParams {
		// Offset index for right column
		paramIndex := leftParamCount + i
		isSelected := paramIndex == m.selectedDroneParam

		// Check if this parameter is locked on the current step
		paramName := param
		if param != "" && m.isParamLockedOnCurrentStep(0, paramIndex) {
			paramName += "*" // Add asterisk for p-locked params
		}

		var line string
		if isSelected {
			line = lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(ColorPrimary).
				Bold(true).
				Width(40).
				Render(fmt.Sprintf(" ▶ %-15s %s", paramName+":", rightValues[i]))
		} else if param == "" {
			line = "" // Separator
		} else {
			paramStyle := lipgloss.NewStyle().Foreground(ColorFg)
			valueStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			line = fmt.Sprintf("   %-15s %s",
				paramStyle.Render(paramName+":"),
				valueStyle.Render(rightValues[i]))
		}
		rightRows = append(rightRows, line)
	}

	rightPanel := FocusedPanelStyle.Width(45).Render(strings.Join(rightRows, "\n"))

	// Combine panels side by side
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)
	rows = append(rows, panels)

	// Add LFO status visualization
	rows = append(rows, "")
	rows = append(rows, lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render("━━━━━━━━━━━━━━━━━━━━━ LFO STATUS ━━━━━━━━━━━━━━━━━━━━━"))
	rows = append(rows, "")

	// LFO1 status
	lfo1 := drone.GetLFO1()
	lfo1Enabled := drone.GetLFO1Enabled()
	lfo1Target := m.getLFOTargetName(drone.GetLFO1Target())
	lfo1Value := lfo1.GetValue()

	lfo1Status := fmt.Sprintf("LFO1: %s | Target: %s",
		m.formatBool(lfo1Enabled),
		lfo1Target)
	rows = append(rows, lipgloss.NewStyle().Foreground(ColorFg).Render(lfo1Status))

	if lfo1Enabled {
		lfo1Bar := m.renderLFOBar(lfo1Value, 50)
		rows = append(rows, lfo1Bar)
	} else {
		rows = append(rows, lipgloss.NewStyle().Foreground(ColorDim).Render("  [Disabled]"))
	}

	rows = append(rows, "")

	// LFO2 status
	lfo2 := drone.GetLFO2()
	lfo2Enabled := drone.GetLFO2Enabled()
	lfo2Target := m.getLFOTargetName(drone.GetLFO2Target())
	lfo2Value := lfo2.GetValue()

	lfo2Status := fmt.Sprintf("LFO2: %s | Target: %s",
		m.formatBool(lfo2Enabled),
		lfo2Target)
	rows = append(rows, lipgloss.NewStyle().Foreground(ColorFg).Render(lfo2Status))

	if lfo2Enabled {
		lfo2Bar := m.renderLFOBar(lfo2Value, 50)
		rows = append(rows, lfo2Bar)
	} else {
		rows = append(rows, lipgloss.NewStyle().Foreground(ColorDim).Render("  [Disabled]"))
	}

	return strings.Join(rows, "\n")
}

// renderLFOBar renders a visual bar showing LFO output value (0.0 to 1.0)
func (m Model) renderLFOBar(value float32, width int) string {
	// Clamp value to 0-1 range
	if value < 0.0 {
		value = 0.0
	}
	if value > 1.0 {
		value = 1.0
	}

	// Calculate bar position
	barPos := int(value * float32(width-1))

	// Build the bar
	var bar string
	bar += "  ["
	for i := 0; i < width; i++ {
		if i == barPos {
			bar += "█" // Current position marker
		} else if i < barPos {
			bar += "▒" // Filled portion
		} else {
			bar += " " // Empty portion
		}
	}
	bar += "]"

	// Add percentage
	percentage := int(value * 100)
	bar += fmt.Sprintf(" %3d%%", percentage)

	// Color based on value
	var style lipgloss.Style
	if value < 0.33 {
		style = lipgloss.NewStyle().Foreground(ColorSuccess)
	} else if value < 0.67 {
		style = lipgloss.NewStyle().Foreground(ColorWarning)
	} else {
		style = lipgloss.NewStyle().Foreground(ColorAccent)
	}

	return style.Render(bar)
}

func (m Model) renderArpView() string {
	drone := m.engine.GetDrone()
	arp := drone.GetArpeggiator()

	// Parameter list
	params := []string{
		"Enabled",
		"Pattern",
		"Rate",
		"Gate Length",
		"Octave Range",
		"Swing",
		"Latch Mode",
	}

	// Get current values
	values := []string{
		m.formatBool(arp.GetEnabled()),
		m.getArpPatternName(arp.GetPattern()),
		m.getArpRateName(arp.GetRate()),
		m.formatPercent(arp.GetGateLength()),
		fmt.Sprintf("%d octaves", arp.GetOctaveRange()),
		m.formatPercent(arp.GetSwing()),
		m.formatBool(arp.GetLatchMode()),
	}

	// Build the view
	var rows []string
	rows = append(rows, TitleStyle.Render("🎹 ARPEGGIATOR"))
	rows = append(rows, "")

	// Instructions
	instructions := HelpStyle.Render("[↑/↓] Navigate  [</>] Adjust  [Space] Toggle  [Tab/Esc] Exit")
	rows = append(rows, instructions)
	rows = append(rows, "")

	// Create two-column layout: Settings (left) | Preview (right)
	var settingsRows []string
	var previewRows []string

	// Settings panel
	settingsRows = append(settingsRows, lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render("▶ SETTINGS"))
	settingsRows = append(settingsRows, "")

	// Render parameters
	for i, param := range params {
		isSelected := i == m.selectedArpParam

		var line string
		if isSelected {
			line = lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(ColorPrimary).
				Bold(true).
				Width(35).
				Render(fmt.Sprintf(" ▶ %-14s %s", param+":", values[i]))
		} else {
			paramStyle := lipgloss.NewStyle().Foreground(ColorFg)
			valueStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			line = fmt.Sprintf("   %-14s %s",
				paramStyle.Render(param+":"),
				valueStyle.Render(values[i]))
		}
		settingsRows = append(settingsRows, line)
	}

	settingsPanel := FocusedPanelStyle.Width(40).Render(strings.Join(settingsRows, "\n"))

	// Preview panel
	previewRows = append(previewRows, lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render("  PREVIEW"))
	previewRows = append(previewRows, "")

	// Show chord editor
	if m.arpChordEditMode {
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true).
			Render("  ⚡ CHORD EDIT MODE"))
		previewRows = append(previewRows, "")

		// Show current buffer
		if len(m.arpChordBuffer) > 0 {
			noteNames := ""
			for i, note := range m.arpChordBuffer {
				if i > 0 {
					noteNames += " "
				}
				noteNames += m.getNoteName(note)
			}
			previewRows = append(previewRows, lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true).
				Render("  "+noteNames+"_"))
		} else {
			previewRows = append(previewRows, lipgloss.NewStyle().
				Foreground(ColorDim).
				Render("  _"))
		}

		previewRows = append(previewRows, "")
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("  Press Z-M keys to add"))
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("  Backspace to remove"))
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("  Enter to save"))
	} else {
		// Show current chord
		inputNotes := arp.GetInputNotes()
		if len(inputNotes) > 0 {
			previewRows = append(previewRows, lipgloss.NewStyle().
				Foreground(ColorFg).
				Render("  Current Chord:"))

			noteNames := ""
			for i, note := range inputNotes {
				if i > 0 {
					noteNames += " "
				}
				noteNames += m.getNoteName(note)
			}
			previewRows = append(previewRows, lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true).
				Render("  "+noteNames))
		} else {
			previewRows = append(previewRows, lipgloss.NewStyle().
				Foreground(ColorDim).
				Render("  No chord set"))
		}

		previewRows = append(previewRows, "")
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("  Press [E] to edit chord"))
	}

	previewRows = append(previewRows, "")

	// Show generated sequence
	sequence := arp.GetSequence()
	if len(sequence) > 0 {
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorFg).
			Render("  Arp Sequence:"))
		
		seqNotes := ""
		for i, note := range sequence {
			if i >= 16 {
				seqNotes += "..."
				break
			}
			if i > 0 {
				seqNotes += " "
			}
			seqNotes += m.getNoteName(note)
		}
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true).
			Render("  "+seqNotes))

		// Show sequence position indicator
		previewRows = append(previewRows, "")
		pos := arp.GetSequencePos()
		indicator := "  "
		for i := 0; i < min(len(sequence), 16); i++ {
			if i == pos {
				indicator += "▲ "
			} else {
				indicator += "  "
			}
		}
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Render(indicator))
	} else {
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("  (Sequence empty)"))
	}

	previewRows = append(previewRows, "")
	previewRows = append(previewRows, "")

	// Status
	if arp.GetEnabled() {
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true).
			Render("  ⚡ ACTIVE"))
	} else {
		previewRows = append(previewRows, lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("  ○ OFF"))
	}

	previewPanel := PanelStyle.Width(40).Render(strings.Join(previewRows, "\n"))

	// Combine panels side by side
	panels := lipgloss.JoinHorizontal(lipgloss.Top, settingsPanel, "  ", previewPanel)
	rows = append(rows, panels)

	rows = append(rows, "")
	rows = append(rows, "")

	// Tips
	tipStyle := lipgloss.NewStyle().Foreground(ColorDim)
	rows = append(rows, tipStyle.Render("💡 Tip: Arpeggiator plays notes from the MegaSynth sequencer step"))
	rows = append(rows, tipStyle.Render("💡 Use multiple notes per step (p-locks) for chords"))

	return strings.Join(rows, "\n")
}
func (m Model) renderDrumsView() string {
	// Get the selected drum's name, params, and title
	var title string
	var params []drumParam

	switch m.selectedDrum {
	case 0:
		title = "KICK"
		params = m.getKickParams()
	case 1:
		title = "SNARE"
		params = m.getSnareParams()
	case 2:
		title = "HH CLOSED"
		params = m.getHihatClosedParams()
	case 3:
		title = "HH OPEN"
		params = m.getHihatOpenParams()
	case 4:
		title = "CLAP"
		params = m.getClapParams()
	case 5:
		title = "RIMSHOT"
		params = m.getRimshotParams()
	case 6:
		title = "CRASH"
		params = m.getCrashParams()
	case 7:
		title = "TOM"
		params = m.getTomParams()
	}

	var rows []string
	rows = append(rows, TitleStyle.Render("DRUM SYNTHESIS - "+title))
	rows = append(rows, "")

	// Instructions
	instructions := HelpStyle.Render("[1-8] Select Drum  [↑/↓] Select Parameter  [</>] Adjust Value  [r] Randomize  [Tab] Next View")
	rows = append(rows, instructions)
	rows = append(rows, "")

	// Render only the selected drum panel (centered, wider)
	panel := m.renderDrumPanel(m.selectedDrum, title, params, true)

	// Center the panel
	centeredPanel := lipgloss.Place(
		m.width, m.height-6,
		lipgloss.Center, lipgloss.Top,
		panel,
	)
	rows = append(rows, centeredPanel)

	return strings.Join(rows, "\n")
}

// renderDrumPanel renders a single drum parameter panel
func (m Model) renderDrumPanel(drumIdx int, title string, params []drumParam, focused bool) string {
	var rows []string

	// Title
	if focused {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			Render("▶ "+title))
	} else {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("  "+title))
	}
	rows = append(rows, "")

	// Parameters
	for i, param := range params {
		isSelected := focused && i == m.selectedDrumParam

		// Skip separator lines (empty param names)
		if param.name == "" {
			rows = append(rows, "")
			continue
		}

		// Check if this parameter is locked on the current step
		paramName := param.name
		if m.isParamLockedOnCurrentStep(0, i) {
			paramName += "*" // Add asterisk for p-locked params
		}

		var line string
		if isSelected {
			line = lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(ColorPrimary).
				Bold(true).
				Width(45).
				Render(fmt.Sprintf(" %-15s %s", paramName+":", param.value))
		} else {
			paramStyle := lipgloss.NewStyle().Foreground(ColorFg)
			valueStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			line = fmt.Sprintf(" %-15s %s",
				paramStyle.Render(paramName+":"),
				valueStyle.Render(param.value))
		}
		rows = append(rows, line)
	}

	// Apply panel style (wider for single-drum view)
	content := strings.Join(rows, "\n")
	if focused {
		return FocusedPanelStyle.Width(50).Render(content)
	}
	return PanelStyle.Width(50).Render(content)
}

// drumParam holds a parameter name and formatted value
type drumParam struct {
	name  string
	value string
}

// getKickParams returns formatted kick parameters
func (m Model) getKickParams() []drumParam {
	kick := m.engine.GetKick()
	mixer := m.engine.GetMixer()
	lfo1 := kick.GetLFO1()

	params := []drumParam{
		{"Tune", m.formatSemitones(kick.GetTune())},
		{"Decay", m.formatTime(kick.GetCurrentDecay())},
		{"Pitch Env", m.formatTime(kick.GetCurrentPitchDecay())},
		{"Drive", m.formatPercent(kick.GetCurrentDrive())},
		{"Click", m.formatPercent(kick.GetClickAmount())},
		{"Volume", m.formatPercent(mixer.GetChannel(2).Level)},
		{"", ""}, // Separator
		{"LFO1 On", m.formatBool(kick.GetLFO1Enabled())},
		{"LFO1 Tgt", m.getKickLFOTargetName(kick.GetLFO1Target())},
		{"LFO1 Wave", m.getLFOWaveformName(lfo1.GetWaveform())},
		{"LFO1 Rate", m.formatLFORate(lfo1)},
		{"LFO1 Depth", m.formatPercent(lfo1.GetDepth())},
		{"LFO1 Min", m.formatKickLFOValue(lfo1.GetMin(), kick.GetLFO1Target())},
		{"LFO1 Max", m.formatKickLFOValue(lfo1.GetMax(), kick.GetLFO1Target())},
		{"LFO1 Sync", m.formatBool(lfo1.GetTempoSync())},
		{"LFO1 Mode", m.formatLFOSyncMode(lfo1.GetSyncMode())},
	}
	return params
}

// getSnareParams returns formatted snare parameters
func (m Model) getSnareParams() []drumParam {
	snare := m.engine.GetSnare()
	mixer := m.engine.GetMixer()
	lfo1 := snare.GetLFO1()

	params := []drumParam{
		{"Tune", m.formatSemitones(snare.GetTune())},
		{"Snap", m.formatPercent(snare.GetCurrentSnap())},
		{"Tone Decay", m.formatTime(snare.GetCurrentToneDecay())},
		{"Noise Decay", m.formatTime(snare.GetCurrentNoiseDecay())},
		{"Tone Mix", m.formatPercent(snare.GetCurrentToneMix())},
		{"Volume", m.formatPercent(mixer.GetChannel(3).Level)},
		{"", ""}, // Separator
		{"LFO1 On", m.formatBool(snare.GetLFO1Enabled())},
		{"LFO1 Tgt", m.getSnareLFOTargetName(snare.GetLFO1Target())},
		{"LFO1 Wave", m.getLFOWaveformName(lfo1.GetWaveform())},
		{"LFO1 Rate", m.formatLFORate(lfo1)},
		{"LFO1 Depth", m.formatPercent(lfo1.GetDepth())},
		{"LFO1 Min", m.formatSnareLFOValue(lfo1.GetMin(), snare.GetLFO1Target())},
		{"LFO1 Max", m.formatSnareLFOValue(lfo1.GetMax(), snare.GetLFO1Target())},
		{"LFO1 Sync", m.formatBool(lfo1.GetTempoSync())},
		{"LFO1 Mode", m.formatLFOSyncMode(lfo1.GetSyncMode())},
	}
	return params
}

// getHihatClosedParams returns formatted closed hi-hat parameters
func (m Model) getHihatClosedParams() []drumParam {
	hh := m.engine.GetHihatClosed()
	mixer := m.engine.GetMixer()
	lfo1 := hh.GetLFO1()

	params := []drumParam{
		{"Decay", m.formatTime(hh.GetCurrentDecay())},
		{"Tone", m.formatPercent(hh.GetCurrentTone())},
		{"Noise Mix", m.formatPercent(hh.GetCurrentNoiseMix())},
		{"Volume", m.formatPercent(mixer.GetChannel(4).Level)},
		{"", ""}, // Separator
		{"LFO1 On", m.formatBool(hh.GetLFO1Enabled())},
		{"LFO1 Tgt", m.getHiHatLFOTargetName(hh.GetLFO1Target())},
		{"LFO1 Wave", m.getLFOWaveformName(lfo1.GetWaveform())},
		{"LFO1 Rate", m.formatLFORate(lfo1)},
		{"LFO1 Depth", m.formatPercent(lfo1.GetDepth())},
		{"LFO1 Min", m.formatHiHatLFOValue(lfo1.GetMin(), hh.GetLFO1Target())},
		{"LFO1 Max", m.formatHiHatLFOValue(lfo1.GetMax(), hh.GetLFO1Target())},
		{"LFO1 Sync", m.formatBool(lfo1.GetTempoSync())},
		{"LFO1 Mode", m.formatLFOSyncMode(lfo1.GetSyncMode())},
	}
	return params
}

// getHihatOpenParams returns formatted open hi-hat parameters
func (m Model) getHihatOpenParams() []drumParam {
	hh := m.engine.GetHihatOpen()
	mixer := m.engine.GetMixer()
	lfo1 := hh.GetLFO1()

	params := []drumParam{
		{"Decay", m.formatTime(hh.GetCurrentDecay())},
		{"Tone", m.formatPercent(hh.GetCurrentTone())},
		{"Noise Mix", m.formatPercent(hh.GetCurrentNoiseMix())},
		{"Volume", m.formatPercent(mixer.GetChannel(5).Level)},
		{"", ""}, // Separator
		{"LFO1 On", m.formatBool(hh.GetLFO1Enabled())},
		{"LFO1 Tgt", m.getHiHatLFOTargetName(hh.GetLFO1Target())},
		{"LFO1 Wave", m.getLFOWaveformName(lfo1.GetWaveform())},
		{"LFO1 Rate", m.formatLFORate(lfo1)},
		{"LFO1 Depth", m.formatPercent(lfo1.GetDepth())},
		{"LFO1 Min", m.formatHiHatLFOValue(lfo1.GetMin(), hh.GetLFO1Target())},
		{"LFO1 Max", m.formatHiHatLFOValue(lfo1.GetMax(), hh.GetLFO1Target())},
		{"LFO1 Sync", m.formatBool(lfo1.GetTempoSync())},
		{"LFO1 Mode", m.formatLFOSyncMode(lfo1.GetSyncMode())},
	}
	return params
}

// getClapParams returns formatted clap parameters
func (m Model) getClapParams() []drumParam {
	clap := m.engine.GetClap()
	mixer := m.engine.GetMixer()
	lfo1 := clap.GetLFO1()
	lfo2 := clap.GetLFO2()

	params := []drumParam{
		{"Tune", m.formatSemitones(clap.GetCurrentTune())},
		{"Decay", m.formatTime(clap.GetCurrentDecay())},
		{"Tone", m.formatPercent(clap.GetCurrentTone())},
		{"Drive", m.formatPercent(clap.GetCurrentDrive())},
		{"Volume", m.formatPercent(mixer.GetChannel(6).Level)},
		{"", ""}, // Separator
		{"LFO1 On", m.formatBool(clap.GetLFO1Enabled())},
		{"LFO1 Tgt", m.getClapLFOTargetName(clap.GetLFO1Target())},
		{"LFO1 Wave", m.getLFOWaveformName(lfo1.GetWaveform())},
		{"LFO1 Rate", m.formatLFORate(lfo1)},
		{"LFO1 Depth", m.formatPercent(lfo1.GetDepth())},
		{"LFO1 Min", m.formatClapLFOValue(lfo1.GetMin(), clap.GetLFO1Target())},
		{"LFO1 Max", m.formatClapLFOValue(lfo1.GetMax(), clap.GetLFO1Target())},
		{"LFO1 Sync", m.formatBool(lfo1.GetTempoSync())},
		{"LFO1 Mode", m.formatLFOSyncMode(lfo1.GetSyncMode())},
		{"", ""}, // Separator
		{"LFO2 On", m.formatBool(clap.GetLFO2Enabled())},
		{"LFO2 Tgt", m.getClapLFOTargetName(clap.GetLFO2Target())},
		{"LFO2 Wave", m.getLFOWaveformName(lfo2.GetWaveform())},
		{"LFO2 Rate", m.formatLFORate(lfo2)},
		{"LFO2 Depth", m.formatPercent(lfo2.GetDepth())},
		{"LFO2 Min", m.formatClapLFOValue(lfo2.GetMin(), clap.GetLFO2Target())},
		{"LFO2 Max", m.formatClapLFOValue(lfo2.GetMax(), clap.GetLFO2Target())},
		{"LFO2 Sync", m.formatBool(lfo2.GetTempoSync())},
		{"LFO2 Mode", m.formatLFOSyncMode(lfo2.GetSyncMode())},
	}
	return params
}

func (m Model) getRimshotParams() []drumParam {
	rimshot := m.engine.GetRimshot()
	mixer := m.engine.GetMixer()
	lfo1 := rimshot.GetLFO1()
	lfo2 := rimshot.GetLFO2()

	params := []drumParam{
		{"Pitch", m.formatFrequency(rimshot.GetCurrentPitch())},
		{"Decay", m.formatTime(rimshot.GetCurrentDecay())},
		{"Tone", m.formatPercent(rimshot.GetCurrentTone())},
		{"Snap", m.formatPercent(rimshot.GetCurrentSnap())},
		{"Volume", m.formatPercent(mixer.GetChannel(7).Level)},
		{"", ""}, // Separator
		{"LFO1 On", m.formatBool(rimshot.GetLFO1Enabled())},
		{"LFO1 Tgt", m.getRimshotLFOTargetName(rimshot.GetLFO1Target())},
		{"LFO1 Wave", m.getLFOWaveformName(lfo1.GetWaveform())},
		{"LFO1 Rate", m.formatLFORate(lfo1)},
		{"LFO1 Depth", m.formatPercent(lfo1.GetDepth())},
		{"LFO1 Min", m.formatRimshotLFOValue(lfo1.GetMin(), rimshot.GetLFO1Target())},
		{"LFO1 Max", m.formatRimshotLFOValue(lfo1.GetMax(), rimshot.GetLFO1Target())},
		{"LFO1 Sync", m.formatBool(lfo1.GetTempoSync())},
		{"LFO1 Mode", m.formatLFOSyncMode(lfo1.GetSyncMode())},
		{"", ""}, // Separator
		{"LFO2 On", m.formatBool(rimshot.GetLFO2Enabled())},
		{"LFO2 Tgt", m.getRimshotLFOTargetName(rimshot.GetLFO2Target())},
		{"LFO2 Wave", m.getLFOWaveformName(lfo2.GetWaveform())},
		{"LFO2 Rate", m.formatLFORate(lfo2)},
		{"LFO2 Depth", m.formatPercent(lfo2.GetDepth())},
		{"LFO2 Min", m.formatRimshotLFOValue(lfo2.GetMin(), rimshot.GetLFO2Target())},
		{"LFO2 Max", m.formatRimshotLFOValue(lfo2.GetMax(), rimshot.GetLFO2Target())},
		{"LFO2 Sync", m.formatBool(lfo2.GetTempoSync())},
		{"LFO2 Mode", m.formatLFOSyncMode(lfo2.GetSyncMode())},
	}
	return params
}

func (m Model) getCrashParams() []drumParam {
	crash := m.engine.GetCrash()
	mixer := m.engine.GetMixer()
	lfo1 := crash.GetLFO1()
	lfo2 := crash.GetLFO2()

	params := []drumParam{
		{"Decay", m.formatTime(crash.GetCurrentDecay())},
		{"Tone", m.formatPercent(crash.GetCurrentTone())},
		{"Metallic", m.formatPercent(crash.GetCurrentMetallic())},
		{"Volume", m.formatPercent(mixer.GetChannel(8).Level)},
		{"", ""}, // Separator
		{"LFO1 On", m.formatBool(crash.GetLFO1Enabled())},
		{"LFO1 Tgt", m.getCrashLFOTargetName(crash.GetLFO1Target())},
		{"LFO1 Wave", m.getLFOWaveformName(lfo1.GetWaveform())},
		{"LFO1 Rate", m.formatLFORate(lfo1)},
		{"LFO1 Depth", m.formatPercent(lfo1.GetDepth())},
		{"LFO1 Min", m.formatCrashLFOValue(lfo1.GetMin(), crash.GetLFO1Target())},
		{"LFO1 Max", m.formatCrashLFOValue(lfo1.GetMax(), crash.GetLFO1Target())},
		{"LFO1 Sync", m.formatBool(lfo1.GetTempoSync())},
		{"LFO1 Mode", m.formatLFOSyncMode(lfo1.GetSyncMode())},
		{"", ""}, // Separator
		{"LFO2 On", m.formatBool(crash.GetLFO2Enabled())},
		{"LFO2 Tgt", m.getCrashLFOTargetName(crash.GetLFO2Target())},
		{"LFO2 Wave", m.getLFOWaveformName(lfo2.GetWaveform())},
		{"LFO2 Rate", m.formatLFORate(lfo2)},
		{"LFO2 Depth", m.formatPercent(lfo2.GetDepth())},
		{"LFO2 Min", m.formatCrashLFOValue(lfo2.GetMin(), crash.GetLFO2Target())},
		{"LFO2 Max", m.formatCrashLFOValue(lfo2.GetMax(), crash.GetLFO2Target())},
		{"LFO2 Sync", m.formatBool(lfo2.GetTempoSync())},
		{"LFO2 Mode", m.formatLFOSyncMode(lfo2.GetSyncMode())},
	}
	return params
}

func (m Model) getTomParams() []drumParam {
	tom := m.engine.GetTom()
	mixer := m.engine.GetMixer()
	lfo1 := tom.GetLFO1()
	lfo2 := tom.GetLFO2()

	params := []drumParam{
		{"Pitch", m.formatFrequency(tom.GetCurrentPitch())},
		{"Decay", m.formatTime(tom.GetCurrentDecay())},
		{"Tune", m.formatSemitones(tom.GetCurrentTune())},
		{"Pitch Decay", m.formatPercent(tom.GetCurrentPitchDecay())},
		{"Tone", m.formatPercent(tom.GetCurrentTone())},
		{"Attack", m.formatPercent(tom.GetCurrentAttack())},
		{"Volume", m.formatPercent(mixer.GetChannel(9).Level)},
		{"", ""}, // Separator
		{"LFO1 On", m.formatBool(tom.GetLFO1Enabled())},
		{"LFO1 Tgt", m.getTomLFOTargetName(tom.GetLFO1Target())},
		{"LFO1 Wave", m.getLFOWaveformName(lfo1.GetWaveform())},
		{"LFO1 Rate", m.formatLFORate(lfo1)},
		{"LFO1 Depth", m.formatPercent(lfo1.GetDepth())},
		{"LFO1 Min", m.formatTomLFOValue(lfo1.GetMin(), tom.GetLFO1Target())},
		{"LFO1 Max", m.formatTomLFOValue(lfo1.GetMax(), tom.GetLFO1Target())},
		{"LFO1 Sync", m.formatBool(lfo1.GetTempoSync())},
		{"LFO1 Mode", m.formatLFOSyncMode(lfo1.GetSyncMode())},
		{"", ""}, // Separator
		{"LFO2 On", m.formatBool(tom.GetLFO2Enabled())},
		{"LFO2 Tgt", m.getTomLFOTargetName(tom.GetLFO2Target())},
		{"LFO2 Wave", m.getLFOWaveformName(lfo2.GetWaveform())},
		{"LFO2 Rate", m.formatLFORate(lfo2)},
		{"LFO2 Depth", m.formatPercent(lfo2.GetDepth())},
		{"LFO2 Min", m.formatTomLFOValue(lfo2.GetMin(), tom.GetLFO2Target())},
		{"LFO2 Max", m.formatTomLFOValue(lfo2.GetMax(), tom.GetLFO2Target())},
		{"LFO2 Sync", m.formatBool(lfo2.GetTempoSync())},
		{"LFO2 Mode", m.formatLFOSyncMode(lfo2.GetSyncMode())},
	}
	return params
}

// formatTime formats a time value in seconds
func (m Model) formatTime(seconds float32) string {
	ms := int(seconds * 1000)
	return fmt.Sprintf("%d ms", ms)
}

// formatSemitones formats semitone values
func (m Model) formatSemitones(semitones float32) string {
	if semitones >= 0 {
		return fmt.Sprintf("+%.0f st", semitones)
	}
	return fmt.Sprintf("%.0f st", semitones)
}

// formatFrequency formats frequency in Hz
func (m Model) formatFrequency(hz float32) string {
	return fmt.Sprintf("%.0f Hz", hz)
}

func (m Model) renderEffectsView() string {
	var rows []string
	rows = append(rows, TitleStyle.Render("EFFECTS PROCESSING"))
	rows = append(rows, "")

	// Channel names
	channelNames := []string{
		"TB-303 #1 (Bass)",
		"TB-303 #2 (Lead)",
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

	// Instructions
	instructions := HelpStyle.Render("[1-9,0] Select Channel  [←/→] Select Effect  [↑/↓] Navigate Params  [</>] Adjust  [E] Toggle")
	rows = append(rows, instructions)
	rows = append(rows, "")

	// Show selected channel
	channelTitle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render(fmt.Sprintf("▶ CHANNEL %d: %s", m.selectedChannel+1, channelNames[m.selectedChannel]))
	rows = append(rows, channelTitle)
	rows = append(rows, "")

	// Get the channel
	mixer := m.engine.GetMixer()
	channel := mixer.GetChannel(m.selectedChannel)

	// Render effects panels (two rows of 3 effects each)
	// Top row: Distortion, Delay, Reverb
	distPanel := m.renderEffectPanel("DISTORTION", 0, m.getDistortionParams(channel), m.selectedEffect == 0)
	delayPanel := m.renderEffectPanel("DELAY", 1, m.getDelayParams(channel), m.selectedEffect == 1)
	reverbPanel := m.renderEffectPanel("REVERB", 2, m.getReverbParams(channel), m.selectedEffect == 2)

	effectsRow1 := lipgloss.JoinHorizontal(lipgloss.Top, distPanel, "  ", delayPanel, "  ", reverbPanel)
	rows = append(rows, effectsRow1)
	rows = append(rows, "") // Spacing between rows

	// Bottom row: PitchShifter, MultimodeFilter, BitCrusher
	pitchPanel := m.renderEffectPanel("PITCH SHIFTER", 3, m.getPitchShifterParams(channel), m.selectedEffect == 3)
	filterPanel := m.renderEffectPanel("FILTER", 4, m.getMultimodeFilterParams(channel), m.selectedEffect == 4)
	crusherPanel := m.renderEffectPanel("BIT CRUSHER", 5, m.getBitCrusherParams(channel), m.selectedEffect == 5)

	effectsRow2 := lipgloss.JoinHorizontal(lipgloss.Top, pitchPanel, "  ", filterPanel, "  ", crusherPanel)
	rows = append(rows, effectsRow2)

	return strings.Join(rows, "\n")
}

// renderEffectPanel renders a single effect panel
func (m Model) renderEffectPanel(title string, effectIdx int, params []effectParam, focused bool) string {
	var rows []string

	// Title
	if focused {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			Render("▶ "+title))
	} else {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(ColorDim).
			Render("  "+title))
	}
	rows = append(rows, "")

	// Parameters
	for i, param := range params {
		isSelected := focused && i == m.selectedEffectParam

		var line string
		if isSelected {
			line = lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(ColorPrimary).
				Bold(true).
				Width(22).
				Render(fmt.Sprintf(" %-9s %s", param.name+":", param.value))
		} else {
			paramStyle := lipgloss.NewStyle().Foreground(ColorFg)
			valueStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			line = fmt.Sprintf(" %-9s %s",
				paramStyle.Render(param.name+":"),
				valueStyle.Render(param.value))
		}
		rows = append(rows, line)
	}

	// Apply panel style
	content := strings.Join(rows, "\n")
	if focused {
		return FocusedPanelStyle.Width(26).Render(content)
	}
	return PanelStyle.Width(26).Render(content)
}

// effectParam holds an effect parameter name and formatted value
type effectParam struct {
	name  string
	value string
}

// getDistortionParams returns formatted distortion parameters
func (m Model) getDistortionParams(channel *audio.Channel) []effectParam {
	// Find distortion effect in chain
	dist := m.findEffectInChain(channel, 0) // Distortion is typically first
	if dist == nil {
		return []effectParam{
			{"Enabled", "OFF"},
		}
	}

	// Type assert to get actual distortion
	if distortion, ok := dist.(*effects.Distortion); ok {
		enabledStr := "OFF"
		if distortion.IsEnabled() {
			enabledStr = "ON"
		}
		return []effectParam{
			{"Enabled", enabledStr},
			{"Drive", m.formatDrive(distortion.GetDrive())},
			{"Tone", m.formatPercent(distortion.GetTone())},
			{"Mix", m.formatPercent(distortion.GetMix())},
		}
	}

	return []effectParam{{"Enabled", "OFF"}}
}

// getDelayParams returns formatted delay parameters
func (m Model) getDelayParams(channel *audio.Channel) []effectParam {
	// Find delay effect in chain
	del := m.findEffectInChain(channel, 1) // Delay is typically second
	if del == nil {
		return []effectParam{
			{"Enabled", "OFF"},
		}
	}

	// Type assert to get actual delay
	if delay, ok := del.(*effects.Delay); ok {
		enabledStr := "OFF"
		if delay.IsEnabled() {
			enabledStr = "ON"
		}
		return []effectParam{
			{"Enabled", enabledStr},
			{"Time", m.formatTime(delay.GetDelayTime())},
			{"Feedback", m.formatPercent(delay.GetFeedback())},
			{"Mix", m.formatPercent(delay.GetMix())},
		}
	}

	return []effectParam{{"Enabled", "OFF"}}
}

// getReverbParams returns formatted reverb parameters
func (m Model) getReverbParams(channel *audio.Channel) []effectParam {
	// Find reverb effect in chain
	rev := m.findEffectInChain(channel, 2) // Reverb is typically third
	if rev == nil {
		return []effectParam{
			{"Enabled", "OFF"},
		}
	}

	// Type assert to get actual reverb
	if reverb, ok := rev.(*effects.Reverb); ok {
		enabledStr := "OFF"
		if reverb.IsEnabled() {
			enabledStr = "ON"
		}
		return []effectParam{
			{"Enabled", enabledStr},
			{"Room Size", m.formatPercent(reverb.GetRoomSize())},
			{"Damping", m.formatPercent(reverb.GetDamping())},
			{"Mix", m.formatPercent(reverb.GetMix())},
		}
	}

	return []effectParam{{"Enabled", "OFF"}}
}

// getPitchShifterParams returns formatted pitch shifter parameters
func (m Model) getPitchShifterParams(channel *audio.Channel) []effectParam {
	// Find pitch shifter effect in chain
	ps := m.findEffectInChain(channel, 3) // PitchShifter is index 3
	if ps == nil {
		return []effectParam{
			{"Enabled", "OFF"},
		}
	}

	// Type assert to get actual pitch shifter
	if pitchShifter, ok := ps.(*effects.PitchShifter); ok {
		enabledStr := "OFF"
		if pitchShifter.IsEnabled() {
			enabledStr = "ON"
		}
		return []effectParam{
			{"Enabled", enabledStr},
			{"Pitch", m.formatSemitones(pitchShifter.GetPitch())},
			{"Ratio", fmt.Sprintf("%.2fx", pitchShifter.GetPitchRatio())},
			{"Mix", m.formatPercent(pitchShifter.GetMix())},
		}
	}

	return []effectParam{{"Enabled", "OFF"}}
}

// getMultimodeFilterParams returns formatted multimode filter parameters
func (m Model) getMultimodeFilterParams(channel *audio.Channel) []effectParam {
	// Find multimode filter effect in chain
	mf := m.findEffectInChain(channel, 4) // MultimodeFilter is index 4
	if mf == nil {
		return []effectParam{
			{"Enabled", "OFF"},
		}
	}

	// Type assert to get actual multimode filter
	if filter, ok := mf.(*effects.MultimodeFilter); ok {
		enabledStr := "OFF"
		if filter.IsEnabled() {
			enabledStr = "ON"
		}

		// Format filter mode
		var modeStr string
		switch filter.GetFilterMode() {
		case effects.FilterModeLowPass:
			modeStr = "LP"
		case effects.FilterModeHighPass:
			modeStr = "HP"
		case effects.FilterModeBandPass:
			modeStr = "BP"
		default:
			modeStr = "LP"
		}

		return []effectParam{
			{"Enabled", enabledStr},
			{"Cutoff", fmt.Sprintf("%.0f Hz", filter.GetCutoff())},
			{"Resonance", m.formatPercent(filter.GetResonance())},
			{"Mode", modeStr},
			{"Mix", m.formatPercent(filter.GetMix())},
		}
	}

	return []effectParam{{"Enabled", "OFF"}}
}

// getBitCrusherParams returns formatted bit crusher parameters
func (m Model) getBitCrusherParams(channel *audio.Channel) []effectParam {
	// Find bit crusher effect in chain
	bc := m.findEffectInChain(channel, 5) // BitCrusher is index 5
	if bc == nil {
		return []effectParam{
			{"Enabled", "OFF"},
		}
	}

	// Type assert to get actual bit crusher
	if crusher, ok := bc.(*effects.BitCrusher); ok {
		enabledStr := "OFF"
		if crusher.IsEnabled() {
			enabledStr = "ON"
		}

		// Calculate effective sample rate
		effectiveSR := 44100.0 / crusher.GetCrushRate()

		return []effectParam{
			{"Enabled", enabledStr},
			{"Bit Depth", fmt.Sprintf("%.1f bit", crusher.GetBitDepth())},
			{"Crush Rate", fmt.Sprintf("%.1fx", crusher.GetCrushRate())},
			{"Eff. SR", fmt.Sprintf("%.0f Hz", effectiveSR)},
			{"Mix", m.formatPercent(crusher.GetMix())},
		}
	}

	return []effectParam{{"Enabled", "OFF"}}
}

// findEffectInChain finds an effect by index in the channel's effect chain
func (m Model) findEffectInChain(channel *audio.Channel, index int) effects.Effect {
	if channel.Effects == nil {
		return nil
	}
	return channel.Effects.GetEffect(index)
}

// formatDrive formats drive values
func (m Model) formatDrive(drive float32) string {
	return fmt.Sprintf("%.1fx", drive)
}

func (m Model) renderMixerView() string {
	var rows []string
	rows = append(rows, TitleStyle.Render("MIXER"))
	rows = append(rows, "")

	// Channel names
	channelNames := []string{
		"TB-303 #1",
		"TB-303 #2",
		"Kick",
		"Snare",
		"HH Closed",
		"HH Open",
	}

	// Instructions
	instructions := HelpStyle.Render("[1-6] Select Channel  [↑/↓] Navigate  [</>] Adjust  [M] Mute  [S] Solo")
	rows = append(rows, instructions)
	rows = append(rows, "")

	mixer := m.engine.GetMixer()

	// Render channels in two rows of three
	var topRow, bottomRow []string

	for i := 0; i < 6; i++ {
		channel := mixer.GetChannel(i)
		if channel == nil {
			continue
		}

		channelPanel := m.renderChannelStrip(i, channelNames[i], channel)

		if i < 3 {
			topRow = append(topRow, channelPanel)
		} else {
			bottomRow = append(bottomRow, channelPanel)
		}
	}

	// Join panels horizontally
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, topRow...))
	rows = append(rows, "")
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, bottomRow...))

	return strings.Join(rows, "\n")
}

// renderChannelStrip renders a single mixer channel strip
func (m Model) renderChannelStrip(channelIdx int, name string, channel *audio.Channel) string {
	var lines []string
	selected := m.selectedMixerChannel == channelIdx

	// Channel name
	if selected {
		lines = append(lines, lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			Width(20).
			Align(lipgloss.Center).
			Render("▶ "+name+" ◀"))
	} else {
		lines = append(lines, lipgloss.NewStyle().
			Foreground(ColorDim).
			Width(20).
			Align(lipgloss.Center).
			Render(name))
	}

	lines = append(lines, "")

	// Peak meter (visual bar)
	meterHeight := 5
	peakLevel := channel.PeakLevel
	filledBars := int(peakLevel * float32(meterHeight))

	var meterLines []string
	for i := meterHeight - 1; i >= 0; i-- {
		var bar string
		if i < filledBars {
			// Filled bar
			if i >= 4 {
				bar = lipgloss.NewStyle().Foreground(ColorAccent).Render("█████")
			} else if i >= 3 {
				bar = lipgloss.NewStyle().Foreground(ColorWarning).Render("█████")
			} else {
				bar = lipgloss.NewStyle().Foreground(ColorSecondary).Render("█████")
			}
		} else {
			// Empty bar
			bar = lipgloss.NewStyle().Foreground(ColorDim).Render("░░░░░")
		}
		meterLines = append(meterLines, "  "+bar)
	}
	lines = append(lines, meterLines...)
	lines = append(lines, "")

	// Parameters
	params := []struct {
		idx   int
		name  string
		value string
	}{
		{0, "Level", m.formatPercent(channel.Level)},
		{1, "Pan", m.formatPan(channel.Pan)},
		{2, "Mute", m.formatBool(channel.Mute)},
		{3, "Solo", m.formatBool(channel.Solo)},
	}

	for _, p := range params {
		isSelected := selected && m.selectedMixerParam == p.idx

		var line string
		if isSelected {
			line = lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(ColorPrimary).
				Bold(true).
				Width(20).
				Render(fmt.Sprintf(" %-6s %s", p.name+":", p.value))
		} else {
			nameStyle := lipgloss.NewStyle().Foreground(ColorFg)
			valueStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			line = fmt.Sprintf(" %-6s %s",
				nameStyle.Render(p.name+":"),
				valueStyle.Render(p.value))
		}
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	if selected {
		return FocusedPanelStyle.Width(22).Render(content)
	}
	return PanelStyle.Width(22).Render(content)
}

// formatPan formats pan value (-1.0 to 1.0) as L/C/R
func (m Model) formatPan(pan float32) string {
	if pan < -0.1 {
		return fmt.Sprintf("L%d", int(-pan*100))
	} else if pan > 0.1 {
		return fmt.Sprintf("R%d", int(pan*100))
	}
	return "C"
}

// formatBool formats boolean as ON/OFF
func (m Model) formatBool(val bool) string {
	if val {
		return "ON"
	}
	return "OFF"
}

// getLFOTargetName returns the name of an LFO target
func (m Model) getLFOTargetName(target synth.LFOTarget) string {
	switch target {
	case synth.LFOTargetNone:
		return "None"
	case synth.LFOTargetCutoff:
		return "Cutoff"
	case synth.LFOTargetResonance:
		return "Resonance"
	case synth.LFOTargetPitch:
		return "Pitch"
	case synth.LFOTargetEnvMod:
		return "Env Mod"
	case synth.LFOTargetPulseWidth:
		return "Pulse Width"
	case synth.LFOTargetVolume:
		return "Volume"
	default:
		return "Unknown"
	}
}

// getLFOWaveformName returns the name of an LFO waveform
func (m Model) getLFOWaveformName(waveform dsp.LFOWaveform) string {
	switch waveform {
	case dsp.LFOSine:
		return "Sine"
	case dsp.LFOTriangle:
		return "Triangle"
	case dsp.LFOSquare:
		return "Square"
	case dsp.LFOSawUp:
		return "Saw Up"
	case dsp.LFOSawDown:
		return "Saw Down"
	case dsp.LFORandom:
		return "Random"
	default:
		return "Unknown"
	}
}

// formatLFORate formats LFO rate (Hz or tempo-synced)
func (m Model) formatLFORate(lfo *dsp.LFO) string {
	if lfo.GetTempoSync() {
		if lfo.GetSyncMode() == dsp.SyncModeSteps {
			// Step-locked mode: show steps per cycle
			return fmt.Sprintf("%d steps", lfo.GetStepsPerCycle())
		}

		// Time-based mode: show musical divisions
		rate := lfo.GetSyncRate()
		if rate >= 1.0 {
			return fmt.Sprintf("%.0f/1", rate)
		} else if rate == 0.5 {
			return "1/2"
		} else if rate == 0.25 {
			return "1/4"
		} else if rate == 0.125 {
			return "1/8"
		} else if rate == 0.0625 {
			return "1/16"
		} else if rate == 0.03125 {
			return "1/32"
		} else if rate == 0.015625 {
			return "1/64"
		} else if rate == 0.0078125 {
			return "1/128"
		} else if rate == 0.00390625 {
			return "1/256"
		} else {
			return fmt.Sprintf("1/%.0f", 1.0/rate)
		}
	}
	return fmt.Sprintf("%.2fHz", lfo.GetRate())
}

// formatLFOSyncMode formats the LFO sync mode
func (m Model) formatLFOSyncMode(mode dsp.LFOSyncMode) string {
	switch mode {
	case dsp.SyncModeTime:
		return "Time"
	case dsp.SyncModeSteps:
		return "Steps"
	default:
		return "Unknown"
	}
}

// formatLFOValue formats LFO min/max values based on target
func (m Model) formatLFOValue(val float32, target synth.LFOTarget) string {
	switch target {
	case synth.LFOTargetPitch:
		// Pitch is a multiplier, show as percentage deviation
		deviation := (val - 1.0) * 100.0
		if deviation >= 0 {
			return fmt.Sprintf("+%.1f%%", deviation)
		}
		return fmt.Sprintf("%.1f%%", deviation)
	default:
		// Cutoff/Resonance are 0-1, show as percentage
		return m.formatPercent(val)
	}
}

// Drum LFO value formatters (for min/max ranges)

// formatKickLFOValue formats kick LFO min/max values based on target
func (m Model) formatKickLFOValue(val float32, target drums.KickLFOTarget) string {
	switch target {
	case drums.KickLFOTargetNone:
		return "---"
	case drums.KickLFOTargetPitch:
		return fmt.Sprintf("%.1fHz", val)
	case drums.KickLFOTargetDecay, drums.KickLFOTargetPitchDecay:
		return m.formatTime(val)
	case drums.KickLFOTargetDrive:
		return m.formatPercent(val)
	default:
		return "---"
	}
}

// formatSnareLFOValue formats snare LFO min/max values based on target
func (m Model) formatSnareLFOValue(val float32, target drums.SnareLFOTarget) string {
	switch target {
	case drums.SnareLFOTargetNone:
		return "---"
	case drums.SnareLFOTargetNoiseDecay, drums.SnareLFOTargetToneDecay:
		return m.formatTime(val)
	case drums.SnareLFOTargetToneMix, drums.SnareLFOTargetSnap:
		return m.formatPercent(val)
	default:
		return "---"
	}
}

// formatHiHatLFOValue formats hihat LFO min/max values based on target
func (m Model) formatHiHatLFOValue(val float32, target drums.HiHatLFOTarget) string {
	switch target {
	case drums.HiHatLFOTargetNone:
		return "---"
	case drums.HiHatLFOTargetDecay:
		return m.formatTime(val)
	case drums.HiHatLFOTargetTone, drums.HiHatLFOTargetNoiseMix:
		return m.formatPercent(val)
	default:
		return "---"
	}
}

// formatClapLFOValue formats clap LFO min/max values based on target
func (m Model) formatClapLFOValue(val float32, target drums.ClapLFOTarget) string {
	switch target {
	case drums.ClapLFOTargetNone:
		return "---"
	case drums.ClapLFOTargetTune:
		return m.formatSemitones(val)
	case drums.ClapLFOTargetDecay:
		return m.formatTime(val)
	case drums.ClapLFOTargetTone, drums.ClapLFOTargetDrive:
		return m.formatPercent(val)
	default:
		return "---"
	}
}

// formatRimshotLFOValue formats rimshot LFO min/max values based on target
func (m Model) formatRimshotLFOValue(val float32, target drums.RimshotLFOTarget) string {
	switch target {
	case drums.RimshotLFOTargetNone:
		return "---"
	case drums.RimshotLFOTargetPitch:
		return fmt.Sprintf("%.1fHz", val)
	case drums.RimshotLFOTargetDecay:
		return m.formatTime(val)
	case drums.RimshotLFOTargetTone, drums.RimshotLFOTargetSnap:
		return m.formatPercent(val)
	default:
		return "---"
	}
}

// formatCrashLFOValue formats crash LFO min/max values based on target
func (m Model) formatCrashLFOValue(val float32, target drums.CrashLFOTarget) string {
	switch target {
	case drums.CrashLFOTargetNone:
		return "---"
	case drums.CrashLFOTargetDecay:
		return m.formatTime(val)
	case drums.CrashLFOTargetTone, drums.CrashLFOTargetMetallic:
		return m.formatPercent(val)
	default:
		return "---"
	}
}

// formatTomLFOValue formats tom LFO min/max values based on target
func (m Model) formatTomLFOValue(val float32, target drums.TomLFOTarget) string {
	switch target {
	case drums.TomLFOTargetNone:
		return "---"
	case drums.TomLFOTargetPitch:
		return fmt.Sprintf("%.1fHz", val)
	case drums.TomLFOTargetTune:
		return m.formatSemitones(val)
	case drums.TomLFOTargetDecay, drums.TomLFOTargetPitchDecay:
		return m.formatTime(val)
	case drums.TomLFOTargetTone, drums.TomLFOTargetAttack:
		return m.formatPercent(val)
	default:
		return "---"
	}
}

// Drum LFO target name helpers

// getKickLFOTargetName returns the display name for a kick LFO target
func (m Model) getKickLFOTargetName(target drums.KickLFOTarget) string {
	switch target {
	case drums.KickLFOTargetNone:
		return "None"
	case drums.KickLFOTargetPitch:
		return "Pitch"
	case drums.KickLFOTargetDecay:
		return "Decay"
	case drums.KickLFOTargetPitchDecay:
		return "Pitch Env"
	case drums.KickLFOTargetDrive:
		return "Drive"
	default:
		return "None"
	}
}

// getSnareLFOTargetName returns the display name for a snare LFO target
func (m Model) getSnareLFOTargetName(target drums.SnareLFOTarget) string {
	switch target {
	case drums.SnareLFOTargetNone:
		return "None"
	case drums.SnareLFOTargetToneMix:
		return "Tone Mix"
	case drums.SnareLFOTargetSnap:
		return "Snap"
	case drums.SnareLFOTargetNoiseDecay:
		return "Noise Decay"
	case drums.SnareLFOTargetToneDecay:
		return "Tone Decay"
	default:
		return "None"
	}
}

// getHiHatLFOTargetName returns the display name for a hihat LFO target
func (m Model) getHiHatLFOTargetName(target drums.HiHatLFOTarget) string {
	switch target {
	case drums.HiHatLFOTargetNone:
		return "None"
	case drums.HiHatLFOTargetTone:
		return "Tone"
	case drums.HiHatLFOTargetNoiseMix:
		return "Noise Mix"
	case drums.HiHatLFOTargetDecay:
		return "Decay"
	default:
		return "None"
	}
}

// getClapLFOTargetName returns the display name for a clap LFO target
func (m Model) getClapLFOTargetName(target drums.ClapLFOTarget) string {
	switch target {
	case drums.ClapLFOTargetNone:
		return "None"
	case drums.ClapLFOTargetTune:
		return "Tune"
	case drums.ClapLFOTargetDecay:
		return "Decay"
	case drums.ClapLFOTargetTone:
		return "Tone"
	case drums.ClapLFOTargetDrive:
		return "Drive"
	default:
		return "None"
	}
}

// getRimshotLFOTargetName returns the display name for a rimshot LFO target
func (m Model) getRimshotLFOTargetName(target drums.RimshotLFOTarget) string {
	switch target {
	case drums.RimshotLFOTargetNone:
		return "None"
	case drums.RimshotLFOTargetPitch:
		return "Pitch"
	case drums.RimshotLFOTargetDecay:
		return "Decay"
	case drums.RimshotLFOTargetTone:
		return "Tone"
	case drums.RimshotLFOTargetSnap:
		return "Snap"
	default:
		return "None"
	}
}

// getCrashLFOTargetName returns the display name for a crash LFO target
func (m Model) getCrashLFOTargetName(target drums.CrashLFOTarget) string {
	switch target {
	case drums.CrashLFOTargetNone:
		return "None"
	case drums.CrashLFOTargetDecay:
		return "Decay"
	case drums.CrashLFOTargetTone:
		return "Tone"
	case drums.CrashLFOTargetMetallic:
		return "Metallic"
	default:
		return "None"
	}
}

// getTomLFOTargetName returns the display name for a tom LFO target
func (m Model) getTomLFOTargetName(target drums.TomLFOTarget) string {
	switch target {
	case drums.TomLFOTargetNone:
		return "None"
	case drums.TomLFOTargetPitch:
		return "Pitch"
	case drums.TomLFOTargetDecay:
		return "Decay"
	case drums.TomLFOTargetTune:
		return "Tune"
	case drums.TomLFOTargetPitchDecay:
		return "Pitch Decay"
	case drums.TomLFOTargetTone:
		return "Tone"
	case drums.TomLFOTargetAttack:
		return "Attack"
	default:
		return "None"
	}
}

// handleSaveLoadInput handles keyboard input when in save/load mode
func (m Model) handleSaveLoadInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle delete confirmation first
	if m.deleteConfirmMode {
		return m.handleDeleteConfirmation(msg)
	}

	switch msg.String() {
	case "esc": // Cancel
		m.saveLoadMode = 0
		m.filenameInput = ""
		m.deleteConfirmMode = false
		m.deleteConfirmProject = ""
		return m, nil

	case "tab": // Toggle sort mode
		m.sortMode = (m.sortMode + 1) % 2
		m.sortProjectFiles()
		return m, nil

	case "up": // Navigate file list up
		if len(m.availableFiles) > 0 {
			m.selectedFileIndex--
			if m.selectedFileIndex < 0 {
				m.selectedFileIndex = len(m.availableFiles) - 1
			}
			// In save mode, update filename input with selected file
			if m.saveLoadMode == 1 {
				m.filenameInput = m.availableFiles[m.selectedFileIndex]
			}
		}
		return m, nil

	case "k": // Navigate file list up (vim-style, load mode only)
		if m.saveLoadMode == 2 && len(m.availableFiles) > 0 {
			m.selectedFileIndex--
			if m.selectedFileIndex < 0 {
				m.selectedFileIndex = len(m.availableFiles) - 1
			}
			return m, nil
		}
		// In save mode, allow typing 'k'
		if m.saveLoadMode == 1 {
			m.filenameInput += "k"
		}
		return m, nil

	case "down": // Navigate file list down
		if len(m.availableFiles) > 0 {
			m.selectedFileIndex++
			if m.selectedFileIndex >= len(m.availableFiles) {
				m.selectedFileIndex = 0
			}
			// In save mode, update filename input with selected file
			if m.saveLoadMode == 1 {
				m.filenameInput = m.availableFiles[m.selectedFileIndex]
			}
		}
		return m, nil

	case "j": // Navigate file list down (vim-style, load mode only)
		if m.saveLoadMode == 2 && len(m.availableFiles) > 0 {
			m.selectedFileIndex++
			if m.selectedFileIndex >= len(m.availableFiles) {
				m.selectedFileIndex = 0
			}
			return m, nil
		}
		// In save mode, allow typing 'j'
		if m.saveLoadMode == 1 {
			m.filenameInput += "j"
		}
		return m, nil

	case "pgup": // Page up (jump 10 files)
		if len(m.availableFiles) > 0 {
			m.selectedFileIndex -= 10
			if m.selectedFileIndex < 0 {
				m.selectedFileIndex = 0
			}
			// In save mode, update filename input with selected file
			if m.saveLoadMode == 1 {
				m.filenameInput = m.availableFiles[m.selectedFileIndex]
			}
		}
		return m, nil

	case "pgdown": // Page down (jump 10 files)
		if len(m.availableFiles) > 0 {
			m.selectedFileIndex += 10
			if m.selectedFileIndex >= len(m.availableFiles) {
				m.selectedFileIndex = len(m.availableFiles) - 1
			}
			// In save mode, update filename input with selected file
			if m.saveLoadMode == 1 {
				m.filenameInput = m.availableFiles[m.selectedFileIndex]
			}
		}
		return m, nil

	case "home": // Jump to first file
		if len(m.availableFiles) > 0 {
			m.selectedFileIndex = 0
			// In save mode, update filename input with selected file
			if m.saveLoadMode == 1 {
				m.filenameInput = m.availableFiles[m.selectedFileIndex]
			}
		}
		return m, nil

	case "g": // Jump to first file (vim-style, load mode only)
		if m.saveLoadMode == 2 && len(m.availableFiles) > 0 {
			m.selectedFileIndex = 0
			return m, nil
		}
		// In save mode, allow typing 'g'
		if m.saveLoadMode == 1 {
			m.filenameInput += "g"
		}
		return m, nil

	case "end": // Jump to last file
		if len(m.availableFiles) > 0 {
			m.selectedFileIndex = len(m.availableFiles) - 1
			// In save mode, update filename input with selected file
			if m.saveLoadMode == 1 {
				m.filenameInput = m.availableFiles[m.selectedFileIndex]
			}
		}
		return m, nil

	case "G": // Jump to last file (vim-style, load mode only)
		if m.saveLoadMode == 2 && len(m.availableFiles) > 0 {
			m.selectedFileIndex = len(m.availableFiles) - 1
			return m, nil
		}
		// In save mode, allow typing 'G'
		if m.saveLoadMode == 1 {
			m.filenameInput += "G"
		}
		return m, nil

	case "enter": // Confirm save/load
		if m.saveLoadMode == 1 {
			// Save
			m.saveProject()
		} else if m.saveLoadMode == 2 {
			// Load - use selected file from list
			if len(m.availableFiles) > 0 && m.selectedFileIndex >= 0 && m.selectedFileIndex < len(m.availableFiles) {
				m.filenameInput = m.availableFiles[m.selectedFileIndex]
			}
			m.loadProject()
		}
		m.saveLoadMode = 0
		return m, nil

	case "backspace": // Delete character (save mode only)
		if m.saveLoadMode == 1 && len(m.filenameInput) > 0 {
			m.filenameInput = m.filenameInput[:len(m.filenameInput)-1]
		}
		return m, nil

	case "delete": // Delete project (load mode only)
		if m.saveLoadMode == 2 && len(m.availableFiles) > 0 && m.selectedFileIndex >= 0 && m.selectedFileIndex < len(m.availableFiles) {
			// Enter delete confirmation mode
			m.deleteConfirmMode = true
			m.deleteConfirmProject = m.availableFiles[m.selectedFileIndex]
		}
		return m, nil

	case "d", "D": // Delete in load mode, type character in save mode
		if m.saveLoadMode == 2 {
			// Load mode: delete project
			if len(m.availableFiles) > 0 && m.selectedFileIndex >= 0 && m.selectedFileIndex < len(m.availableFiles) {
				m.deleteConfirmMode = true
				m.deleteConfirmProject = m.availableFiles[m.selectedFileIndex]
			}
			return m, nil
		}
		// Save mode: fall through to add character
		if m.saveLoadMode == 1 && len(msg.String()) == 1 {
			m.filenameInput += msg.String()
		}
		return m, nil

	default: // Add character (save mode only)
		// Only add printable characters in save mode
		if m.saveLoadMode == 1 && len(msg.String()) == 1 {
			m.filenameInput += msg.String()
		}
		return m, nil
	}
}

// handleDeleteConfirmation handles keyboard input during delete confirmation
func (m Model) handleDeleteConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y": // Confirm delete
		m.deleteProject()
		m.deleteConfirmMode = false
		m.deleteConfirmProject = ""
		return m, nil

	case "n", "N", "esc": // Cancel delete
		m.deleteConfirmMode = false
		m.deleteConfirmProject = ""
		return m, nil
	}
	return m, nil
}

// deleteProject deletes the selected project file
func (m *Model) deleteProject() {
	if m.deleteConfirmProject == "" {
		return
	}

	filename := filepath.Join(config.ProjectsDir, m.deleteConfirmProject)
	if !strings.HasSuffix(filename, ".gruvbox") {
		filename += ".gruvbox"
	}

	err := os.Remove(filename)
	if err != nil {
		m.statusMessage = fmt.Sprintf("Error deleting: %v", err)
		m.statusDuration = 120
	} else {
		m.statusMessage = fmt.Sprintf("Deleted: %s", m.deleteConfirmProject)
		m.statusDuration = 120

		// Refresh file list
		m.scanProjectFiles()

		// Adjust selection if necessary
		if m.selectedFileIndex >= len(m.availableFiles) && len(m.availableFiles) > 0 {
			m.selectedFileIndex = len(m.availableFiles) - 1
		} else if len(m.availableFiles) == 0 {
			m.selectedFileIndex = 0
		}
	}
}

// saveProject saves the current project to a file
func (m *Model) saveProject() {
	if m.filenameInput == "" {
		m.statusMessage = "Error: Filename cannot be empty"
		m.statusDuration = 60 // Show for ~1 second
		return
	}

	// Build filename with projects directory
	filename := filepath.Join(config.ProjectsDir, m.filenameInput)

	// Export project from engine
	proj := project.ExportFromEngine(m.engine, m.filenameInput)

	// Save to file
	if err := proj.Save(filename); err != nil {
		m.statusMessage = fmt.Sprintf("Error saving: %v", err)
		m.statusDuration = 120 // Show for ~2 seconds
		return
	}

	m.currentProjectName = m.filenameInput
	m.statusMessage = fmt.Sprintf("Saved to %s.gruvbox", filename)
	m.statusDuration = 60
	m.filenameInput = ""
}

// loadProject loads a project from a file
func (m *Model) loadProject() {
	if m.filenameInput == "" {
		m.statusMessage = "Error: Filename cannot be empty"
		m.statusDuration = 60
		return
	}

	filename := filepath.Join(config.ProjectsDir, m.filenameInput)
	if !strings.HasSuffix(filename, ".gruvbox") {
		filename += ".gruvbox"
	}

	// Load project from file
	proj, err := project.Load(filename)
	if err != nil {
		m.statusMessage = fmt.Sprintf("Error loading: %v", err)
		m.statusDuration = 120
		return
	}

	// Import project to engine
	if err := project.ImportToEngine(proj, m.engine); err != nil {
		m.statusMessage = fmt.Sprintf("Error importing: %v", err)
		m.statusDuration = 120
		return
	}

	// Load mute/solo states from patterns after project import
	m.loadMuteSoloForAllTracks()
	m.loadAllSoundsFromPattern() // Load sound settings for ALL tracks

	// Update UI state to match loaded project
	m.bpm = proj.BPM
	m.swing = m.seq.GetSwing()
	m.currentProjectName = m.filenameInput

	m.statusMessage = fmt.Sprintf("Loaded %s", filename)
	m.statusDuration = 60
	m.filenameInput = ""
}

// performAutosave performs a silent autosave in the background
func (m *Model) performAutosave() {
	if m.autosavePath == "" {
		return
	}

	// Export project from engine
	proj := project.ExportFromEngine(m.engine, "autosave")

	// Save to file silently (don't update status)
	_ = proj.Save(m.autosavePath)
}

// renderSaveLoadPrompt renders the save/load input prompt
func (m Model) renderSaveLoadPrompt() string {
	if m.saveLoadMode == 0 {
		return ""
	}

	var prompt string
	if m.saveLoadMode == 1 {
		prompt = "Save Project: "
	} else {
		prompt = "Load Project: "
	}

	input := m.filenameInput + "_"
	promptStyle := lipgloss.NewStyle().
		Background(ColorPrimary).
		Foreground(ColorBg).
		Bold(true).
		Padding(0, 1)

	return "\n" + promptStyle.Render(prompt+input) + "\n" +
		HelpStyle.Render("[Enter] Confirm  [Esc] Cancel")
}

// renderStatusMessage renders the status message if active
func (m Model) renderStatusMessage() string {
	if m.statusDuration <= 0 || m.statusMessage == "" {
		return ""
	}

	// Determine style based on message type
	style := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	if strings.Contains(m.statusMessage, "Error") {
		style = style.Foreground(ColorAccent)
	}

	return "\n" + style.Render(m.statusMessage)
}

// scanProjectFiles scans the projects/ directory for .gruvbox files
func (m *Model) scanProjectFiles() {
	// Use configured projects directory
	projectDir := config.ProjectsDir

	// Scan for .gruvbox files
	files, err := filepath.Glob(filepath.Join(projectDir, "*.gruvbox"))
	if err != nil {
		m.availableFiles = []string{}
		return
	}

	// Extract just the filenames (without path and extension)
	m.availableFiles = make([]string, 0, len(files))
	for _, file := range files {
		baseName := filepath.Base(file)
		// Remove .gruvbox extension
		name := strings.TrimSuffix(baseName, ".gruvbox")
		m.availableFiles = append(m.availableFiles, name)
	}

	// Sort files based on sort mode
	m.sortProjectFiles()
}

// sortProjectFiles sorts the available files list
func (m *Model) sortProjectFiles() {
	switch m.sortMode {
	case 0: // Sort by name
		sort.Strings(m.availableFiles)
	case 1: // Sort by date modified (newest first)
		projectDir := config.ProjectsDir
		type fileInfo struct {
			name    string
			modTime time.Time
		}
		fileInfos := make([]fileInfo, 0, len(m.availableFiles))

		for _, name := range m.availableFiles {
			path := filepath.Join(projectDir, name+".gruvbox")
			if info, err := os.Stat(path); err == nil {
				fileInfos = append(fileInfos, fileInfo{name: name, modTime: info.ModTime()})
			}
		}

		// Sort by modification time (newest first)
		sort.Slice(fileInfos, func(i, j int) bool {
			return fileInfos[i].modTime.After(fileInfos[j].modTime)
		})

		// Update availableFiles with sorted names
		m.availableFiles = make([]string, 0, len(fileInfos))
		for _, fi := range fileInfos {
			m.availableFiles = append(m.availableFiles, fi.name)
		}
	}
}

// renderFileBrowser renders the file browser popup
func (m Model) renderFileBrowser() string {
	title := "LOAD PROJECT"
	if m.saveLoadMode == 1 {
		title = "SAVE PROJECT"
	}

	var rows []string
	rows = append(rows, TitleStyle.Render(title))
	rows = append(rows, "")

	// Show current filename input for save mode
	if m.saveLoadMode == 1 {
		promptStyle := lipgloss.NewStyle().Foreground(ColorFg)
		inputStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
		rows = append(rows, promptStyle.Render("Filename: ")+inputStyle.Render(m.filenameInput+"_"))
		rows = append(rows, "")
	}

	// Sort mode indicator
	sortLabel := "Name"
	if m.sortMode == 1 {
		sortLabel = "Date"
	}
	rows = append(rows, HelpStyle.Render("Sort: [Tab] "+sortLabel))
	rows = append(rows, "")

	// File list
	if len(m.availableFiles) == 0 {
		rows = append(rows, HelpStyle.Render("No saved projects found"))
	} else {
		// Show up to 10 files at a time
		startIdx := 0
		endIdx := len(m.availableFiles)

		if len(m.availableFiles) > 10 {
			// Center selection in view
			startIdx = m.selectedFileIndex - 5
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx = startIdx + 10
			if endIdx > len(m.availableFiles) {
				endIdx = len(m.availableFiles)
				startIdx = endIdx - 10
				if startIdx < 0 {
					startIdx = 0
				}
			}
		}

		for i := startIdx; i < endIdx; i++ {
			fileName := m.availableFiles[i]

			// Get file modification date
			filePath := filepath.Join(config.ProjectsDir, fileName+".gruvbox")
			dateStr := ""
			if info, err := os.Stat(filePath); err == nil {
				dateStr = info.ModTime().Format("2006-01-02 15:04")
			}

			// Format: "filename                    2024-12-29 14:30"
			nameWidth := 30
			paddedName := fileName
			if len(fileName) > nameWidth {
				paddedName = fileName[:nameWidth-3] + "..."
			} else {
				paddedName = fileName + strings.Repeat(" ", nameWidth-len(fileName))
			}

			displayLine := paddedName + "  " + dateStr

			// Style based on selection
			if i == m.selectedFileIndex {
				fileStyle := lipgloss.NewStyle().
					Background(ColorSecondary).
					Foreground(ColorBg).
					Bold(true).
					Padding(0, 1)
				rows = append(rows, fileStyle.Render("► "+displayLine))
			} else {
				nameStyle := lipgloss.NewStyle().Foreground(ColorFg)
				dateStyle := lipgloss.NewStyle().Foreground(ColorDim)
				rows = append(rows, "  "+nameStyle.Render(paddedName)+"  "+dateStyle.Render(dateStr))
			}
		}
	}

	rows = append(rows, "")

	// Instructions
	if m.saveLoadMode == 1 {
		rows = append(rows, HelpStyle.Render("[↑/↓] Navigate  [Type] Edit Name  [Enter] Save  [Esc] Cancel"))
	} else {
		rows = append(rows, HelpStyle.Render("[↑/↓] Navigate  [Enter] Load  [d] Delete  [Esc] Cancel"))
	}

	// Box it up nicely
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(1, 2)

	browserContent := boxStyle.Render(content)

	// Show delete confirmation overlay if active
	if m.deleteConfirmMode {
		confirmOverlay := m.renderDeleteConfirmation()
		// Place confirmation overlay on top of browser
		return lipgloss.Place(
			lipgloss.Width(browserContent),
			lipgloss.Height(browserContent),
			lipgloss.Center,
			lipgloss.Center,
			confirmOverlay,
		)
	}

	return browserContent
}

// renderDeleteConfirmation renders the delete confirmation dialog
func (m Model) renderDeleteConfirmation() string {
	var rows []string

	rows = append(rows, TitleStyle.Render("⚠ DELETE PROJECT"))
	rows = append(rows, "")
	rows = append(rows, lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true).
		Render("Are you sure you want to delete:"))
	rows = append(rows, "")
	rows = append(rows, lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render(m.deleteConfirmProject))
	rows = append(rows, "")
	rows = append(rows, HelpStyle.Render("This action cannot be undone!"))
	rows = append(rows, "")
	rows = append(rows, HelpStyle.Render("[Y] Yes, delete  [N] No, cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarning).
		Background(ColorBg).
		Padding(1, 2)

	return boxStyle.Render(content)
}

// renderHelp renders the help screen with all keybindings in two columns
func (m Model) renderHelp() string {
	// Left column sections
	var leftCol []string

	// Global
	leftCol = append(leftCol, KeyStyle.Render("GLOBAL"))
	leftCol = append(leftCol, "  ?           - Show/hide help")
	leftCol = append(leftCol, "  Ctrl+Q      - Quit")
	leftCol = append(leftCol, "  Ctrl+S      - Save project")
	leftCol = append(leftCol, "  Ctrl+L      - Load project")
	leftCol = append(leftCol, "  Tab         - Smart navigate (seq→sound→fx)")
	leftCol = append(leftCol, "  Esc         - Return to sequencer")
	leftCol = append(leftCol, "  m/s/e/a/d   - Mixer/Song/Effects/Sampler/Drone")
	leftCol = append(leftCol, "  `           - Play/Stop")
	leftCol = append(leftCol, "  +/-         - Adjust BPM")
	leftCol = append(leftCol, "")

	// Sequencer View
	leftCol = append(leftCol, KeyStyle.Render("SEQUENCER VIEW"))
	leftCol = append(leftCol, "  Arrow Keys  - Navigate grid")
	leftCol = append(leftCol, "  Space       - Toggle step")
	leftCol = append(leftCol, "  Shift+A     - Toggle accent")
	leftCol = append(leftCol, "  Shift+L     - Toggle slide (303)")
	leftCol = append(leftCol, "  Shift+M     - Mute track")
	leftCol = append(leftCol, "  t / T       - Transpose ±1 oct (303)")
	leftCol = append(leftCol, "  backspace   - Clear track")
	leftCol = append(leftCol, "  o           - Solo track")
	leftCol = append(leftCol, "  , / .       - Nudge pattern L/R")
	leftCol = append(leftCol, "  [ / ]       - Switch patterns")
	leftCol = append(leftCol, "  { / }       - Pattern length")
	leftCol = append(leftCol, "  Shift+C     - Copy pattern")
	leftCol = append(leftCol, "  Shift+P     - Save sound")
	leftCol = append(leftCol, "  x           - P-lock generator")
	leftCol = append(leftCol, "")

	// Pattern Generators
	leftCol = append(leftCol, KeyStyle.Render("PATTERN GENERATORS"))
	leftCol = append(leftCol, "  N           - Note entry (303)")
	leftCol = append(leftCol, "    Z-M       - Play notes")
	leftCol = append(leftCol, "    Q/W       - Octave ±")
	leftCol = append(leftCol, "  G           - Dense bassline")
	leftCol = append(leftCol, "  Shift+G     - Sparse bassline")
	leftCol = append(leftCol, "  Shift+R     - Random sound")
	leftCol = append(leftCol, "  Shift+D     - Drum patterns")
	leftCol = append(leftCol, "  Shift+F     - Cycle fills")
	leftCol = append(leftCol, "  Shift+B     - Cycle buildups")

	// Right column sections
	var rightCol []string

	// Synth View
	rightCol = append(rightCol, KeyStyle.Render("SYNTH VIEW (TB-303)"))
	rightCol = append(rightCol, "  Left/Right  - Switch 303 #1/#2")
	rightCol = append(rightCol, "  Up/Down     - Select param")
	rightCol = append(rightCol, "  < / >       - Adjust value")
	rightCol = append(rightCol, "  W           - Toggle waveform")
	rightCol = append(rightCol, "  R           - Randomize")
	rightCol = append(rightCol, "  l           - Lock param to step")
	rightCol = append(rightCol, "  Shift+P     - P-lock generator")
	rightCol = append(rightCol, "")

	// Drums View
	rightCol = append(rightCol, KeyStyle.Render("DRUMS VIEW"))
	rightCol = append(rightCol, "  1-4         - Select drum")
	rightCol = append(rightCol, "  Up/Down     - Select param")
	rightCol = append(rightCol, "  < / >       - Adjust value")
	rightCol = append(rightCol, "  R           - Randomize")
	rightCol = append(rightCol, "  l           - Lock param to step")
	rightCol = append(rightCol, "  Shift+P     - P-lock generator")
	rightCol = append(rightCol, "")

	// Drone View
	rightCol = append(rightCol, KeyStyle.Render("DRONE VIEW"))
	rightCol = append(rightCol, "  Up/Down     - Select param")
	rightCol = append(rightCol, "  < / >       - Adjust value")
	rightCol = append(rightCol, "  R           - Randomize")
	rightCol = append(rightCol, "  l           - Lock param to step")
	rightCol = append(rightCol, "  Shift+P     - P-lock generator")
	rightCol = append(rightCol, "  d           - Go to Drone")
	rightCol = append(rightCol, "")

	// Effects View
	rightCol = append(rightCol, KeyStyle.Render("EFFECTS VIEW"))
	rightCol = append(rightCol, "  1-6         - Select channel")
	rightCol = append(rightCol, "  Left/Right  - Select effect")
	rightCol = append(rightCol, "  Up/Down     - Navigate params")
	rightCol = append(rightCol, "  < / >       - Adjust value")
	rightCol = append(rightCol, "  Shift+E     - Toggle effect")
	rightCol = append(rightCol, "")

	// Mixer View
	rightCol = append(rightCol, KeyStyle.Render("MIXER VIEW"))
	rightCol = append(rightCol, "  1-6         - Select channel")
	rightCol = append(rightCol, "  Up/Down     - Navigate params")
	rightCol = append(rightCol, "  < / >       - Adjust level/pan")
	rightCol = append(rightCol, "  m           - Mute (pattern)")
	rightCol = append(rightCol, "  M           - Mute (4-bar)")
	rightCol = append(rightCol, "  s           - Solo (pattern)")
	rightCol = append(rightCol, "  S           - Solo (4-bar)")

	// Build columns
	leftContent := lipgloss.JoinVertical(lipgloss.Left, leftCol...)
	rightContent := lipgloss.JoinVertical(lipgloss.Left, rightCol...)

	// Join columns horizontally
	columns := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftContent,
		"    ", // Spacing between columns
		rightContent,
	)

	// Title and footer
	title := TitleStyle.Render("GRUVBOX KEYBINDINGS")
	footer := HelpStyle.Render("Press ? to close")

	// Combine all parts
	allSections := []string{
		title,
		"",
		columns,
		"",
		footer,
	}

	content := lipgloss.JoinVertical(lipgloss.Left, allSections...)

	// Box it up nicely
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(1, 2)

	return boxStyle.Render(content)
}

// ============================================================================
// STEP CONDITION EDITOR
// ============================================================================

func (m Model) openConditionEditor() Model {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil {
		return m
	}

	step := track.Pattern.GetStep(m.cursorX)
	if step == nil || !step.Active {
		m.statusMessage = "Select an active step to edit condition"
		m.statusDuration = 60
		return m
	}

	m.conditionEditMode = true
	m.conditionEditStep = step
	m.conditionEditSelection = int(step.Condition)
	m.conditionEditParameter = step.ConditionParameter
	if m.conditionEditParameter == 0 {
		m.conditionEditParameter = 4 // Default to every 4th
	}

	return m
}

func (m Model) handleConditionEditorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.conditionEditMode = false
		return m, nil

	case "enter":
		// Apply condition
		m.conditionEditStep.Condition = sequencer.StepCondition(m.conditionEditSelection)
		m.conditionEditStep.ConditionParameter = m.conditionEditParameter
		m.conditionEditMode = false
		return m, nil

	case "up":
		m.conditionEditSelection--
		if m.conditionEditSelection < 0 {
			m.conditionEditSelection = int(sequencer.ConditionEvenLoops)
		}
		return m, nil

	case "down":
		m.conditionEditSelection++
		if m.conditionEditSelection > int(sequencer.ConditionEvenLoops) {
			m.conditionEditSelection = 0
		}
		return m, nil

	case ",", "<":
		// Decrease parameter (for EVERY_NTH)
		if m.conditionEditSelection == int(sequencer.ConditionEveryNth) {
			m.conditionEditParameter--
			if m.conditionEditParameter < 2 {
				m.conditionEditParameter = 2
			}
		}
		return m, nil

	case ".", ">":
		// Increase parameter (for EVERY_NTH)
		if m.conditionEditSelection == int(sequencer.ConditionEveryNth) {
			m.conditionEditParameter++
			if m.conditionEditParameter > 16 {
				m.conditionEditParameter = 16
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) renderConditionEditor() string {
	var rows []string

	rows = append(rows, TitleStyle.Render("STEP CONDITION"))
	rows = append(rows, "")
	rows = append(rows, HelpStyle.Render(fmt.Sprintf("Step %d", m.cursorX+1)))
	rows = append(rows, "")

	// List conditions
	conditions := []sequencer.StepCondition{
		sequencer.ConditionAlways,
		sequencer.ConditionFirstLoop,
		sequencer.ConditionNotFirstLoop,
		sequencer.ConditionEveryNth,
		sequencer.ConditionOddLoops,
		sequencer.ConditionEvenLoops,
	}

	for i, cond := range conditions {
		line := "  "
		if i == m.conditionEditSelection {
			line = "▶ "
		}

		line += cond.String()

		// Show parameter if EVERY_NTH
		if cond == sequencer.ConditionEveryNth {
			line += fmt.Sprintf(" (N=%d)", m.conditionEditParameter)
		}

		if i == m.conditionEditSelection {
			line = SelectedStepStyle.Render(line)
		}

		rows = append(rows, line)
	}

	rows = append(rows, "")
	rows = append(rows, HelpStyle.Render("[↑↓] Select  [</>] Adjust N  [Enter] Apply  [Esc] Cancel"))

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

// ============================================================================
// EUCLIDEAN RHYTHM GENERATOR
// ============================================================================

func (m Model) openEuclideanGenerator() Model {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil {
		return m
	}

	m.euclideanMode = true
	m.euclideanPulses = track.Pattern.Length / 4 // Default: 25% density
	m.euclideanRotation = 0
	m.euclideanEditParam = 0
	m.euclideanReplace = true // Default to replace

	return m
}

func (m Model) handleEuclideanGeneratorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil {
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.euclideanMode = false
		return m, nil

	case "enter":
		// Apply Euclidean pattern
		if m.euclideanReplace {
			track.Pattern.ApplyEuclideanClear(m.euclideanPulses, m.euclideanRotation, 0.8)
		} else {
			track.Pattern.ApplyEuclidean(m.euclideanPulses, m.euclideanRotation, 0.8)
		}
		m.euclideanMode = false
		return m, nil

	case "up":
		// Navigate to previous parameter
		m.euclideanEditParam--
		if m.euclideanEditParam < 0 {
			m.euclideanEditParam = 1
		}
		return m, nil

	case "down":
		// Navigate to next parameter
		m.euclideanEditParam++
		if m.euclideanEditParam > 1 {
			m.euclideanEditParam = 0
		}
		return m, nil

	case ",", "<":
		// Decrease selected parameter
		if m.euclideanEditParam == 0 {
			// Adjust pulses
			m.euclideanPulses--
			if m.euclideanPulses < 1 {
				m.euclideanPulses = 1
			}
		} else {
			// Adjust rotation
			m.euclideanRotation--
			if m.euclideanRotation < 0 {
				m.euclideanRotation = track.Pattern.Length - 1
			}
		}
		return m, nil

	case ".", ">":
		// Increase selected parameter
		if m.euclideanEditParam == 0 {
			// Adjust pulses
			m.euclideanPulses++
			if m.euclideanPulses > track.Pattern.Length {
				m.euclideanPulses = track.Pattern.Length
			}
		} else {
			// Adjust rotation
			m.euclideanRotation++
			if m.euclideanRotation >= track.Pattern.Length {
				m.euclideanRotation = 0
			}
		}
		return m, nil

	case "r":
		// Toggle replace/layer mode
		m.euclideanReplace = !m.euclideanReplace
		return m, nil
	}

	return m, nil
}

func (m Model) renderEuclideanGenerator() string {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil {
		return ""
	}

	var rows []string

	rows = append(rows, TitleStyle.Render("EUCLIDEAN RHYTHM GENERATOR"))
	rows = append(rows, "")
	rows = append(rows, HelpStyle.Render(fmt.Sprintf("Track: %s  Length: %d steps", trackNames[m.selectedTrack], track.Pattern.Length)))
	rows = append(rows, "")

	// Pulses parameter
	pulsesLabel := fmt.Sprintf("Pulses:   %2d", m.euclideanPulses)
	if m.euclideanEditParam == 0 {
		rows = append(rows, SelectedStepStyle.Render("▶ "+pulsesLabel))
	} else {
		rows = append(rows, "  "+pulsesLabel)
	}

	// Rotation parameter
	rotationLabel := fmt.Sprintf("Rotation: %2d", m.euclideanRotation)
	if m.euclideanEditParam == 1 {
		rows = append(rows, SelectedStepStyle.Render("▶ "+rotationLabel))
	} else {
		rows = append(rows, "  "+rotationLabel)
	}

	rows = append(rows, "")

	// Mode indicator
	modeText := "Replace pattern"
	if !m.euclideanReplace {
		modeText = "Layer on pattern"
	}
	rows = append(rows, HelpStyle.Render("Mode: "+modeText))

	rows = append(rows, "")

	// Preview pattern
	rows = append(rows, "Preview:")
	preview := ""
	indices := sequencer.GenerateEuclidean(m.euclideanPulses, track.Pattern.Length)
	indexMap := make(map[int]bool)
	for _, idx := range indices {
		rotatedIdx := (idx + m.euclideanRotation) % track.Pattern.Length
		indexMap[rotatedIdx] = true
	}

	for i := 0; i < track.Pattern.Length; i++ {
		if indexMap[i] {
			preview += "█"
		} else {
			preview += "░"
		}
		if (i+1)%4 == 0 && i < track.Pattern.Length-1 {
			preview += " " // Add space every 4 steps for readability
		}
	}
	rows = append(rows, preview)

	rows = append(rows, "")
	rows = append(rows, HelpStyle.Render("[↑↓] Select  [</>] Adjust  [R] Toggle Mode  [Enter] Apply  [Esc] Cancel"))

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

// openLengthEditor opens the pattern length editor for the current track
func (m Model) openLengthEditor() Model {
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	if track == nil || track.Pattern == nil {
		m.statusMessage = "No track selected"
		m.statusDuration = 60
		return m
	}

	m.lengthEditMode = true
	m.lengthEditValue = track.Pattern.Length

	return m
}

// handleLengthEditorKeys handles keyboard input in the length editor
func (m Model) handleLengthEditorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.lengthEditMode = false
		return m, nil

	case "enter":
		// Apply the new length
		instrument := trackInstruments[m.selectedTrack]
		track := m.seq.GetTrack(instrument)
		if track != nil && track.Pattern != nil {
			track.Pattern.SetLength(m.lengthEditValue)
			// Update clock to accommodate new length if needed
			m.seq.UpdateClockLength()
		}
		m.lengthEditMode = false
		return m, nil

	case "up":
		m.lengthEditValue++
		if m.lengthEditValue > 128 {
			m.lengthEditValue = 128
		}
		return m, nil

	case "down":
		m.lengthEditValue--
		if m.lengthEditValue < 1 {
			m.lengthEditValue = 1
		}
		return m, nil

	case "pageup":
		m.lengthEditValue += 4
		if m.lengthEditValue > 128 {
			m.lengthEditValue = 128
		}
		return m, nil

	case "pagedown":
		m.lengthEditValue -= 4
		if m.lengthEditValue < 1 {
			m.lengthEditValue = 1
		}
		return m, nil
	}

	return m, nil
}

// handlePLockGenMenuKeys handles key input for p-lock generator menu
func (m Model) handlePLockGenMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Filter generators by category based on current view or selected track
	category := ""
	switch m.currentView {
	case ViewSynth, ViewDrone:
		category = "synth"
	case ViewDrums:
		category = "drum"
	case ViewSequencer:
		// Determine category from selected track
		track := m.selectedTrack
		if track >= 0 && track < len(trackInstruments) {
			instrument := trackInstruments[track]
			switch instrument {
			case sequencer.InstrumentTB303_1, sequencer.InstrumentTB303_2, sequencer.InstrumentDrone:
				category = "synth"
			case sequencer.InstrumentKick, sequencer.InstrumentSnare, sequencer.InstrumentHihatClosed,
				sequencer.InstrumentHihatOpen, sequencer.InstrumentClap, sequencer.InstrumentRimshot,
				sequencer.InstrumentCrash, sequencer.InstrumentTom:
				category = "drum"
			}
		}
	}

	// Get filtered generators
	generators := []PLockGenerator{}
	for _, gen := range PLockGeneratorRegistry {
		if gen.Category == "universal" || gen.Category == category {
			generators = append(generators, gen)
		}
	}

	if len(generators) == 0 {
		m.plockGenMenuOpen = false
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.plockGenMenuOpen = false
		return m, nil

	case "up":
		m.plockGenSelectedIndex--
		if m.plockGenSelectedIndex < 0 {
			m.plockGenSelectedIndex = len(generators) - 1
		}
		return m, nil

	case "down":
		m.plockGenSelectedIndex++
		if m.plockGenSelectedIndex >= len(generators) {
			m.plockGenSelectedIndex = 0
		}
		return m, nil

	case "enter":
		// Apply selected generator
		if m.plockGenSelectedIndex >= 0 && m.plockGenSelectedIndex < len(generators) {
			m.applyPLockGenerator(generators[m.plockGenSelectedIndex])
		}
		m.plockGenMenuOpen = false
		return m, nil
	}

	return m, nil
}

// handleParamSelectMenuKeys handles key input for parameter selection menu (from sequencer view)
func (m Model) handleParamSelectMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Get available parameters for current track
	params := m.getParamsForTrack(m.selectedTrack)

	if len(params) == 0 {
		m.paramSelectMenuOpen = false
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.paramSelectMenuOpen = false
		m.paramSelectForGenParam = ""
		return m, nil

	case "up":
		m.paramSelectIndex--
		if m.paramSelectIndex < 0 {
			m.paramSelectIndex = len(params) - 1
		}
		return m, nil

	case "down":
		m.paramSelectIndex++
		if m.paramSelectIndex >= len(params) {
			m.paramSelectIndex = 0
		}
		return m, nil

	case "enter":
		// Store selected parameter and open generator menu
		if m.paramSelectIndex >= 0 && m.paramSelectIndex < len(params) {
			m.paramSelectForGenParam = params[m.paramSelectIndex]
			m.paramSelectMenuOpen = false
			m.plockGenMenuOpen = true
			m.plockGenSelectedIndex = 0
		}
		return m, nil
	}

	return m, nil
}

// getParamsForTrack returns a list of parameter names for a given track
func (m Model) getParamsForTrack(track int) []string {
	switch track {
	case 0, 1: // TB-303 #1 and #2
		return []string{
			"tb303_cutoff", "tb303_resonance", "tb303_envmod", "tb303_decay",
			"tb303_accent", "tb303_volume", "tb303_wavemix", "tb303_sub", "tb303_drive",
		}
	case 2: // Kick
		return []string{
			"kick_pitch", "kick_decay", "kick_drive", "kick_tone", "kick_click", "kick_volume",
		}
	case 3: // Snare
		return []string{
			"snare_pitch", "snare_decay", "snare_snap", "snare_tone", "snare_drive", "snare_volume",
		}
	case 4: // HH Closed
		return []string{
			"hhclosed_pitch", "hhclosed_decay", "hhclosed_tone", "hhclosed_volume",
		}
	case 5: // HH Open
		return []string{
			"hhopen_pitch", "hhopen_decay", "hhopen_tone", "hhopen_volume",
		}
	case 6: // Clap
		return []string{
			"clap_pitch", "clap_decay", "clap_density", "clap_tone", "clap_volume",
		}
	case 7: // Rimshot
		return []string{
			"rimshot_pitch", "rimshot_decay", "rimshot_snap", "rimshot_tone", "rimshot_volume",
		}
	case 8: // Crash
		return []string{
			"crash_pitch", "crash_decay", "crash_tone", "crash_volume",
		}
	case 9: // Tom
		return []string{
			"tom_pitch", "tom_decay", "tom_tone", "tom_volume",
		}
	case 12: // Drone
		return []string{
			"drone_volume", "drone_pan", "drone_pitch", "drone_detune",
			"drone_wavemix", "drone_sub", "drone_noise",
			"drone_cutoff", "drone_resonance", "drone_envmod", "drone_decay",
			"drone_lfo1rate", "drone_lfo1depth", "drone_lfo1dest",
			"drone_lfo2rate", "drone_lfo2depth", "drone_lfo2dest",
		}
	default:
		return []string{}
	}
}

// getParameterValue gets the current value of a parameter from the engine
func (m *Model) getParameterValue(instrument sequencer.InstrumentType, paramName string) float32 {
	switch instrument {
	case sequencer.InstrumentTB303_1:
		return m.getTB303ParameterValue(m.engine.GetTB303(), paramName)
	case sequencer.InstrumentTB303_2:
		return m.getTB303ParameterValue(m.engine.GetTB303_2(), paramName)
	case sequencer.InstrumentKick:
		return m.getKickParameterValue(paramName)
	case sequencer.InstrumentSnare:
		return m.getSnareParameterValue(paramName)
	case sequencer.InstrumentHihatClosed:
		return m.getHihatClosedParameterValue(paramName)
	case sequencer.InstrumentHihatOpen:
		return m.getHihatOpenParameterValue(paramName)
	case sequencer.InstrumentClap:
		return m.getClapParameterValue(paramName)
	case sequencer.InstrumentDrone:
		return m.getDroneParameterValue(paramName)
	}
	return 0.5 // Default fallback
}

// Helper functions to get parameter values from specific instruments
func (m *Model) getTB303ParameterValue(tb *synth.TB303, paramName string) float32 {
	switch paramName {
	case "tb303_cutoff":
		return tb.GetCutoff()
	case "tb303_resonance":
		return tb.GetResonance()
	case "tb303_envmod":
		return tb.GetEnvMod()
	case "tb303_decay":
		return tb.GetDecay() / 2.0 // Normalize from 0-2 to 0-1
	case "tb303_accent":
		return tb.GetAccent()
	case "tb303_slidetime":
		return (tb.GetSlideTime() - 0.01) / 0.49 // Normalize from 0.01-0.5 to 0-1
	case "tb303_sublevel":
		return tb.GetSubLevel()
	}
	return 0.5
}

func (m *Model) getKickParameterValue(paramName string) float32 {
	kick := m.engine.GetKick()
	switch paramName {
	case "kick_tune":
		return kick.GetTune()
	case "kick_decay":
		return kick.GetDecay()
	case "kick_pitchdecay":
		return kick.GetPitchDecay()
	case "kick_drive":
		return kick.GetDrive()
	case "kick_click":
		return kick.GetClickAmount()
	}
	return 0.5
}

func (m *Model) getSnareParameterValue(paramName string) float32 {
	snare := m.engine.GetSnare()
	switch paramName {
	case "snare_tune":
		return snare.GetTune()
	case "snare_snap":
		return snare.GetSnap()
	case "snare_tonedecay":
		return snare.GetToneDecay()
	case "snare_noisedecay":
		return snare.GetNoiseDecay()
	case "snare_tonemix":
		return snare.GetToneMix()
	}
	return 0.5
}

func (m *Model) getHihatClosedParameterValue(paramName string) float32 {
	hh := m.engine.GetHihatClosed()
	switch paramName {
	case "hhclosed_decay":
		return hh.GetDecay()
	case "hhclosed_tone":
		return hh.GetTone()
	case "hhclosed_noisemix":
		return hh.GetNoiseMix()
	}
	return 0.5
}

func (m *Model) getHihatOpenParameterValue(paramName string) float32 {
	hh := m.engine.GetHihatOpen()
	switch paramName {
	case "hhopen_decay":
		return hh.GetDecay()
	case "hhopen_tone":
		return hh.GetTone()
	case "hhopen_noisemix":
		return hh.GetNoiseMix()
	}
	return 0.5
}

func (m *Model) getClapParameterValue(paramName string) float32 {
	clap := m.engine.GetClap()
	switch paramName {
	case "clap_tune":
		return clap.GetTune()
	case "clap_decay":
		return clap.GetDecay()
	case "clap_tone":
		return clap.GetTone()
	}
	return 0.5
}

func (m *Model) getDroneParameterValue(paramName string) float32 {
	drone := m.engine.GetDrone()
	switch paramName {
	case "drone_cutoff":
		return drone.GetCutoff()
	case "drone_resonance":
		return drone.GetResonance()
	case "drone_envmod":
		return drone.GetEnvMod()
	}
	return 0.5
}

// applyPLockGenerator applies a generator to the current parameter across the pattern
func (m *Model) applyPLockGenerator(gen PLockGenerator) {
	// Get current track and pattern
	track := m.selectedTrack

	// Convert track to instrument using the trackInstruments mapping
	if track < 0 || track >= len(trackInstruments) {
		return
	}
	instrument := trackInstruments[track]

	// Get pattern ID - use playback pattern if in song mode, otherwise use edit pattern
	var patternID int
	if m.seq.IsSongMode() {
		// In song mode, apply p-locks to the pattern currently being played
		patternID = m.seq.GetSongCurrentPatternID()
	} else {
		// In pattern mode, apply p-locks to the edit pattern
		patternID = m.seq.GetCurrentPatternID()
	}

	pattern := m.seq.GetPattern(instrument, patternID)
	if pattern == nil {
		return
	}

	// Determine parameter name
	var paramName string
	var ok bool

	// If coming from sequencer view (parameter was pre-selected), use stored param
	if m.paramSelectForGenParam != "" {
		paramName = m.paramSelectForGenParam
		ok = true
	} else {
		// Otherwise, get from current view's selected parameter
		switch m.currentView {
		case ViewSynth:
			paramName, _, ok = m.getSynthParamNameFromIndex(m.selectedParam)
		case ViewDrone:
			paramName, _, ok = m.getDroneParamNameFromIndex(m.selectedParam)
		case ViewDrums:
			paramName, _, ok = m.getDrumParamNameFromIndex(m.selectedDrum, m.selectedDrumParam)
		}
	}

	if !ok || paramName == "" {
		return
	}

	// Get current parameter value from the instrument for humanize generators
	// Store it in SoundParams so generators can access it
	baseValue := m.getParameterValue(instrument, paramName)
	if pattern.SoundParams == nil {
		pattern.SoundParams = make(map[string]float32)
	}
	pattern.SoundParams["_base_"+paramName] = baseValue

	// Apply generator to all steps (0 to pattern length - 1)
	patternLength := pattern.Length
	gen.Generate(pattern, paramName, 0, patternLength-1)

	// Clean up temporary base value
	delete(pattern.SoundParams, "_base_"+paramName)

	// Clear stored parameter after use
	m.paramSelectForGenParam = ""
}

// renderLengthEditor renders the pattern length editor modal
func (m Model) renderLengthEditor() string {
	var rows []string

	rows = append(rows, TitleStyle.Render("PATTERN LENGTH"))
	rows = append(rows, "")

	trackName := trackNames[m.selectedTrack]
	rows = append(rows, HelpStyle.Render(fmt.Sprintf("Track: %s", trackName)))
	rows = append(rows, "")

	// Show current length value (large and centered)
	lengthStr := fmt.Sprintf("%d steps", m.lengthEditValue)
	lengthStyle := lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true).
		Width(20).
		Align(lipgloss.Center)
	rows = append(rows, lengthStyle.Render(lengthStr))
	rows = append(rows, "")

	// Show common lengths for quick reference
	commonLengths := []int{4, 7, 8, 12, 13, 16, 24, 32, 48, 64, 128}
	commonRow := "Common: "
	for _, l := range commonLengths {
		if l == m.lengthEditValue {
			commonRow += SelectedStepStyle.Render(fmt.Sprintf("[%d]", l)) + " "
		} else {
			commonRow += fmt.Sprintf(" %d ", l)
		}
	}
	rows = append(rows, HelpStyle.Render(commonRow))

	rows = append(rows, "")
	rows = append(rows, HelpStyle.Render("[↑↓] Adjust (±1)  [PgUp/PgDn] Adjust (±4)"))
	rows = append(rows, HelpStyle.Render("[Enter] Apply  [Esc] Cancel"))

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

// renderPLockGenMenu renders the p-lock generator selection menu
func (m Model) renderPLockGenMenu() string {
	// Filter generators by category based on current view or selected track
	category := ""
	switch m.currentView {
	case ViewSynth, ViewDrone:
		category = "synth"
	case ViewDrums:
		category = "drum"
	case ViewSequencer:
		// Determine category from selected track
		track := m.selectedTrack
		if track >= 0 && track < len(trackInstruments) {
			instrument := trackInstruments[track]
			switch instrument {
			case sequencer.InstrumentTB303_1, sequencer.InstrumentTB303_2, sequencer.InstrumentDrone:
				category = "synth"
			case sequencer.InstrumentKick, sequencer.InstrumentSnare, sequencer.InstrumentHihatClosed,
				sequencer.InstrumentHihatOpen, sequencer.InstrumentClap, sequencer.InstrumentRimshot,
				sequencer.InstrumentCrash, sequencer.InstrumentTom:
				category = "drum"
			}
		}
	}

	// Get filtered generators
	generators := []PLockGenerator{}
	for _, gen := range PLockGeneratorRegistry {
		if gen.Category == "universal" || gen.Category == category {
			generators = append(generators, gen)
		}
	}

	var rows []string

	// Title
	rows = append(rows, TitleStyle.Render("P-LOCK GENERATOR"))
	rows = append(rows, "")

	// Get current parameter name
	var paramName string
	var ok bool

	switch m.currentView {
	case ViewSynth:
		paramName, _, ok = m.getSynthParamNameFromIndex(m.selectedParam)
	case ViewDrone:
		paramName, _, ok = m.getDroneParamNameFromIndex(m.selectedParam)
	case ViewDrums:
		paramName, _, ok = m.getDrumParamNameFromIndex(m.selectedDrum, m.selectedDrumParam)
	}

	if !ok {
		paramName = ""
	}

	// Show current parameter (strip prefix for display)
	displayParam := paramName
	if idx := strings.Index(paramName, "_"); idx >= 0 {
		displayParam = paramName[idx+1:]
	}
	rows = append(rows, HelpStyle.Render(fmt.Sprintf("Parameter: %s", displayParam)))
	rows = append(rows, "")

	// Ensure selected index is valid
	if m.plockGenSelectedIndex >= len(generators) {
		m.plockGenSelectedIndex = 0
	}

	// Show generators list
	for i, gen := range generators {
		genStyle := lipgloss.NewStyle().Padding(0, 1)

		if i == m.plockGenSelectedIndex {
			// Highlight selected generator
			genStyle = genStyle.
				Foreground(ColorAccent).
				Background(ColorSecondary).
				Bold(true)
		}

		genLine := fmt.Sprintf("%s", gen.Name)
		rows = append(rows, genStyle.Render(genLine))

		// Show description for selected generator
		if i == m.plockGenSelectedIndex {
			descStyle := lipgloss.NewStyle().
				Foreground(ColorDim).
				Padding(0, 2)
			rows = append(rows, descStyle.Render(gen.Description))
		}
	}

	rows = append(rows, "")
	rows = append(rows, HelpStyle.Render("[↑↓] Select  [Enter] Apply  [Esc] Cancel"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(1, 2).
		Width(50)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		boxStyle.Render(strings.Join(rows, "\n")),
	)
}

// renderParamSelectMenu renders the parameter selection menu (for sequencer view)
func (m Model) renderParamSelectMenu() string {
	var rows []string

	// Title
	rows = append(rows, TitleStyle.Render("SELECT PARAMETER"))
	rows = append(rows, "")

	// Get track name
	trackName := trackNames[m.selectedTrack]
	rows = append(rows, HelpStyle.Render(fmt.Sprintf("Track: %s", trackName)))
	rows = append(rows, "")

	// Get available parameters
	params := m.getParamsForTrack(m.selectedTrack)

	if len(params) == 0 {
		rows = append(rows, HelpStyle.Render("No parameters available for this track"))
		rows = append(rows, "")
		rows = append(rows, HelpStyle.Render("[Esc] Cancel"))
	} else {
		// Ensure selected index is valid
		if m.paramSelectIndex >= len(params) {
			m.paramSelectIndex = 0
		}

		// Show parameters list
		for i, param := range params {
			// Strip prefix for display
			displayParam := param
			if idx := strings.Index(param, "_"); idx >= 0 {
				displayParam = param[idx+1:]
			}

			paramStyle := lipgloss.NewStyle().Padding(0, 1)

			if i == m.paramSelectIndex {
				// Highlight selected parameter
				paramStyle = paramStyle.
					Foreground(ColorAccent).
					Background(ColorSecondary).
					Bold(true)
			}

			rows = append(rows, paramStyle.Render(displayParam))
		}

		rows = append(rows, "")
		rows = append(rows, HelpStyle.Render("[↑↓] Select  [Enter] Continue to Generator  [Esc] Cancel"))
	}

	boxStyle2 := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(1, 2).
		Width(50)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		boxStyle2.Render(strings.Join(rows, "\n")),
	)
}

// handleDrumPatternMenuKeys handles key input for drum pattern menu
func (m Model) handleDrumPatternMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Filter patterns by current instrument
	instrument := trackInstruments[m.selectedTrack]
	filteredPatterns := []DrumPattern{}
	for _, pattern := range DrumPatternRegistry {
		if pattern.InstrumentType == instrument {
			filteredPatterns = append(filteredPatterns, pattern)
		}
	}

	switch msg.String() {
	case "esc":
		m.drumPatternMenuOpen = false
		return m, nil

	case "up":
		m.drumPatternSelectedIdx--
		if m.drumPatternSelectedIdx < 0 {
			m.drumPatternSelectedIdx = len(filteredPatterns) - 1
		}
		return m, nil

	case "down":
		m.drumPatternSelectedIdx++
		if m.drumPatternSelectedIdx >= len(filteredPatterns) {
			m.drumPatternSelectedIdx = 0
		}
		return m, nil

	case "enter":
		// Apply selected pattern to currently selected track
		if m.drumPatternSelectedIdx >= 0 && m.drumPatternSelectedIdx < len(filteredPatterns) {
			m.applyDrumPattern(filteredPatterns[m.drumPatternSelectedIdx])
		}
		m.drumPatternMenuOpen = false
		return m, nil
	}

	return m, nil
}

// applyDrumPattern applies a drum pattern to the currently selected track
func (m *Model) applyDrumPattern(pattern DrumPattern) {
	// Get currently selected track
	track := m.selectedTrack
	if track < 0 || track >= len(trackInstruments) {
		return
	}

	// Get the instrument for this track
	instrument := trackInstruments[track]

	// Get current pattern ID
	patternID := m.seq.GetCurrentPatternID()

	// Get the track and pattern
	instrumentTrack := m.seq.GetTrack(instrument)
	if instrumentTrack == nil || instrumentTrack.Pattern == nil {
		return
	}

	drumPattern := m.seq.GetPattern(instrument, patternID)
	if drumPattern == nil {
		return
	}

	patternLength := drumPattern.Length

	// Clear existing pattern first
	drumPattern.ClearAll()

	// Apply the generator to this track only
	pattern.Generate(drumPattern, patternLength)
}

// renderDrumPatternMenu renders the drum pattern selection menu
func (m Model) renderDrumPatternMenu() string {
	var rows []string

	// Title
	rows = append(rows, TitleStyle.Render("DRUM PATTERNS"))
	rows = append(rows, "")

	// Show which track we're applying to
	trackName := trackNames[m.selectedTrack]
	rows = append(rows, HelpStyle.Render(fmt.Sprintf("Applying to: %s", trackName)))

	// Get current pattern length from selected track
	instrument := trackInstruments[m.selectedTrack]
	track := m.seq.GetTrack(instrument)
	patternLength := 16
	if track != nil && track.Pattern != nil {
		patternLength = track.Pattern.Length
	}
	rows = append(rows, HelpStyle.Render(fmt.Sprintf("Pattern Length: %d steps", patternLength)))
	rows = append(rows, "")

	// Filter patterns by instrument type
	filteredPatterns := []DrumPattern{}
	for _, pattern := range DrumPatternRegistry {
		if pattern.InstrumentType == instrument {
			filteredPatterns = append(filteredPatterns, pattern)
		}
	}

	if len(filteredPatterns) == 0 {
		rows = append(rows, HelpStyle.Render("No patterns available for this instrument"))
		rows = append(rows, "")
		rows = append(rows, HelpStyle.Render("[Esc] Close"))

		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSecondary).
			Padding(1, 2).
			Width(60)

		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			boxStyle.Render(strings.Join(rows, "\n")),
		)
	}

	// Ensure selected index is valid
	if m.drumPatternSelectedIdx >= len(filteredPatterns) {
		m.drumPatternSelectedIdx = 0
	}

	// Show pattern list
	for i, pattern := range filteredPatterns {
		patternStyle := lipgloss.NewStyle().Padding(0, 1)

		if i == m.drumPatternSelectedIdx {
			// Highlight selected pattern
			patternStyle = patternStyle.
				Foreground(ColorAccent).
				Background(ColorSecondary).
				Bold(true)
		}

		rows = append(rows, patternStyle.Render(pattern.Name))

		// Show description for selected pattern
		if i == m.drumPatternSelectedIdx {
			descStyle := lipgloss.NewStyle().
				Foreground(ColorDim).
				Padding(0, 2)
			rows = append(rows, descStyle.Render(pattern.Description))
		}
	}

	rows = append(rows, "")
	rows = append(rows, HelpStyle.Render("[↑↓] Select  [Enter] Apply to Track  [Esc] Cancel"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(1, 2).
		Width(60)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		boxStyle.Render(strings.Join(rows, "\n")),
	)
}

// getArpPatternName returns a display name for arp pattern
func (m Model) getArpPatternName(pattern synth.ArpPattern) string {
	switch pattern {
	case synth.ArpPatternUp:
		return "Up"
	case synth.ArpPatternDown:
		return "Down"
	case synth.ArpPatternUpDown:
		return "Up-Down"
	case synth.ArpPatternDownUp:
		return "Down-Up"
	case synth.ArpPatternRandom:
		return "Random"
	case synth.ArpPatternAsPlayed:
		return "As Entered" // Plays notes in the order they were entered
	default:
		return "Unknown"
	}
}

// getArpRateName returns a display name for arp rate
func (m Model) getArpRateName(rate synth.ArpRate) string {
	switch rate {
	case synth.ArpRate1_32:
		return "1/32"
	case synth.ArpRate1_16T:
		return "1/16T"
	case synth.ArpRate1_16:
		return "1/16"
	case synth.ArpRate1_8T:
		return "1/8T"
	case synth.ArpRate1_8:
		return "1/8"
	case synth.ArpRate1_4:
		return "1/4"
	default:
		return "Unknown"
	}
}

// getNoteName converts MIDI note number to note name (e.g. 60 -> C4)
func (m Model) getNoteName(note int) string {
	noteNames := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	octave := (note / 12) - 1
	noteName := noteNames[note%12]
	return fmt.Sprintf("%s%d", noteName, octave)
}
