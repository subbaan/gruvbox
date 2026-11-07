package groove

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
)

// Grow into a track (alpha.43): the selected pattern is the peak of a track.
// The other pattern slots get parts derived from it (intro, build, riser,
// fills, breakdown, a second peak) and the song is laid out so Play in song
// mode plays a whole arrangement. See TRACK_BUILDER_PLAN.md.

// Roles of the derived slots, in the order they take the free slots.
const (
	growIntro = iota
	growBuild
	growRiser
	growFill
	growBreakdown
	growPeak2
	growFill2
	growSlots
)

// GrowRoleNames names the derived slots, in growIntro… order.
var GrowRoleNames = [growSlots]string{"Intro", "Build", "Riser", "Fill", "Breakdown", "Peak 2", "Fill 2"}

// riserBars is the riser's length; patterns hold at most four bars.
const riserBars = 4

// GrowSlots returns the slots Grow writes, in growIntro… order: the other
// pads of the source's bank, lowest first. Each bank can hold one song.
func GrowSlots(source int) []int {
	slots := make([]int, 0, BankSize-1)
	first := BankOf(source) * BankSize
	for slot := first; slot < first+BankSize; slot++ {
		if slot != source {
			slots = append(slots, slot)
		}
	}
	return slots
}

// GrowReplaces lists the slots Grow would overwrite that hold something, and
// whether a song arrangement would be replaced.
func GrowReplaces(p Project) (slots []int, song bool) {
	for _, slot := range GrowSlots(p.Pattern)[:growSlots] {
		if !PatternEmpty(p, slot) {
			slots = append(slots, slot)
		}
	}
	return slots, len(p.Song) > 0
}

// growTrack is a track's part in the arrangement.
type growPart int

const (
	partKick growPart = iota
	partBackbeat
	partHat
	partOpen
	partPerc
	partTom
	partCrash
	partSynth
	partSampler
)

func partOf(k Kind) growPart {
	switch k {
	case Kick:
		return partKick
	case Snare, Clap:
		return partBackbeat
	case ClosedHat:
		return partHat
	case OpenHat:
		return partOpen
	case Rimshot:
		return partPerc
	case Tom:
		return partTom
	case Crash:
		return partCrash
	case Acid, Dual:
		return partSynth
	}
	return partSampler
}

// GrowTrack builds the arrangement. full doubles the long sections (about four
// minutes at 128 BPM rather than two). It returns a status line.
func GrowTrack(p *Project, r *rand.Rand, full bool) (string, error) {
	source := p.Pattern
	if source < 0 || source >= PatternCount || PatternEmpty(*p, source) {
		return "", fmt.Errorf("make or randomise a groove in this pattern first")
	}
	audible := false
	for _, t := range p.Tracks {
		audible = audible || hasNotes(t.Patterns[source])
	}
	if !audible {
		return "", fmt.Errorf("this pattern has no notes to grow from")
	}
	slots := GrowSlots(source)
	g := grower{p: p, r: r, source: source, roller: rollTrack(*p, source), fillTrack: fillTrack(*p, source), lead: leadSynth(*p, source)}
	peak := 16
	long := 8
	if full {
		peak, long = 32, 16
	}
	for i := range p.Tracks {
		t := &p.Tracks[i]
		src := t.Patterns[source]
		base := g.sound(*t)
		intro := g.intro(*t, src)
		build := g.build(*t, src, base)
		riser := g.riser(*t, src, base)
		breakdown := g.breakdown(*t, src, base)
		peak2 := g.peak2(*t, src, base)
		fill := g.fill(*t, src, peak-1)
		fill2 := g.fill(*t, peak2, peak-1)
		for role, pattern := range [growSlots]Pattern{intro, build, riser, fill, breakdown, peak2, fill2} {
			t.Patterns[slots[role]] = pattern
		}
	}
	// A slot that came out silent (say, an intro for a project with no kick
	// or hats) plays a quieter copy of the build instead.
	for _, role := range []int{growIntro, growBuild, growBreakdown} {
		if silent(*p, slots[role]) {
			for i := range p.Tracks {
				t := &p.Tracks[i]
				pattern := clonePattern(t.Patterns[slots[growBuild]])
				if silent(*p, slots[growBuild]) {
					pattern = clonePattern(t.Patterns[source])
				}
				for n := range pattern.Steps {
					pattern.Steps[n].Velocity *= .7
				}
				t.Patterns[slots[role]] = pattern
			}
		}
	}
	section := func(role int, name string, bars int) SongSection {
		s := NewSongSection(source)
		if role >= 0 {
			s.Pattern = slots[role]
		}
		s.Name, s.Repeats = name, bars
		return s
	}
	p.Song = []SongSection{
		section(growIntro, "Intro", long),
		section(growBuild, "Build", long),
		section(growRiser, "Riser", riserBars),
		section(-1, "Peak", peak-1),
		section(growFill, "Fill", 1),
		section(growBreakdown, "Breakdown", long),
		section(growRiser, "Riser", riserBars),
		section(growPeak2, "Peak 2", peak-1),
		section(growFill2, "Fill", 1),
		section(growIntro, "Outro", long),
	}
	bars := 0
	for _, s := range p.Song {
		bars += s.Repeats
	}
	seconds := int(math.Round(float64(bars) * 4 * 60 / p.BPM))
	used := make([]string, 0, growSlots)
	for _, slot := range slots[:growSlots] {
		used = append(used, SlotName(slot))
	}
	return fmt.Sprintf("%d sections · %d bars · %d:%02d · peak is pattern %s, parts in %s", len(p.Song), bars, seconds/60, seconds%60, SlotName(source), strings.Join(used, ", ")), nil
}

