package groove

import "math/rand/v2"

func Actions(kind Kind) []string {
	switch kind {
	case Dual:
		return []string{"Bassline", "Sparse", "Busy", "Mutate", "Variation", "Random", "Clear"}
	case Sampler:
		return []string{"4 Floor", "8ths", "Offbeat", "Sparse", "Busy", "Mutate", "Variation", "Random", "Clear"}
	case Kick:
		return []string{"4 Floor", "Broken", "Sparse", "Mutate", "Variation", "Random", "Clear"}
	case Snare, Clap:
		return []string{"2 & 4", "Halftime", "Ghosts", "Mutate", "Variation", "Random", "Clear"}
	case Rimshot:
		return []string{"Offbeat", "Syncopated", "8ths", "Sparse", "Mutate", "Variation", "Random", "Clear"}
	case Tom:
		return []string{"Toms", "Fill", "Bassline", "Sparse", "Mutate", "Variation", "Random", "Clear"}
	case Crash:
		return []string{"Downbeat", "Sparse", "Mutate", "Variation", "Random", "Clear"}
	case ClosedHat, OpenHat:
		return []string{"8ths", "16ths", "Offbeat", "Sparse", "Mutate", "Variation", "Random", "Clear"}
	default:
		return []string{"Acid", "Sparse", "Busy", "Mutate", "Variation", "Random", "Clear"}
	}
}

// melodic reports whether action writes a phrase (and so needs a key).
func melodic(kind Kind, action string) bool {
	switch kind {
	case Acid, Dual:
		return action == "Acid" || action == "Bassline" || action == "Sparse" || action == "Busy" || action == "Mutate" || action == "Variation" || action == "Random"
	case Tom:
		return action == "Bassline"
	}
	return false
}

func newRand() *rand.Rand { return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) }

// Generate changes musical content only; callers wrap it in a Session command.
// Melodic parts use a random key; use GenerateIn so parts share the project's.
func Generate(t *Track, slot int, action string) {
	r := newRand()
	var key *Key
	if melodic(t.Kind, action) {
		k := RandomKey(r, nil)
		key = &k
	}
	generate(t, slot, action, key, r)
}

// GenerateIn is Generate with the project's key. The first melodic
// generation in a project without a key picks one and stores it.
func GenerateIn(p *Project, t *Track, slot int, action string) {
	r := newRand()
	if melodic(t.Kind, action) && p.Key == nil {
		k := RandomKey(r, nil)
		p.Key = &k
	}
	generate(t, slot, action, p.Key, r)
}

// phraseAction maps melodic buttons to phrase settings.
func phraseAction(kind Kind, action string, r *rand.Rand) (PhraseOptions, bool) {
	styles := []PhraseStyle{PhraseAcid, PhraseHypnotic, PhraseRolling}
	options := PhraseOptions{Mode: PhraseAll, Movement: .35 + r.Float64()*.5, Variation: .15 + r.Float64()*.45, Feel: .45 + r.Float64()*.45, RhythmSeed: r.Uint64(), PitchSeed: r.Uint64(), ArticulationSeed: r.Uint64()}
	switch action {
	case "Acid", "Bassline":
		options.Density = .45 + r.Float64()*.3
		if kind == Acid {
			styles = []PhraseStyle{PhraseAcid, PhraseAcid, PhraseRolling, PhraseHypnotic}
		}
	case "Sparse":
		options.Density = .2 + r.Float64()*.15
		options.Movement *= .7
		styles = []PhraseStyle{PhraseHypnotic, PhraseHypnotic, PhraseAcid}
	case "Busy":
		options.Density = .75 + r.Float64()*.2
		styles = []PhraseStyle{PhraseRolling, PhraseAcid}
	default:
		return options, false
	}
	if kind == Tom {
		options.Feel *= .5
	}
	options.Style = styles[r.IntN(len(styles))]
	return options, true
}

