package groove

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type shortWAVWriter struct{}

func (shortWAVWriter) Write(data []byte) (int, error) { return len(data) - 1, nil }
func TestWAVExportMatchesPlaybackAndHasCorrectHeader(t *testing.T) {
	p := NewProject()
	p.Tracks = []Track{NewTrack(Kick, 0)}
	p.BPM = 300
	Generate(&p.Tracks[0], 0, "4 Floor")
	var wav bytes.Buffer
	progress := 0.
	if err := RenderWAV(context.Background(), &wav, p, false, 0, func(value float64) { progress = value }); err != nil {
		t.Fatal(err)
	}
	data := wav.Bytes()
	frames := SampleRate * 240 / 300
	if string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" || binary.LittleEndian.Uint32(data[40:44]) != uint32(frames*4) || len(data) != 44+frames*4 || progress != 1 {
		t.Fatal("invalid WAV size/header/progress")
	}
	offline := NewPlayer()
	offline.Publish(p)
	offline.SetPlaying(true)
	out := make([]float32, frames*2)
	offline.Render(out)
	for i, sample := range out {
		if int16(binary.LittleEndian.Uint16(data[44+i*2:])) != int16(sample*32767) {
			t.Fatalf("export differs from live engine at %d", i)
		}
	}
	decoded, err := LoadSample("export.wav", bytes.NewReader(data))
	if err != nil || decoded.Frames() != frames || decoded.Channels() != 2 {
		t.Fatalf("export is not a valid stereo WAV: %v", err)
	}
}
func TestSongExportLengthTailAndAtomicCancellation(t *testing.T) {
	p := NewProject()
	p.Tracks = []Track{NewTrack(Kick, 0)}
	p.BPM = 300
	Generate(&p.Tracks[0], 0, "4 Floor")
	section := NewSongSection(0)
	section.Repeats = 1
	p.Song = []SongSection{section}
	var wav bytes.Buffer
	if err := RenderWAV(context.Background(), &wav, p, true, .5, nil); err != nil {
		t.Fatal(err)
	}
	want := 44 + (SampleRate*240/300+SampleRate/2)*4
	if wav.Len() != want {
		t.Fatalf("song/tail length %d != %d", wav.Len(), want)
	}
	path := filepath.Join(t.TempDir(), "keep.wav")
	os.WriteFile(path, []byte("existing"), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	err := ExportWAV(ctx, path, p, true, 2, func(float64) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "existing" {
		t.Fatal("cancelled export replaced destination")
	}
	temps, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".gruvbox-wav-*"))
	if len(temps) != 0 {
		t.Fatal("cancelled export leaked temporary file")
	}
	if err := ExportWAV(context.Background(), path, p, true, 0, nil); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data[:4]) != "RIFF" {
		t.Fatal("successful export not committed")
	}
	if err := RenderWAV(context.Background(), shortWAVWriter{}, p, false, 0, nil); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short writes silently accepted")
	}
	p.Song = nil
	if RenderWAV(context.Background(), io.Discard, p, true, 0, nil) == nil {
		t.Fatal("empty song exported")
	}
}

func TestExportTailKeepsDelayWithoutStartingAnotherPattern(t *testing.T) {
	p := NewProject()
	p.BPM = 300
	track := NewTrack(Sampler, 0)
	track.Sample = pcmSample(t, 1, 8000, 32767, 0, 0, 0)
	track.Patterns[0].Steps[0].Active = true
	delay := NewEffect(Delay)
	delay.Parameters["sync"] = 0
	delay.Parameters["time"] = .4
	delay.Parameters["feedback"] = .6
	delay.Parameters["mix"] = 1
	track.Effects = []Effect{delay}
	p.Tracks = []Track{track}
	var wet bytes.Buffer
	if err := RenderWAV(context.Background(), &wet, p, false, .5, nil); err != nil {
		t.Fatal(err)
	}
	boundary := 44 + (SampleRate*240/300)*4
	tailSound := false
	for _, b := range wet.Bytes()[boundary:] {
		tailSound = tailSound || b != 0
	}
	if !tailSound {
		t.Fatal("export cleared the delay tail")
	}
	p.Tracks[0].Effects = nil
	var dry bytes.Buffer
	if err := RenderWAV(context.Background(), &dry, p, false, .5, nil); err != nil {
		t.Fatal(err)
	}
	for _, b := range dry.Bytes()[boundary:] {
		if b != 0 {
			t.Fatal("export tail started another pattern")
		}
	}
}