type grower struct {
	p                       *Project
	r                       *rand.Rand
	source                  int
	roller, fillTrack, lead string // track IDs; "" when absent
}

// sound is the sound the source slot plays: its stored state, else the
// track's own.
func (g grower) sound(t Track) Track {
	return withTrackState(t, t.Patterns[g.source].State)
}

func hasNotes(p Pattern) bool {
	for _, s := range p.Steps {
		if s.Active {
			return true
		}
	}
	return false
}
func silent(p Project, slot int) bool {
	for _, t := range p.Tracks {
		if hasNotes(t.Patterns[slot]) && !t.Mute {
			return false
		}
	}
	return true
}

// rollTrack picks the drum that plays the riser's roll; fillTrack the one
// that plays the fill at the end of a peak.
func rollTrack(p Project, source int) string {
	return firstOf(p, source, Snare, Clap, Rimshot, Tom)
}
func fillTrack(p Project, source int) string {
	return firstOf(p, source, Tom, Snare, Clap, Rimshot)
}
func firstOf(p Project, source int, kinds ...Kind) string {
	for _, kind := range kinds {
		for _, t := range p.Tracks {
			if t.Kind == kind && !t.Mute {
				return t.ID
			}
		}
	}
	return ""
}

// leadSynth is the first synth with notes; it alone plays the breakdown.
func leadSynth(p Project, source int) string {
	for _, t := range p.Tracks {
		if partOf(t.Kind) == partSynth && !t.Mute && hasNotes(t.Patterns[source]) {
			return t.ID
		}
	}
	return ""
}

// emptied keeps a pattern's length and resolution and the source sound, with
// no notes or locks.
func emptied(src Pattern) Pattern {
	result := Pattern{Steps: make([]Step, len(src.Steps)), Resolution: src.Resolution, State: cloneTrackState(src.State)}
	for i := range result.Steps {
		result.Steps[i] = DefaultStep()
	}
	return result
}

// tiled repeats src to n steps.
func tiled(src Pattern, n int) Pattern {
	result := clonePattern(src)
	result.Steps = make([]Step, n)
	for i := range result.Steps {
		step := src.Steps[i%len(src.Steps)]
		step.Locks = cloneMap(step.Locks)
		result.Steps[i] = step
	}
	return result
}

// barAt is one bar of src: the steps that play in bar index bar (from 0) of a
// section looping src.
func barAt(src Pattern, bar int) Pattern {
	result := clonePattern(src)
	n := max(1, src.Resolution)
	result.Steps = make([]Step, min(n, 64))
	start := bar * n
	for i := range result.Steps {
		step := src.Steps[(start+i)%len(src.Steps)]
		step.Locks = cloneMap(step.Locks)
		result.Steps[i] = step
	}
	return result
}

