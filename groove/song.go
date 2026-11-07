package groove

import "fmt"

// Repeats counts 4/4 bars, independent of individual track lengths.
type SongSection struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Pattern int             `json:"pattern"`
	Repeats int             `json:"repeats"`
	Mutes   map[string]bool `json:"mutes,omitempty"`
}

func NewSongSection(pattern int) SongSection {
	return SongSection{ID: newID(), Name: "Section", Pattern: pattern, Repeats: 4}
}
func (p *Project) Section(id string) *SongSection {
	for i := range p.Song {
		if p.Song[i].ID == id {
			return &p.Song[i]
		}
	}
	return nil
}
func (p Project) validateSong() error {
	if len(p.Song) > 256 {
		return fmt.Errorf("song has more than 256 sections")
	}
	tracks := map[string]bool{}
	for _, track := range p.Tracks {
		tracks[track.ID] = true
	}
	ids := map[string]bool{}
	for _, section := range p.Song {
		if section.ID == "" || ids[section.ID] || section.Pattern < 0 || section.Pattern >= PatternCount || section.Repeats < 1 || section.Repeats > 64 {
			return fmt.Errorf("invalid song section")
		}
		for id := range section.Mutes {
			if !tracks[id] {
				return fmt.Errorf("unknown track in section mutes")
			}
		}
		ids[section.ID] = true
	}
	return nil
}

func CloneSection(section SongSection) SongSection {
	result := section
	if section.Mutes != nil {
		result.Mutes = make(map[string]bool, len(section.Mutes))
		for id, value := range section.Mutes {
			result.Mutes[id] = value
		}
	}
	return result
}
