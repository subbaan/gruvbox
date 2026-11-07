package groove

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

// Every style must produce audible, unclipped audio across many seeds.
// Levels are logged so they can be compared between styles.
func TestGrooveStylesRenderAudibly(t *testing.T) {
	r := rand.New(rand.NewPCG(21, 34))
	for _, style := range GrooveStyles {
		var worstPeak, quietest float64 = 0, math.Inf(1)
		for n := 0; n < 6; n++ {
			p := NewGrooveInStyle(style, r)
			player := NewPlayer()
			player.Publish(p)
			player.SetPlaying(true)
			// Two bars.
			frames := int(SampleRate * 60 / p.BPM * 8)
			out := make([]float32, frames*2)
			player.Render(out)
			peak, sum := 0.0, 0.0
			for _, v := range out {
				x := math.Abs(float64(v))
				peak = max(peak, x)
				sum += x * x
			}
			rms := math.Sqrt(sum / float64(len(out)))
			worstPeak, quietest = max(worstPeak, peak), min(quietest, rms)
			if math.IsNaN(sum) || peak > 1.0001 {
				t.Fatalf("%s: peak %.3f", style.Name, peak)
			}
		}
		t.Logf("%-9s worst peak %.3f, quietest RMS %.4f", style.Name, worstPeak, quietest)
		if quietest < .01 {
			t.Errorf("%s: a groove is nearly silent (RMS %.4f)", style.Name, quietest)
		}
	}
}

// TestWriteStyleSamples writes two 8-bar WAVs per style for listening:
// GRUVBOX_GROOVE_WAVS=<dir> go test ./groove -run TestWriteStyleSamples
func TestWriteStyleSamples(t *testing.T) {
	dir := os.Getenv("GRUVBOX_GROOVE_WAVS")
	if dir == "" {
		t.Skip("set GRUVBOX_GROOVE_WAVS to a directory to write example grooves")
	}
	r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	for _, style := range GrooveStyles {
		for n := 1; n <= 2; n++ {
			p := NewGrooveInStyle(style, r)
			// Eight bars, as a one-section song, so a four-bar pattern plays twice.
			section := NewSongSection(0)
			section.Repeats = 8
			p.Song = []SongSection{section}
			name := fmt.Sprintf("%s-%d %dbar %.0fbpm %s.wav", style.Name, n, len(p.Tracks[0].Patterns[0].Steps)/16, p.BPM, p.Key.Name())
			path := filepath.Join(dir, name)
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := RenderWAV(context.Background(), f, p, true, 1, nil); err != nil {
				t.Fatal(err)
			}
			f.Close()
			t.Log(path)
		}
	}
}
