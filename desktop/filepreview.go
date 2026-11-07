package desktop

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
)

// filePreview is what the dialog's preview pane shows for a file: a title,
// some lines and, for themes, the palette.
type filePreview struct {
	title    string
	lines    []string
	swatches []string
}

// maxPreviewBytes caps how much of a file a preview reads.
const maxPreviewBytes = 64 << 20

// previewFile describes a file by what it holds. It reads the file, so the
// dialog runs it off the UI thread and only for the selected file.
func previewFile(e fileEntry) filePreview {
	if e.Dir {
		return filePreview{title: e.Name, lines: []string{"Folder"}}
	}
	if e.Special {
		return filePreview{title: e.Name, lines: []string{"Not a regular file"}}
	}
	p := filePreview{title: e.Name}
	switch strings.ToLower(filepath.Ext(e.Name)) {
	case ".gruvbox3":
		p.lines = projectPreview(e.Path)
	case ".wav", ".wave":
		p.lines = wavPreview(e.Path)
	case ".gruvsound":
		p.lines = soundPreview(e.Path)
	case ".yaml", ".yml", ".json":
		p.lines, p.swatches = themePreview(e.Path)
	}
	if e.HasSize {
		p.lines = append(p.lines, "", humanSize(e.Size))
	}
	return p
}

func openLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxPreviewBytes+1))
	if len(data) > maxPreviewBytes {
		return nil, fmt.Errorf("too large to preview")
	}
	return data, err
}

func projectPreview(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return []string{"Can't read it: " + err.Error()}
	}
	defer f.Close()
	p, err := groove.Decode(f)
	if err != nil {
		return []string{"Not a Gruvbox project"}
	}
	lines := []string{"Gruvbox project"}
	if p.Name != "" && p.Name != "Untitled groove" {
		lines = append(lines, p.Name)
	}
	feel := fmt.Sprintf("%.0f BPM", p.BPM)
	if p.Swing > 0 {
		feel += fmt.Sprintf(" · swing %.0f%%", p.Swing*100)
	}
	if p.Key != nil {
		feel += " · " + p.Key.Name()
	}
	lines = append(lines, feel, "")
	// Tracks by kind, in order: "303 ×2, Kick, Snare, …".
	order, counts := []string{}, map[string]int{}
	for _, t := range p.Tracks {
		name := t.Kind.Name()
		if t.Kind == groove.Acid {
			name = "303"
		}
		if counts[name] == 0 {
			order = append(order, name)
		}
		counts[name]++
	}
	parts := []string{}
	for _, name := range order {
		if counts[name] > 1 {
			name = fmt.Sprintf("%s ×%d", name, counts[name])
		}
		parts = append(parts, name)
	}
	lines = append(lines, fmt.Sprintf("%d tracks: %s", len(p.Tracks), strings.Join(parts, ", ")))
	used := 0
	for slot := 0; slot < groove.PatternCount; slot++ {
		for _, t := range p.Tracks {
			if slot < len(t.Patterns) && patternUsed(t.Patterns[slot]) {
				used++
				break
			}
		}
	}
	lines = append(lines, fmt.Sprintf("%d of %d patterns used", used, groove.PatternCount))
	if len(p.Song) > 0 {
		bars := 0
		for _, s := range p.Song {
			bars += s.Repeats
		}
		seconds := float64(bars) * 4 * 60 / max(p.BPM, 1)
		lines = append(lines, fmt.Sprintf("Song: %d sections · %d bars · %d:%02d", len(p.Song), bars, int(seconds)/60, int(seconds)%60))
	} else {
		lines = append(lines, "No song arrangement")
	}
	return lines
}

func patternUsed(p groove.Pattern) bool {
	for _, s := range p.Steps {
		if s.Active {
			return true
		}
	}
	return false
}

// wavPreview reads a WAV's format chunk and data size, without decoding it.
func wavPreview(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return []string{"Can't read it: " + err.Error()}
	}
	defer f.Close()
	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil || string(header[:4]) != "RIFF" || string(header[8:]) != "WAVE" {
		return []string{"Not a WAV file"}
	}
	var format, channels, bits uint16
	var rate, data uint32
	for format == 0 || data == 0 {
		var chunk [8]byte
		if _, err := io.ReadFull(f, chunk[:]); err != nil {
			break
		}
		size := binary.LittleEndian.Uint32(chunk[4:])
		switch string(chunk[:4]) {
		case "fmt ":
			var body [16]byte
			if size < 16 {
				return []string{"Damaged WAV file"}
			}
			if _, err := io.ReadFull(f, body[:]); err != nil {
				return []string{"Damaged WAV file"}
			}
			format, channels = binary.LittleEndian.Uint16(body[0:]), binary.LittleEndian.Uint16(body[2:])
			rate, bits = binary.LittleEndian.Uint32(body[4:]), binary.LittleEndian.Uint16(body[14:])
			if _, err := f.Seek(int64(size-16+size%2), io.SeekCurrent); err != nil {
				return []string{"Damaged WAV file"}
			}
		case "data":
			data = size
		default:
			if _, err := f.Seek(int64(size+size%2), io.SeekCurrent); err != nil {
				return []string{"Damaged WAV file"}
			}
		}
	}
	if format == 0 || rate == 0 || channels == 0 || bits == 0 {
		return []string{"WAV file"}
	}
	layout := map[uint16]string{1: "mono", 2: "stereo"}[channels]
	if layout == "" {
		layout = fmt.Sprintf("%d channels", channels)
	}
	kind := fmt.Sprintf("%d-bit", bits)
	if format == 3 {
		kind += " float"
	}
	lines := []string{"WAV audio", fmt.Sprintf("%.1f kHz · %s · %s", float64(rate)/1000, layout, kind)}
	if frameBytes := uint32(channels) * uint32(bits) / 8; frameBytes > 0 && data > 0 {
		seconds := float64(data/frameBytes) / float64(rate)
		lines = append(lines, fmt.Sprintf("%d:%04.1f long", int(seconds)/60, seconds-float64(int(seconds)/60*60)))
	}
	return lines
}

func soundPreview(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return []string{"Can't read it: " + err.Error()}
	}
	defer f.Close()
	p, err := groove.DecodeSoundPreset(f)
	if err != nil {
		return []string{"Not a sound preset"}
	}
	lines := []string{"Sound preset · " + p.Kind.Name()}
	if p.Name != "" {
		lines = append(lines, p.Name)
	}
	if p.Description != "" {
		lines = append(lines, "", p.Description)
	}
	if n := len(p.Effects); n > 0 {
		lines = append(lines, "", fmt.Sprintf("%d effect%s", n, map[bool]string{true: "", false: "s"}[n == 1]))
	}
	return lines
}

func themePreview(path string) ([]string, []string) {
	data, err := openLimited(path)
	if err != nil {
		return []string{"Can't read it: " + err.Error()}, nil
	}
	s, err := theme.Parse(data)
	if err != nil {
		return []string{"Not a colour theme"}, nil
	}
	lines := []string{"Colour theme"}
	if s.Name != "" {
		lines = append(lines, s.Name+" · "+s.Variant)
	}
	if s.Author != "" {
		lines = append(lines, s.Author)
	}
	return lines, s.Palette[:]
}
