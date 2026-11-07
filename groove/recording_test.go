package groove

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestCapturedPCMPortableWAVAndOriginalSave(t *testing.T) {
	var pcm bytes.Buffer
	binary.Write(&pcm, binary.LittleEndian, []int16{16384, -8192, 4096, -2048})
	sample, err := SampleFromPCM("capture.wav", pcm.Bytes(), 44100, 2)
	if err != nil || sample.Frames() != 2 || sample.Channels() != 2 || sample.data[0] != .5 || sample.data[1] != -.25 {
		t.Fatal("captured sample conversion", err)
	}
	path := filepath.Join(t.TempDir(), "takes", "capture.wav")
	if err := SaveSampleWAV(path, sample); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	loaded, err := LoadSample("saved.wav", bytes.NewReader(data))
	if err != nil || !bytes.Equal(sample.wav, loaded.wav) {
		t.Fatal("saved recording is not the original portable WAV", err)
	}
	if err := SaveSampleWAV(path, nil); err == nil {
		t.Fatal("empty take overwrote a WAV")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(data, after) {
		t.Fatal("failed save damaged the existing recording")
	}
	for _, test := range []struct {
		data           []byte
		rate, channels int
	}{{nil, 44100, 2}, {[]byte{0, 0}, 44100, 2}, {[]byte{0, 0}, 1000, 1}, {make([]byte, MaxSampleBytes), 44100, 2}} {
		if _, err := SampleFromPCM("bad", test.data, test.rate, test.channels); err == nil {
			t.Fatal("invalid captured PCM accepted")
		}
	}
}
