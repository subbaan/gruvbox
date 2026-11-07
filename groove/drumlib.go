package groove

import "math/rand/v2"

// Drum families are one-bar templates, one character per sixteenth:
//
//	x  strong hit     o  medium hit     g  ghost
//	?  hit half the time                .  rest
//
// Later bars repeat the first with a small chance of extra ghosts; the last
// bar of a multi-bar pattern may vary its final beat like a fill.
type drumFamily struct {
	name     string
	template string
}

var drumFamilies = map[Kind][]drumFamily{
	Kick: {
		{"Four", "x...x...x...x..."},
		{"Four push", "x...x...x...x..o"},
		{"Four skip", "x...x...x..?x..."},
		{"Broken", "x......x..x....."},
		{"Electro", "x.....x...x....."},
		{"Electro double", "x..x....x.?....."},
		{"Halftime", "x.........?....."},
		{"Rolling", "x...x..ox...x.o."},
		{"Dub", "x.......x..?...."},
	},
	Snare: {
		{"Backbeat", "....x.......x..."},
		{"Backbeat ghosts", "..g.x..g..g.x.g."},
		{"Halftime", "........x......."},
		{"Electro", "....x.......x..g"},
		{"Broken", "....x....g..x..?"},
		{"Late", "....x........x.."},
	},
	Clap: {
		{"Backbeat", "....x.......x..."},
		{"Stacked", "....x.......x.?."},
		{"Halftime", "........x......."},
		{"Offbeat flam", "....x..g....x..g"},
	},
	ClosedHat: {
		{"8ths", "x.o.x.o.x.o.x.o."},
		{"16ths", "xgogxgogxgogxgog"},
		{"Offbeat 8ths", "..x...x...x...x."},
		{"Shuffle", "x..ox..ox..ox..o"},
		{"Sparse ticks", "..x..g.?..x..g.?"},
		{"Rolling", "gxgogxgogxgogxgo"},
		{"Gallop", "x.ox..x.x.ox..x."},
	},
	OpenHat: {
		{"Offbeat", "..x...x...x...x."},
		{"Lift", "..............x."},
		{"Two lifts", "......x.......x."},
		{"Sparse", "..x.........?..."},
	},
	Rimshot: {
		{"Syncopated", "...x..x....x..x."},
		{"Clave", "x..x..x...x.x..."},
		{"Sparse", "......x.......?."},
		{"Rolling", "..g...x...g..x.."},
	},
	Tom: {
		{"Toms", "x.....o...o...o."},
		{"Call", "..........x.o.o."},
		{"Sparse", "......x.......?."},
	},
	Crash: {
		{"Downbeat", "x..............."},
		{"Sparse", "x.......?......."},
	},
}

// DrumFamilies lists the pattern families available for kind.
func DrumFamilies(kind Kind) []string {
	var names []string
	for _, f := range drumFamilies[kind] {
		names = append(names, f.name)
	}
	return names
}

func drumFamilyNamed(kind Kind, name string) (drumFamily, bool) {
	for _, f := range drumFamilies[kind] {
		if f.name == name {
			return f, true
		}
	}
	return drumFamily{}, false
}

// DrumPattern writes a family into steps (whose length is kept) with
// humanised velocity. It reports false if kind has no such family.
func DrumPattern(steps []Step, kind Kind, family string, r *rand.Rand) bool {
	f, ok := drumFamilyNamed(kind, family)
	if !ok {
		return false
	}
	bars := max(1, (len(steps)+15)/16)
	extraGhosts := r.Float64() * .08
	for i := range steps {
		bar, pos := i/16, i%16
		step := DefaultStep()
		c := byte('.')
		if pos < len(f.template) {
			c = f.template[pos]
		}
		// The final beat of the last bar in a multi-bar pattern may vary.
		if bars > 1 && bar == bars-1 && pos >= 12 && r.Float64() < .35 {
			c = []byte{'.', 'g', 'o', '?'}[r.IntN(4)]
		}
		// In four bars, the end of bar two answers more gently.
		if bars == 4 && bar == 1 && pos >= 12 && r.Float64() < .15 {
			c = []byte{'.', 'g', 'o'}[r.IntN(3)]
		}
		if c == '.' && bar > 0 && kind != Kick && kind != Crash && r.Float64() < extraGhosts {
			c = 'g'
		}
		switch c {
		case 'x':
			step.Active, step.Velocity = true, .88
		case 'o':
			step.Active, step.Velocity = true, .66
		case 'g':
			step.Active, step.Velocity = true, .36
		case '?':
			step.Active, step.Velocity = r.Float64() < .5, .55
		}
		if step.Active {
			step.Velocity = min(1, max(.1, step.Velocity+(r.Float64()-.5)*.12))
		}
		steps[i] = step
	}
	return true
}

// randomDrumFamily picks a family for kind, or "" if it has none.
func randomDrumFamily(kind Kind, r *rand.Rand) string {
	families := drumFamilies[kind]
	if len(families) == 0 {
		return ""
	}
	return families[r.IntN(len(families))].name
}
