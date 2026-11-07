package groove

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestStyleRecipesNameRealParts(t *testing.T) {
	for _, style := range GrooveStyles {
		for _, part := range []partRecipe{style.lead, style.second} {
			for _, action := range part.actions {
				if action != "" && !slices.Contains(Actions(Acid), action) {
					t.Errorf("%s: unknown 303 action %q", style.Name, action)
				}
			}
			for id := range part.sound {
				if !hasParameter(Acid, id) {
					t.Errorf("%s: unknown 303 parameter %q", style.Name, id)
				}
			}
		}
		for kind, part := range style.drums {
			for _, family := range part.actions {
				if _, ok := drumFamilyNamed(kind, family); family != "" && !ok {
					t.Errorf("%s: %s has no family %q", style.Name, kind, family)
				}
			}
			for id := range part.sound {
				if !hasParameter(kind, id) {
					t.Errorf("%s: %s has no parameter %q", style.Name, kind, id)
				}
			}
		}
	}
}

func hasParameter(kind Kind, id string) bool {
	for _, p := range Parameters(kind) {
		if p.ID == id {
			return true
		}
	}
	return false
}

func TestNewGrooveVariesAndValidates(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 11))
	styles, keys, tempos, kicks := map[string]bool{}, map[string]bool{}, map[float64]bool{}, map[string]bool{}
	for n := 0; n < 60; n++ {
		p, style := NewGroove(r)
		if err := p.Validate(); err != nil {
			t.Fatalf("%s groove invalid: %v", style, err)
		}
		if p.Tracks[0].Kind != Acid || trackOf(p, Kick) == nil || trackOf(p, ClosedHat) == nil {
			t.Fatal("a groove needs a 303 lead, a kick and a closed hat")
		}
		styles[style], keys[p.Key.Name()], tempos[p.BPM] = true, true, true
		kick := ""
		for _, s := range trackOf(p, Kick).Patterns[0].Steps {
			kick += map[bool]string{true: "x", false: "."}[s.Active]
		}
		kicks[kick] = true
		if GrooveInfo(style, p) == "" {
			t.Fatal("empty groove info")
		}
	}
	if len(styles) < 4 || len(keys) < 8 || len(tempos) < 8 || len(kicks) < 4 {
		t.Fatalf("too little variety: %d styles, %d keys, %d tempos, %d kick patterns", len(styles), len(keys), len(tempos), len(kicks))
	}
}

func TestSecondPartSitsAboveTheLead(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 5))
	for n := 0; n < 20; n++ {
		p, _ := NewGroove(r)
		lowest := func(tr Track) int {
			low := 128
			for _, s := range tr.Patterns[0].Steps {
				if s.Active {
					low = min(low, s.Note)
				}
			}
			return low
		}
		if p.Tracks[1].Kind != Acid {
			continue // a Dual layer sits below on purpose
		}
		lead, second := lowest(p.Tracks[0]), lowest(p.Tracks[1])
		if second < 128 && lead < 128 && second < lead {
			t.Fatalf("second 303 (%d) below the lead (%d)", second, lead)
		}
	}
}

func TestRandomGrooveNamesBankSounds(t *testing.T) {
	r := rand.New(rand.NewPCG(8, 9))
	named := 0
	for n := 0; n < 40; n++ {
		p, info := RandomGroove(r)
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(info, "BPM") {
			t.Fatalf("info %q", info)
		}
		if strings.Contains(info, p.Tracks[0].Name+": ") || strings.Contains(info, p.Tracks[1].Name+": ") {
			named++
		}
	}
	if named < 10 {
		t.Fatalf("only %d of 40 grooves named a bank sound", named)
	}
	kits := 0
	for n := 0; n < 40; n++ {
		if _, info := RandomGroove(r); strings.Contains(info, " · kit: ") {
			kits++
		}
	}
	if kits < 15 {
		t.Fatalf("only %d of 40 grooves used a bank drum sound", kits)
	}
}

func TestStyleSoundsAllExist(t *testing.T) {
	for _, style := range GrooveStyles {
		recipes := map[Kind][]partRecipe{Acid: {style.lead, style.second}}
		for kind, recipe := range style.drums {
			recipes[kind] = append(recipes[kind], recipe)
		}
		for _, x := range style.extras {
			recipes[x.kind] = append(recipes[x.kind], x.recipe)
			for _, action := range x.recipe.actions {
				if _, ok := drumFamilyNamed(x.kind, action); !ok {
					t.Errorf("%s %s: no family %q", style.Name, x.kind.Name(), action)
				}
			}
		}
		for kind, list := range recipes {
			for _, recipe := range list {
				for _, name := range recipe.patches {
					if _, ok := FactorySoundNamed(kind, name); !ok {
						t.Errorf("%s %s: no sound %q", style.Name, kind.Name(), name)
					}
				}
			}
		}
	}
}

