package groove

import (
	"math"
	"sync"
	"testing"
)

func TestAudioTimingSilenceAndNoAllocations(t *testing.T) {
	p := NewProject()
	p.BPM = 120
	for i := range p.Tracks {
		Generate(&p.Tracks[i], 0, "16ths")
	}
	p.Tracks[0].Patterns[0].Steps = p.Tracks[0].Patterns[0].Steps[:13]
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	buffer := make([]float32, BlockSize*2)
	player.Render(buffer)
	energy := 0.0
	for _, sample := range buffer {
		energy += math.Abs(float64(sample))
	}
	if energy == 0 {
		t.Fatal("active groove rendered silence")
	}
	allocs := testing.AllocsPerRun(100, func() { player.Render(buffer) })
	if allocs != 0 {
		t.Fatalf("audio allocated %.2f times per block", allocs)
	}
	player.SetPlaying(false)
	player.Render(buffer)
	if !silentAfterMasterLatency(buffer) {
		t.Fatal("stop did not silence output")
	}
	player.SetPlaying(true)
	player.Render(make([]float32, 5513*2))
	if player.Position(p.Tracks[0].ID) != 0 {
		t.Fatal("sixteenth advanced before its sample boundary")
	}
	player.Render(make([]float32, 2))
	if player.Position(p.Tracks[0].ID) != 1 {
		t.Fatal("sixteenth note timing is wrong")
	}
}
func TestConcurrentPublicationAndRendering(t *testing.T) {
	player := NewPlayer()
	p := NewProject()
	player.Publish(p)
	player.SetPlaying(true)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var out [BlockSize * 2]float32
		for i := 0; i < 500; i++ {
			player.Render(out[:])
		}
	}()
	for i := 0; i < 100; i++ {
		p.Tracks[0].Parameters["cutoff"] = float64(i) / 100
		if i%10 == 0 {
			p.Tracks = append(p.Tracks, NewTrack(Kick, i))
		}
		player.Publish(p)
		_ = player.Position(p.Tracks[0].ID)
	}
	wg.Wait()
}
func BenchmarkRenderSixTracks(b *testing.B) {
	p := NewProject()
	for i := range p.Tracks {
		Generate(&p.Tracks[i], 0, "16ths")
	}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	var out [BlockSize * 2]float32
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		player.Render(out[:])
	}
}
