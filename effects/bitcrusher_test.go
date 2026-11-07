package effects

import "testing"

func TestBitCrusherLiveRateChangesKeepHoldCounterInRange(t *testing.T) {
	c := NewBitCrusher(44100, 1)
	c.SetCrushRate(32)
	sample := []float32{.5}
	for i := 0; i < 31; i++ {
		sample[0] = .5
		c.Process(sample)
	}
	c.SetCrushRate(1)
	sample[0] = .25
	c.Process(sample)
	if c.holdCounter >= c.crushRate || sample[0] != .25 {
		t.Fatal("live rate decrease left a stale hold phase")
	}
	c.SetCrushRate(32)
	sample[0] = .75
	c.Process(sample)
	if sample[0] != .25 {
		t.Fatal("restoring crush rate failed to hold sample")
	}
}
