package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/subbaan/gruvbox/sequencer"
)

// Project represents a complete Gruvbox project
type Project struct {
	// Metadata
	Name      string    `json:"name"`
	Version   string    `json:"version"` // Gruvbox version
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Global settings
	BPM               float32 `json:"bpm"`
	Swing             float32 `json:"swing"`
	CurrentPatternID  int     `json:"current_pattern_id"`
	SongMode          bool    `json:"song_mode"`
	SongLoop          bool    `json:"song_loop"`

	// Pattern bank - 16 patterns per instrument
	Patterns PatternBank `json:"patterns"`

	// Song arrangement
	Song SongArrangement `json:"song"`

	// Instrument settings
	TB303_1  TB303Settings `json:"tb303_1"`
	TB303_2  TB303Settings `json:"tb303_2"`
	Kick     DrumSettings  `json:"kick"`
	Snare    DrumSettings  `json:"snare"`
	HHClosed DrumSettings  `json:"hh_closed"`
	HHOpen   DrumSettings  `json:"hh_open"`
	Clap     DrumSettings  `json:"clap"`

	// Sampler settings
	Sampler1 SamplerSettings `json:"sampler1"`
	Sampler2 SamplerSettings `json:"sampler2"`

	// Drone synth settings
	Drone DroneSettings `json:"drone"`

	// Track settings (mute, solo, transpose)
	Tracks TrackSettings `json:"tracks"`

	// Effects settings (per channel)
	Effects [10]ChannelEffects `json:"effects"` // 10 channels (all instruments)

	// Mixer settings
	Mixer MixerSettings `json:"mixer"`
}

// SamplerSettings stores sampler parameters and sample data
type SamplerSettings struct {
	SampleName string  `json:"sample_name"`
	SamplePath string  `json:"sample_path,omitempty"` // Optional path to external WAV
	TrimStart  int     `json:"trim_start"`
	TrimEnd    int     `json:"trim_end"`
	Volume     float32 `json:"volume"`
}

// PatternBank stores all patterns for all instruments
type PatternBank struct {
	TB303_1  [16]Pattern `json:"tb303_1"`
	TB303_2  [16]Pattern `json:"tb303_2"`
	Kick     [16]Pattern `json:"kick"`
	Snare    [16]Pattern `json:"snare"`
	HHClosed [16]Pattern `json:"hh_closed"`
	HHOpen   [16]Pattern `json:"hh_open"`
	Clap     [16]Pattern `json:"clap"`
	Sampler1 [16]Pattern `json:"sampler1"`
	Sampler2 [16]Pattern `json:"sampler2"`
	Drone    [16]Pattern `json:"drone"`
}

// Pattern represents a single pattern
type Pattern struct {
	Name        string             `json:"name,omitempty"`
	Length      int                `json:"length"`
	Steps       []Step             `json:"steps"`
	SoundParams map[string]float32 `json:"sound_params,omitempty"`
}

// Step represents a single step in a pattern
type Step struct {
	Active             bool               `json:"active"`
	Note               int                `json:"note"`
	Velocity           float32            `json:"velocity"`
	Accent             bool               `json:"accent"`
	Slide              bool               `json:"slide"`
	Probability        float32            `json:"probability"`
	Condition          int                `json:"condition"`
	ConditionParameter int                `json:"condition_parameter"`
	ParamLocks         map[string]float32 `json:"param_locks,omitempty"`
}

// SongArrangement represents the song structure
type SongArrangement struct {
	Steps []SongStep `json:"steps"`
}

// SongStep represents one step in the song arrangement
type SongStep struct {
	PatternID int `json:"pattern_id"`
	Repeats   int `json:"repeats"`
}

