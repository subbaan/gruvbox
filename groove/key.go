package groove

import (
	"fmt"
	"math/rand/v2"
)

// Key is the project's musical key. Melodic generators read it so that parts
// written at different times agree.
type Key struct {
	Root  int         `json:"root"` // pitch class, 0 = C
	Scale PhraseScale `json:"scale"`
}

var noteNames = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

func (k Key) Name() string {
	return fmt.Sprintf("%s %s", noteNames[((k.Root%12)+12)%12], k.Scale)
}

func (k Key) valid() bool {
	if k.Root < 0 || k.Root > 11 {
		return false
	}
	return scaleNotes(k.Scale) != nil
}

// BassRoot places the key root between G#1 and G2, the 303's sweet spot.
func (k Key) BassRoot() int {
	if k.Root <= 7 {
		return 36 + k.Root
	}
	return 24 + k.Root
}

func scaleNotes(scale PhraseScale) []int {
	for _, s := range phraseScales {
		if s.name == scale {
			return s.notes
		}
	}
	return nil
}

// bassScales are the scales random keys draw from; chromatic is left out.
var bassScales = []PhraseScale{PhraseMinorPentatonic, PhraseMinorPentatonic, PhraseMinor, PhraseDorian, PhrasePhrygian, PhraseMajorPentatonic}

func RandomKey(r *rand.Rand, scales []PhraseScale) Key {
	if len(scales) == 0 {
		scales = bassScales
	}
	return Key{Root: r.IntN(12), Scale: scales[r.IntN(len(scales))]}
}
