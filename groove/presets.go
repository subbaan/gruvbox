package groove

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const SoundPresetVersion = 1
const MaxSoundPresetBytes = 32 * 1024 * 1024

// SoundPreset contains timbre, not sequence or mixer state. Samples are immutable
// assets, shared in memory and embedded in the portable .gruvsound file.
type SoundPreset struct {
	FormatVersion int                `json:"sound_preset_version"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	Kind          Kind               `json:"kind"`
	Parameters    map[string]float64 `json:"parameters"`
	LFOs          [2]LFO             `json:"lfos"`
	Effects       []Effect           `json:"effects,omitempty"`
	FXBypassed    bool               `json:"effects_bypassed,omitempty"`
	Sample        *Sample            `json:"sample,omitempty"`
}

func CaptureSound(t Track, name string) SoundPreset {
	p := SoundPreset{FormatVersion: SoundPresetVersion, Name: strings.TrimSpace(name), Kind: t.Kind,
		Parameters: cloneMap(t.Parameters), LFOs: t.LFOs, FXBypassed: t.EffectsBypassed, Sample: t.Sample}
	for _, e := range t.Effects {
		e.Parameters = cloneMap(e.Parameters)
		p.Effects = append(p.Effects, e)
	}
	return p
}

func (p SoundPreset) Validate() error {
	if p.FormatVersion != SoundPresetVersion {
		return fmt.Errorf("unsupported sound preset format %d", p.FormatVersion)
	}
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 120 || len(p.Description) > 2000 {
		return fmt.Errorf("preset needs a name of 1–120 bytes and a description under 2000 bytes")
	}
	t := NewTrack(p.Kind, 0)
	t.Parameters, t.LFOs, t.Effects, t.EffectsBypassed, t.Sample = p.Parameters, p.LFOs, p.Effects, p.FXBypassed, p.Sample
	return (Project{FormatVersion: FormatVersion, BPM: 126, Tracks: []Track{t}}).Validate()
}

// ApplySound validates before editing. FX recall deliberately removes locks on
// the replaced chain; instrument locks and all sequence/mixer state survive.
func ApplySound(t *Track, p SoundPreset, includeFX bool) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if t == nil || t.Kind != p.Kind {
		return fmt.Errorf("select a %s track for this preset", p.Kind.Name())
	}
	t.Parameters, t.LFOs, t.Sample = cloneMap(p.Parameters), p.LFOs, p.Sample
	if includeFX {
		for len(t.Effects) > 0 {
			RemoveEffect(t, t.Effects[0].ID)
		}
		for _, e := range p.Effects {
			e.ID, e.Parameters = newID(), cloneMap(e.Parameters)
			t.Effects = append(t.Effects, e)
		}
		t.EffectsBypassed = p.FXBypassed
	}
	return nil
}

func DecodeSoundPreset(reader io.Reader) (SoundPreset, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxSoundPresetBytes+1))
	if err != nil {
		return SoundPreset{}, err
	}
	if len(data) > MaxSoundPresetBytes {
		return SoundPreset{}, fmt.Errorf("sound preset exceeds 32 MiB")
	}
	var p SoundPreset
	if err := json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	fillParameterDefaults(p.Kind, p.Parameters)
	return p, p.Validate()
}

func SaveSoundPreset(path string, p SoundPreset) error {
	if err := p.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if len(data)+1 > MaxSoundPresetBytes {
		return fmt.Errorf("sound preset exceeds 32 MiB")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".gruvbox-sound-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

type SoundPresetEntry struct {
	ID, Name, Description string
	Kind                  Kind
	Factory               bool
}
type SoundLibrary struct{ Dir string }

var soundID = regexp.MustCompile(`^[a-f0-9]{24}$`)

func (l SoundLibrary) Load(id string) (SoundPreset, error) {
	for _, kind := range Kinds {
		for i, p := range FactorySounds(kind) {
			if id == factorySoundID(kind, i) {
				return p, nil
			}
		}
	}
	if !soundID.MatchString(id) {
		return SoundPreset{}, fmt.Errorf("invalid sound preset ID")
	}
	f, err := os.Open(filepath.Join(l.Dir, id+".gruvsound"))
	if err != nil {
		return SoundPreset{}, err
	}
	defer f.Close()
	return DecodeSoundPreset(f)
}

func (l SoundLibrary) Save(p SoundPreset) (string, error) {
	id := newID()
	return id, SaveSoundPreset(filepath.Join(l.Dir, id+".gruvsound"), p)
}

func (l SoundLibrary) Delete(id string) error {
	if !soundID.MatchString(id) {
		return fmt.Errorf("only saved user presets can be deleted")
	}
	return os.Remove(filepath.Join(l.Dir, id+".gruvsound"))
}

func factorySoundID(kind Kind, index int) string { return fmt.Sprintf("factory-%s-%d", kind, index) }

// List reads metadata without decoding sample audio. A damaged file is reported
// alongside the usable entries, so one bad import cannot hide the whole library.
func (l SoundLibrary) List(kind Kind) ([]SoundPresetEntry, error) {
	var entries []SoundPresetEntry
	for i, p := range FactorySounds(kind) {
		entries = append(entries, SoundPresetEntry{factorySoundID(kind, i), p.Name, p.Description, kind, true})
	}
	files, err := filepath.Glob(filepath.Join(l.Dir, "*.gruvsound"))
	if err != nil {
		return entries, err
	}
	var firstErr error
	for _, path := range files {
		id := strings.TrimSuffix(filepath.Base(path), ".gruvsound")
		if !soundID.MatchString(id) {
			continue
		}
		// Stop after the small canonical header, before parameters or an embedded
		// WAV. Files entering the library always pass through SaveSoundPreset.
		entry, err := readSoundHeader(path)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("read preset %s: %w", id, err)
			}
			continue
		}
		if entry.Kind == kind {
			entry.ID = id
			entries = append(entries, entry)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Factory != entries[j].Factory {
			return entries[i].Factory
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, firstErr
}

func readSoundHeader(path string) (SoundPresetEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return SoundPresetEntry{}, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 16*1024))
	entry := SoundPresetEntry{}
	version := 0
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return entry, fmt.Errorf("invalid sound preset header")
	}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return entry, err
		}
		switch token {
		case "sound_preset_version":
			err = d.Decode(&version)
		case "name":
			err = d.Decode(&entry.Name)
		case "description":
			err = d.Decode(&entry.Description)
		case "kind":
			err = d.Decode(&entry.Kind)
		case "parameters":
			if version != SoundPresetVersion || strings.TrimSpace(entry.Name) == "" || len(Parameters(entry.Kind)) == 0 {
				return entry, fmt.Errorf("invalid sound preset header")
			}
			return entry, nil
		default:
			return entry, fmt.Errorf("unexpected sound preset header field")
		}
		if err != nil {
			return entry, err
		}
	}
	return entry, fmt.Errorf("incomplete sound preset")
}
