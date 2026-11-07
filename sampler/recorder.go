package sampler

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Recorder captures system audio using PulseAudio
type Recorder struct {
	recording   bool
	recordingMu sync.Mutex

	cmd         *exec.Cmd
	sampleRate  int
	channels    int

	recordedData []int16 // Raw PCM data
	dataMu       sync.Mutex

	tempFile string
}

// NewRecorder creates a new audio recorder
func NewRecorder() *Recorder {
	return &Recorder{
		recording:    false,
		sampleRate:   44100,
		channels:     2, // Stereo
		recordedData: make([]int16, 0),
		tempFile:     "",
	}
}

// getDefaultMonitor gets the default sink's monitor source
func getDefaultMonitor() (string, error) {
	// Get default sink name
	cmd := exec.Command("pactl", "info")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run pactl info: %w", err)
	}

	// Parse output to find "Default Sink:"
	lines := strings.Split(string(output), "\n")
	var defaultSink string
	for _, line := range lines {
		if strings.Contains(line, "Default Sink:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				defaultSink = strings.TrimSpace(parts[1])
				break
			}
		}
	}

	if defaultSink == "" {
		return "", fmt.Errorf("could not find default sink")
	}

	// Monitor source is the sink name + ".monitor"
	return defaultSink + ".monitor", nil
}

// StartRecording starts capturing system audio
// Uses PulseAudio's monitor source to capture what's playing
func (r *Recorder) StartRecording() error {
	r.recordingMu.Lock()
	defer r.recordingMu.Unlock()

	if r.recording {
		return fmt.Errorf("already recording")
	}

	// Clear previous recording
	r.dataMu.Lock()
	r.recordedData = make([]int16, 0, r.sampleRate*r.channels*10) // Pre-allocate for ~10 seconds
	r.dataMu.Unlock()

	// Get the default monitor source
	monitorSource, err := getDefaultMonitor()
	if err != nil {
		return fmt.Errorf("failed to get monitor source: %w", err)
	}

	// Create temporary file for recording
	tmpFile, err := os.CreateTemp("", "gruvbox_rec_*.raw")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	r.tempFile = tmpFile.Name()
	tmpFile.Close()

	// Start parec to capture monitor (system audio)
	r.cmd = exec.Command("parec",
		"--format=s16le",           // 16-bit signed little-endian
		"--rate=44100",              // 44.1kHz sample rate
		"--channels=2",              // Stereo
		"--device="+monitorSource,  // Capture from monitor source
		r.tempFile,                  // Output to temp file
	)

	// Start the recording process
	if err := r.cmd.Start(); err != nil {
		os.Remove(r.tempFile)
		return fmt.Errorf("failed to start parec: %w (is PulseAudio installed?)", err)
	}

	r.recording = true
	return nil
}

// StopRecording stops the recording and returns the captured audio as a Sample
func (r *Recorder) StopRecording() (*Sample, error) {
	r.recordingMu.Lock()
	defer r.recordingMu.Unlock()

	if !r.recording {
		return nil, fmt.Errorf("not recording")
	}

	// Stop the parec process
	if r.cmd != nil && r.cmd.Process != nil {
		// Send SIGTERM to gracefully stop
		r.cmd.Process.Signal(os.Interrupt)

		// Wait for process to finish (with timeout)
		done := make(chan error, 1)
		go func() {
			done <- r.cmd.Wait()
		}()

		select {
		case <-done:
			// Process finished
		case <-time.After(2 * time.Second):
			// Timeout - force kill
			r.cmd.Process.Kill()
		}
	}

	r.recording = false

	// Read the recorded data from temp file
	if r.tempFile == "" {
		return nil, fmt.Errorf("no temp file")
	}

	file, err := os.Open(r.tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open recorded file: %w", err)
	}
	defer file.Close()
	defer os.Remove(r.tempFile) // Clean up temp file

	// Read raw PCM data
	reader := bufio.NewReader(file)
	var samples []int16

	for {
		var sample int16
		err := binary.Read(reader, binary.LittleEndian, &sample)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read sample data: %w", err)
		}
		samples = append(samples, sample)
	}

	if len(samples) == 0 {
		return nil, fmt.Errorf("no audio data recorded")
	}

	// Convert to Sample
	sample := &Sample{
		Name:       fmt.Sprintf("Recording_%s", time.Now().Format("15:04:05")),
		Data:       make([]float32, len(samples)),
		Channels:   r.channels,
		SampleRate: r.sampleRate,
		Length:     len(samples) / r.channels,
		TrimStart:  0,
		TrimEnd:    len(samples) / r.channels,
	}

	// Convert int16 to float32
	for i, s := range samples {
		sample.Data[i] = float32(s) / 32768.0
	}

	return sample, nil
}

// IsRecording returns whether currently recording
func (r *Recorder) IsRecording() bool {
	r.recordingMu.Lock()
	defer r.recordingMu.Unlock()
	return r.recording
}

// GetDuration returns the current recording duration
func (r *Recorder) GetDuration() time.Duration {
	if !r.IsRecording() {
		return 0
	}

	// Get file size to estimate duration
	if r.tempFile == "" {
		return 0
	}

	info, err := os.Stat(r.tempFile)
	if err != nil {
		return 0
	}

	// Calculate duration from file size
	// bytes = samples * 2 (16-bit) * channels
	bytes := info.Size()
	samples := bytes / (2 * int64(r.channels))
	duration := time.Duration(samples) * time.Second / time.Duration(r.sampleRate)

	return duration
}
