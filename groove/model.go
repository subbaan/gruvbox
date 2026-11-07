// Package groove is Gruvbox 3's renderer-independent project and command model.
package groove

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/subbaan/gruvbox/synth"
	"math"
	"reflect"
)

// FormatVersion 2 has 64 pattern slots and writes blank ones as {}.
// Version 1 files (8 slots) are padded when they are read.
const FormatVersion = 2

// Patterns are 8 banks (A–H) of 8 pads.
const (
	BankSize     = 8
	BankCount    = 8
	PatternCount = BankSize * BankCount
)

// SlotName names a pattern slot: "A1" … "H8".
func SlotName(slot int) string {
	if slot < 0 || slot >= PatternCount {
		return "?"
	}
	return fmt.Sprintf("%c%d", 'A'+slot/BankSize, slot%BankSize+1)
}

// BankOf is the bank (0 = A) that holds slot.
func BankOf(slot int) int { return slot / BankSize }

// NextInBank is the pad after slot in its own bank, wrapping to the first.
func NextInBank(slot int) int { return BankOf(slot)*BankSize + (slot%BankSize+1)%BankSize }

type Kind string

const (
	Acid      Kind = "tb303"
	Dual      Kind = "dual-synth"
	Kick      Kind = "kick"
	Snare     Kind = "snare"
	ClosedHat Kind = "closed-hat"
	OpenHat   Kind = "open-hat"
	Sampler   Kind = "sampler"
	Clap      Kind = "clap"
	Rimshot   Kind = "rimshot"
	Tom       Kind = "tom"
	Crash     Kind = "crash"
)

var Kinds = []Kind{Acid, Kick, Snare, ClosedHat, OpenHat, Clap, Rimshot, Tom, Crash, Dual, Sampler}

func (k Kind) Name() string {
	switch k {
	case Acid:
		return "TB-303"
	case Dual:
		return "Dual synth"
	case Kick:
		return "Kick"
	case Snare:
		return "Snare"
	case ClosedHat:
		return "Closed Hat"
	case OpenHat:
		return "Open Hat"
	case Clap:
		return "Clap"
	case Rimshot:
		return "Rimshot"
	case Tom:
		return "Tom"
	case Crash:
		return "Crash"
	case Sampler:
		return "Sampler"
	}
	return string(k)
}

type Capabilities struct{ Notes, Velocity, Accent, Slide, Locks, NoteLengths bool }

func (k Kind) Capabilities() Capabilities {
	return Capabilities{Notes: k == Acid || k == Dual || k == Sampler || k == Tom, Velocity: true, Accent: true, Slide: k == Acid || k == Dual, Locks: true, NoteLengths: k == Acid || k == Dual || k == Sampler}
}

type Parameter struct {
	ID, Name                     string
	Min, Max, Default, Increment float64
	Lockable                     bool
}

