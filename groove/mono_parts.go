package groove

import (
	"math/rand/v2"
	"slices"
)

// Part writing that belongs to one of the two mono synths (alpha.62), so a
// Dual and an SH-101 in a groove play different kinds of line:
//
//   - SH-101, after its arpeggiator and step sequencer: Arp runs up and down
//     the chord of each bar; Sequence loops a short 3–8 note figure that
//     cycles over the bar line.
//   - Dual, whose two oscillators stack intervals and sync leads: Lead is a
//     slow melody of long notes that slide into each other; Stabs are short
//     offbeat hits on the bar's chord root (the sound's interval makes each
//     one a chord).
//
// All write from the key's bass root; a recipe's octave lifts upper parts.

// monoIdioms lists the actions writeIdiom knows.
var monoIdioms = []string{"Arp", "Sequence", "Lead", "Stabs"}

// progressions are chord roots per bar, in semitones above the key root,
// snapped to the scale. A loop uses as many bars as it has.
var progressions = [][]int{{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 10, 0, 10}, {0, 8, 10, 0}, {0, 5, 0, 7}, {0, 0, 8, 10}, {0, 3, 5, 0}}

// writeIdiom writes action's line into steps and reports whether it knows
// the action.
func writeIdiom(steps []Step, action string, key Key, r *rand.Rand) bool {
	if !slices.Contains(monoIdioms, action) || len(steps) == 0 {
		return false
	}
	for i := range steps {
		steps[i] = DefaultStep()
	}
	h := harmony{key: key, base: key.BassRoot(), roots: progressions[r.IntN(len(progressions))]}
	switch action {
	case "Arp":
		writeArp(steps, h, r)
	case "Sequence":
		writeSequence(steps, h, r)
	case "Lead":
		writeLead(steps, h, r)
	case "Stabs":
		writeStabs(steps, h, r)
	}
	return true
}

// harmony answers pitch questions for one line: the key, its bass root and
// the chord root of each bar.
type harmony struct {
	key   Key
	base  int
	roots []int
}

func (h harmony) inScale(note int) bool {
	pc := ((note-h.key.Root)%12 + 12) % 12
	return slices.Contains(scaleNotes(h.key.Scale), pc)
}

// snap moves note to the nearest scale note, preferring the lower one.
func (h harmony) snap(note int) int {
	for d := 0; d < 12; d++ {
		if h.inScale(note - d) {
			return note - d
		}
		if h.inScale(note + d) {
			return note + d
		}
	}
	return note
}

// root is bar's chord root.
func (h harmony) root(bar int) int {
	return h.snap(h.base + h.roots[bar%len(h.roots)])
}

// chord is the triad on bar's root, built from the scale: a third (minor
// when the scale has it), a fifth, and a seventh if asked for.
func (h harmony) chord(bar int, seventh bool) []int {
	root := h.root(bar)
	pick := func(wanted ...int) int {
		for _, w := range wanted {
			if h.inScale(root + w) {
				return root + w
			}
		}
		return h.snap(root + wanted[0])
	}
	tones := []int{root, pick(3, 4), pick(7, 6, 8)}
	if seventh {
		tones = append(tones, pick(10, 11))
	}
	return tones
}

// degree is the note n scale steps above (or below, for negative n) note.
func (h harmony) degree(note, n int) int {
	for ; n > 0; n-- {
		note = h.nextUp(note)
	}
	for ; n < 0; n++ {
		note = h.nextDown(note)
	}
	return note
}

// nearestTone is the chord tone, in any octave, closest to note.
func nearestTone(note int, tones []int) int {
	best := tones[0]
	for _, t := range tones {
		for c := t - 24; c <= t+24; c += 12 {
			if phraseAbs(c-note) < phraseAbs(best-note) {
				best = c
			}
		}
	}
	return best
}

func (h harmony) nextUp(note int) int {
	for n := note + 1; n < note+13; n++ {
		if h.inScale(n) {
			return n
		}
	}
	return note + 1
}

func (h harmony) nextDown(note int) int {
	for n := note - 1; n > note-13; n-- {
		if h.inScale(n) {
			return n
		}
	}
	return note - 1
}

// writeArp: the chord of each bar, up, down, up and down or in a fixed
// shuffled order, over one or two octaves, in sixteenths or eighths. Rests
// and accents fall on the same steps of every bar, like a held pattern.
func writeArp(steps []Step, h harmony, r *rand.Rand) {
	rate := 1
	if r.Float64() < .35 {
		rate = 2
	}
	octaves := 1 + r.IntN(2)
	order := r.IntN(4)
	seventh := r.Float64() < .3
	perm := r.Perm(octaves * len(h.chord(0, seventh)))
	var rests, accents [16]bool
	for i := 1; i < 16; i++ {
		rests[i] = i%4 != 0 && r.Float64() < .12
		accents[i] = i%4 == 0 && r.Float64() < .5
	}
	accents[0] = true
	for bar := 0; bar*16 < len(steps); bar++ {
		tones := []int{}
		for o := 0; o < octaves; o++ {
			for _, t := range h.chord(bar, seventh) {
				tones = append(tones, t+12*o)
			}
		}
		run := tones
		switch order {
		case 1:
			run = slices.Clone(tones)
			slices.Reverse(run)
		case 2:
			for i := len(tones) - 2; i > 0; i-- {
				run = append(run, tones[i])
			}
		case 3:
			run = make([]int, len(tones))
			for i := range run {
				run[i] = tones[perm[i]]
			}
		}
		k := 0
		for i := bar * 16; i < min(len(steps), bar*16+16); i += rate {
			in := i % 16
			s := &steps[i]
			s.Note = run[k%len(run)]
			k++
			if rests[in] {
				continue
			}
			s.Active, s.Accent = true, accents[in]
		}
	}
}

