package drums

import (
	"math"
	"testing"

	"github.com/subbaan/gruvbox/dsp"
)

func TestClapDelayedLayersRespectVelocity(t *testing.T) {
	loud, quiet := NewClapDrum(44100), NewClapDrum(44100)
	loud.SetDrive(0)
	quiet.SetDrive(0)
	loud.Trigger(1)
	quiet.Trigger(.25)
	var a, b [2000]float32
	loud.Process(a[:])
	quiet.Process(b[:])
	for i := 300; i < len(a); i++ {
		if math.Abs(float64(b[i]-a[i]*.25)) > 1e-5 {
			t.Fatalf("delayed clap velocity at frame %d", i)
		}
	}
	silent := NewClapDrum(44100)
	silent.Trigger(0)
	silent.Process(a[:])
	for _, value := range a {
		if value != 0 {
			t.Fatal("zero velocity clap produced delayed hits")
		}
	}
}
func TestCrashMetallicLFOChangesTheMix(t *testing.T) {
	modulated, reference := NewCrashDrum(44100), NewCrashDrum(44100)
	modulated.SetMetallic(0)
	modulated.SetLFO1Target(CrashLFOTargetMetallic)
	modulated.SetLFO1Enabled(true)
	modulated.GetLFO1().SetRange(.5, .5)
	modulated.GetLFO1().SetWaveform(dsp.LFOSine)
	// A constant modulation range lets us compare against the same base value.
	reference.SetMetallic(.5)
	modulated.Trigger(1)
	reference.Trigger(1)
	var a, b [1024]float32
	modulated.Process(a[:])
	reference.Process(b[:])
	for i := range a {
		if math.Abs(float64(a[i]-b[i])) > 1e-6 {
			t.Fatalf("metallic mix ignored LFO at frame %d", i)
		}
	}
}