func Parameters(k Kind) []Parameter {
	switch k {
	case Dual:
		params := make([]Parameter, 0, len(synth.DualParameters()))
		for _, p := range synth.DualParameters() {
			params = append(params, Parameter{p.ID, p.Name, p.Min, p.Max, p.Default, p.Increment, p.Lockable})
		}
		return params
	case Sampler:
		return []Parameter{{"pitch", "Pitch (semitones)", -24, 24, 0, 1, true}, {"start", "Trim start", 0, 1, 0, .001, false}, {"end", "Trim end", 0, 1, 1, .001, false}, {"reverse", "Reverse", 0, 1, 0, 1, false}, {"mode", "Playback mode", 0, 2, 0, 1, false}, {"gain_db", "Sample gain (dB)", -24, 12, 0, .1, true}, {"normalize", "Normalize", 0, 1, 0, 1, false}, {"fade_in", "Fade in (ms)", 0, 2000, 0, 1, false}, {"fade_out", "Fade out (ms)", 0, 2000, 0, 1, false}}
	case Acid:
		return []Parameter{{"cutoff", "Cutoff", 0, 1, .55, .01, true}, {"resonance", "Resonance", 0, 1, .65, .01, true}, {"env", "Envelope", 0, 1, .6, .01, true}, {"decay", "Decay", 0, 1, .3, .01, true}, {"accent", "Accent", 0, 1, .6, .01, true}, {"slide", "Slide time", .01, .3, .06, .005, true}, {"square", "Waveform", 0, 1, 0, 1, false}, {"tune", "Tune", -12, 12, 0, 1, true}, {"sub", "Sub level", 0, 1, .3, .01, true},
			{"pulse", "Pulse width", .1, .9, .5, .01, true}, {"drive", "Drive", 0, 1, 0, .01, true}, {"sweep", "Accent sweep", 0, 1, 0, .01, true}, {"gate", "Gate", .05, 1, 1, .01, true},
			{"blend", "Saw/square mix", 0, 1, 0, .01, true}, {"noise", "Noise", 0, 1, 0, .01, true}, {"filter_mode", "Filter mode", 0, 2, 0, 1, false}}
	case Kick:
		return []Parameter{{"decay", "Decay", .05, 1, .4, .01, true}, {"drive", "Drive", 0, 1, .3, .01, true}, {"tune", "Tune", -12, 12, 0, 1, true}, {"click", "Click", 0, 1, .15, .01, true}, {"pitch_decay", "Pitch decay", .001, .1, .025, .001, true}}
	case Snare:
		return []Parameter{{"snap", "Snap", 0, 1, .6, .01, true}, {"tone", "Tone mix", 0, 1, .35, .01, true}, {"decay", "Noise decay", .05, .5, .2, .01, true}, {"tune", "Tune", -12, 12, 0, 1, true}, {"tone_decay", "Tone decay", .01, .3, .08, .005, true}}
	case Clap:
		return []Parameter{{"tune", "Tune", -12, 12, 0, 1, true}, {"decay", "Decay", .01, .3, .08, .005, true}, {"tone", "Tone", 0, 1, .6, .01, true}, {"drive", "Drive", 0, 1, .3, .01, true}}
	case Rimshot:
		return []Parameter{{"pitch", "Pitch (Hz)", 200, 800, 550, 1, true}, {"decay", "Decay", .01, .15, .04, .001, true}, {"tone", "Tone mix", 0, 1, .6, .01, true}, {"snap", "Snap", 0, 1, .8, .01, true}}
	case Tom:
		return []Parameter{{"pitch", "Root pitch (Hz)", 40, 300, 100, .1, true}, {"tune", "Tune", -12, 12, 0, 1, true}, {"decay", "Decay", .1, 2, .5, .01, true}, {"pitch_decay", "Pitch sweep", 0, 1, .5, .01, true}, {"tone", "Tone mix", 0, 1, .7, .01, true}, {"attack", "Attack", 0, 1, .5, .01, true}}
	case Crash:
		return []Parameter{{"decay", "Decay", .3, 3, 1, .01, true}, {"tone", "Tone", 0, 1, .6, .01, true}, {"metallic", "Metallic", 0, 1, .7, .01, true}}
	case ClosedHat, OpenHat:
		decay := .05
		if k == OpenHat {
			decay = .25
		}
		return []Parameter{{"decay", "Decay", .01, 1, decay, .01, true}, {"tone", "Tone", 0, 1, .7, .01, true}, {"noise", "Noise mix", 0, 1, .4, .01, true}}
	}
	return nil
}

type Step struct {
	Length      int                `json:"length,omitempty"` // Zero retains the legacy one-step gate.
	Active      bool               `json:"active"`
	Note        int                `json:"note"`
	Velocity    float64            `json:"velocity"`
	Accent      bool               `json:"accent,omitempty"`
	Slide       bool               `json:"slide,omitempty"`
	Probability float64            `json:"probability"`
	Every       int                `json:"every,omitempty"`
	Locks       map[string]float64 `json:"locks,omitempty"`
}
type Pattern struct {
	Steps      []Step      `json:"steps,omitempty"`      // nil only in a saved blank slot ({})
	Resolution int         `json:"resolution,omitempty"` // see Steps
	State      *TrackState `json:"state,omitempty"`
}
type Track struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Kind            Kind               `json:"kind"`
	Colour          int                `json:"colour_slot"`
	Mute            bool               `json:"mute"`
	Solo            bool               `json:"solo"`
	Hold            bool               `json:"hold,omitempty"` // kept by Randomise all
	Level           float64            `json:"level"`
	Pan             float64            `json:"pan"`
	Parameters      map[string]float64 `json:"parameters"`
	Patterns        []Pattern          `json:"patterns"`
	Effects         []Effect           `json:"effects,omitempty"`
	EffectsBypassed bool               `json:"effects_bypassed,omitempty"`
	LFOs            [2]LFO             `json:"lfos"`
	Sample          *Sample            `json:"sample,omitempty"` // immutable asset shared by history snapshots
}
type Project struct {
	FormatVersion int           `json:"format_version"`
	Name          string        `json:"name"`
	BPM           float64       `json:"bpm"`
	Swing         float64       `json:"swing"`
	Pattern       int           `json:"pattern"`
	Tracks        []Track       `json:"tracks"`
	Song          []SongSection `json:"song,omitempty"`
	Key           *Key          `json:"key,omitempty"` // set by the first melodic generation
}

