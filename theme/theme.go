// Package theme loads Base16 palettes and resolves shared application colours.
// It has no dependency on a renderer or the audio engine.
package theme

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const MaxSchemeBytes = 64 * 1024

var hexColour = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

type Scheme struct {
	Name    string     `json:"name"`
	Author  string     `json:"author"`
	Variant string     `json:"variant"`
	Palette [16]string `json:"palette"`
}

// Parse accepts Tinted Theming's nested palette and legacy flat Base16 YAML.
func Parse(data []byte) (Scheme, error) {
	var result Scheme
	if len(data) > MaxSchemeBytes {
		return result, fmt.Errorf("theme exceeds %d bytes", MaxSchemeBytes)
	}
	var doc struct {
		System  string            `yaml:"system"`
		Name    string            `yaml:"name"`
		Scheme  string            `yaml:"scheme"`
		Author  string            `yaml:"author"`
		Variant string            `yaml:"variant"`
		Palette map[string]string `yaml:"palette"`
		Legacy  map[string]string `yaml:",inline"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&doc); err != nil {
		return result, fmt.Errorf("parse Base16 theme: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return result, fmt.Errorf("theme must contain exactly one YAML document")
	}
	if doc.System != "" && doc.System != "base16" {
		return result, fmt.Errorf("unsupported theme system %q", doc.System)
	}
	result.Name = strings.TrimSpace(doc.Name)
	if result.Name == "" {
		result.Name = strings.TrimSpace(doc.Scheme)
	}
	result.Author, result.Variant = doc.Author, doc.Variant
	palette := doc.Palette
	if palette == nil {
		palette = doc.Legacy
	}
	for key, value := range palette {
		key = strings.ToLower(key)
		for i := range result.Palette {
			if key == fmt.Sprintf("base%02x", i) {
				result.Palette[i] = "#" + strings.ToLower(strings.TrimPrefix(value, "#"))
			}
		}
	}
	if result.Variant == "" {
		result.Variant = "dark"
		if luminance(result.Palette[0]) > luminance(result.Palette[5]) {
			result.Variant = "light"
		}
	}
	return result, result.Validate()
}

func (s Scheme) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("theme name is required")
	}
	if s.Variant != "light" && s.Variant != "dark" {
		return fmt.Errorf("theme variant must be light or dark")
	}
	for i, colour := range s.Palette {
		if !strings.HasPrefix(colour, "#") || !hexColour.MatchString(strings.TrimPrefix(colour, "#")) {
			return fmt.Errorf("base%02X must be a six-digit hex colour", i)
		}
	}
	return nil
}

// Roles describe meaning, leaving rendering and interaction to each UI.
type Roles struct {
	Background, Panel, Alternate, Selection, Text, Secondary, Disabled string
	Border, Focus, SelectedText, Primary, Success, Warning, Error      string
	InactiveStep, ActiveStep, Playback, Accent, Slide, Lock            string
	Waveform, Marker, Loop, Meter, Clipping, Recording                 string
	Tracks                                                             [8]string
}

func (s Scheme) Roles() Roles {
	p := s.Palette
	return Roles{
		Background: p[0], Panel: p[1], Alternate: p[1], Selection: p[2],
		Text: p[5], Secondary: p[4], Disabled: p[3], Border: p[3],
		Focus: p[10], SelectedText: s.TextOn(p[10]), Primary: p[13],
		Success: p[11], Warning: p[10], Error: p[8],
		InactiveStep: p[1], ActiveStep: p[9], Playback: p[5],
		Accent: p[10], Slide: p[12], Lock: p[14], Waveform: p[13],
		Marker: p[10], Loop: p[2], Meter: p[11], Clipping: p[8], Recording: p[8],
		Tracks: [8]string{p[9], p[11], p[13], p[8], p[10], p[12], p[14], p[15]},
	}
}

// TextOn selects the strongest contrast among the scheme's neutral colours,
// or black or white when none of them reaches 4.5:1.
func (s Scheme) TextOn(background string) string {
	best, score := s.Palette[5], 0.0
	for _, colour := range s.Palette[:8] {
		if contrast := Contrast(colour, background); contrast > score {
			best, score = colour, contrast
		}
	}
	if score < 4.5 {
		for _, colour := range []string{"#000000", "#ffffff"} {
			if contrast := Contrast(colour, background); contrast > score {
				best, score = colour, contrast
			}
		}
	}
	return best
}

// Contrast is the WCAG contrast ratio of two #rrggbb colours (1 to 21).
func Contrast(x, y string) float64 {
	a, b := luminance(x), luminance(y)
	return (math.Max(a, b) + 0.05) / (math.Min(a, b) + 0.05)
}

func luminance(colour string) float64 {
	n, _ := strconv.ParseUint(strings.TrimPrefix(colour, "#"), 16, 32)
	linear := func(v uint64) float64 {
		c := float64(v) / 255
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*linear((n>>16)&255) + 0.7152*linear((n>>8)&255) + 0.0722*linear(n&255)
}

func Bundled() map[string]Scheme {
	return map[string]Scheme{
		"warm-dark": {
			Name: "Warm Dark", Author: "Gruvbox", Variant: "dark",
			Palette: [16]string{"#22211f", "#2b2a27", "#3a3833", "#716d63", "#aaa394", "#ded7c8", "#ebe5d9", "#f5f0e6", "#c57965", "#bd8f68", "#d4b373", "#a5ad80", "#8fa99c", "#91a0ad", "#b09baa", "#a38a74"},
		},
		"gruvbox-dark": {
			Name: "Gruvbox Dark", Author: "morhetz", Variant: "dark",
			Palette: [16]string{"#282828", "#3c3836", "#504945", "#665c54", "#bdae93", "#d5c4a1", "#ebdbb2", "#fbf1c7", "#fb4934", "#fe8019", "#fabd2f", "#b8bb26", "#8ec07c", "#83a598", "#d3869b", "#d65d0e"},
		},
	}
}