// TB303Settings stores TB-303 synth parameters
type TB303Settings struct {
	Waveform  string  `json:"waveform"` // "saw" or "square"
	Cutoff    float32 `json:"cutoff"`
	Resonance float32 `json:"resonance"`
	EnvMod    float32 `json:"env_mod"`
	Decay     float32 `json:"decay"`
	Accent    float32 `json:"accent"`
	SlideTime float32 `json:"slide_time"`
	SubLevel  float32 `json:"sub_level"`
	Tuning    float32 `json:"tuning"`

	// LFO1 settings
	LFO1Enabled       bool    `json:"lfo1_enabled"`
	LFO1Target        string  `json:"lfo1_target"`
	LFO1Waveform      string  `json:"lfo1_waveform"`
	LFO1Rate          float32 `json:"lfo1_rate"`
	LFO1Depth         float32 `json:"lfo1_depth"`
	LFO1Min           float32 `json:"lfo1_min"`
	LFO1Max           float32 `json:"lfo1_max"`
	LFO1TempoSync     bool    `json:"lfo1_tempo_sync"`
	LFO1SyncRate      float32 `json:"lfo1_sync_rate"`
	LFO1SyncMode      int     `json:"lfo1_sync_mode"`       // Time vs Steps mode
	LFO1StepsPerCycle int     `json:"lfo1_steps_per_cycle"` // For step-locked mode

	// LFO2 settings
	LFO2Enabled       bool    `json:"lfo2_enabled"`
	LFO2Target        string  `json:"lfo2_target"`
	LFO2Waveform      string  `json:"lfo2_waveform"`
	LFO2Rate          float32 `json:"lfo2_rate"`
	LFO2Depth         float32 `json:"lfo2_depth"`
	LFO2Min           float32 `json:"lfo2_min"`
	LFO2Max           float32 `json:"lfo2_max"`
	LFO2TempoSync     bool    `json:"lfo2_tempo_sync"`
	LFO2SyncRate      float32 `json:"lfo2_sync_rate"`
	LFO2SyncMode      int     `json:"lfo2_sync_mode"`       // Time vs Steps mode
	LFO2StepsPerCycle int     `json:"lfo2_steps_per_cycle"` // For step-locked mode
}

// DroneSettings stores drone synth parameters
type DroneSettings struct {
	// Master
	MasterVolume float32 `json:"master_volume"`
	MasterTune   int     `json:"master_tune"`

	// Oscillators
	Osc1Level  float32 `json:"osc1_level"`
	Osc2Level  float32 `json:"osc2_level"`
	Osc3Level  float32 `json:"osc3_level"`
	Osc2Detune float32 `json:"osc2_detune"`
	Osc3Detune   float32 `json:"osc3_detune"`
	Sub1Level    float32 `json:"sub1_level"`
	Sub2Level    float32 `json:"sub2_level"`
	NoiseLevel   float32 `json:"noise_level"`
	RingModAmount float32 `json:"ring_mod_amount"`
	DriveAmount   float32 `json:"drive_amount"`

	// Filter
	Cutoff    float32 `json:"cutoff"`
	Resonance float32 `json:"resonance"`
	EnvMod    float32 `json:"env_mod"`

	// Envelope
	Attack  float32 `json:"attack"`
	Decay   float32 `json:"decay"`
	Sustain float32 `json:"sustain"`
	Release float32 `json:"release"`

	// LFO1
	LFO1Enabled       bool    `json:"lfo1_enabled"`
	LFO1Target        string  `json:"lfo1_target"`
	LFO1Waveform      string  `json:"lfo1_waveform"`
	LFO1Rate          float32 `json:"lfo1_rate"`
	LFO1Depth         float32 `json:"lfo1_depth"`
	LFO1Min           float32 `json:"lfo1_min"`
	LFO1Max           float32 `json:"lfo1_max"`
	LFO1TempoSync     bool    `json:"lfo1_tempo_sync"`
	LFO1SyncMode      int     `json:"lfo1_sync_mode"`
	LFO1StepsPerCycle int     `json:"lfo1_steps_per_cycle"`

	// LFO2
	LFO2Enabled       bool    `json:"lfo2_enabled"`
	LFO2Target        string  `json:"lfo2_target"`
	LFO2Waveform      string  `json:"lfo2_waveform"`
	LFO2Rate          float32 `json:"lfo2_rate"`
	LFO2Depth         float32 `json:"lfo2_depth"`
	LFO2Min           float32 `json:"lfo2_min"`
	LFO2Max           float32 `json:"lfo2_max"`
	LFO2TempoSync     bool    `json:"lfo2_tempo_sync"`
	LFO2SyncMode      int     `json:"lfo2_sync_mode"`
	LFO2StepsPerCycle int     `json:"lfo2_steps_per_cycle"`
}

// DrumSettings stores drum synthesis parameters
type DrumSettings struct {
	Pitch  float32 `json:"pitch"`
	Decay  float32 `json:"decay"`
	Tone   float32 `json:"tone"`
	Drive  float32 `json:"drive"`
	Snap   float32 `json:"snap,omitempty"`   // For snare
	Attack float32 `json:"attack,omitempty"` // For kick

	// LFO1 settings
	LFO1Enabled       bool    `json:"lfo1_enabled"`
	LFO1Target        int     `json:"lfo1_target"` // Drum-specific target enum
	LFO1Waveform      int     `json:"lfo1_waveform"`
	LFO1Rate          float32 `json:"lfo1_rate"`
	LFO1Depth         float32 `json:"lfo1_depth"`
	LFO1Min           float32 `json:"lfo1_min"`
	LFO1Max           float32 `json:"lfo1_max"`
	LFO1TempoSync     bool    `json:"lfo1_tempo_sync"`
	LFO1SyncRate      float32 `json:"lfo1_sync_rate"`
	LFO1SyncMode      int     `json:"lfo1_sync_mode"`       // Time vs Steps mode
	LFO1StepsPerCycle int     `json:"lfo1_steps_per_cycle"` // For step-locked mode
}

