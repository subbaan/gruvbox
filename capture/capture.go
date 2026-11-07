// Package capture records Linux audio independently of the playback callback.
// PulseAudio tools also work with PipeWire's PulseAudio compatibility service.
package capture

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
)

const Rate = 44100
const MaxSeconds = 90 // 15.2 MiB stereo PCM: fits the sampler's 16 MiB WAV limit.

type Source struct {
	Name, Description string
	Monitor           bool
}

func DefaultSources() []Source {
	return []Source{{"@DEFAULT_MONITOR@", "System output (default)", true}, {"@DEFAULT_SOURCE@", "Input (default)", false}}
}

// Source discovery is optional: the default aliases work even without pactl.
func Sources(ctx context.Context) ([]Source, error) {
	command := exec.CommandContext(ctx, "pactl", "--format=json", "list", "sources")
	var stdout, stderr boundedText
	stdout.limit, stderr.limit = 1024*1024, 4096
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("list audio sources: %w: %s", err, strings.TrimSpace(string(stderr.data)))
	}
	if stdout.overflow {
		return nil, fmt.Errorf("audio source list exceeds 1 MiB")
	}
	return parseSources(stdout.data)
}

func parseSources(data []byte) ([]Source, error) {
	var values []struct {
		Name          string            `json:"name"`
		Description   string            `json:"description"`
		Monitor       json.RawMessage   `json:"monitor_of_sink"`
		MonitorSource string            `json:"monitor_source"`
		Properties    map[string]string `json:"properties"`
	}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	var sources []Source
	seen := map[string]bool{}
	for _, value := range values {
		if value.Name == "" || seen[value.Name] {
			continue
		}
		seen[value.Name] = true
		monitor := strings.HasSuffix(value.Name, ".monitor") || value.MonitorSource != "" || value.Properties["device.class"] == "monitor"
		raw := strings.Trim(string(value.Monitor), "\"")
		if index, err := strconv.ParseInt(raw, 10, 64); err == nil && index >= 0 && index != 4294967295 {
			monitor = true
		}
		description := value.Description
		if description == "" {
			description = value.Name
		}
		sources = append(sources, Source{value.Name, description, monitor})
	}
	return sources, nil
}

type boundedText struct {
	data     []byte
	limit    int
	overflow bool
}

func (b *boundedText) Write(data []byte) (int, error) {
	room := max(0, b.limit-len(b.data))
	b.data = append(b.data, data[:min(len(data), room)]...)
	b.overflow = b.overflow || len(data) > room
	return len(data), nil
}

type Stats struct {
	Frames           int64
	Peak             float64
	Clipped, Limited bool
}

type Recording struct {
	channels                  int
	pcm                       []byte // writer-owned until done closes
	maximum                   int
	frames                    atomic.Int64
	peak                      atomic.Uint64
	clipped, limited, stopped atomic.Bool
	partial                   byte
	hasPartial                bool
	cancel                    context.CancelFunc
	done                      chan struct{}
	err                       error // published by closing done
}

func Start(ctx context.Context, source string, channels, seconds int) (*Recording, error) {
	if _, err := exec.LookPath("parec"); err != nil {
		return nil, fmt.Errorf("recording needs parec from PulseAudio tools (also works with PipeWire's PulseAudio service): %w", err)
	}
	return start(ctx, source, channels, seconds, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "parec", "--raw", "--format=s16le", "--rate=44100", "--latency-msec=100", "--channels="+strconv.Itoa(channels), "--device="+source, "--client-name=Gruvbox", "--stream-name=Sample recording")
	})
}

func start(ctx context.Context, source string, channels, seconds int, command func(context.Context) *exec.Cmd) (*Recording, error) {
	if strings.TrimSpace(source) == "" || (channels != 1 && channels != 2) || seconds < 1 || seconds > MaxSeconds {
		return nil, fmt.Errorf("choose an audio source, mono/stereo and a 1–90 second limit")
	}
	ctx, cancel := context.WithCancel(ctx)
	r := &Recording{channels: channels, maximum: Rate * channels * 2 * seconds, cancel: cancel, done: make(chan struct{})}
	r.pcm = make([]byte, 0, r.maximum)
	cmd := command(ctx)
	var stderr boundedText
	stderr.limit = 4096
	cmd.Stdout, cmd.Stderr = r, &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start recording: %w", err)
	}
	go func() {
		err := cmd.Wait() // also waits for stdout/stderr copying to finish
		if err != nil && !r.stopped.Load() && !r.limited.Load() {
			r.err = fmt.Errorf("audio capture failed: %w: %s", err, strings.TrimSpace(string(stderr.data)))
		}
		cancel()
		close(r.done)
	}()
	return r, nil
}

// Write consumes the stream into bounded memory. Reaching the duration limit
// kills the helper and drains remaining pipe bytes without retaining more audio.
func (r *Recording) Write(data []byte) (int, error) {
	count := len(data)
	data = data[:min(len(data), r.maximum-len(r.pcm))]
	r.pcm = append(r.pcm, data...)
	peak := 0.0
	measure := func(value int16) {
		v := math.Abs(float64(value)) / 32768
		peak = max(peak, v)
		if value == -32768 || value == 32767 {
			r.clipped.Store(true)
		}
	}
	if r.hasPartial && len(data) > 0 {
		measure(int16(uint16(r.partial) | uint16(data[0])<<8))
		data = data[1:]
		r.hasPartial = false
	}
	for len(data) >= 2 {
		measure(int16(binary.LittleEndian.Uint16(data)))
		data = data[2:]
	}
	if len(data) > 0 {
		r.partial, r.hasPartial = data[0], true
	}
	r.peak.Store(math.Float64bits(peak))
	r.frames.Store(int64(len(r.pcm) / (2 * r.channels)))
	if len(r.pcm) == r.maximum {
		r.limited.Store(true)
		r.cancel()
	}
	return count, nil
}

func (r *Recording) Stats() Stats {
	return Stats{r.frames.Load(), math.Float64frombits(r.peak.Load()), r.clipped.Load(), r.limited.Load()}
}
func (r *Recording) Done() <-chan struct{} { return r.done }
func (r *Recording) Stop()                 { r.stopped.Store(true); r.cancel() }

// Wait returns complete interleaved PCM frames. A failed/disconnected source is
// reported even if it produced partial audio, rather than silently using a take.
func (r *Recording) Wait() ([]byte, error) {
	<-r.done
	if r.err != nil {
		return nil, r.err
	}
	pcm := r.pcm[:len(r.pcm)/(2*r.channels)*(2*r.channels)]
	if len(pcm) == 0 {
		return nil, fmt.Errorf("no audio captured; check the selected source and its input level")
	}
	return pcm, nil
}
