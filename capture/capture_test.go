package capture

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSourcesClassificationAndDefaults(t *testing.T) {
	sources, err := parseSources([]byte(`[
		{"name":"speaker.monitor","description":"Speakers","monitor_of_sink":0},
		{"name":"mic","description":"Microphone","monitor_of_sink":4294967295},
		{"name":"custom-output","monitor_of_sink":"1"},
		{"name":"line","monitor_of_sink":"4294967295"},
		{"name":"pulse-monitor","monitor_source":"sink"},
		{"name":"mic"},{"description":"missing name"}
	]`))
	if err != nil || len(sources) != 5 || !sources[0].Monitor || sources[1].Monitor || !sources[2].Monitor || sources[3].Monitor || !sources[4].Monitor || sources[2].Description != sources[2].Name {
		t.Fatal("source classification", sources, err)
	}
	defaults := DefaultSources()
	if len(defaults) != 2 || defaults[0].Name != "@DEFAULT_MONITOR@" || defaults[1].Name != "@DEFAULT_SOURCE@" {
		t.Fatal("default aliases unavailable")
	}
	if _, err := parseSources([]byte("not json")); err == nil {
		t.Fatal("invalid source list accepted")
	}
}

func TestCaptureWriterOddChunksLimitsAndClipping(t *testing.T) {
	var pcm bytes.Buffer
	binary.Write(&pcm, binary.LittleEndian, []int16{12000, -32768, 32767, -8000, 1000, -1000})
	cancelled := false
	r := &Recording{channels: 2, maximum: 8, cancel: func() { cancelled = true }}
	data := pcm.Bytes()
	for _, chunk := range [][]byte{data[:1], data[1:3], data[3:7], data[7:]} {
		if n, err := r.Write(chunk); err != nil || n != len(chunk) {
			t.Fatal("stream write failed")
		}
	}
	stats := r.Stats()
	if !cancelled || !stats.Limited || !stats.Clipped || stats.Frames != 2 || !bytes.Equal(r.pcm, data[:8]) {
		t.Fatal("capture bound, alignment, meter or clipping failed", stats)
	}
	// Further bytes can drain without growing the take.
	r.Write(make([]byte, 100))
	if len(r.pcm) != 8 {
		t.Fatal("recording grew past its limit")
	}
}

func TestCaptureHelperProcess(t *testing.T) {
	if os.Getenv("GRUVBOX_CAPTURE_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	switch mode {
	case "short":
		os.Stdout.Write([]byte{0, 32, 0, 224, 0, 16, 0, 240, 7})
	case "error":
		fmt.Fprintln(os.Stderr, "source disappeared")
		os.Exit(3)
	case "stream":
		chunk := bytes.Repeat([]byte{0, 32, 0, 224}, 256)
		for {
			if _, err := os.Stdout.Write(chunk); err != nil {
				os.Exit(0)
			}
			time.Sleep(time.Millisecond)
		}
	}
	os.Exit(0)
}

func helper(mode string) func(context.Context) *exec.Cmd {
	return func(ctx context.Context) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCaptureHelperProcess$", "--", mode)
		cmd.Env = append(os.Environ(), "GRUVBOX_CAPTURE_HELPER=1")
		return cmd
	}
}

func waitDeadline(t *testing.T, r *Recording) {
	t.Helper()
	select {
	case <-r.Done():
	case <-time.After(5 * time.Second):
		r.Stop()
		t.Fatal("capture process did not terminate")
	}
}

func TestCaptureProcessStopLimitErrorAndFrameAlignment(t *testing.T) {
	r, err := start(context.Background(), "test", 2, 1, helper("short"))
	if err != nil {
		t.Fatal(err)
	}
	waitDeadline(t, r)
	pcm, err := r.Wait()
	if err != nil || len(pcm) != 8 || r.Stats().Frames != 2 {
		t.Fatal("partial final stereo frame was not discarded", len(pcm), err)
	}
	r, err = start(context.Background(), "test", 2, 1, helper("stream"))
	if err != nil {
		t.Fatal(err)
	}
	waitDeadline(t, r)
	pcm, err = r.Wait()
	if err != nil || len(pcm) != Rate*4 || !r.Stats().Limited {
		t.Fatal("automatic duration limit failed", len(pcm), err)
	}
	r, err = start(context.Background(), "test", 1, 90, helper("stream"))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for r.Stats().Frames == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 10; i++ {
		go r.Stop()
	}
	waitDeadline(t, r)
	pcm, err = r.Wait()
	if err != nil || len(pcm) == 0 || len(pcm) >= r.maximum {
		t.Fatal("manual stop did not preserve a short take", len(pcm), err)
	}
	r, err = start(context.Background(), "test", 2, 1, helper("error"))
	if err != nil {
		t.Fatal(err)
	}
	waitDeadline(t, r)
	if _, err := r.Wait(); err == nil || !strings.Contains(err.Error(), "source disappeared") {
		t.Fatal("backend error was hidden", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r, err = start(ctx, "test", 2, 90, helper("stream"))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	waitDeadline(t, r)
	if _, err := r.Wait(); err == nil {
		t.Fatal("cancelled capture returned a successful take")
	}
}

func TestCaptureConfigurationRejectsUnboundedRequests(t *testing.T) {
	for _, config := range []struct {
		source            string
		channels, seconds int
	}{{"", 2, 1}, {"test", 3, 1}, {"test", 2, 0}, {"test", 2, 91}} {
		if _, err := start(context.Background(), config.source, config.channels, config.seconds, helper("short")); err == nil {
			t.Fatal("invalid recording configuration accepted")
		}
	}
}
