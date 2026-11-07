package sampler

import (
	"encoding/binary"
	"fmt"
	"os"
)

// Sample represents an audio sample that can be triggered
type Sample struct {
	Name       string    // Sample name
	FilePath   string    // Path to source WAV file (if loaded from file)
	Data       []float32 // Audio data (mono or interleaved stereo)
	Channels   int       // 1 = mono, 2 = stereo
	SampleRate int       // Sample rate (usually 44100)
	Length     int       // Length in samples

	// Trim points (in samples)
	TrimStart int
	TrimEnd   int
}

// NewSample creates a new empty sample
func NewSample(name string) *Sample {
	return &Sample{
		Name:       name,
		Data:       make([]float32, 0),
		Channels:   1,
		SampleRate: 44100,
		Length:     0,
		TrimStart:  0,
		TrimEnd:    0,
	}
}

// LoadWAV loads a WAV file into the sample
// Supports 16-bit PCM mono/stereo WAV files
func (s *Sample) LoadWAV(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open WAV file: %w", err)
	}
	defer file.Close()

	// Read RIFF header
	var riff [12]byte
	if _, err := file.Read(riff[:]); err != nil {
		return fmt.Errorf("failed to read RIFF header: %w", err)
	}

	// Verify RIFF/WAVE format
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return fmt.Errorf("not a valid WAV file")
	}

	// Read chunks until we find fmt and data
	var fmtFound, dataFound bool
	var numChannels, sampleRate, bitsPerSample int
	var dataSize int

	for !fmtFound || !dataFound {
		var chunkHeader [8]byte
		if _, err := file.Read(chunkHeader[:]); err != nil {
			return fmt.Errorf("failed to read chunk header: %w", err)
		}

		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		switch chunkID {
		case "fmt ":
			// Read format chunk
			var fmtData [16]byte
			if _, err := file.Read(fmtData[:]); err != nil {
				return fmt.Errorf("failed to read fmt chunk: %w", err)
			}

			audioFormat := binary.LittleEndian.Uint16(fmtData[0:2])
			if audioFormat != 1 {
				return fmt.Errorf("only PCM format supported (got format %d)", audioFormat)
			}

			numChannels = int(binary.LittleEndian.Uint16(fmtData[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(fmtData[4:8]))
			bitsPerSample = int(binary.LittleEndian.Uint16(fmtData[14:16]))

			if bitsPerSample != 16 {
				return fmt.Errorf("only 16-bit samples supported (got %d-bit)", bitsPerSample)
			}

			// Skip any extra format bytes
			if chunkSize > 16 {
				file.Seek(int64(chunkSize-16), 1)
			}

			fmtFound = true

		case "data":
			dataSize = int(chunkSize)
			dataFound = true

		default:
			// Skip unknown chunks
			file.Seek(int64(chunkSize), 1)
		}
	}

	// Read audio data
	numSamples := dataSize / (bitsPerSample / 8)
	rawData := make([]int16, numSamples)
	if err := binary.Read(file, binary.LittleEndian, rawData); err != nil {
		return fmt.Errorf("failed to read audio data: %w", err)
	}

	// Convert to float32 [-1.0, 1.0]
	s.Data = make([]float32, numSamples)
	for i, sample := range rawData {
		s.Data[i] = float32(sample) / 32768.0
	}

	s.Channels = numChannels
	s.SampleRate = sampleRate
	s.Length = numSamples / numChannels
	s.TrimStart = 0
	s.TrimEnd = s.Length
	s.FilePath = filename // Store the file path for project saving

	return nil
}

// FindNearestZeroCrossing finds the nearest zero-crossing point to the given index
// This helps prevent clicks when trimming samples
func (s *Sample) FindNearestZeroCrossing(index int, searchRadius int) int {
	if s.Channels == 1 {
		return s.findZeroCrossingMono(index, searchRadius)
	}
	return s.findZeroCrossingStereo(index, searchRadius)
}

// findZeroCrossingMono finds zero crossing for mono samples
func (s *Sample) findZeroCrossingMono(index int, searchRadius int) int {
	if index < 0 || index >= len(s.Data) {
		return index
	}

	// Search forward and backward for zero crossing
	bestIndex := index
	minValue := abs(s.Data[index])

	for offset := 1; offset <= searchRadius; offset++ {
		// Check forward
		if index+offset < len(s.Data) {
			val := abs(s.Data[index+offset])
			if val < minValue {
				minValue = val
				bestIndex = index + offset
			}
			// Found actual zero crossing (sign change)
			if index+offset > 0 &&
			   ((s.Data[index+offset-1] >= 0 && s.Data[index+offset] <= 0) ||
			    (s.Data[index+offset-1] <= 0 && s.Data[index+offset] >= 0)) {
				return index + offset
			}
		}

		// Check backward
		if index-offset >= 0 {
			val := abs(s.Data[index-offset])
			if val < minValue {
				minValue = val
				bestIndex = index - offset
			}
			// Found actual zero crossing (sign change)
			if index-offset < len(s.Data)-1 &&
			   ((s.Data[index-offset] >= 0 && s.Data[index-offset+1] <= 0) ||
			    (s.Data[index-offset] <= 0 && s.Data[index-offset+1] >= 0)) {
				return index - offset
			}
		}
	}

	return bestIndex
}

// findZeroCrossingStereo finds zero crossing for stereo samples
func (s *Sample) findZeroCrossingStereo(index int, searchRadius int) int {
	dataIndex := index * 2
	if dataIndex < 0 || dataIndex >= len(s.Data)-1 {
		return index
	}

	bestIndex := index
	minValue := abs(s.Data[dataIndex]) + abs(s.Data[dataIndex+1])

	for offset := 1; offset <= searchRadius; offset++ {
		// Check forward
		forwardIdx := (index + offset) * 2
		if forwardIdx < len(s.Data)-1 {
			val := abs(s.Data[forwardIdx]) + abs(s.Data[forwardIdx+1])
			if val < minValue {
				minValue = val
				bestIndex = index + offset
			}
		}

		// Check backward
		backwardIdx := (index - offset) * 2
		if backwardIdx >= 0 && backwardIdx < len(s.Data)-1 {
			val := abs(s.Data[backwardIdx]) + abs(s.Data[backwardIdx+1])
			if val < minValue {
				minValue = val
				bestIndex = index - offset
			}
		}
	}

	return bestIndex
}

// GetTrimmedData returns the audio data between trim points with click-free boundaries
func (s *Sample) GetTrimmedData() []float32 {
	// Find zero crossings near trim points (search within 100 samples)
	start := s.FindNearestZeroCrossing(s.TrimStart, 100)
	end := s.FindNearestZeroCrossing(s.TrimEnd, 100)

	if s.Channels == 1 {
		if start < 0 {
			start = 0
		}
		if end > len(s.Data) {
			end = len(s.Data)
		}
		if start >= end {
			return []float32{}
		}

		// Get trimmed data
		trimmed := make([]float32, end-start)
		copy(trimmed, s.Data[start:end])

		// Apply short fade-in and fade-out to eliminate any remaining clicks
		fadeLength := 64 // Very short fade (about 1.5ms at 44.1kHz)
		if len(trimmed) > fadeLength*2 {
			// Fade in
			for i := 0; i < fadeLength; i++ {
				fade := float32(i) / float32(fadeLength)
				trimmed[i] *= fade
			}
			// Fade out
			for i := 0; i < fadeLength; i++ {
				fade := float32(fadeLength-i) / float32(fadeLength)
				trimmed[len(trimmed)-1-i] *= fade
			}
		}

		return trimmed
	}

	// Stereo: trim in stereo pairs
	startIdx := start * 2
	endIdx := end * 2
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(s.Data) {
		endIdx = len(s.Data)
	}
	if startIdx >= endIdx {
		return []float32{}
	}

	// Get trimmed data
	trimmed := make([]float32, endIdx-startIdx)
	copy(trimmed, s.Data[startIdx:endIdx])

	// Apply short fade-in and fade-out (in stereo)
	fadeLength := 64
	if len(trimmed)/2 > fadeLength*2 {
		// Fade in
		for i := 0; i < fadeLength; i++ {
			fade := float32(i) / float32(fadeLength)
			trimmed[i*2] *= fade     // Left
			trimmed[i*2+1] *= fade   // Right
		}
		// Fade out
		for i := 0; i < fadeLength; i++ {
			fade := float32(fadeLength-i) / float32(fadeLength)
			trimmed[len(trimmed)-2-(i*2)] *= fade     // Left
			trimmed[len(trimmed)-1-(i*2)] *= fade     // Right
		}
	}

	return trimmed
}

// GetDuration returns the sample duration in seconds
func (s *Sample) GetDuration() float64 {
	if s.SampleRate == 0 {
		return 0
	}
	return float64(s.Length) / float64(s.SampleRate)
}

// GetTrimmedDuration returns the trimmed sample duration in seconds
func (s *Sample) GetTrimmedDuration() float64 {
	if s.SampleRate == 0 {
		return 0
	}
	return float64(s.TrimEnd-s.TrimStart) / float64(s.SampleRate)
}

// SaveWAV saves the sample as a WAV file
func (s *Sample) SaveWAV(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create WAV file: %w", err)
	}
	defer file.Close()

	// Calculate sizes
	numSamples := len(s.Data)
	dataSize := numSamples * 2 // 16-bit = 2 bytes per sample
	fileSize := 36 + dataSize

	// Write RIFF header
	file.Write([]byte("RIFF"))
	binary.Write(file, binary.LittleEndian, uint32(fileSize))
	file.Write([]byte("WAVE"))

	// Write fmt chunk
	file.Write([]byte("fmt "))
	binary.Write(file, binary.LittleEndian, uint32(16)) // fmt chunk size
	binary.Write(file, binary.LittleEndian, uint16(1))  // PCM format
	binary.Write(file, binary.LittleEndian, uint16(s.Channels))
	binary.Write(file, binary.LittleEndian, uint32(s.SampleRate))
	binary.Write(file, binary.LittleEndian, uint32(s.SampleRate*s.Channels*2)) // Byte rate
	binary.Write(file, binary.LittleEndian, uint16(s.Channels*2))              // Block align
	binary.Write(file, binary.LittleEndian, uint16(16))                        // Bits per sample

	// Write data chunk
	file.Write([]byte("data"))
	binary.Write(file, binary.LittleEndian, uint32(dataSize))

	// Convert float32 to int16 and write
	for _, sample := range s.Data {
		// Clamp to [-1.0, 1.0]
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}
		// Convert to int16
		int16Sample := int16(sample * 32767.0)
		binary.Write(file, binary.LittleEndian, int16Sample)
	}

	return nil
}