// ChannelEffects stores effect settings for one channel
type ChannelEffects struct {
	Distortion      EffectSettings `json:"distortion"`
	Delay           EffectSettings `json:"delay"`
	Reverb          EffectSettings `json:"reverb"`
	PitchShifter    EffectSettings `json:"pitch_shifter"`
	MultimodeFilter EffectSettings `json:"multimode_filter"`
	BitCrusher      EffectSettings `json:"bit_crusher"`
}

// EffectSettings stores parameters for one effect
type EffectSettings struct {
	Enabled bool               `json:"enabled"`
	Params  map[string]float32 `json:"params"`
}

// MixerSettings stores mixer channel settings
type MixerSettings struct {
	MasterLevel float32             `json:"master_level"`
	Channels    [10]ChannelSettings `json:"channels"`
}

// TrackSettings stores track-level settings for all instruments
type TrackSettings struct {
	TB303_1  TrackState `json:"tb303_1"`
	TB303_2  TrackState `json:"tb303_2"`
	Kick     TrackState `json:"kick"`
	Snare    TrackState `json:"snare"`
	HHClosed TrackState `json:"hh_closed"`
	HHOpen   TrackState `json:"hh_open"`
	Clap     TrackState `json:"clap"`
	Sampler1 TrackState `json:"sampler1"`
	Sampler2 TrackState `json:"sampler2"`
	Drone    TrackState `json:"drone"`
}

// TrackState stores state for one track
type TrackState struct {
	Mute      bool `json:"mute"`
	Solo      bool `json:"solo"`
	Transpose int  `json:"transpose"`
}

// ChannelSettings stores settings for one mixer channel
type ChannelSettings struct {
	Level float32 `json:"level"`
	Pan   float32 `json:"pan"`
	Mute  bool    `json:"mute"`
	Solo  bool    `json:"solo"`
}

// NewProject creates a new empty project
func NewProject(name string) *Project {
	return &Project{
		Name:             name,
		Version:          "1.2.0",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		BPM:              128.0,
		CurrentPatternID: 0,
		SongMode:         false,
		SongLoop:         true,
	}
}

// Save saves the project to a JSON file
func (p *Project) Save(filename string) error {
	// Update timestamp
	p.UpdatedAt = time.Now()

	// Ensure .gruvbox extension
	if filepath.Ext(filename) != ".gruvbox" {
		filename += ".gruvbox"
	}

	// Create directory if needed
	dir := filepath.Dir(filename)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Load loads a project from a JSON file
func Load(filename string) (*Project, error) {
	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Unmarshal JSON
	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project: %w", err)
	}

	return &project, nil
}

// ConvertSequencerStep converts a sequencer.Step to a project.Step
func ConvertSequencerStep(s sequencer.Step) Step {
	// Deep copy ParamLocks map
	var paramLocks map[string]float32
	if len(s.ParamLocks) > 0 {
		paramLocks = make(map[string]float32, len(s.ParamLocks))
		for k, v := range s.ParamLocks {
			paramLocks[k] = v
		}
	}

	return Step{
		Active:             s.Active,
		Note:               s.Note,
		Velocity:           s.Velocity,
		Accent:             s.Accent,
		Slide:              s.Slide,
		Probability:        s.Probability,
		Condition:          int(s.Condition),
		ConditionParameter: s.ConditionParameter,
		ParamLocks:         paramLocks,
	}
}

// ConvertToSequencerStep converts a project.Step to a sequencer.Step
func ConvertToSequencerStep(s Step) sequencer.Step {
	// Deep copy ParamLocks map
	var paramLocks map[string]float32
	if len(s.ParamLocks) > 0 {
		paramLocks = make(map[string]float32, len(s.ParamLocks))
		for k, v := range s.ParamLocks {
			paramLocks[k] = v
		}
	}

	return sequencer.Step{
		Active:             s.Active,
		Note:               s.Note,
		Velocity:           s.Velocity,
		Accent:             s.Accent,
		Slide:              s.Slide,
		Probability:        s.Probability,
		Condition:          sequencer.StepCondition(s.Condition),
		ConditionParameter: s.ConditionParameter,
		ParamLocks:         paramLocks,
	}
}
