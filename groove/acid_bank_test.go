package groove

import (
	"context"
	"fmt"
	"math"
	"math/cmplx"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// bankLine renders two bars of a fixed acid line through a 303 sound and
// returns its RMS and spectral centroid.
func bankLine(t *testing.T, preset SoundPreset) (rms, centroid float64) {
	t.Helper()
	p := NewProject()
	key := Key{Root: 9, Scale: PhraseMinor}
	p.Key = &key
	generate(&p.Tracks[0], 0, "Acid", &key, rand.New(rand.NewPCG(3, 4)))
	if err := ApplySound(&p.Tracks[0], preset, true); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(p.Tracks); i++ {
		p.Tracks[i].Mute = true
	}
	player := NewPlayer()
	player.Publish(p)
	player.SetPlaying(true)
	out := make([]float32, int(SampleRate*60/p.BPM*8)*2)
	player.Render(out)
	mono := make([]float64, 1<<16)
	sum := 0.0
	for i := range out[:len(out)/2] {
		v := float64(out[i*2]+out[i*2+1]) / 2
		sum += v * v
		if i < len(mono) {
			mono[i] = v
		}
	}
	spectrum := fft(mono)
	weighted, total := 0.0, 0.0
	for k := 1; k < len(spectrum)/2; k++ {
		m := cmplx.Abs(spectrum[k])
		weighted += m * float64(k) * SampleRate / float64(len(spectrum))
		total += m
	}
	return math.Sqrt(sum / float64(len(out)/2)), weighted / total
}

// fft is a small radix-2 transform for test measurements.
func fft(x []float64) []complex128 {
	n := len(x)
	out := make([]complex128, n)
	for i, v := range x {
		out[i] = complex(v, 0)
	}
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			out[i], out[j] = out[j], out[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		w := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			wn := complex(1, 0)
			for k := 0; k < size/2; k++ {
				a, b := out[start+k], out[start+k+size/2]*wn
				out[start+k], out[start+k+size/2] = a+b, a-b
				wn *= w
			}
		}
	}
	return out
}

// The bank must be broad (sounds spread across the spectrum), have unique
// names, and every patch must sit near the plain voice's loudness.
func TestAcidBankIsVariedAndBalanced(t *testing.T) {
	bank := FactorySounds(Acid)
	if len(bank) < 24 {
		t.Fatalf("bank has %d patches", len(bank))
	}
	plain, _ := bankLine(t, CaptureSound(NewTrack(Acid, 0), "Plain"))
	names := map[string]bool{}
	centroids := []float64{}
	for _, preset := range bank {
		if names[preset.Name] {
			t.Fatalf("duplicate patch name %q", preset.Name)
		}
		names[preset.Name] = true
		rms, centroid := bankLine(t, preset)
		db := 20 * math.Log10(rms/plain)
		t.Logf("%-15s %+5.1f dB  centroid %5.0f Hz", preset.Name, db, centroid)
		if db < -12 || db > 9 {
			t.Errorf("%s is %+.1f dB against the plain voice", preset.Name, db)
		}
		centroids = append(centroids, centroid)
	}
	sort.Float64s(centroids)
	if spread := centroids[len(centroids)-1] / centroids[0]; spread < 3 {
		t.Errorf("bank brightness spans only %.1f×", spread)
	}
}

func TestStylesNameRealBankSoundsAndBoards(t *testing.T) {
	for _, style := range GrooveStyles {
		recipes := map[string]partRecipe{"lead": style.lead, "second": style.second}
		for kind, recipe := range style.drums {
			recipes[string(kind)] = recipe
		}
		for part, recipe := range recipes {
			kind := Acid
			if part != "lead" && part != "second" {
				kind = Kind(part)
			}
			for _, name := range recipe.patches {
				if _, ok := FactorySoundNamed(kind, name); !ok {
					t.Errorf("%s %s: no bank sound %q", style.Name, part, name)
				}
			}
			for _, name := range recipe.boards {
				if _, ok := PedalboardNamed(name); !ok {
					t.Errorf("%s %s: no pedalboard %q", style.Name, part, name)
				}
			}
		}
	}
}

// TestWriteBankSamples plays one groove with every 303 bank sound on the
// lead (second 303 muted):
// GRUVBOX_BANK_WAVS=<dir> go test ./groove -run TestWriteBankSamples
func TestWriteBankSamples(t *testing.T) {
	dir := os.Getenv("GRUVBOX_BANK_WAVS")
	if dir == "" {
		t.Skip("set GRUVBOX_BANK_WAVS to a directory to hear the 303 sound bank")
	}
	r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	base := NewGrooveInStyle(GrooveStyles[0], r)
	base.Song = []SongSection{NewSongSection(0)}
	base.Tracks[1].Mute = true
	for i, preset := range FactorySounds(Acid) {
		p := base.Clone()
		if err := ApplySound(&p.Tracks[0], preset, true); err != nil {
			t.Fatal(err)
		}
		p.Tracks[0].Parameters["tune"] = base.Tracks[0].Parameters["tune"]
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%02d %s.wav", i+1, preset.Name)))
		if err != nil {
			t.Fatal(err)
		}
		if err := RenderWAV(context.Background(), f, p, true, 1, nil); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}

// drumRMS plays a hit on every beat for two bars with the given sound.
func drumRMS(t *testing.T, kind Kind, preset SoundPreset) float64 {
	t.Helper()
	track := NewTrack(kind, 0)
	if err := ApplySound(&track, preset, true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i += 4 {
		track.Patterns[0].Steps[i].Active = true
	}
	player := NewPlayer()
	player.Publish(Project{FormatVersion: FormatVersion, BPM: 120, Tracks: []Track{track}})
	player.SetPlaying(true)
	out := make([]float32, int(SampleRate*4)*2)
	player.Render(out)
	sum := 0.0
	for _, v := range out {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum / float64(len(out)))
}

func TestDrumBanksAreBalanced(t *testing.T) {
	for _, kind := range Kinds {
		if kind == Acid || kind == Dual || kind == Sampler {
			continue
		}
		bank := FactorySounds(kind)
		if len(bank) < 4 {
			t.Errorf("%s has only %d sounds", kind, len(bank))
		}
		plain := drumRMS(t, kind, CaptureSound(NewTrack(kind, 0), "Plain"))
		names := map[string]bool{}
		for _, preset := range bank {
			if names[preset.Name] {
				t.Fatalf("%s: duplicate %q", kind, preset.Name)
			}
			names[preset.Name] = true
			db := 20 * math.Log10(drumRMS(t, kind, preset)/plain)
			t.Logf("%-10s %-15s %+5.1f dB", kind, preset.Name, db)
			if db < -14 || db > 9 {
				t.Errorf("%s %s is %+.1f dB against the default", kind, preset.Name, db)
			}
		}
	}
}
