package groove

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestEuclideanCountsRotationAndEvenSpacing(t *testing.T) {
	for n := 1; n <= 64; n++ {
		for k := 0; k <= n; k++ {
			source := NewTrack(Kick, 0).Patterns[0]
			for _, rotation := range []int{0, n - 1} {
				pattern, err := EuclideanPattern(source, EuclideanOptions{Steps: n, Pulses: k, Rotation: rotation, Velocity: .7})
				if err != nil {
					t.Fatal(err)
				}
				indices := []int{}
				for i, step := range pattern.Steps {
					if step.Active {
						indices = append(indices, i)
					}
				}
				if len(indices) != k {
					t.Fatalf("%d/%d rotation %d: %v", k, n, rotation, indices)
				}
				if k > 0 {
					short, long := n, 0
					for i, index := range indices {
						gap := (indices[(i+1)%k] - index + n) % n
						if k == 1 {
							gap = n
						}
						short = min(short, gap)
						long = max(long, gap)
					}
					if long-short > 1 {
						t.Fatal("uneven Euclidean spacing", k, n, indices)
					}
				}
			}
		}
	}
}

func TestEuclideanPreservesNotesLocksLayersAndUndo(t *testing.T) {
	p := NewProject()
	track := &p.Tracks[0]
	track.Patterns[0].Steps[1] = Step{Active: true, Note: 47, Length: 3, Velocity: .6, Accent: true, Slide: true, Probability: .75, Every: 2, Locks: map[string]float64{"cutoff": .5}}
	before := p.Clone()
	options := EuclideanOptions{Steps: 16, Pulses: 4, Velocity: .9, Layer: true}
	draft, err := EuclideanPattern(track.Patterns[0], options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(draft.Steps[1], track.Patterns[0].Steps[1]) || !draft.Steps[0].Active || draft.Steps[0].Velocity != .9 {
		t.Fatal("layering lost existing notes or new-hit velocity")
	}
	draft.Steps[1].Locks["cutoff"] = .9
	if track.Patterns[0].Steps[1].Locks["cutoff"] != .5 {
		t.Fatal("draft shares source lock maps")
	}
	options.Layer = false
	options.Steps = 8
	draft, _ = EuclideanPattern(track.Patterns[0], options)
	if draft.Steps[1].Active || draft.Steps[1].Note != 47 || draft.Steps[1].Length != 3 || draft.Steps[1].Locks["cutoff"] != .5 {
		t.Fatal("rhythm replacement erased note details")
	}
	session := NewSession(p)
	if err := session.Do("Euclidean", func(p *Project) error { p.Tracks[0].Patterns[0] = draft; return nil }); err != nil {
		t.Fatal(err)
	}
	if !session.Undo() || !reflect.DeepEqual(session.Project, before) {
		t.Fatal("rhythm resize did not undo in one command")
	}
	options.Steps = 64
	draft, _ = EuclideanPattern(track.Patterns[0], options)
	if len(draft.Steps) != 64 || draft.Steps[63].Note != 36 || draft.Steps[63].Probability != 1 {
		t.Fatal("extension did not use valid default steps")
	}
}

func TestLockShapesUseNativeRangesAndPreserveOtherData(t *testing.T) {
	for _, kind := range Kinds {
		track := NewTrack(kind, 0)
		track.Effects = []Effect{NewEffect(Chorus), NewEffect(Delay)}
		for i := range track.Patterns[0].Steps {
			track.Patterns[0].Steps[i].Active = i%2 == 0
			track.Patterns[0].Steps[i].Note = 40 + i
		}
		for _, parameter := range LockParameters(track) {
			if !parameter.Lockable {
				continue
			}
			for _, shape := range LockShapes {
				options := LockOptions{Parameter: parameter.ID, Shape: shape, First: 0, Last: 15, Low: parameter.Min, High: parameter.Max, ActiveOnly: true, Seed: 123}
				draft, err := LockPattern(track, 0, options)
				if err != nil {
					t.Fatal(kind, parameter.ID, shape, err)
				}
				for i, step := range draft.Steps {
					value, exists := step.Locks[parameter.ID]
					if step.Active && shape != LockClear && (!exists || !inRange(value, parameter.Min, parameter.Max)) {
						t.Fatal("native range violated", kind, parameter.ID, shape, value)
					}
					if !step.Active && exists {
						t.Fatal("active-only wrote a rest")
					}
					step.Locks = nil
					if !reflect.DeepEqual(step, track.Patterns[0].Steps[i]) {
						t.Fatal("lock generator changed a note")
					}
				}
				if !reflect.DeepEqual(track.Parameters, NewTrack(kind, 0).Parameters) {
					t.Fatal("generator changed sound knobs")
				}
			}
		}
	}
}

func TestLockPreviewSeedsHumaniseSingleStepClearAndSave(t *testing.T) {
	track := NewTrack(Acid, 0)
	fx := NewEffect(Filter)
	track.Effects = []Effect{fx}
	key := EffectLockID(fx.ID, "cutoff")
	for i := range track.Patterns[0].Steps {
		track.Patterns[0].Steps[i].Active = true
		track.Patterns[0].Steps[i].Locks = map[string]float64{key: 2000, "resonance": .4}
	}
	options := LockOptions{Parameter: key, Shape: LockHumaniseMild, First: 0, Last: 15, Low: 1000, High: 3000, Seed: 2}
	a, err := LockPattern(track, 0, options)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := LockPattern(track, 0, options)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("preview changed without reroll")
	}
	for _, s := range a.Steps {
		if math.Abs(s.Locks[key]-2000) > 100 || s.Locks["resonance"] != .4 {
			t.Fatal("humanise did not centre on existing locks")
		}
	}
	options.Seed++
	b, _ = LockPattern(track, 0, options)
	if reflect.DeepEqual(a, b) {
		t.Fatal("reroll did not explore a new take")
	}
	options.Shape = LockClear
	options.First = 4
	options.Last = 4
	b, _ = LockPattern(track, 0, options)
	if _, exists := b.Steps[4].Locks[key]; exists {
		t.Fatal("clear retained the chosen lock")
	}
	if b.Steps[3].Locks[key] != 2000 || b.Steps[4].Locks["resonance"] != .4 {
		t.Fatal("clear erased unrelated locks or steps")
	}
	for _, shape := range LockShapes {
		options.Shape = shape
		if _, err := LockPattern(track, 0, options); err != nil {
			t.Fatal("single-step generator failed", shape, err)
		}
	}
	p := Project{FormatVersion: FormatVersion, BPM: 126, Tracks: []Track{track}}
	session := NewSession(p)
	if err := session.Do("Locks", func(p *Project) error { p.Tracks[0].Patterns[0] = a; return nil }); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(session.Project)
	decoded, err := Decode(bytes.NewReader(data))
	if err != nil || !reflect.DeepEqual(decoded, session.Project) {
		t.Fatal("generated FX locks did not save", err)
	}
	if !session.Undo() || !reflect.DeepEqual(session.Project, p) {
		t.Fatal("generated locks did not undo")
	}
}