// filtered is cutoff closed by amount (0 = as is, 1 = well closed). A
// high-pass 303 closes the other way, by thinning.
func filtered(t Track, amount float64) float64 {
	base := t.Parameters["cutoff"]
	if t.Kind == Acid && t.Parameters["filter_mode"] > 1.5 {
		return base + (.9-base)*.6*amount
	}
	return base * (1 - .6*amount)
}

// lockAll writes a lock on every step, notes or rests, so the value holds for
// the whole pattern.
func lockAll(p *Pattern, id string, value func(i int) float64) {
	for i := range p.Steps {
		locks := cloneMap(p.Steps[i].Locks)
		if locks == nil {
			locks = map[string]float64{}
		}
		locks[id] = max(0, min(1, value(i)))
		p.Steps[i].Locks = locks
	}
}

// lockValues locks each value on every step.
func lockValues(p *Pattern, values map[string]float64) {
	for id, value := range values {
		lockAll(p, id, func(int) float64 { return value })
	}
}

// intro: kick and closed hats.
func (g grower) intro(t Track, src Pattern) Pattern {
	switch partOf(t.Kind) {
	case partKick, partHat:
		return clonePattern(src)
	}
	return emptied(src)
}

// build: no snare, clap, toms or crash; synths filtered down.
func (g grower) build(t Track, src Pattern, base Track) Pattern {
	switch partOf(t.Kind) {
	case partBackbeat, partTom, partCrash:
		return emptied(src)
	case partSynth:
		result := clonePattern(src)
		lockAll(&result, "cutoff", func(int) float64 { return filtered(base, 1) })
		return result
	}
	return clonePattern(src)
}

// riser: four bars, the roll track rolling faster and louder, synths opening
// from closed to a little past the peak's cutoff, the kick out on the last beat.
func (g grower) riser(t Track, src Pattern, base Track) Pattern {
	n := min(64, riserBars*max(1, src.Resolution))
	result := tiled(src, n)
	beat := max(1, src.Resolution/4)
	bar := max(1, src.Resolution)
	switch {
	case t.ID == g.roller:
		for i := range result.Steps {
			stage := 4 * i / n // quarters, eighths, sixteenths, sixteenths
			every := []int{beat, max(1, beat/2), max(1, beat/4), max(1, beat/4)}[stage]
			step := DefaultStep()
			step.Active = i%every == 0
			step.Velocity = .35 + .65*math.Pow(float64(i)/float64(n-1), 1.3)
			step.Accent = stage == 3 && i%beat == 0
			if t.Kind == Tom {
				step.Note = src.Steps[i%len(src.Steps)].Note
			}
			result.Steps[i] = step
		}
		result.Steps[n-1].Active = false // a breath before the drop
	case partOf(t.Kind) == partBackbeat:
		// Other snares and claps hold back in the last bar.
		for i := n - bar; i < n; i++ {
			result.Steps[i].Active = false
		}
	case partOf(t.Kind) == partTom || partOf(t.Kind) == partCrash:
		return emptied(result)
	case partOf(t.Kind) == partKick:
		for i := n - beat; i < n; i++ {
			result.Steps[i].Active = false
		}
	case partOf(t.Kind) == partSynth:
		low, high := filtered(base, 1), filtered(base, -.25)
		lockAll(&result, "cutoff", func(i int) float64 {
			x := float64(i) / float64(n-1)
			return low + (high-low)*math.Pow(x, 1.6)
		})
	}
	return result
}

