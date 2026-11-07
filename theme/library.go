package theme

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`)

type Entry struct {
	ID       string `json:"id"`
	Scheme   Scheme `json:"scheme"`
	Source   string `json:"source,omitempty"`
	Revision string `json:"revision,omitempty"`
}

type Library struct{ Dir string }

func (l Library) Load(id string) (Entry, error) {
	if !validID.MatchString(id) {
		return Entry{}, fmt.Errorf("invalid theme ID %q", id)
	}
	if s, ok := Bundled()[id]; ok {
		return Entry{ID: id, Scheme: s}, nil
	}
	data, err := readLimited(filepath.Join(l.Dir, id+".json"), 128*1024)
	if err != nil {
		return Entry{}, err
	}
	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return Entry{}, err
	}
	if entry.ID != id {
		return Entry{}, fmt.Errorf("theme ID does not match filename")
	}
	return entry, entry.Scheme.Validate()
}

func (l Library) List() ([]Entry, error) {
	var entries []Entry
	for id, scheme := range Bundled() {
		entries = append(entries, Entry{ID: id, Scheme: scheme})
	}
	files, err := filepath.Glob(filepath.Join(l.Dir, "*.json"))
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		id := filepath.Base(file)
		id = id[:len(id)-len(".json")]
		if id == "appearance" {
			continue
		}
		entry, err := l.Load(id)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", id, err)
		}
		if _, bundled := Bundled()[id]; !bundled {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

func (l Library) Save(entry Entry) error {
	if !validID.MatchString(entry.ID) || entry.ID == "appearance" {
		return fmt.Errorf("invalid theme ID %q", entry.ID)
	}
	if _, bundled := Bundled()[entry.ID]; bundled {
		return fmt.Errorf("cannot overwrite a bundled theme")
	}
	if err := entry.Scheme.Validate(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(l.Dir, entry.ID+".json"), entry)
}

func (l Library) Import(id, path string) (Entry, error) {
	data, err := readLimited(path, MaxSchemeBytes)
	if err != nil {
		return Entry{}, err
	}
	scheme, err := Parse(data)
	if err != nil {
		return Entry{}, err
	}
	entry := Entry{ID: "local-" + id, Scheme: scheme}
	return entry, l.Save(entry)
}

func (l Library) Select(id string) error {
	if _, err := l.Load(id); err != nil {
		return err
	}
	return writeJSON(filepath.Join(l.Dir, "appearance.json"), struct {
		Theme string `json:"theme"`
	}{id})
}

func (l Library) Selected() (Entry, error) {
	data, err := readLimited(filepath.Join(l.Dir, "appearance.json"), 4096)
	if os.IsNotExist(err) {
		return l.Load("warm-dark")
	}
	if err != nil {
		return Entry{}, err
	}
	var settings struct {
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return Entry{}, err
	}
	return l.Load(settings.Theme)
}

func readLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = fmt.Errorf("file exceeds %d bytes", limit)
	}
	return data, err
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".theme-*")
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
