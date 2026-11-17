package groove

import (
	"fmt"
	"math/rand/v2"
	"slices"
)

// FitAction is every track's first generator (the G key): write a part that
// suits the groove around it, in the groove's style, key, register and length.
const FitAction = "Fit groove"

// StyleNamed finds a groove style by name.
func StyleNamed(name string) (GrooveStyle, bool) {
	for _, s := range GrooveStyles {
		if s.Name == name {
			return s, true
		}
	}
	return GrooveStyle{}, false
}

// synthRole names a new groove's synth part: the first 303 is the lead,
// later ones the second part, and a Dual takes its recipe's role.
func synthRole(kind Kind, acids int, recipe partRecipe) string {
	switch kind {
	case Acid:
		if acids == 0 {
			return "lead"
		}
		return "second"
	case Dual, SH101:
		return recipe.role
	}
	return ""
}

// roleLabels describe synth roles in status lines.
var roleLabels = map[string]string{
	"lead": "lead line", "bass": "bass line", "second": "counter line",
	"counter": "counter line", "upper": "line above the bass",
	"pad": "pad", "stab": "stabs", "layer": "layer under the lead",
}

// grooveSteps is the loop length a part in slot should have: the longest
// sixteenth-note pattern among the other tracks that play there, else among
// all the other tracks (0 if there are none).
func grooveSteps(p *Project, id string, slot int) int {
	playing, other := 0, 0
	for _, o := range p.Tracks {
		if o.ID == id || slot >= len(o.Patterns) {
			continue
		}
		pattern := o.Patterns[slot]
		if pattern.Resolution != 16 || len(pattern.Steps) == 0 || len(pattern.Steps)%16 != 0 {
			continue
		}
		other = max(other, len(pattern.Steps))
		if hasNotes(pattern) {
			playing = max(playing, len(pattern.Steps))
		}
	}
	if playing > 0 {
		return playing
	}
	return other
}

// bassLine finds the bass among the other synths in slot: a track whose
// role is the lead or bass, or, for a track with no role yet, whose lower
// notes reach below the octave above the key's bass root. It returns the
// lowest such line and its middle note. Muted tracks count, since Jam drops
// parts by muting them and brings them back.
func bassLine(p *Project, id string, slot int) (*Track, int) {
	limit := 48
	if p.Key != nil {
		limit = p.Key.BassRoot() + 12
	}
	var bass *Track
	middle := 0
	for i := range p.Tracks {
		o := &p.Tracks[i]
		if o.ID == id || !o.Kind.synthPart() || slot >= len(o.Patterns) {
			continue
		}
		notes := []int{}
		for _, s := range o.Patterns[slot].Steps {
			if s.Active {
				notes = append(notes, s.Note)
			}
		}
		if len(notes) == 0 {
			continue
		}
		slices.Sort(notes)
		isBass := o.Role == "lead" || o.Role == "bass"
		if o.Role == "" {
			isBass = notes[len(notes)/4] < limit
		}
		if isBass && (bass == nil || notes[len(notes)/2] < middle) {
			bass, middle = o, notes[len(notes)/2]
		}
	}
	return bass, middle
}

func bassPlaying(p *Project, id string, slot int) bool {
	bass, _ := bassLine(p, id, slot)
	return bass != nil
}

// answers reports whether a role's line should leave room for the bass:
// counter lines, stabs and second lines do; pads ring over it and layers
// double it on purpose.
func answers(role string) bool {
	switch role {
	case "second", "counter", "stab", "upper":
		return true
	}
	return false
}

