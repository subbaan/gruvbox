package groove

import (
	"math"
	"math/rand/v2"
	"testing"
)

// silentAfterMasterLatency reports whether a block rendered just after Stop is
// silent apart from the master bus look-ahead still draining.
func silentAfterMasterLatency(out []float32) bool {
	for _, v := range out[min(len(out), masterLatency*2):] {
		if v != 0 {
			return false
		}
	}
	return true
}

func TestMasterBusPassesQuietAudioWithGainAndDelay(t *testing.T) {
	m := newMasterBus()
	frames := 4000
	in := make([]float32, frames*2)
	for i := range frames {
		in[i*2] = float32(.1 * math.Sin(float64(i)*.05))
		in[i*2+1] = float32(-.08 * math.Sin(float64(i)*.03))
	}
	out := append([]float32(nil), in...)
	m.process(out)
	gain := float32(math.Pow(10, MasterGainDB/20))
	for i := range frames {
		want := [2]float32{}
		if i >= masterLatency {
			want = [2]float32{in[(i-masterLatency)*2] * gain, in[(i-masterLatency)*2+1] * gain}
		}
		if math.Abs(float64(out[i*2]-want[0])) > 1e-6 || math.Abs(float64(out[i*2+1]-want[1])) > 1e-6 {
			t.Fatalf("frame %d: got %v,%v want %v (quiet audio should only gain and delay)", i, out[i*2], out[i*2+1], want)
		}
	}
	if m.reduction != 1 {
		t.Fatalf("quiet audio was limited to %v", m.reduction)
	}
}

func TestMasterBusLimitsSmoothlyBelowCeilingAndRecovers(t *testing.T) {
	m := newMasterBus()
	ceiling := float32(math.Pow(10, MasterCeilingDB/20))
	// A loud 80 Hz tone with sudden 2× bursts, then a quiet tail.
	loud, quiet := SampleRate, SampleRate
	out := make([]float32, (loud+quiet)*2)
	for i := range loud {
		amp := .6
		if (i/2000)%3 == 0 {
			amp = 1.2
		}
		v := float32(amp * math.Sin(2*math.Pi*80*float64(i)/SampleRate))
		out[i*2], out[i*2+1] = v, -v
	}
	for i := loud; i < loud+quiet; i++ {
		v := float32(.01 * math.Sin(2*math.Pi*80*float64(i)/SampleRate))
		out[i*2], out[i*2+1] = v, v
	}
	in := append([]float32(nil), out...)
	m.process(out)
	for i, v := range out {
		if v > ceiling+1e-6 || v < -ceiling-1e-6 {
			t.Fatalf("sample %d is %v, over the %v ceiling", i, v, ceiling)
		}
	}
	// No clicks: the applied gain is a moving average over the look-ahead, so
	// it can move by at most 1/masterLookahead per frame.
	makeup := float32(math.Pow(10, MasterGainDB/20))
	previous := float32(-1)
	for i := masterLatency; i < loud; i++ {
		source := in[(i-masterLatency)*2] * makeup
		if math.Abs(float64(source)) < .05 {
			previous = -1 // too quiet to measure the gain reliably
			continue
		}
		g := out[i*2] / source
		if previous >= 0 && math.Abs(float64(g-previous)) > 1./masterLookahead+1e-4 {
			t.Fatalf("gain jumps from %v to %v at frame %d", previous, g, i)
		}
		previous = g
	}
	if m.reduction > .5 {
		t.Fatalf("bursts were not limited (lowest gain %v)", m.reduction)
	}
	if m.held < .99 {
		t.Fatalf("gain did not recover after the bursts (%v)", m.held)
	}
	var block [BlockSize * 2]float32
	if n := testing.AllocsPerRun(100, func() { m.process(block[:]) }); n != 0 {
		t.Fatalf("master bus allocated %v", n)
	}
}

// Generated grooves used to peak near -10 dBFS with RMS near -27 dBFS. Through
// the master bus they should sit at a normal level and never pass the ceiling.
func TestGeneratedGroovesComeOutAtANormalLevel(t *testing.T) {
	ceiling := math.Pow(10, MasterCeilingDB/20)
	for _, style := range GrooveStyles {
		for seed := uint64(1); seed <= 2; seed++ {
			p := NewGrooveInStyle(style, rand.New(rand.NewPCG(seed, 7)))
			player := NewPlayer()
			player.Publish(p)
			player.SetPlaying(true)
			frames := int(4 * 4 * 60 / p.BPM * SampleRate)
			var buf [BlockSize * 2]float32
			peak, sum, n := 0., 0., 0
			for done := 0; done < frames; done += BlockSize {
				player.Render(buf[:])
				for _, s := range buf {
					a := math.Abs(float64(s))
					peak = max(peak, a)
					sum += a * a
					n++
				}
			}
			rms := 10 * math.Log10(sum/float64(n))
			if peak > ceiling+1e-6 || rms < -24 || rms > -12 {
				t.Errorf("%s seed %d: peak %.1f dBFS, RMS %.1f dBFS; want peak ≤ %.0f and RMS -24…-12",
					style.Name, seed, 20*math.Log10(peak), rms, MasterCeilingDB)
			}
		}
	}
}