func TestGeneratorRejectsInvalidDraftsWithoutMutation(t *testing.T) {
	track := NewTrack(Kick, 0)
	before := NewSession(Project{Tracks: []Track{track}}).Project.Tracks[0]
	for _, o := range []EuclideanOptions{{Steps: 0}, {Steps: 65}, {Steps: 16, Pulses: 17}, {Steps: 16, Rotation: 16}, {Steps: 16, Velocity: math.NaN()}} {
		if _, err := EuclideanPattern(track.Patterns[0], o); err == nil {
			t.Fatal("invalid Euclidean configuration accepted")
		}
	}
	base := LockOptions{Parameter: "decay", Shape: LockUp, First: 0, Last: 15, Low: .05, High: 1}
	for _, edit := range []func(*LockOptions){func(o *LockOptions) { o.First = -1 }, func(o *LockOptions) { o.Last = 16 }, func(o *LockOptions) { o.Parameter = "missing" }, func(o *LockOptions) { o.Low = math.NaN() }, func(o *LockOptions) { o.Low = 2 }, func(o *LockOptions) { o.Shape = "unknown" }} {
		bad := base
		edit(&bad)
		if _, err := LockPattern(track, 0, bad); err == nil {
			t.Fatal("invalid lock draft accepted")
		}
	}
	if !reflect.DeepEqual(track, before) {
		t.Fatal("failed generator mutated the source")
	}
}
