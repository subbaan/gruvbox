package groove

import "fmt"

type Template string

const (
	ClassicTemplate Template = "Classic acid"
	DrumsTemplate   Template = "Drum machine"
	TomBassTemplate Template = "Tom bass groove"
	SamplerTemplate Template = "Sampler kit"
	EmptyTemplate   Template = "Empty project"
)

var Templates = []Template{ClassicTemplate, DrumsTemplate, TomBassTemplate, SamplerTemplate, EmptyTemplate}

// NewFromTemplate creates independent voices and patterns on every call.
func NewFromTemplate(template Template) (Project, error) {
	p := Project{FormatVersion: FormatVersion, Name: string(template), BPM: 126}
	add := func(kind Kind, name, action string) *Track {
		track := NewTrack(kind, len(p.Tracks))
		if name != "" {
			track.Name = name
		}
		if action != "" {
			GenerateIn(&p, &track, 0, action)
		}
		p.Tracks = append(p.Tracks, track)
		return &p.Tracks[len(p.Tracks)-1]
	}
	switch template {
	case ClassicTemplate:
		p = NewProject()
		p.Name = string(template)
		for i, action := range []string{"Acid", "Sparse", "4 Floor", "2 & 4", "8ths", "Clear"} {
			GenerateIn(&p, &p.Tracks[i], 0, action)
		}
	case DrumsTemplate:
		for _, part := range []struct {
			kind   Kind
			action string
		}{{Kick, "4 Floor"}, {Snare, "2 & 4"}, {ClosedHat, "8ths"}, {OpenHat, "Offbeat"}, {Clap, "Clear"}, {Rimshot, "Syncopated"}, {Tom, "Fill"}, {Crash, "Downbeat"}} {
			add(part.kind, "", part.action)
		}
	case TomBassTemplate:
		bass := add(Tom, "Tom bass", "")
		if err := TomBassTone(bass); err != nil {
			return Project{}, err
		}
		GenerateIn(&p, bass, 0, "Bassline")
		add(Kick, "", "4 Floor")
		add(Clap, "", "2 & 4")
		add(ClosedHat, "", "8ths")
		add(OpenHat, "", "Offbeat")
	case SamplerTemplate:
		for _, name := range []string{"Sample kick", "Sample snare", "Sample hats", "Sample texture"} {
			add(Sampler, name, "")
		}
	case EmptyTemplate:
	default:
		return Project{}, fmt.Errorf("unknown template %q", template)
	}
	return p, p.Validate()
}