func newID() string {
	var id [12]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(id[:])
}
func NewTrack(kind Kind, colour int) Track {
	t := Track{LFOs: DefaultLFOs(), ID: newID(), Name: kind.Name(), Kind: kind, Colour: colour % 8, Level: .7, Parameters: map[string]float64{}, Patterns: make([]Pattern, PatternCount)}
	for _, p := range Parameters(kind) {
		t.Parameters[p.ID] = p.Default
	}
	for i := range t.Patterns {
		t.Patterns[i] = NewPattern()
	}
	return t
}

// NewPattern is a blank 16-step pattern.
func NewPattern() Pattern {
	p := Pattern{Steps: make([]Step, 16), Resolution: 16}
	for j := range p.Steps {
		p.Steps[j] = DefaultStep()
	}
	return p
}

// blank reports whether a pattern is exactly NewPattern(): nothing to save.
func (p Pattern) blank() bool {
	if p.State != nil || p.Resolution != 16 || len(p.Steps) != 16 {
		return false
	}
	def := DefaultStep()
	for _, s := range p.Steps {
		if s.Locks != nil || s.Length != 0 || s.Active || s.Note != def.Note || s.Velocity != def.Velocity ||
			s.Accent || s.Slide || s.Probability != def.Probability || s.Every != 0 {
			return false
		}
	}
	return true
}
func (s Step) GateSteps() int { return max(1, s.Length) }

func DefaultStep() Step { return Step{Note: 36, Velocity: .8, Probability: 1} }
func NewProject() Project {
	p := Project{FormatVersion: FormatVersion, Name: "Untitled groove", BPM: 126}
	for i, k := range []Kind{Acid, Acid, Kick, Snare, ClosedHat, OpenHat} {
		t := NewTrack(k, i)
		if i < 2 {
			t.Name = fmt.Sprintf("303-%d", i+1)
		}
		p.Tracks = append(p.Tracks, t)
	}
	return p
}
func (p *Project) Track(id string) *Track {
	for i := range p.Tracks {
		if p.Tracks[i].ID == id {
			return &p.Tracks[i]
		}
	}
	return nil
}
func (p Project) Clone() Project {
	q := p
	if p.Key != nil {
		key := *p.Key
		q.Key = &key
	}
	if p.Song != nil {
		q.Song = make([]SongSection, len(p.Song))
		for i, section := range p.Song {
			q.Song[i] = CloneSection(section)
		}
	}
	q.Tracks = make([]Track, len(p.Tracks))
	for i, t := range p.Tracks {
		q.Tracks[i] = t
		q.Tracks[i].Parameters = cloneMap(t.Parameters)
		if t.Effects != nil {
			q.Tracks[i].Effects = make([]Effect, len(t.Effects))
			for n, e := range t.Effects {
				q.Tracks[i].Effects[n] = e
				q.Tracks[i].Effects[n].Parameters = cloneMap(e.Parameters)
			}
		}
		q.Tracks[i].Patterns = make([]Pattern, len(t.Patterns))
		for j, pattern := range t.Patterns {
			q.Tracks[i].Patterns[j] = pattern
			q.Tracks[i].Patterns[j].State = cloneTrackState(pattern.State)
			q.Tracks[i].Patterns[j].Steps = append([]Step(nil), pattern.Steps...)
			for n, step := range pattern.Steps {
				q.Tracks[i].Patterns[j].Steps[n].Locks = cloneMap(step.Locks)
			}
		}
	}
	return q
}
func cloneMap(m map[string]float64) map[string]float64 {
	if m == nil {
		return nil
	}
	copy := make(map[string]float64, len(m))
	for k, v := range m {
		copy[k] = v
	}
	return copy
}
func inRange(value, min, max float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= min && value <= max
}
func (p Project) Validate() error {
	if p.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported project format %d", p.FormatVersion)
	}
	if !inRange(p.BPM, 40, 300) || !inRange(p.Swing, 0, .5) || p.Pattern < 0 || p.Pattern >= PatternCount {
		return fmt.Errorf("invalid transport settings")
	}
	if p.Key != nil && !p.Key.valid() {
		return fmt.Errorf("invalid key")
	}
	if err := p.validateSong(); err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, t := range p.Tracks {
		if t.ID == "" || ids[t.ID] {
			return fmt.Errorf("track IDs must be unique and nonempty")
		}
		ids[t.ID] = true
		if t.Colour < 0 || t.Colour >= 8 || len(t.Patterns) != PatternCount {
			return fmt.Errorf("invalid track %q", t.Name)
		}
		if err := validateTrackControls(t); err != nil {
			return err
		}
		for _, pattern := range t.Patterns {
			definitions := map[string]Parameter{}
			for _, parameter := range LockParameters(t) {
				definitions[parameter.ID] = parameter
			}
			if pattern.State != nil {
				stored := withTrackState(t, pattern.State)
				if err := validateTrackControls(stored); err != nil {
					return fmt.Errorf("stored state on %s: %w", t.Name, err)
				}
				for _, parameter := range LockParameters(stored) {
					definitions[parameter.ID] = parameter
				}
			}
			if len(pattern.Steps) < 1 || len(pattern.Steps) > 64 {
				return fmt.Errorf("pattern length must be 1..64")
			}
			switch pattern.Resolution {
			case 4, 8, 16, 32, 64:
			default:
				return fmt.Errorf("invalid step resolution")
			}
			for _, step := range pattern.Steps {
				if step.Length < 0 || step.Length > 64 || step.Note < 0 || step.Note > 127 || !inRange(step.Velocity, 0, 1) || !inRange(step.Probability, 0, 1) || step.Every < 0 || step.Every > 16 {
					return fmt.Errorf("invalid step on %s", t.Name)
				}
				for id, v := range step.Locks {
					d, ok := definitions[id]
					if !ok || !d.Lockable || !inRange(v, d.Min, d.Max) {
						return fmt.Errorf("invalid parameter lock %s", id)
					}
				}
			}
		}
	}
	return nil
}

