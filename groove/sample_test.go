package groove

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func testWAV(format, bits, channels, rate int, audio []byte) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(audio)+len(audio)%2))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	for _, v := range []uint16{uint16(format), uint16(channels)} {
		binary.Write(&b, binary.LittleEndian, v)
	}
	binary.Write(&b, binary.LittleEndian, uint32(rate))
	binary.Write(&b, binary.LittleEndian, uint32(rate*channels*bits/8))
	binary.Write(&b, binary.LittleEndian, uint16(channels*bits/8))
	binary.Write(&b, binary.LittleEndian, uint16(bits))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(audio)))
	b.Write(audio)
	if len(audio)%2 != 0 {
		b.WriteByte(0)
	}
	return b.Bytes()
}
func pcmSample(t *testing.T, channels, rate int, values ...int16) *Sample {
	t.Helper()
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, values)
	s, err := LoadSample("test.wav", bytes.NewReader(testWAV(1, 16, channels, rate, b.Bytes())))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestWAVFormatsBoundsAndChunkPadding(t *testing.T) {
	cases := []struct {
		format, bits int
		audio        []byte
		want         float32
	}{
		{1, 8, []byte{192}, .5}, {1, 16, []byte{0, 64}, .5}, {1, 24, []byte{0, 0, 64}, .5}, {1, 32, []byte{0, 0, 0, 64}, .5}, {3, 32, []byte{0, 0, 0, 63}, .5},
	}
	for _, c := range cases {
		wav := testWAV(c.format, c.bits, 1, 48000, c.audio)
		// Add an odd-sized metadata chunk before fmt, including its required padding.
		wav = append(append(append([]byte{}, wav[:12]...), []byte{'J', 'U', 'N', 'K', 1, 0, 0, 0, 7, 0}...), wav[12:]...)
		binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
		s, err := LoadSample("test", bytes.NewReader(wav))
		if err != nil || s.data[0] != c.want {
			t.Fatalf("format %d/%d: %v", c.format, c.bits, err)
		}
	}
	valid := testWAV(1, 16, 1, 44100, []byte{0, 64})
	for _, bad := range [][]byte{nil, valid[:20], testWAV(1, 16, 0, 44100, []byte{0, 0}), testWAV(1, 16, 3, 44100, make([]byte, 6)), testWAV(1, 16, 1, 44100, []byte{0}), testWAV(3, 32, 1, 44100, []byte{0, 0, 128, 127}), make([]byte, MaxSampleBytes+1)} {
		if _, err := LoadSample("bad", bytes.NewReader(bad)); err == nil {
			t.Fatal("invalid WAV accepted")
		}
	}
}
func TestEmbeddedSampleRoundTripAndUndo(t *testing.T) {
	p := NewProject()
	track := NewTrack(Sampler, 6)
	p.Tracks = append(p.Tracks, track)
	session := NewSession(p)
	sample := pcmSample(t, 2, 48000, 16384, -16384, 8192, -8192)
	if err := session.Do("Load", func(p *Project) error { p.Track(track.ID).Sample = sample; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := session.Do("Trim", func(p *Project) error { p.Track(track.ID).Parameters["start"] = .5; return nil }); err != nil {
		t.Fatal(err)
	}
	if !session.Undo() || session.Project.Track(track.ID).Parameters["start"] != 0 {
		t.Fatal("trim undo failed")
	}
	clone := session.Project.Clone()
	if clone.Track(track.ID).Sample != sample {
		t.Fatal("history copies immutable audio")
	}
	path := filepath.Join(t.TempDir(), "sample.gruvbox3")
	if err := Save(path, session.Project); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	decoded, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	asset := decoded.Track(track.ID).Sample
	if asset.Name() != sample.Name() || asset.Rate() != 48000 || asset.Channels() != 2 || asset.Frames() != 2 || !bytes.Equal(asset.wav, sample.wav) {
		t.Fatal("embedded sample did not round trip")
	}
	if !session.Undo() || session.Project.Track(track.ID).Sample != nil {
		t.Fatal("load undo failed")
	}
	var obj map[string]any
	json.Unmarshal(data, &obj)
	if err := session.Do("Invalid trim", func(p *Project) error { p.Track(track.ID).Parameters["start"] = 1; return nil }); err == nil {
		t.Fatal("inverted trim accepted")
	}
}
func TestSamplerStereoPitchTrimReverseLoop(t *testing.T) {
	s := pcmSample(t, 2, 44100, 8192, -8192, 16384, -16384, 24576, -24576, 32767, -32768)
	track := NewTrack(Sampler, 0)
	track.Sample = s
	step := DefaultStep()
	step.Velocity = 1
	v := sampleVoice{sample: s}
	v.trigger(track, step)
	l, r := v.next()
	if l != .25 || r != -.25 {
		t.Fatalf("stereo %v %v", l, r)
	}
	track.Parameters["start"] = .25
	track.Parameters["end"] = .75
	v.trigger(track, step)
	l, _ = v.next()
	if l != .5 {
		t.Fatal("trim start")
	}
	v.next()
	l, _ = v.next()
	if l != 0 {
		t.Fatal("one shot did not stop at trim end")
	}
	track.Parameters["reverse"] = 1
	v.trigger(track, step)
	l, _ = v.next()
	if l != .75 {
		t.Fatal("reverse start")
	}
	l, _ = v.next()
	if l != .5 {
		t.Fatal("reverse direction")
	}
	track.Parameters["mode"] = 2
	v.trigger(track, step)
	v.next()
	v.next()
	l, _ = v.next()
	if l != .75 {
		t.Fatal("reverse loop failed")
	}
	track.Parameters["reverse"] = 0
	v.pitch = 12
	v.trigger(track, step)
	if v.speed != 2 {
		t.Fatal("pitch lock rate")
	}
	v.pitch = 0
	step.Note = 48
	v.trigger(track, step)
	if v.speed != 2 {
		t.Fatal("melodic pitch rate")
	}
}
func TestSamplerGateAndAllocationFreePublication(t *testing.T) {
	p := NewProject()
	track := NewTrack(Sampler, 0)
	track.Sample = pcmSample(t, 1, 44100, 16384, 16384, 16384, 16384)
	track.Parameters["mode"] = 2
	track.Patterns[0].Steps[0].Active = true
	p.Tracks = []Track{track}
	p.BPM = 120
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var out [BlockSize * 2]float32
	player.Render(out[:])
	if out[masterLatency*2] == 0 {
		t.Fatal("sampler did not render")
	}
	if n := testing.AllocsPerRun(100, func() { player.Render(out[:]) }); n != 0 {
		t.Fatalf("sampler allocated %v", n)
	}
	// Gate mode sustains through the active step then ends on the empty step.
	long := make([]int16, 6000)
	for i := range long {
		long[i] = 16384
	}
	p.Tracks[0].Sample = pcmSample(t, 1, 44100, long...)
	p.Tracks[0].Parameters["mode"] = 1
	player.SetPlaying(false)
	player.Publish(p)
	player.SetPlaying(true)
	player.Render(make([]float32, 5513*2))
	player.Render(out[:])
	if !silentAfterMasterLatency(out[:]) {
		t.Fatal("gate did not close at next empty step")
	}
	// Replacement and clear while rendering must not mutate audio-owned state.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			player.Render(out[:])
		}
	}()
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			p.Tracks[0].Sample = track.Sample
		} else {
			p.Tracks[0].Sample = nil
		}
		player.Publish(p)
	}
	wg.Wait()
	player.SetPlaying(false)
	player.Render(out[:])
	for _, v := range out {
		if math.Abs(float64(v)) != 0 {
			t.Fatal("stop")
		}
	}
}