// writeSequence: a short figure (3–8 steps, odd lengths cycle against the
// bar) from the scale, with an octave jump, rests, accents and slides,
// played in sixteenths or eighths over the whole loop. Each bar moves it to
// the bar's chord root.
func writeSequence(steps []Step, h harmony, r *rand.Rand) {
	n := []int{3, 5, 5, 6, 7, 7, 8}[r.IntN(7)]
	rate := 1
	if r.Float64() < .3 {
		rate = 2
	}
	type element struct {
		offset        int // semitones from the bar's root, in scale
		rest, accent  bool
		slide, octave bool
	}
	figure := make([]element, n)
	note := h.base
	for i := range figure {
		if i > 0 {
			move := []int{-2, -1, -1, 1, 1, 2, 2, 3, 4}[r.IntN(9)]
			note = h.degree(note, move)
			for note < h.base-3 {
				note = h.nextUp(note)
			}
			for note > h.base+14 {
				note = h.nextDown(note)
			}
			figure[i].rest = r.Float64() < .18
			figure[i].slide = r.Float64() < .15
		}
		figure[i].offset = note - h.base
		figure[i].accent = r.Float64() < .3
	}
	figure[r.IntN(n)].octave = r.Float64() < .5
	for i, k := 0, 0; i < len(steps); i, k = i+rate, k+1 {
		e := figure[k%n]
		if e.rest {
			continue
		}
		s := &steps[i]
		root := h.root(i / 16)
		s.Active, s.Accent = true, e.accent
		s.Note = h.snap(root + e.offset)
		if e.octave {
			s.Note += 12
		}
		prev := figure[(k+n-1)%n]
		if e.slide && !prev.rest && i >= rate {
			s.Slide = true
			if rate > 1 {
				// The gate has to stay open across the rest step to glide.
				steps[i-rate].Length = rate
			}
		}
	}
}

// writeLead: one to three long notes a bar on strong and syncopated steps,
// walking the scale with the odd leap, sliding into some, and coming home to
// the root or fifth at the end of the loop.
func writeLead(steps []Step, h harmony, r *rand.Rand) {
	places := []int{0, 3, 6, 8, 10, 12, 14}
	note := h.chord(0, false)[r.IntN(3)]
	onsets := []int{}
	for bar := 0; bar*16 < len(steps); bar++ {
		count := 1 + r.IntN(3)
		chosen := map[int]bool{0: bar == 0 || r.Float64() < .6}
		for len(chosen) < count+1 {
			chosen[places[1+r.IntN(len(places)-1)]] = true
		}
		for _, p := range places {
			if i := bar*16 + p; chosen[p] && i < len(steps) {
				onsets = append(onsets, i)
			}
		}
	}
	for k, i := range onsets {
		s := &steps[i]
		if k > 0 {
			move := []int{-2, -1, -1, 1, 1, 2}[r.IntN(6)]
			if r.Float64() < .2 {
				move = []int{-4, -3, 3, 4}[r.IntN(4)]
			}
			note = h.degree(note, move)
			if i%16 == 0 {
				// A bar's first note lands on its chord.
				note = nearestTone(note, h.chord(i/16, false))
			}
			s.Slide = r.Float64() < .45
		}
		for note < h.base {
			note += 12
		}
		for note > h.base+19 {
			note -= 12
		}
		if k == len(onsets)-1 && len(onsets) > 2 {
			end := h.chord(0, false)
			note = []int{end[0], end[2]}[r.IntN(2)]
		}
		s.Active, s.Note = true, note
		s.Accent = i%16 == 0 && r.Float64() < .3
	}
	holdNotes(steps)
}

// stabRhythms are one-bar stab placements.
var stabRhythms = []string{
	"..x...x...x...x.", // offbeat eighths
	"....x.......x...", // dub skank on two and four
	"..x..x....x..x..",
	"x..x..x...x..x..",
	"...x..x....x..x.",
	"..x.x...x...x.x.",
}

// writeStabs: short hits in one rhythm (the last bar of a long loop may
// change it), on the bar's chord root, now and then its fifth.
func writeStabs(steps []Step, h harmony, r *rand.Rand) {
	rhythm := stabRhythms[r.IntN(len(stabRhythms))]
	turn := stabRhythms[r.IntN(len(stabRhythms))]
	bars := (len(steps) + 15) / 16
	for i := range steps {
		bar := i / 16
		pattern := rhythm
		if bars >= 4 && bar == bars-1 && r.Float64() < .4 {
			pattern = turn
		}
		if pattern[i%16] != 'x' {
			continue
		}
		s := &steps[i]
		chord := h.chord(bar, false)
		s.Active, s.Note, s.Length = true, chord[0], 1
		if r.Float64() < .2 {
			s.Note = chord[2]
		}
		s.Accent = r.Float64() < .35
		s.Velocity = .7 + r.Float64()*.2
	}
}