// Session owns editable state on the control thread. Every command publishes a
// separate deep snapshot; audio never reads this mutable project.
type Session struct {
	Project    Project
	undo, redo []Project
	saved      Project
	LastAction string
	gesture    *Project
}

func NewSession(p Project) *Session { return &Session{Project: p.Clone(), saved: p.Clone()} }

// FollowPattern moves the editing context with song playback. Mirror the same
// view change in the saved baseline so playing a saved song creates no edits;
// actual differences in notes, snapshots or other controls remain dirty.
func (s *Session) FollowPattern(slot int) error {
	if err := SelectPattern(&s.Project, slot); err != nil {
		return err
	}
	return SelectPattern(&s.saved, slot)
}

func (s *Session) Do(label string, edit func(*Project) error) error {
	next := s.Project.Clone()
	if err := edit(&next); err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return err
	}
	if reflect.DeepEqual(next, s.Project) {
		return nil
	}
	if s.gesture == nil {
		s.pushUndo(s.Project)
	}
	s.redo = nil
	s.Project, s.LastAction = next, label
	return nil
}
func (s *Session) pushUndo(p Project) {
	s.undo = append(s.undo, p)
	if len(s.undo) > 64 {
		s.undo = s.undo[1:]
	}
}

// Live gestures publish each validated edit immediately, while keeping a single
// history boundary. Undo/redo safely finalise an interrupted gesture first.
func (s *Session) BeginGesture() {
	if s.gesture == nil {
		before := s.Project.Clone()
		s.gesture = &before
	}
}
func (s *Session) EndGesture() {
	if s.gesture == nil {
		return
	}
	before := *s.gesture
	s.gesture = nil
	if !reflect.DeepEqual(before, s.Project) {
		s.pushUndo(before)
	}
}
func (s *Session) Undo() bool {
	s.EndGesture()
	if len(s.undo) == 0 {
		return false
	}
	s.redo = append(s.redo, s.Project)
	s.Project = s.undo[len(s.undo)-1]
	s.undo = s.undo[:len(s.undo)-1]
	s.LastAction = "Undo"
	return true
}
func (s *Session) Redo() bool {
	s.EndGesture()
	if len(s.redo) == 0 {
		return false
	}
	s.undo = append(s.undo, s.Project)
	s.Project = s.redo[len(s.redo)-1]
	s.redo = s.redo[:len(s.redo)-1]
	s.LastAction = "Redo"
	return true
}
func (s *Session) Dirty() bool                { return !reflect.DeepEqual(s.saved, s.Project) }
func (s *Session) MarkSaved()                 { s.saved = s.Project.Clone() }
func (s *Session) MarkSavedProject(p Project) { s.saved = p.Clone() }

// ParameterChoices names the options of a stepped instrument control, or
// nil if it is continuous.
func ParameterChoices(kind Kind, id string) []string {
	if kind != Acid {
		return nil
	}
	switch id {
	case "square":
		return []string{"Saw", "Square"}
	case "filter_mode":
		return []string{"Low-pass", "Band-pass", "High-pass"}
	}
	return nil
}