func hitCount(steps []Step) int {
	n := 0
	for _, s := range steps {
		if s.Active {
			n++
		}
	}
	return n
}

func TestNewGrooveLayoutsComeFromThePool(t *testing.T) {
	r := rand.New(rand.NewPCG(21, 22))
	layouts := map[string]bool{}
	present, playing := map[Kind]int{}, map[Kind]int{}
	withSilent, layered := 0, 0
	const grooves = 300
	for n := 0; n < grooves; n++ {
		p, style := NewGroove(r)
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		names, layout, silent := map[string]bool{}, "", 0
		order := map[Kind]int{}
		for i, kind := range trackOrder {
			order[kind] = i
		}
		for i, track := range p.Tracks {
			if names[track.Name] {
				t.Fatalf("%s: two tracks named %s", style, track.Name)
			}
			names[track.Name] = true
			if i > 0 && order[track.Kind] < order[p.Tracks[i-1].Kind] {
				t.Fatalf("%s: %s after %s", style, track.Name, p.Tracks[i-1].Name)
			}
			layout += string(track.Kind) + " "
			steps := track.Patterns[0].Steps
			if len(steps) != len(p.Tracks[0].Patterns[0].Steps) {
				t.Fatalf("%s %s is %d steps", style, track.Name, len(steps))
			}
			present[track.Kind]++
			hits := hitCount(steps)
			if hits == 0 {
				silent++
				continue
			}
			playing[track.Kind]++
			switch track.Kind {
			case Crash:
				if hits != 1 || len(steps) < 32 {
					t.Fatalf("%s crash: %d hits in %d steps", style, hits, len(steps))
				}
			case Tom:
				scale := map[int]bool{}
				for _, d := range scaleNotes(p.Key.Scale) {
					scale[(p.Key.Root+d)%12] = true
				}
				for _, s := range steps {
					if s.Active && (s.Note < 36 || s.Note > 48 || !scale[s.Note%12]) {
						t.Fatalf("%s tom note %d out of key %s", style, s.Note, p.Key.Name())
					}
				}
			case Dual:
				// A layer doubles the lead an octave down.
				lead := p.Tracks[0].Patterns[0].Steps
				same := true
				for i, s := range steps {
					same = same && s.Active == lead[i].Active && (!s.Active || s.Note == max(24, lead[i].Note-12))
				}
				if same {
					layered++
				}
			}
		}
		layouts[layout] = true
		if silent > 0 {
			withSilent++
		}
		if trackOf(p, Snare) == nil && trackOf(p, Clap) == nil {
			t.Fatalf("%s: no backbeat track", style)
		}
		if len(p.Tracks) < 6 || len(p.Tracks) > 11 {
			t.Fatalf("%s: %d tracks", style, len(p.Tracks))
		}
	}
	if len(layouts) < 40 {
		t.Errorf("only %d different layouts", len(layouts))
	}
	if withSilent < grooves*9/10 {
		t.Errorf("only %d of %d grooves have a silent track for Jam", withSilent, grooves)
	}
	if layered == 0 {
		t.Error("no Dual layer doubled the lead")
	}
	for _, kind := range []Kind{Dual, Snare, Clap, OpenHat, Rimshot, Tom, Crash} {
		if playing[kind] < grooves/10 || present[kind] == grooves {
			t.Errorf("%s: in %d grooves, playing in %d", kind.Name(), present[kind], playing[kind])
		}
	}
	t.Logf("%d layouts; present %v; playing %v", len(layouts), present, playing)
}

func TestRandomiseAllUsesTheStyleForEveryKind(t *testing.T) {
	r := rand.New(rand.NewPCG(31, 32))
	p := NewProject()
	for _, kind := range []Kind{Dual, Clap, Rimshot, Tom, Crash} {
		p.Tracks = append(p.Tracks, NewTrack(kind, len(p.Tracks)))
	}
	dual := len(p.Tracks) - 5
	dualPlayed := 0
	for n := 0; n < 40; n++ {
		RandomiseGroove(&p, r)
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		if hits := hitCount(trackOf(p, Crash).Patterns[p.Pattern].Steps); hits > 1 {
			t.Fatalf("crash hits %d times a loop", hits)
		}
		if hitCount(p.Tracks[dual].Patterns[p.Pattern].Steps) > 0 {
			dualPlayed++
		}
	}
	if dualPlayed < 30 {
		t.Fatalf("the Dual synth played in %d of 40 grooves", dualPlayed)
	}
}
