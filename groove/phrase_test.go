package groove

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func testPhraseOptions() PhraseOptions {
	return PhraseOptions{Bars: 4, Root: 36, Style: PhraseAcid, Scale: PhraseMinorPentatonic, Mode: PhraseAll, Density: .5, Movement: .7, Variation: .5, Feel: .8, RhythmSeed: 17, PitchSeed: 23, ArticulationSeed: 31}
}

func TestPhraseScaleDensityMotifAndConnectedSlides(t *testing.T) {
	for _, kind := range []Kind{Acid, Dual, Tom, Sampler} {
		for _, recipe := range phraseRecipes {
			for _, scale := range phraseScales {
				for _, bars := range []int{1, 2, 4} {
					for _, density := range []float64{.125, .5, 1} {
						for seed := uint64(0); seed < 6; seed++ {
							opts := testPhraseOptions()
							opts.Bars, opts.Style, opts.Scale, opts.Density, opts.Root = bars, recipe.style, scale.name, density, 25
							opts.RhythmSeed, opts.PitchSeed, opts.ArticulationSeed = seed, seed, seed
							track := NewTrack(kind, 0)
							pattern, err := PhrasePattern(track.Patterns[0], kind, opts)
							if err != nil {
								t.Fatal(err)
							}
							if len(pattern.Steps) != bars*16 || pattern.Resolution != 16 {
								t.Fatal("incorrect musical length")
							}
							for bar := 0; bar < bars; bar++ {
								hits := 0
								for j, step := range pattern.Steps[bar*16 : (bar+1)*16] {
									inScale := false
									for _, offset := range scale.notes {
										inScale = inScale || step.Note == opts.Root+offset
									}
									if !inScale || step.Note < opts.Root || step.Note > opts.Root+12 {
										t.Fatal("note escaped chosen scale/register", step.Note)
									}
									if step.Active {
										hits++
									}
									if j < 8 && (step.Active != pattern.Steps[j].Active || step.Note != pattern.Steps[j].Note) {
										t.Fatal("development lost the motif's first half")
									}
									if step.Slide {
										index := bar*16 + j
										connected := false
										for gap := 1; gap <= 2; gap++ {
											previous := pattern.Steps[(index-gap+len(pattern.Steps))%len(pattern.Steps)]
											if previous.Active {
												connected = previous.Note != step.Note && previous.GateSteps() >= gap
												break
											}
										}
										if !connected || !kind.Capabilities().Slide {
											t.Fatal("orphan/unsupported slide")
										}
									}
									if !kind.Capabilities().NoteLengths && step.Length != 0 {
										t.Fatal("added unsupported note length")
									}
								}
								if hits != int(math.Round(16*density)) {
									t.Fatal("variation changed requested density", hits)
								}
							}
							track.Patterns[0] = pattern
							p := NewProject()
							p.Tracks = []Track{track}
							if err := p.Validate(); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
			}
		}
	}
}

func TestPhraseRepetitionDevelopmentAndIndependentRerolls(t *testing.T) {
	source := NewTrack(Acid, 0).Patterns[0]
	for seed := uint64(0); seed < 100; seed++ {
		options := testPhraseOptions()
		options.Variation, options.RhythmSeed, options.PitchSeed, options.ArticulationSeed = 0, seed, seed, seed
		pattern, err := PhrasePattern(source, Acid, options)
		if err != nil {
			t.Fatal(err)
		}
		for bar := 1; bar < 4; bar++ {
			if !reflect.DeepEqual(pattern.Steps[:16], pattern.Steps[bar*16:(bar+1)*16]) {
				t.Fatal("zero variation should repeat exact musical data", seed, bar)
			}
		}
		options.Variation = 1
		developed, _ := PhrasePattern(source, Acid, options)
		last := len(developed.Steps) - 1
		for !developed.Steps[last].Active {
			last--
		}
		if last >= 56 && developed.Steps[last].Note != options.Root {
			t.Fatal("final response did not resolve")
		}
	}
	options := testPhraseOptions()
	options.Variation = 0
	first, _ := PhrasePattern(source, Acid, options)
	second, _ := PhrasePattern(source, Acid, options)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("seeded preview changed")
	}
	options.PitchSeed++
	second, _ = PhrasePattern(source, Acid, options)
	for i, step := range first.Steps {
		if step.Active != second.Steps[i].Active || step.Velocity != second.Steps[i].Velocity || step.Accent != second.Steps[i].Accent {
			t.Fatal("pitch reroll changed rhythm/velocity/accent")
		}
	}
	options = testPhraseOptions()
	first, _ = PhrasePattern(source, Acid, options)
	options.ArticulationSeed++
	second, _ = PhrasePattern(source, Acid, options)
	if reflect.DeepEqual(first, second) {
		t.Fatal("feel reroll had no effect")
	}
	for i, step := range first.Steps {
		if step.Active != second.Steps[i].Active || step.Note != second.Steps[i].Note {
			t.Fatal("feel reroll changed rhythm/pitch")
		}
	}
	options = testPhraseOptions()
	options.Variation = 0
	first, _ = PhrasePattern(source, Acid, options)
	options.RhythmSeed++
	second, _ = PhrasePattern(source, Acid, options)
	for i, step := range first.Steps {
		if step.Note != second.Steps[i].Note {
			t.Fatal("rhythm reroll changed underlying pitch motif")
		}
	}
}

func TestPhrasePartialModesLocksResizeUndoAndSave(t *testing.T) {
	track := NewTrack(Acid, 0)
	track.Patterns[0].Resolution = 32
	for i := range track.Patterns[0].Steps {
		track.Patterns[0].Steps[i] = Step{Active: i%3 == 0, Note: 48 + i, Length: 3, Velocity: .5, Accent: i%2 == 0, Slide: i%5 == 0, Probability: .75, Every: 2, Locks: map[string]float64{"cutoff": .5}}
	}
	source := clonePattern(track.Patterns[0])
	options := testPhraseOptions()
	options.Mode = PhrasePitches
	pitches, err := PhrasePattern(source, Acid, options)
	if err != nil {
		t.Fatal(err)
	}
	for i, step := range pitches.Steps {
		original := source.Steps[i%16]
		step.Note = original.Note
		if i >= 16 {
			original.Locks = nil
		}
		if !reflect.DeepEqual(step, original) {
			t.Fatal("pitch-only changed rhythm, gates, conditions or articulation", i)
		}
	}
	options.Mode = PhraseFeel
	feel, err := PhrasePattern(source, Acid, options)
	if err != nil {
		t.Fatal(err)
	}
	for i, step := range feel.Steps {
		original := source.Steps[i%16]
		if step.Note != original.Note || step.Active != original.Active || step.Probability != original.Probability || step.Every != original.Every {
			t.Fatal("feel-only changed notes/rhythm/conditions")
		}
	}
	pitches.Steps[0].Locks["cutoff"] = .9
	if source.Steps[0].Locks["cutoff"] != .5 || !reflect.DeepEqual(source, track.Patterns[0]) {
		t.Fatal("draft shared source data")
	}
	p := NewProject()
	p.Tracks = []Track{track}
	s := NewSession(p)
	if err := s.Do("Phrase", func(p *Project) error { p.Tracks[0].Patterns[0] = feel; return nil }); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(s.Project)
	var loaded Project
	if err != nil || json.Unmarshal(data, &loaded) != nil || loaded.Validate() != nil || !reflect.DeepEqual(loaded, s.Project) {
		t.Fatal("phrase did not roundtrip through project format")
	}
	if !s.Undo() || !reflect.DeepEqual(s.Project, p) || !s.Redo() || !reflect.DeepEqual(s.Project.Tracks[0].Patterns[0], feel) {
		t.Fatal("resize/phrase did not undo/redo once")
	}
}

func TestPhraseControlExtremesAndKeyTransposition(t *testing.T) {
	source := NewTrack(Acid, 0).Patterns[0]
	source.Steps[0].Locks = map[string]float64{"cutoff": .6}
	source.Steps[0].Probability, source.Steps[0].Every = .5, 3
	options := testPhraseOptions()
	options.Movement, options.Feel = 0, 0
	plain, err := PhrasePattern(source, Acid, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range plain.Steps {
		if step.Note != options.Root || step.Accent || step.Slide || step.Length != 0 || step.Velocity != .8 || step.Probability != 1 || step.Every != 0 {
			t.Fatal("zero movement/feel did not produce plain root notes", step)
		}
	}
	if plain.Steps[0].Locks["cutoff"] != .6 || plain.Steps[16].Locks != nil {
		t.Fatal("whole-phrase generation lost or duplicated stored locks")
	}
	plain.Steps[0].Locks["cutoff"] = .9
	if source.Steps[0].Locks["cutoff"] != .6 || source.Steps[0].Probability != .5 {
		t.Fatal("whole-phrase draft edited its source")
	}
	options = testPhraseOptions()
	first, _ := PhrasePattern(source, Acid, options)
	options.Root += 12
	transposed, _ := PhrasePattern(source, Acid, options)
	for i, step := range transposed.Steps {
		step.Note -= 12
		if !reflect.DeepEqual(step, first.Steps[i]) {
			t.Fatal("changing key/register recomposed the phrase")
		}
	}
}

func TestPhraseRejectsInvalidOptionsAndRendersAllLengths(t *testing.T) {
	source := NewTrack(Acid, 0).Patterns[0]
	invalid := []func(*PhraseOptions){
		func(o *PhraseOptions) { o.Bars = 3 }, func(o *PhraseOptions) { o.Root = 116 },
		func(o *PhraseOptions) { o.Root = -1 }, func(o *PhraseOptions) { o.Density = 0 },
		func(o *PhraseOptions) { o.Movement = math.NaN() }, func(o *PhraseOptions) { o.Variation = 1.1 },
		func(o *PhraseOptions) { o.Feel = math.Inf(1) }, func(o *PhraseOptions) { o.Style = "missing" },
		func(o *PhraseOptions) { o.Scale = "missing" }, func(o *PhraseOptions) { o.Mode = "missing" },
	}
	for _, change := range invalid {
		options := testPhraseOptions()
		change(&options)
		if _, err := PhrasePattern(source, Acid, options); err == nil {
			t.Fatal("accepted invalid options", options)
		}
	}
	if _, err := PhrasePattern(source, Kick, testPhraseOptions()); err == nil {
		t.Fatal("accepted unpitched instrument")
	}
	for _, recipe := range phraseRecipes {
		for _, bars := range []int{1, 2, 4} {
			options := testPhraseOptions()
			options.Bars, options.Style = bars, recipe.style
			p := NewProject()
			p.Tracks = p.Tracks[:1]
			p.Tracks[0].Patterns[0], _ = PhrasePattern(source, Acid, options)
			var wav bytes.Buffer
			if err := RenderWAV(context.Background(), &wav, p, false, 0, nil); err != nil {
				t.Fatal(err)
			}
			frames := int(math.Ceil(float64(bars*4) * SampleRate * 60 / p.BPM))
			if wav.Len() != 44+frames*4 {
				t.Fatal("incorrect exported phrase duration", bars, wav.Len())
			}
			peak := 0
			for i := 44; i < wav.Len(); i += 2 {
				peak = max(peak, phraseAbs(int(int16(binary.LittleEndian.Uint16(wav.Bytes()[i:])))))
			}
			if peak < 100 || peak >= 32767 {
				t.Fatal("silent or clipping phrase render", peak)
			}
		}
	}
}
