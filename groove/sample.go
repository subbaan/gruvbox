package groove

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

const MaxSampleBytes = 16 * 1024 * 1024
const WaveformBins = 256

// Sample is immutable after decoding. Projects and undo history can safely share
// decoded audio and peaks; the original WAV is embedded when saving a project.
type Sample struct {
	peak           float32
	name           string
	wav            []byte
	data           []float32
	rate, channels int
	peaks          [WaveformBins][2]float32
}

func (s *Sample) Name() string { return s.name }
func (s *Sample) Frames() int {
	if s.channels == 0 {
		return 0
	}
	return len(s.data) / s.channels
}
func (s *Sample) Rate() int                       { return s.rate }
func (s *Sample) Channels() int                   { return s.channels }
func (s *Sample) Peaks() [WaveformBins][2]float32 { return s.peaks }

func (s *Sample) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name string `json:"name"`
		WAV  []byte `json:"wav"`
	}{s.name, s.wav})
}
func (s *Sample) UnmarshalJSON(data []byte) error {
	var asset struct {
		Name string `json:"name"`
		WAV  []byte `json:"wav"`
	}
	if err := json.Unmarshal(data, &asset); err != nil {
		return err
	}
	decoded, err := LoadSample(asset.Name, bytes.NewReader(asset.WAV))
	if err != nil {
		return err
	}
	*s = *decoded
	return nil
}

// LoadSample accepts PCM 8/16/24/32-bit and IEEE float32 WAV, mono or stereo.
// Chunk bounds, padding and frame alignment are validated before allocating DSP data.
func LoadSample(name string, reader io.Reader) (*Sample, error) {
	wav, err := io.ReadAll(io.LimitReader(reader, MaxSampleBytes+1))
	if err != nil {
		return nil, err
	}
	if len(wav) > MaxSampleBytes {
		return nil, fmt.Errorf("WAV exceeds 16 MiB")
	}
	if len(wav) < 12 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, fmt.Errorf("expected a RIFF/WAVE file")
	}
	end := uint64(binary.LittleEndian.Uint32(wav[4:8])) + 8
	if end < 12 || end > uint64(len(wav)) {
		return nil, fmt.Errorf("truncated WAV container")
	}
	var format, channels, bits, align int
	var rate int
	var audio []byte
	for pos := 12; pos < int(end); {
		if int(end)-pos < 8 {
			return nil, fmt.Errorf("truncated WAV chunk")
		}
		size := uint64(binary.LittleEndian.Uint32(wav[pos+4 : pos+8]))
		stop := uint64(pos) + 8 + size
		if stop > end {
			return nil, fmt.Errorf("truncated WAV chunk data")
		}
		chunk := wav[pos+8 : int(stop)]
		switch string(wav[pos : pos+4]) {
		case "fmt ":
			if len(chunk) < 16 || format != 0 {
				return nil, fmt.Errorf("invalid WAV format chunk")
			}
			format = int(binary.LittleEndian.Uint16(chunk))
			channels = int(binary.LittleEndian.Uint16(chunk[2:]))
			rate = int(binary.LittleEndian.Uint32(chunk[4:]))
			align = int(binary.LittleEndian.Uint16(chunk[12:]))
			bits = int(binary.LittleEndian.Uint16(chunk[14:]))
		case "data":
			if audio != nil {
				return nil, fmt.Errorf("multiple WAV data chunks are unsupported")
			}
			audio = chunk
		}
		pos = int(stop + size%2)
		if uint64(pos) > end {
			return nil, fmt.Errorf("missing WAV chunk padding")
		}
	}
	if (channels != 1 && channels != 2) || rate < 8000 || rate > 192000 {
		return nil, fmt.Errorf("WAV must be mono/stereo at 8–192 kHz")
	}
	if !((format == 1 && (bits == 8 || bits == 16 || bits == 24 || bits == 32)) || (format == 3 && bits == 32)) {
		return nil, fmt.Errorf("use PCM 8/16/24/32-bit or float32 WAV")
	}
	width := bits / 8
	if align != channels*width || len(audio) == 0 || len(audio)%align != 0 {
		return nil, fmt.Errorf("invalid or empty WAV audio frames")
	}
	s := &Sample{name: name, wav: wav, rate: rate, channels: channels, data: make([]float32, len(audio)/width)}
	for i := range s.data {
		b := audio[i*width:]
		var value float32
		if format == 3 {
			value = math.Float32frombits(binary.LittleEndian.Uint32(b))
		} else {
			switch bits {
			case 8:
				value = float32(int(b[0])-128) / 128
			case 16:
				value = float32(int16(binary.LittleEndian.Uint16(b))) / 32768
			case 24:
				n := int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16
				n = n << 8 >> 8
				value = float32(n) / 8388608
			case 32:
				value = float32(int32(binary.LittleEndian.Uint32(b))) / 2147483648
			}
		}
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, fmt.Errorf("WAV contains non-finite audio")
		}
		s.data[i] = max(-1, min(1, value))
		s.peak = max(s.peak, float32(math.Abs(float64(s.data[i]))))
	}
	for bin := range s.peaks {
		start := bin * s.Frames() / WaveformBins
		stop := min(s.Frames(), max(start+1, (bin+1)*s.Frames()/WaveformBins))
		for frame := start; frame < stop; frame++ {
			for ch := 0; ch < channels; ch++ {
				v := s.data[frame*channels+ch]
				s.peaks[bin][0] = min(s.peaks[bin][0], v)
				s.peaks[bin][1] = max(s.peaks[bin][1], v)
			}
		}
	}
	return s, nil
}

// NormalizationGain uses a peak cached during decoding. Silence stays silent.
// Both stereo channels share one gain so their balance is preserved.
func (s *Sample) NormalizationGain() float64 {
	if s == nil || s.peak == 0 {
		return 1
	}
	return .95 / float64(s.peak)
}

// PeaksRange analyzes a viewport on the UI/control thread, never in Render.
func (s *Sample) PeaksRange(start, end float64) [WaveformBins][2]float32 {
	var peaks [WaveformBins][2]float32
	first := max(0, min(s.Frames()-1, int(start*float64(s.Frames()))))
	last := max(first+1, min(s.Frames(), int(end*float64(s.Frames()))))
	for bin := range peaks {
		from := first + bin*(last-first)/WaveformBins
		to := min(last, max(from+1, first+(bin+1)*(last-first)/WaveformBins))
		for frame := from; frame < to; frame++ {
			for ch := 0; ch < s.channels; ch++ {
				v := s.data[frame*s.channels+ch]
				peaks[bin][0] = min(peaks[bin][0], v)
				peaks[bin][1] = max(peaks[bin][1], v)
			}
		}
	}
	return peaks
}
