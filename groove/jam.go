package groove

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
)

// Jam mode (alpha.44, bigger moves alpha.46): while a pattern loops, a
// change every few bars, each one an ordinary undoable edit. Held tracks are never touched. See
// TRACK_BUILDER_PLAN.md.

// JamAmount sets how much a jam changes at a time (alpha.46).
type JamAmount int

const (
	JamGentle   JamAmount = iota // one small move: a couple of steps, a drop, a nudge
	JamEvolving                  // one or two moves, including rewritten parts, new phrases, parts brought in and new drum sounds
	JamWild                      // two or three moves, bigger nudges
)

// JamAmounts names the amounts, in order, for menus.
var JamAmounts = []string{"Gentle", "Evolving", "Wild"}

// JamState is what a jam remembers between changes: the tracks it dropped,
// so it brings back only those and never unmutes one you muted, and how many
// changes it has made, which sets where it is in its build and release.
type JamState struct {
	Dropped map[string]bool
	Amount  JamAmount
	Moves   int
	touched map[string]bool // tracks changed by this change's moves
}

// Clone copies the state so a change can be worked out on the side.
func (s JamState) Clone() JamState {
	c := s
	c.Dropped = map[string]bool{}
	for id := range s.Dropped {
		c.Dropped[id] = true
	}
	c.touched = nil
	return c
}

// jamWave is the length, in changes, of one build and release: half of it
// leans towards adding parts and opening filters, half towards thinning out.
const jamWave = 12

// Rising reports whether the jam is in the building half of its wave.
func (s JamState) Rising() bool { return s.Moves%jamWave < jamWave/2 }

func (s JamState) maxDropped() int {
	if s.Amount == JamGentle {
		return 2
	}
	return 3
}

// jamRanges keeps nudged synth controls musical over a long jam.
var jamRanges = map[string]span{
	"cutoff":    {.15, .85},
	"resonance": {.3, .92},
	"decay":     {.1, .8},
	"env":       {.2, .95},
	"accent":    {.3, .95},
	"sweep":     {0, .9},
	"drive":     {0, .55},
}

type jamMove struct {
	move    func(*Project, *rand.Rand, *JamState) string
	weights [3]float64 // by JamAmount
}

var jamMoves = []jamMove{
	{jamVary, [3]float64{.35, .18, .14}},
	{jamDropOrReturn, [3]float64{.25, .17, .16}},
	{jamNudge, [3]float64{.4, .17, .15}},
	{jamRewrite, [3]float64{0, .15, .16}},
	{jamRephrase, [3]float64{0, .13, .15}},
	{jamBringIn, [3]float64{0, .12, .12}},
	{jamSwapSound, [3]float64{0, .08, .12}},
}

// Jam makes one change to the selected pattern or the tracks' sounds and
// describes it, or returns "" when nothing could change (everything held).
// A change is one move when Gentle, one or two when Evolving and two or
// three when Wild, each on a different track.
func Jam(p *Project, r *rand.Rand, state *JamState) string {
	if state.Dropped == nil {
		state.Dropped = map[string]bool{}
	}
	// Forget drops that were undone or unmuted by hand.
	for id := range state.Dropped {
		if t := p.Track(id); t == nil || !t.Mute {
			delete(state.Dropped, id)
		}
	}
	count := 1
	switch state.Amount {
	case JamEvolving:
		if r.Float64() < .5 {
			count = 2
		}
	case JamWild:
		count = 2 + r.IntN(2)
	}
	state.touched = map[string]bool{}
	defer func() { state.touched = nil }()
	infos := []string{}
	for n := 0; n < count; n++ {
		if info := jamOne(p, r, state); info != "" {
			infos = append(infos, info)
		}
	}
	if len(infos) == 0 {
		return ""
	}
	state.Moves++
	return strings.Join(infos, " · ")
}

// jamOne makes one move, chosen by weight, falling back to any move that can
// happen.
func jamOne(p *Project, r *rand.Rand, state *JamState) string {
	amount := min(int(state.Amount), 2)
	total := 0.
	for _, m := range jamMoves {
		total += m.weights[amount]
	}
	for attempt := 0; attempt < 6; attempt++ {
		x, n := r.Float64()*total, 0
		for n < len(jamMoves)-1 && (jamMoves[n].weights[amount] == 0 || x > jamMoves[n].weights[amount]) {
			x -= jamMoves[n].weights[amount]
			n++
		}
		if info := jamMoves[n].move(p, r, state); info != "" {
			return info
		}
	}
	for _, m := range jamMoves {
		if m.weights[amount] == 0 {
			continue
		}
		if info := m.move(p, r, state); info != "" {
			return info
		}
	}
	return ""
}