// fill: one bar, the one a section looping src would play at bar index bar,
// with a fill over its last beat.
func (g grower) fill(t Track, src Pattern, bar int) Pattern {
	result := barAt(src, bar)
	n := len(result.Steps)
	beat := max(1, src.Resolution/4)
	switch {
	case t.ID == g.fillTrack:
		for i := n - beat; i < n; i++ {
			x := float64(i-(n-beat)) / float64(max(1, beat-1))
			step := DefaultStep()
			step.Active = true
			step.Velocity = .75 + .25*x
			step.Accent = i >= n-2
			if t.Kind == Tom {
				// Falling toms: high to low over the beat, from 1.6× to 0.8×
				// the root pitch, clamped first so a high tom still falls.
				step.Note = result.Steps[i].Note
				high, low := max(40, min(300, t.Parameters["pitch"]*1.6)), max(40, min(300, t.Parameters["pitch"]*.8))
				if high-low < 20 {
					low = max(40, high-60)
				}
				step.Locks = map[string]float64{"pitch": high + (low-high)*x}
			}
			result.Steps[i] = step
		}
	case partOf(t.Kind) == partKick:
		for i := n - max(1, beat/2); i < n; i++ {
			result.Steps[i].Active = false
		}
	case partOf(t.Kind) == partOpen || partOf(t.Kind) == partCrash:
		for i := n - beat; i < n; i++ {
			result.Steps[i].Active = false
		}
	}
	return result
}

// breakdown: no kick, snare, clap or toms; offbeat hats; only the lead synth,
// with a longer, darker, wetter sound; a crash once at the start.
//
// Sound changes here and in peak2 are step locks rather than stored pattern
// sounds: following the song recalls a slot's stored sound into the track,
// which would carry into every slot without one. Locks stay put, and later
// edits to the track's sound still reach every section.
func (g grower) breakdown(t Track, src Pattern, base Track) Pattern {
	switch partOf(t.Kind) {
	case partKick, partBackbeat, partTom, partOpen, partSampler:
		return emptied(src)
	case partHat:
		result := clonePattern(src)
		half := max(1, src.Resolution/8)
		offbeats := 0
		for i, s := range result.Steps {
			if s.Active && i%(2*half) == half {
				offbeats++
			}
		}
		for i := range result.Steps {
			if offbeats > 0 {
				result.Steps[i].Active = result.Steps[i].Active && i%(2*half) == half
			} else {
				result.Steps[i].Active = result.Steps[i].Active && i%(4*half) == 0
			}
			result.Steps[i].Velocity *= .8
		}
		return result
	case partCrash:
		result := emptied(tiled(src, min(64, riserBars*max(1, src.Resolution))))
		result.Steps[0].Active = true
		result.Steps[0].Velocity = .8
		return result
	case partSynth:
		if t.ID != g.lead {
			return emptied(src)
		}
		result := clonePattern(src)
		values := map[string]float64{"cutoff": filtered(base, .5)}
		if t.Kind == Acid {
			values["decay"] = min(.95, base.Parameters["decay"]+.25)
			values["resonance"] = min(.95, base.Parameters["resonance"]+.08)
		}
		for _, e := range base.Effects {
			switch e.Kind {
			case Delay, TapeEcho:
				values[EffectLockID(e.ID, "mix")] = min(.6, e.Parameters["mix"]+.15)
				values[EffectLockID(e.ID, "feedback")] = min(.75, e.Parameters["feedback"]+.15)
			case Reverb:
				values[EffectLockID(e.ID, "mix")] = min(.5, e.Parameters["mix"]+.1)
			}
		}
		lockValues(&result, values)
		return result
	}
	return clonePattern(src)
}

// peak2: the peak with a varied synth line, hotter synth sounds and a few
// ghost hats.
func (g grower) peak2(t Track, src Pattern, base Track) Pattern {
	result := clonePattern(src)
	switch partOf(t.Kind) {
	case partSynth:
		if !hasNotes(src) {
			return result
		}
		scratch := t
		scratch.Patterns = []Pattern{result}
		generate(&scratch, 0, "Mutate", g.p.Key, g.r)
		result = scratch.Patterns[0]
		if t.Kind == Acid {
			lockValues(&result, map[string]float64{
				"sweep":  min(1, base.Parameters["sweep"]+.3),
				"drive":  min(.8, base.Parameters["drive"]+.15),
				"accent": min(1, base.Parameters["accent"]+.1),
			})
		}
	case partHat:
		for i := range result.Steps {
			if !result.Steps[i].Active && g.r.Float64() < .2 {
				result.Steps[i].Active = true
				result.Steps[i].Velocity = .3 + .15*g.r.Float64()
				result.Steps[i].Probability = 1
			}
		}
	}
	return result
}
