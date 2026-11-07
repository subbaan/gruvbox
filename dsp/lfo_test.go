package dsp

import "testing"

func TestSampleHoldHonoursRateAndTempo(t *testing.T) {
	lfo := NewLFO(1000)
	lfo.SetWaveform(LFORandom)
	lfo.SetSyncMode(SyncModeTime)
	lfo.SetTempoSync(false)
	lfo.SetRate(4)
	for i := 0; i < 249; i++ {
		lfo.Process()
	}
	if lfo.randomCounter != 249 {
		t.Fatal("sample and hold updated before its period")
	}
	lfo.Process()
	if lfo.randomCounter != 0 {
		t.Fatal("sample and hold ignored free rate")
	}
	lfo.SetTempoSync(true)
	lfo.SetSyncRate(.25)
	lfo.UpdateTempoRate(120)
	if lfo.randomRate != 500 {
		t.Fatal("sample and hold ignored tempo division")
	}
}