func generate(t *Track, slot int, action string, key *Key, r *rand.Rand) {
	pattern := &t.Patterns[slot]
	if action == "Variation" {
		next := NextInBank(slot)
		copy := Project{Tracks: []Track{*t}}.Clone().Tracks[0].Patterns[slot]
		t.Patterns[next] = copy
		pattern = &t.Patterns[next]
		action = "Mutate"
	}
	scale, root := []int{0, 0, 3, 5, 7, 10, 12}, 36
	if key != nil {
		scale, root = scaleNotes(key.Scale), key.BassRoot()
		if t.Kind == Tom {
			// Tom notes transpose from its C2 root; keep them within C2 to C3.
			root = 36 + key.Root
		}
	}
	if options, ok := phraseAction(t.Kind, action, r); ok && key != nil {
		if bars := len(pattern.Steps) / 16; len(pattern.Steps)%16 == 0 && (bars == 1 || bars == 2 || bars == 4) {
			options.Bars, options.Root, options.Scale = bars, root, key.Scale
			if phrase, err := PhrasePattern(*pattern, t.Kind, options); err == nil {
				for i := range phrase.Steps {
					phrase.Steps[i].Locks = nil
					if t.Kind == Tom {
						phrase.Steps[i].Note = foldNote(phrase.Steps[i].Note, 36, 48)
					}
				}
				*pattern = phrase
				return
			}
		}
	}
	if action == "Random" && t.Kind != Sampler && !melodic(t.Kind, action) {
		// Drum "Random" picks a musical family rather than uniform noise.
		if family := randomDrumFamily(t.Kind, r); family != "" && DrumPattern(pattern.Steps, t.Kind, family, r) {
			return
		}
	}
	for i := range pattern.Steps {
		if action == "Mutate" {
			if r.Float64() < .18 {
				pattern.Steps[i].Active = !pattern.Steps[i].Active
				if t.Kind == Acid || t.Kind == Dual {
					pattern.Steps[i].Note = root + scale[r.IntN(len(scale))]
				}
			}
			continue
		}
		step := DefaultStep()
		switch action {
		case "Clear":
		case "Downbeat":
			step.Active = i == 0
		case "Syncopated":
			step.Active = i%16 == 3 || i%16 == 6 || i%16 == 11 || i%16 == 14
		case "Toms":
			step.Active = i%16 == 0 || i%16 == 6 || i%16 == 10 || i%16 == 14
		case "Fill":
			step.Active = i >= max(0, len(pattern.Steps)-4)
		case "4 Floor":
			step.Active = i%4 == 0
		case "2 & 4":
			step.Active = i%16 == 4 || i%16 == 12
		case "Halftime":
			step.Active = i%16 == 8
		case "8ths":
			step.Active = i%2 == 0
		case "16ths":
			step.Active = true
		case "Offbeat":
			step.Active = i%4 == 2
		case "Ghosts":
			step.Active = i%16 == 4 || i%16 == 12 || r.Float64() < .2
			if i%4 != 0 {
				step.Velocity = .35
			}
		case "Broken":
			step.Active = i%16 == 0 || i%16 == 7 || i%16 == 10
		default:
			density := .6
			if action == "Sparse" {
				density = .3
			}
			if action == "Busy" {
				density = .85
			}
			step.Active = r.Float64() < density || i == 0
		}
		if t.Kind == Tom && step.Active && (action == "Toms" || action == "Fill") {
			multiplier := []float64{1.5, 1.25, 1, .8}[i%4]
			step.Locks = map[string]float64{"pitch": max(40, min(300, t.Parameters["pitch"]*multiplier))}
		}
		if t.Kind == Acid || t.Kind == Dual || (t.Kind == Tom && action == "Bassline") {
			step.Note = root + scale[r.IntN(len(scale))]
			step.Accent = step.Active && r.Float64() < .3
			step.Slide = t.Kind.Capabilities().Slide && step.Active && r.Float64() < .2
		}
		if t.Kind == ClosedHat || t.Kind == OpenHat {
			step.Velocity = .45 + r.Float64()*.4
		}
		pattern.Steps[i] = step
	}
}

// foldNote moves note by octaves into [low, high].
func foldNote(note, low, high int) int {
	for note > high {
		note -= 12
	}
	for note < low {
		note += 12
	}
	return note
}
