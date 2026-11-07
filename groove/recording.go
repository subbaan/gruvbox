package groove

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// SampleFromPCM wraps complete little-endian int16 frames in a portable WAV and
// passes through the same validation/peak caching as a loaded sample.
func SampleFromPCM(name string, pcm []byte, rate, channels int) (*Sample, error) {
	if (channels != 1 && channels != 2) || len(pcm) == 0 || len(pcm)%(channels*2) != 0 || len(pcm)+44 > MaxSampleBytes || rate < 8000 || rate > 192000 {
		return nil, fmt.Errorf("invalid or oversized captured PCM")
	}
	wav := make([]byte, 44+len(pcm))
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], uint16(channels))
	binary.LittleEndian.PutUint32(wav[24:], uint32(rate))
	binary.LittleEndian.PutUint32(wav[28:], uint32(rate*channels*2))
	binary.LittleEndian.PutUint16(wav[32:], uint16(channels*2))
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(len(pcm)))
	copy(wav[44:], pcm)
	return LoadSample(name, bytes.NewReader(wav))
}

func SaveSampleWAV(path string, sample *Sample) error {
	if sample == nil || sample.Frames() == 0 {
		return fmt.Errorf("record or load a sample first")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".gruvbox-sample-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(sample.wav); err != nil {
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