// bassOnsets marks the steps of an n-step line that land with a bass note
// starting, on any pass of the two loops (they may differ in length).
func bassOnsets(bass Pattern, n int) []bool {
	hits := make([]bool, n)
	m := len(bass.Steps)
	if m == 0 || n == 0 {
		return hits
	}
	passes := m / gcd(m, n) // passes of the n-step line before both loops realign
	for i := range hits {
		for k := 0; k < passes && !hits[i]; k++ {
			hits[i] = bass.Steps[(i+k*n)%m].Active
		}
	}
	return hits
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// answerBass moves a line's notes off the bass's onsets into the nearest gap
// (a step later first, then earlier, then two steps), so it plays in the
// bass's rests instead of on top of it. A few collisions stay (more often on
// a bar's downbeat, where a shared hit sounds deliberate); a note with no gap
// nearby is dropped, but never more than half the line, so it keeps its shape
// over a busy bass.
func answerBass(p *Project, t *Track, slot int, r *rand.Rand) {
	bass, _ := bassLine(p, t.ID, slot)
	pattern := &t.Patterns[slot]
	if bass == nil || bass.Patterns[slot].Resolution != pattern.Resolution {
		return
	}
	steps := pattern.Steps
	hits := bassOnsets(bass.Patterns[slot], len(steps))
	notes := 0
	for _, s := range steps {
		if s.Active {
			notes++
		}
	}
	dropped := 0
	for i := range steps {
		if !steps[i].Active || !hits[i] {
			continue
		}
		keep := .1
		if i%16 == 0 {
			keep = .4
		}
		if r.Float64() < keep {
			continue
		}
		moved := false
		for _, d := range []int{1, -1, 2, -2} {
			j := i + d
			if j < 0 || j >= len(steps) || steps[j].Active || hits[j] {
				continue
			}
			steps[j] = steps[i]
			steps[j].Slide, steps[j].Length = false, 1
			steps[i] = DefaultStep()
			moved = true
			break
		}
		if !moved && (dropped+1)*2 <= notes {
			steps[i] = DefaultStep()
			dropped++
		}
	}
}

// chooseRole picks a synth's role: the bass (a 303's lead) when no other
// synth holds the bass register, otherwise a part above it: a 303's second
// line, or one of the style's Dual parts.
func chooseRole(p *Project, t *Track, slot int, style GrooveStyle, styled bool, r *rand.Rand) string {
	bass := bassPlaying(p, t.ID, slot)
	switch {
	case !bass && t.Kind == Acid:
		return "lead"
	case !bass:
		return "bass"
	case t.Kind == Acid:
		return "second"
	case styled && len(style.dual) > 0:
		return style.dual[r.IntN(len(style.dual))].role
	}
	return "upper"
}

// roleRecipe is the recipe for a synth role: the style's part when the
// groove has a style, else a plain bass line or a sparse line an octave up.
func roleRecipe(kind Kind, role string, style GrooveStyle, styled bool) partRecipe {
	if kind == SH101 {
		return sh101Recipe(role, style, styled)
	}
	if styled {
		switch {
		case kind == Acid && role == "lead":
			return style.lead
		case kind == Acid:
			return style.second
		case role == "bass":
			// A Dual bass: the round layer sounds playing the style's lead lines.
			bass := dualLayer
			bass.role, bass.level, bass.actions = "bass", 0, nil
			for _, action := range style.lead.actions {
				if action == "Acid" {
					action = "Bassline"
				}
				bass.actions = append(bass.actions, action)
			}
			return bass
		}
		for _, recipe := range style.dual {
			if recipe.role == role {
				return recipe
			}
		}
	}
	if role == "lead" || role == "bass" {
		return partRecipe{role: role, actions: []string{map[Kind]string{Acid: "Acid", Dual: "Bassline"}[kind]}}
	}
	return partRecipe{role: role, actions: []string{"Sparse"}, octave: 12}
}

// pickAction chooses one of a recipe's actions, skipping the empty ("leave
// it silent") ones, or "" if it has none.
func pickAction(recipe partRecipe, r *rand.Rand) string {
	actions := []string{}
	for _, action := range recipe.actions {
		if action != "" {
			actions = append(actions, action)
		}
	}
	if len(actions) == 0 {
		return ""
	}
	return actions[r.IntN(len(actions))]
}

// synthRecipe gives a synth its role (choosing one if it has none) and the
// recipe for it.
func synthRecipe(p *Project, t *Track, slot int, r *rand.Rand) partRecipe {
	style, styled := StyleNamed(p.Style)
	if p.Key == nil {
		var scales []PhraseScale
		if styled {
			scales = style.scales
		}
		k := RandomKey(r, scales)
		p.Key = &k
	}
	if t.Role == "" {
		t.Role = chooseRole(p, t, slot, style, styled, r)
	}
	return roleRecipe(t.Kind, t.Role, style, styled)
}

// fitSynth writes a synth's notes in slot for its role. It returns a label
// for the part.
func fitSynth(p *Project, t *Track, slot int, r *rand.Rand) string {
	recipe := synthRecipe(p, t, slot, r)
	label := roleLabels[t.Role]
	if label == "" {
		label = t.Role
	}
	action := pickAction(recipe, r)
	if action == "Layer" {
		if layerLead(p, t, slot) {
			return label
		}
		// Nothing to double: a sparse line above the bass instead.
		action, recipe.octave, recipe.hold, label = "Sparse", 12, false, roleLabels["upper"]
	}
	if action == "" {
		action = "Sparse"
	}
	generate(t, slot, action, p.Key, r)
	if recipe.octave != 0 {
		for i := range t.Patterns[slot].Steps {
			s := &t.Patterns[slot].Steps[i]
			s.Note = min(127, s.Note+recipe.octave)
		}
	}
	placeAboveBass(p, t, slot, r)
	if recipe.hold {
		holdNotes(t.Patterns[slot].Steps)
	}
	return label
}

// placeAboveBass fits a part that is not the bass around it: an octave or
// more above it, and, for the roles that answer it, in its rests.
func placeAboveBass(p *Project, t *Track, slot int, r *rand.Rand) {
	if !t.Kind.synthPart() || t.Role == "" || t.Role == "lead" || t.Role == "bass" {
		return
	}
	// Answer first: moving and dropping notes changes the line's middle.
	if answers(t.Role) {
		answerBass(p, t, slot, r)
	}
	liftAboveBass(p, t, slot)
}

// liftAboveBass moves a line up by octaves until its middle note is at least
// an octave above the bass line's, so a part above the bass never sits in
// its register.
func liftAboveBass(p *Project, t *Track, slot int) {
	line, bass := bassLine(p, t.ID, slot)
	if line == nil {
		return
	}
	steps := t.Patterns[slot].Steps
	notes := []int{}
	for _, s := range steps {
		if s.Active {
			notes = append(notes, s.Note)
		}
	}
	if len(notes) == 0 {
		return
	}
	slices.Sort(notes)
	shift := 0
	for notes[len(notes)/2]+shift < bass+12 && notes[len(notes)-1]+shift+12 <= 108 {
		shift += 12
	}
	for i := range steps {
		steps[i].Note = min(127, steps[i].Note+shift)
	}
}

// fitDrum writes a drum's pattern in slot: one of the style's families for
// it, else the drum's usual first pattern. It returns the pattern's name.
func fitDrum(p *Project, t *Track, slot int, r *rand.Rand) string {
	style, styled := StyleNamed(p.Style)
	recipe, ok := style.drums[t.Kind]
	if x, extra := style.extra(t.Kind); extra {
		recipe, ok = x.recipe, true
	}
	if styled && ok {
		if family := pickAction(recipe, r); family != "" && DrumPattern(t.Patterns[slot].Steps, t.Kind, family, r) {
			shapeDrumPart(t, slot, p.Key)
			return family
		}
	}
	action := kindActions(t.Kind)[0]
	generate(t, slot, action, p.Key, r)
	return action
}

// FitPart rewrites t's pattern in slot as a part that suits the groove: as
// long as the longest playing pattern (or a whole number of its loops), in the groove's style and key, and
// for a synth in its role's register (above the bass when another synth
// already plays one). The pattern's stored sound state is kept and the sound
// is not changed. It returns a status line.
func FitPart(p *Project, t *Track, slot int, r *rand.Rand) string {
	pattern := &t.Patterns[slot]
	steps := len(pattern.Steps)
	if pattern.Resolution == 16 {
		// Match the groove unless the part is already a whole number of
		// its loops (a 2-bar line over 1-bar drums still lines up).
		if n := grooveSteps(p, t.ID, slot); n > 0 && (steps < n || steps%n != 0) {
			steps = n
		}
		state := pattern.State
		resetPatternSlot(t, slot, steps)
		t.Patterns[slot].State = state
	}
	var part string
	switch {
	case t.Kind.synthPart():
		part = fitSynth(p, t, slot, r)
	case t.Kind == Sampler:
		part = kindActions(Sampler)[0]
		generate(t, slot, part, p.Key, r)
	default:
		part = fitDrum(p, t, slot, r)
	}
	info := FitAction + " · " + t.Name + ": "
	if _, styled := StyleNamed(p.Style); styled {
		info += p.Style + " "
	}
	info += part
	if bars := steps / 16; bars > 1 {
		info += fmt.Sprintf(" · %d bars", bars)
	}
	return info
}

// AddTrack appends a new track of kind set up to join the groove. Each
// pattern slot takes the length of the parts playing there, and in a styled
// groove the track gets the sound, level and effects the style gives that
// part (a synth also gets its role), as a new groove would. Its patterns
// start empty; Fit groove writes a part. It returns the track's index and a
// status line.
func AddTrack(p *Project, kind Kind, r *rand.Rand) (int, string) {
	t := NewTrack(kind, len(p.Tracks))
	for slot := range t.Patterns {
		if n := grooveSteps(p, t.ID, slot); n > 16 {
			resetPatternSlot(&t, slot, n)
		}
	}
	info := "Added " + t.Name
	if style, styled := StyleNamed(p.Style); styled && kind != Sampler {
		recipe, ok := style.drums[kind]
		if x, extra := style.extra(kind); extra {
			recipe, ok = x.recipe, true
			t.Level = x.level
		}
		if kind.synthPart() {
			t.Role = chooseRole(p, &t, p.Pattern, style, true, r)
			recipe, ok = roleRecipe(kind, t.Role, style, true), true
			info += " as the " + style.Name + " " + roleLabels[t.Role]
		} else if ok {
			info += " with a " + style.Name + " sound"
		}
		if ok {
			if recipe.level > 0 {
				t.Level = recipe.level
			}
			jitterSound(&t, r)
			if sound := applyRecipePatch(&t, recipe, r); sound != "" {
				info += " (" + sound + ")"
			} else {
				applyRecipeSound(&t, recipe, r)
			}
		}
	}
	p.Tracks = append(p.Tracks, t)
	return len(p.Tracks) - 1, info + " · press G to write a part that fits"
}