// jamCandidates lists unheld tracks this change has not touched yet.
func jamCandidates(p *Project, state *JamState, ok func(Track) bool) []int {
	list := []int{}
	for i, t := range p.Tracks {
		if !t.Hold && !state.touched[t.ID] && ok(t) {
			list = append(list, i)
		}
	}
	return list
}

// jamPick picks one candidate and marks it touched, or returns nil.
func jamPick(p *Project, r *rand.Rand, state *JamState, ok func(Track) bool) *Track {
	list := jamCandidates(p, state, ok)
	if len(list) == 0 {
		return nil
	}
	t := &p.Tracks[list[r.IntN(len(list))]]
	if state.touched != nil {
		state.touched[t.ID] = true
	}
	return t
}

// jamVary changes a few steps of one playing part: drum hits off the beat,
// synth notes to a neighbouring scale note, an accent or a slide.
func jamVary(p *Project, r *rand.Rand, state *JamState) string {
	slot := p.Pattern
	// A crash keeps its one hit per loop.
	t := jamPick(p, r, state, func(t Track) bool {
		return !t.Mute && hasNotes(t.Patterns[slot]) && t.Kind != Sampler && t.Kind != Crash
	})
	if t == nil {
		return ""
	}
	want := []int{2, 3, 4}[min(int(state.Amount), 2)]
	steps := t.Patterns[slot].Steps
	beat := max(1, t.Patterns[slot].Resolution/4)
	changed := 0
	for tries := 0; tries < 24 && changed < want; tries++ {
		i := r.IntN(len(steps))
		s := &steps[i]
		if t.Kind.synthPart() {
			switch {
			case s.Active && r.Float64() < .6:
				s.Note = neighbourNote(s.Note, p.Key, r)
			case s.Active:
				s.Accent = !s.Accent
				if t.Kind.Capabilities().Slide && r.Float64() < .5 {
					s.Slide, s.Accent = !s.Slide, !s.Accent
				}
			case i > 0 && steps[i-1].Active:
				// A new note next to an existing one, in the same register.
				s.Active, s.Note, s.Velocity, s.Probability = true, neighbourNote(steps[i-1].Note, p.Key, r), steps[i-1].Velocity, 1
			default:
				continue
			}
			changed++
			continue
		}
		// Drums keep their downbeats so the groove stays anchored.
		if i%beat == 0 && (t.Kind == Kick || t.Kind == Snare || t.Kind == Clap) {
			continue
		}
		s.Active = !s.Active
		if s.Active {
			s.Probability = 1
			s.Velocity = .4 + .4*r.Float64()
		}
		changed++
	}
	if changed == 0 {
		return ""
	}
	return "varied " + t.Name
}

