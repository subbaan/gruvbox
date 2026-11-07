package groove

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"
)

func TestSampleNormalizationStereoSilenceAndGain(t *testing.T) {
	sample := pcmSample(t, 2, 8000, 8192, -16384, 8192, -16384)
	track := NewTrack(Sampler, 0)
	track.Sample = sample
	v := sampleVoice{sample: sample}
	v.parameter("normalize", 1)
	step := DefaultStep()
	step.Velocity = 1
	v.trigger(track, step)
	left, right := v.next()
	if math.Abs(float64(left)-.475) > 1e-6 || math.Abs(float64(right)+.95) > 1e-6 {
		t.Fatalf("stereo normalization: %v %v", left, right)
	}
	v.parameter("gain_db", -6)
	v.trigger(track, step)
	left, _ = v.next()
	if math.Abs(float64(left)-.475*math.Pow(10, -6./20)) > 1e-6 {
		t.Fatal("sample gain not applied")
	}
	silent := pcmSample(t, 1, 8000, 0, 0)
	if silent.NormalizationGain() != 1 {
		t.Fatal("silence normalization divides by zero")
	}
	v.sample = silent
	v.trigger(track, step)
	left, right = v.next()
	if left != 0 || right != 0 {
		t.Fatal("silence acquired noise")
	}
}

func TestSampleFadesTrimReverseLoopAndLiveUpdates(t *testing.T) {
	values := make([]int16, 16)
	for i := range values {
		values[i] = 16384
	}
	sample := pcmSample(t, 1, 8000, values...)
	track := NewTrack(Sampler, 0)
	track.Sample = sample
	track.Parameters["start"], track.Parameters["end"] = .25, .75
	v := sampleVoice{sample: sample}
	v.parameter("fade_in", .25) // Two source frames.
	v.parameter("fade_out", .25)
	step := DefaultStep()
	step.Velocity = 1
	v.trigger(track, step)
	for _, cursor := range []float64{4, 5, 6, 9, 10, 11} {
		v.cursor = cursor
		got, _ := v.next()
		want := map[int]float32{4: 0, 5: .25, 6: .5, 9: .5, 10: .25, 11: 0}[int(cursor)]
		if math.Abs(float64(got-want)) > 1e-6 {
			t.Fatalf("fade frame %v: %v, want %v", cursor, got, want)
		}
	}
	// Reverse swaps the fade directions, including unequal lengths.
	v.parameter("fade_out", 0)
	track.Parameters["reverse"] = 1
	v.trigger(track, step)
	got, _ := v.next()
	if got != 0 {
		t.Fatal("reverse fade did not begin at zero")
	}
	v.cursor = 4
	got, _ = v.next()
	if got != .5 {
		t.Fatal("reverse fade applied at wrong edge")
	}
	// Editing fades affects an already sounding voice without retriggering.
	v.cursor = 11
	v.parameter("fade_in", 0)
	got, _ = v.next()
	if got != .5 {
		t.Fatal("fade update withheld until next trigger")
	}
	track.Parameters["mode"] = 2
	v.parameter("fade_in", .25)
	v.trigger(track, step)
	v.cursor = 3
	got, _ = v.next() // Reverse loop wraps to the playback start.
	if got != 0 {
		t.Fatal("loop did not use trimmed fade envelope")
	}
	if allocs := testing.AllocsPerRun(100, func() { v.next() }); allocs != 0 {
		t.Fatalf("sampler processing allocated %v", allocs)
	}
}

func TestSampleProcessingSavedUndoAndOldProjectDefaults(t *testing.T) {
	p := NewProject()
	track := NewTrack(Sampler, 0)
	track.Sample = pcmSample(t, 1, 8000, 8192, 16384)
	p.Tracks = append(p.Tracks, track)
	session := NewSession(p)
	if err := session.Do("Process sample", func(p *Project) error {
		params := p.Track(track.ID).Parameters
		params["normalize"], params["gain_db"], params["fade_in"], params["fade_out"] = 1, -3, 35, 80
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(session.Project)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	params := loaded.Track(track.ID).Parameters
	if params["gain_db"] != -3 || params["normalize"] != 1 || params["fade_in"] != 35 || params["fade_out"] != 80 {
		t.Fatal("processing settings lost on save")
	}
	if loaded.Track(track.ID).Sample.NormalizationGain() != track.Sample.NormalizationGain() {
		t.Fatal("cached normalization peak lost on reload")
	}
	if !session.Undo() || session.Project.Track(track.ID).Parameters["fade_in"] != 0 || session.Project.Track(track.ID).Sample != track.Sample {
		t.Fatal("undo did not restore source/settings")
	}
	for _, id := range []string{"gain_db", "normalize", "fade_in", "fade_out"} {
		delete(p.Track(track.ID).Parameters, id)
	}
	data, _ = json.Marshal(p)
	loaded, err = Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("old sampler project: %v", err)
	}
	for _, id := range []string{"gain_db", "normalize", "fade_in", "fade_out"} {
		if val, ok := loaded.Track(track.ID).Parameters[id]; !ok || val != 0 {
			t.Fatal("old sample processing defaults change sound")
		}
	}
	loaded.Track(track.ID).Parameters["normalize"] = .5
	if loaded.Validate() == nil {
		t.Fatal("fractional normalization switch accepted")
	}
}

func TestNormalizeVeryQuietFloatSampleRemainsFinite(t *testing.T) {
	audio := make([]byte, 4)
	binary.LittleEndian.PutUint32(audio, math.Float32bits(math.SmallestNonzeroFloat32))
	sample, err := LoadSample("quiet.wav", bytes.NewReader(testWAV(3, 32, 1, 8000, audio)))
	if err != nil {
		t.Fatal(err)
	}
	track := NewTrack(Sampler, 0)
	track.Sample = sample
	v := sampleVoice{sample: sample}
	v.parameter("normalize", 1)
	step := DefaultStep()
	step.Velocity = 1
	v.trigger(track, step)
	left, right := v.next()
	if math.Abs(float64(left)-.95) > 1e-6 || right != left {
		t.Fatalf("very quiet normalization overflow: %v, %v", left, right)
	}
}
