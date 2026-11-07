package groove

import (
	"encoding/binary"
	"io"
	"math"
	"sync"
	"testing"
)

func TestBufferedPlayheadFollowsConsumptionNotReadAhead(t *testing.T) {
	p := NewProject()
	p.Tracks = p.Tracks[:1]
	p.BPM = 120
	Generate(&p.Tracks[0], 0, "16ths")
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	buffer := make([]byte, SourceBufferFrames*4)
	queued := 0
	consumed := 0
	seen := map[int]bool{}
	for period := 0; period < 110; period++ {
		// Oto fills its source buffer asynchronously, separately from device pulls.
		if queued < SourceBufferFrames*4 {
			if _, err := io.ReadFull(player, buffer); err != nil {
				t.Fatal(err)
			}
			queued += len(buffer)
		}
		deviceBytes := 1024 * 4 // device periods can exceed the former 256-frame buffer
		if queued < deviceBytes {
			t.Fatal("source buffer cannot cover a device period")
		}
		queued -= deviceBytes
		consumed += 1024
		frame := consumedFrame(player.readBytes.Load(), queued)
		if frame != int64(consumed-1) {
			t.Fatal("queued audio was not subtracted from read-ahead")
		}
		got := player.cache[p.Tracks[0].ID].positionAt(frame)
		expected := int(math.Floor(float64(consumed-1)/5512.5)) % 16
		if got != expected {
			t.Fatalf("period %d: audible step %d, got %d, rendered %d", period, expected, got, player.Position(p.Tracks[0].ID))
		}
		seen[got] = true
	}
	if len(seen) != 16 {
		t.Fatalf("buffered display skipped notes: %v", seen)
	}
}

func TestStreamingBassAndKickMatchesOfflineSignal(t *testing.T) {
	p := NewProject()
	p.Tracks = []Track{NewTrack(Acid, 0), NewTrack(Kick, 1)}
	for i := range p.Tracks {
		Generate(&p.Tracks[i], 0, "16ths")
	}
	stream, offline := NewPlayer(), NewPlayer()
	stream.Publish(p)
	offline.Publish(p)
	stream.SetPlaying(true)
	offline.SetPlaying(true)
	const frames = BlockSize * 64
	expected := make([]float32, frames*2)
	offline.Render(expected)
	encoded := make([]byte, frames*4)
	// Exercise hardware-sized reads as well as byte reads across block boundaries.
	offsets := []int{4096, 17, 8192, 1024, 1, 527}
	for offset, n := 0, 0; offset < len(encoded); n++ {
		count := min(offsets[n%len(offsets)], len(encoded)-offset)
		if _, err := io.ReadFull(stream, encoded[offset:offset+count]); err != nil {
			t.Fatal(err)
		}
		offset += count
	}
	for i, v := range expected {
		got := int16(binary.LittleEndian.Uint16(encoded[i*2:]))
		want := int16(v * 32767)
		if got != want {
			t.Fatalf("stream introduced a gap or changed sample %d: %d != %d", i, got, want)
		}
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatal("non-finite synth output")
		}
	}
}

func TestPositionHistoryWrapAndConcurrentAccess(t *testing.T) {
	v := &voice{}
	if v.positionAt(-1) != -1 || v.positionAt(0) != -1 {
		t.Fatal("unstarted track has a position")
	}
	for n := 0; n < 512; n++ {
		v.recordPosition(uint64(n*100), n%64)
	}
	if v.positionAt(0) != -1 || v.positionAt(51099) != 510%64 || v.positionAt(51100) != 511%64 {
		t.Fatal("position history wrap lookup")
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for n := 512; n < 4096; n++ {
			v.recordPosition(uint64(n*100), n%64)
		}
	}()
	for frame := int64(51200); frame < 409600; frame += 100 {
		index := v.positionAt(frame)
		if index < -1 || index > 63 {
			t.Fatal("torn position event")
		}
	}
	wg.Wait()
	if n := testing.AllocsPerRun(100, func() { v.recordPosition(409600, 0); v.positionAt(409600) }); n != 0 {
		t.Fatalf("position publication allocated %v", n)
	}
}
