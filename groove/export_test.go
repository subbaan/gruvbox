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
	// Live output lags by the master bus look-ahead; the export removes that
	// delay, so it equals live playback shifted back by masterLatency frames.
	live := make([]float32, (frames+masterLatency)*2)
	offline.Render(live)
	out := live[masterLatency*2:]
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

// A single-track render holds only that track: it matches a project with
// the other tracks removed, even when another track was soloed or it was muted.
func TestIsolateTrackRendersOnlyThatTrack(t *testing.T) {
	p := NewProject()
	p.Tracks = []Track{NewTrack(Kick, 0), NewTrack(Snare, 1)}
	p.BPM = 300
	Generate(&p.Tracks[0], 0, "4 Floor")
	Generate(&p.Tracks[1], 0, "Backbeat")
	p.Tracks[0].Solo, p.Tracks[1].Mute = true, true
	alone := p.Clone()
	alone.Tracks = alone.Tracks[1:]
	alone.Tracks[0].Mute = false
	if !IsolateTrack(&p, p.Tracks[1].ID) || IsolateTrack(&alone, "missing") {
		t.Fatal("IsolateTrack reported the wrong result")
	}
	var got, want bytes.Buffer
	if err := RenderWAV(context.Background(), &got, p, false, .5, nil); err != nil {
		t.Fatal(err)
	}
	if err := RenderWAV(context.Background(), &want, alone, false, .5, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatal("the isolated render differs from the track on its own")
	}
	if bytes.Count(got.Bytes()[44:], []byte{0}) == len(got.Bytes())-44 {
		t.Fatal("silent render")
	}
}

// A wrapped loop is one loop long and is exactly what a running loop sounds
// like after the warm-up passes: the delay from the previous pass is already
// in its start, so the file loops without a seam.
func TestWrappedLoopCarriesItsTailIntoTheStart(t *testing.T) {
	p := NewProject()
	p.BPM = 300
	track := NewTrack(Kick, 0)
	Generate(&track, 0, "4 Floor")
	delay := NewEffect(Delay)
	delay.Parameters["sync"] = 0
	delay.Parameters["time"] = .3
	delay.Parameters["feedback"] = .6
	delay.Parameters["mix"] = .5
	track.Effects = []Effect{delay}
	p.Tracks = []Track{track}
	frames := SampleRate * 240 / 300
	var wrapped, plain bytes.Buffer
	progress := 0.
	if err := RenderWAVMode(context.Background(), &wrapped, p, false, 2, TailWrap, func(v float64) { progress = v }); err != nil {
		t.Fatal(err)
	}
	if err := RenderWAV(context.Background(), &plain, p, false, 0, nil); err != nil {
		t.Fatal(err)
	}
	data := wrapped.Bytes()
	if len(data) != 44+frames*4 || binary.LittleEndian.Uint32(data[40:44]) != uint32(frames*4) || progress != 1 {
		t.Fatalf("wrapped loop is %d bytes, want one loop (%d)", len(data), 44+frames*4)
	}
	if bytes.Equal(data, plain.Bytes()) {
		t.Fatal("the wrapped loop has no tail in it")
	}
	// 2 s of warm-up is three 0.8 s passes.
	warm := 3 * frames
	live := make([]float32, (masterLatency+warm+frames)*2)
	offline := NewPlayer()
	offline.Publish(p)
	offline.SetPlaying(true)
	offline.Render(live)
	out := live[(masterLatency+warm)*2:]
	for i, sample := range out {
		if int16(binary.LittleEndian.Uint16(data[44+i*2:])) != int16(sample*32767) {
			t.Fatalf("wrapped loop differs from the running loop at %d", i)
		}
	}
	if RenderWAVMode(context.Background(), io.Discard, p, true, 2, TailWrap, nil) == nil {
		t.Fatal("a song cannot be wrapped")
	}
}

func TestLoopCycleLinesUpTracksAndEverySteps(t *testing.T) {
	p := NewProject()
	a, b := NewTrack(Kick, 0), NewTrack(Snare, 1)
	b.Patterns[0].Steps = b.Patterns[0].Steps[:12]
	p.Tracks = []Track{a, b}
	if got := loopCycle(p); got != 3 {
		t.Fatalf("16 and 12 steps: cycle %d, want 3", got)
	}
	p.Tracks[0].Patterns[0].Steps[4].Active, p.Tracks[0].Patterns[0].Steps[4].Every = true, 2
	if got := loopCycle(p); got != 6 {
		t.Fatalf("with an every-2 step: cycle %d, want 6", got)
	}
	p.Tracks[1].Patterns[0].Steps = p.Tracks[1].Patterns[0].Steps[:11]
	p.Swing = .1
	if got := loopCycle(p); got != 1 {
		t.Fatalf("swung odd length: cycle %d, want 1", got)
	}
	p.Swing = 0
	p.Tracks[1].Patterns[0].Steps = p.Tracks[1].Patterns[0].Steps[:7]
	p.Tracks[0].Patterns[0].Steps[4].Every = 16
	if got := loopCycle(p); got != 1 {
		t.Fatalf("too long a cycle (%d) should fall back to whole loops", got)
	}
}