// neighbourNote moves a note one or two steps along the key's scale.
func neighbourNote(note int, key *Key, r *rand.Rand) int {
	scale, root := []int{0, 3, 5, 7, 10}, 36
	if key != nil {
		scale, root = scaleNotes(key.Scale), key.BassRoot()
	}
	notes := []int{}
	for octave := -2; octave <= 4; octave++ {
		for _, degree := range scale {
			if n := root + 12*octave + degree; n >= 24 && n <= 96 && (len(notes) == 0 || n > notes[len(notes)-1]) {
				notes = append(notes, n)
			}
		}
	}
	nearest := 0
	for i, n := range notes {
		if abs(n-note) < abs(notes[nearest]-note) {
			nearest = i
		}
	}
	move := []int{-2, -1, 1, 2}[r.IntN(4)]
	return notes[max(0, min(len(notes)-1, nearest+move))]
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// jamDropOrReturn brings back a part the jam dropped, or drops one: never the
// kick, nor the last synth still playing. While the jam builds it mostly
// brings parts back; while it releases it mostly drops them.
func jamDropOrReturn(p *Project, r *rand.Rand, state *JamState) string {
	slot := p.Pattern
	back := .6
	if state.Amount != JamGentle {
		back = map[bool]float64{true: .85, false: .3}[state.Rising()]
	}
	if len(state.Dropped) > 0 && (len(state.Dropped) >= state.maxDropped() || r.Float64() < back) {
		if t := jamPick(p, r, state, func(t Track) bool { return state.Dropped[t.ID] }); t != nil {
			t.Mute = false
			delete(state.Dropped, t.ID)
			return "brought back " + t.Name
		}
	}
	if len(state.Dropped) >= state.maxDropped() || state.Amount != JamGentle && state.Rising() && r.Float64() < .6 {
		return ""
	}
	synths := 0
	for _, t := range p.Tracks {
		if t.Kind.synthPart() && !t.Mute && hasNotes(t.Patterns[slot]) {
			synths++
		}
	}
	t := jamPick(p, r, state, func(t Track) bool {
		return !t.Mute && t.Kind != Kick && hasNotes(t.Patterns[slot]) && !(t.Kind.synthPart() && synths < 2)
	})
	if t == nil {
		return ""
	}
	t.Mute = true
	state.Dropped[t.ID] = true
	return "dropped " + t.Name
}

// jamNudge moves one control a little: a synth's filter, envelope or
// accent within jamRanges, or (when not Gentle) a drum's decay or tone, or
// the mix of a synth's echo, reverb or chorus. Cutoff leans up while the jam
// builds and down while it releases.
func jamNudge(p *Project, r *rand.Rand, state *JamState) string {
	slot := p.Pattern
	gentle := state.Amount == JamGentle
	// Mostly the 303s, which there are fewer of but which a nudge changes most.
	drums := !gentle && r.Float64() < .3
	t := jamPick(p, r, state, func(t Track) bool {
		return !t.Mute && hasNotes(t.Patterns[slot]) && (t.Kind.synthPart() != drums) && t.Kind != Sampler
	})
	if t == nil && drums {
		t = jamPick(p, r, state, func(t Track) bool { return !t.Mute && hasNotes(t.Patterns[slot]) && t.Kind.synthPart() })
	}
	if t == nil {
		return ""
	}
	size := []float64{1, 1.4, 2}[min(int(state.Amount), 2)]
	step := (.04 + .06*r.Float64()) * size
	if !t.Kind.synthPart() {
		return jamNudgeDrum(t, r, step)
	}
	if !gentle && r.Float64() < .25 {
		if info := jamNudgeMix(t, r, state, step); info != "" {
			return info
		}
	}
	ids := []string{"cutoff", "cutoff", "resonance", "decay", "env"}
	if !gentle {
		ids = append(ids, "cutoff", "accent", "sweep", "drive")
	}
	if t.Kind == Dual {
		ids = []string{"cutoff"}
	}
	id := ids[r.IntN(len(ids))]
	if _, ok := t.Parameters[id]; !ok {
		id = "cutoff"
	}
	limits := jamRanges[id]
	value := t.Parameters[id]
	// Lean back towards the middle of the range so a long jam wanders
	// rather than pinning a control at one end.
	lean := .5 + .8*((limits[0]+limits[1])/2-value)
	if id == "cutoff" && !gentle {
		lean += map[bool]float64{true: .25, false: -.25}[state.Rising()]
	}
	if r.Float64() >= lean {
		step = -step
	}
	next := max(limits[0], min(limits[1], value+step))
	if next == value {
		return ""
	}
	t.Parameters[id] = float64(int(next*100+.5)) / 100
	return fmt.Sprintf("%s %s %s %.2f", t.Name, parameterName(t.Kind, id), arrow(next < value), t.Parameters[id])
}

func arrow(down bool) string {
	if down {
		return "↓"
	}
	return "↑"
}

// jamNudgeDrum moves a drum's decay or tone by share of its range, inside
// the middle 70% of the range.
func jamNudgeDrum(t *Track, r *rand.Rand, share float64) string {
	params := []Parameter{}
	for _, param := range Parameters(t.Kind) {
		if param.ID == "decay" || param.ID == "tone" {
			params = append(params, param)
		}
	}
	if len(params) == 0 {
		return ""
	}
	param := params[r.IntN(len(params))]
	width := param.Max - param.Min
	low, high := param.Min+.15*width, param.Max-.15*width
	value := t.Parameters[param.ID]
	delta := share * width
	if r.Float64() >= .5+.8*((low+high)/2-value)/width {
		delta = -delta
	}
	next := max(low, min(high, value+delta))
	if next == value {
		return ""
	}
	t.Parameters[param.ID] = next
	return fmt.Sprintf("%s %s %s", t.Name, param.Name, arrow(next < value))
}

// jamMixes are the effects whose wet mix a jam rides, and the range it keeps.
var jamMixes = map[EffectKind]span{Delay: {.05, .45}, TapeEcho: {.05, .45}, Reverb: {.04, .35}, Chorus: {.05, .45}}

// jamNudgeMix rides the mix of one of the synth's echoes, reverbs or
// choruses: wetter while the jam releases, drier while it builds.
func jamNudgeMix(t *Track, r *rand.Rand, state *JamState, step float64) string {
	list := []int{}
	for i, e := range t.Effects {
		if _, ok := jamMixes[e.Kind]; ok && e.Enabled {
			list = append(list, i)
		}
	}
	if len(list) == 0 {
		return ""
	}
	e := &t.Effects[list[r.IntN(len(list))]]
	limits := jamMixes[e.Kind]
	value := e.Parameters["mix"]
	lean := .5 + 1.5*((limits[0]+limits[1])/2-value) + map[bool]float64{true: -.2, false: .2}[state.Rising()]
	step *= .6
	if r.Float64() >= lean {
		step = -step
	}
	next := max(limits[0], min(limits[1], value+step))
	if next == value {
		return ""
	}
	e.Parameters["mix"] = float64(int(next*100+.5)) / 100
	return fmt.Sprintf("%s %s mix %s %.2f", t.Name, e.Kind.Name(), arrow(next < value), e.Parameters["mix"])
}

// jamDrum reports whether a track plays drum family patterns.
func jamDrum(t Track) bool { return !t.Kind.synthPart() && len(drumFamilies[t.Kind]) > 0 }

// jamRewrite gives one playing drum a different pattern from its families:
// hats from eighths to a shuffle, a rimshot from clave to syncopated. The
// kick only changes when Wild.
func jamRewrite(p *Project, r *rand.Rand, state *JamState) string {
	slot := p.Pattern
	t := jamPick(p, r, state, func(t Track) bool {
		return !t.Mute && hasNotes(t.Patterns[slot]) && jamDrum(t) && (t.Kind != Kick || state.Amount == JamWild)
	})
	if t == nil {
		return ""
	}
	families := DrumFamilies(t.Kind)
	pattern := &t.Patterns[slot]
	before := append([]Step(nil), pattern.Steps...)
	current := closestFamily(t.Kind, pattern.Steps)
	for _, n := range r.Perm(len(families)) {
		if families[n] == current {
			continue
		}
		DrumPattern(pattern.Steps, t.Kind, families[n], r)
		shapeDrumPart(t, slot, p.Key)
		if !sameHits(before, pattern.Steps) {
			return t.Name + " → " + families[n]
		}
	}
	copy(pattern.Steps, before)
	return ""
}

// closestFamily names the family whose template best matches the first bar
// of steps ("?" positions count either way).
func closestFamily(kind Kind, steps []Step) string {
	best, score := "", -1
	for _, f := range drumFamilies[kind] {
		n := 0
		for i := 0; i < 16 && i < len(steps); i++ {
			c := byte('.')
			if i < len(f.template) {
				c = f.template[i]
			}
			if c == '?' || (c != '.') == steps[i].Active {
				n++
			}
		}
		if n > score {
			best, score = f.name, n
		}
	}
	return best
}

func sameHits(a, b []Step) bool {
	for i := range a {
		if a[i].Active != b[i].Active {
			return false
		}
	}
	return true
}

// jamRephrase writes a new phrase into part of a synth line, in the key and
// register of the old one: one bar of a longer line or the end of its first
// bar (never the first half-bar, so the line keeps its opening), or the
// second half of a one-bar line. Wild
// sometimes writes a whole new line.
func jamRephrase(p *Project, r *rand.Rand, state *JamState) string {
	slot := p.Pattern
	t := jamPick(p, r, state, func(t Track) bool {
		n := len(t.Patterns[slot].Steps)
		return !t.Mute && t.Kind.synthPart() && hasNotes(t.Patterns[slot]) && n%16 == 0 && n > 0
	})
	if t == nil || p.Key == nil {
		return ""
	}
	pattern := &t.Patterns[slot]
	n := len(pattern.Steps)
	from, to, info := n/2, n, t.Name+" new second half"
	if bars := n / 16; bars > 1 {
		bar := 1 + r.IntN(bars-1)
		from, to, info = bar*16, bar*16+16, fmt.Sprintf("%s new phrase in bar %d", t.Name, bar+1)
		if r.Float64() < .3 {
			from, to, info = 8, 16, t.Name+" new end to bar 1"
		}
	}
	if state.Amount == JamWild && r.Float64() < .3 {
		from, to, info = 0, n, t.Name+" new line"
	}
	density := 0
	for _, s := range pattern.Steps {
		if s.Active {
			density++
		}
	}
	action := "Acid"
	switch {
	case t.Kind == Dual:
		action = "Bassline"
	case density*4 < n:
		action = "Sparse"
	case density*4 > 3*n:
		action = "Busy"
	}
	// A sparse phrase can leave the same rests; try a few.
	for attempt := 0; attempt < 4; attempt++ {
		fresh := Project{Tracks: []Track{*t}}.Clone().Tracks[0]
		generate(&fresh, slot, action, p.Key, r)
		shift := registerShift(pattern.Steps, fresh.Patterns[slot].Steps)
		steps := append([]Step(nil), pattern.Steps...)
		changed := false
		for i := from; i < to && i < len(fresh.Patterns[slot].Steps); i++ {
			s := fresh.Patterns[slot].Steps[i]
			s.Note = max(0, min(127, s.Note+shift))
			s.Locks = cloneMap(pattern.Steps[i].Locks)
			changed = changed || s.Active != steps[i].Active || s.Active && s.Note != steps[i].Note
			steps[i] = s
		}
		if changed {
			pattern.Steps = steps
			return info
		}
	}
	return ""
}

// registerShift is the octave shift that puts new notes nearest the old
// line's average pitch.
func registerShift(old, fresh []Step) int {
	mean := func(steps []Step) (float64, bool) {
		sum, n := 0., 0
		for _, s := range steps {
			if s.Active {
				sum, n = sum+float64(s.Note), n+1
			}
		}
		return sum / float64(max(n, 1)), n > 0
	}
	a, okA := mean(old)
	b, okB := mean(fresh)
	if !okA || !okB {
		return 0
	}
	return 12 * int(math.Round((a-b)/12))
}

// jamBringIn starts a silent track playing: a drum gets one of its families,
// a silent synth a sparse line in the key an octave up. It happens while the
// jam builds (any time when Wild). A crash only comes in on loops of two bars
// or more, as it hits once per loop.
func jamBringIn(p *Project, r *rand.Rand, state *JamState) string {
	if !state.Rising() && state.Amount != JamWild {
		return ""
	}
	slot := p.Pattern
	t := jamPick(p, r, state, func(t Track) bool {
		n := len(t.Patterns[slot].Steps)
		return !t.Mute && !hasNotes(t.Patterns[slot]) && (jamDrum(t) && (t.Kind != Crash || n >= 32) || t.Kind.synthPart() && p.Key != nil && n%16 == 0)
	})
	if t == nil {
		return ""
	}
	if t.Kind.synthPart() {
		generate(t, slot, map[bool]string{true: "Bassline", false: "Sparse"}[t.Kind == Dual], p.Key, r)
		for i := range t.Patterns[slot].Steps {
			s := &t.Patterns[slot].Steps[i]
			s.Note = min(127, s.Note+12)
		}
		return "brought in " + t.Name
	}
	families := DrumFamilies(t.Kind)
	family := families[r.IntN(len(families))]
	DrumPattern(t.Patterns[slot].Steps, t.Kind, family, r)
	shapeDrumPart(t, slot, p.Key)
	return "brought in " + t.Name + " (" + family + ")"
}

// jamSwapSound gives one playing drum another sound from the bank, keeping a
// tom's tuning so its notes stay in key.
func jamSwapSound(p *Project, r *rand.Rand, state *JamState) string {
	slot := p.Pattern
	t := jamPick(p, r, state, func(t Track) bool {
		return !t.Mute && hasNotes(t.Patterns[slot]) && jamDrum(t) && len(FactorySounds(t.Kind)) > 1
	})
	if t == nil {
		return ""
	}
	bank := FactorySounds(t.Kind)
	preset := bank[r.IntN(len(bank))]
	tune := t.Parameters["tune"]
	if ApplySound(t, preset, true) != nil {
		return ""
	}
	jitterSound(t, r)
	if t.Kind == Tom {
		t.Parameters["tune"] = tune
	}
	return t.Name + " sound → " + preset.Name
}

func parameterName(kind Kind, id string) string {
	for _, param := range Parameters(kind) {
		if param.ID == id {
			return param.Name
		}
	}
	return id
}
