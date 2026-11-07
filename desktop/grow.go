package desktop

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// growFull remembers the last length chosen in this run.
var growFull = true

// growTrack asks how long, says what will be replaced, then turns the
// selected pattern into a whole arrangement in one undoable edit and switches
// to song mode on the Song tab.
func (u *UI) growTrack() {
	u.finishLiveSlider()
	p := u.session.Project
	if groove.PatternEmpty(p, p.Pattern) {
		u.status.SetText("Make or randomise a groove in this pattern first, then grow it")
		return
	}
	minutes := func(bars int) string {
		s := int(float64(bars)*4*60/p.BPM + .5)
		return fmt.Sprintf("%d:%02d", s/60, s%60)
	}
	choice := widget.NewRadioGroup([]string{"Short · " + minutes(72), "Full · " + minutes(136)}, nil)
	choice.Required = true
	choice.SetSelected(choice.Options[0])
	if growFull {
		choice.SetSelected(choice.Options[1])
	}
	lines := []string{fmt.Sprintf("Pattern %s becomes the peak. The other seven pads of bank %c get an intro, build, riser, fills, breakdown and a second peak made from it, and the song lays them out.", groove.SlotName(p.Pattern), 'A'+groove.BankOf(p.Pattern))}
	slots, song := groove.GrowReplaces(p)
	if len(slots) > 0 || song {
		replaced := []string{}
		for _, slot := range slots {
			replaced = append(replaced, groove.SlotName(slot))
		}
		what := "the song"
		if len(slots) > 0 {
			what = "pattern" + plural(len(slots)) + " " + strings.Join(replaced, ", ")
			if song {
				what += " and the song"
			}
		}
		lines = append(lines, "This replaces "+what+". Undo brings "+them(len(slots), song)+" back.")
	}
	text := widget.NewLabel(strings.Join(lines, "\n\n"))
	text.Wrapping = fyne.TextWrapWord
	u.confirm("Grow into a track", "Grow", "Cancel", container.NewVBox(text, choice), func(ok bool) {
		if !ok {
			return
		}
		growFull = choice.Selected == choice.Options[1]
		u.applyGrow(growFull)
	})
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
func them(slots int, song bool) string {
	if slots+btoi(song) == 1 {
		return "it"
	}
	return "them"
}
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (u *UI) applyGrow(full bool) {
	seed1, seed2 := rand.Uint64(), rand.Uint64()
	preview := u.session.Project.Clone()
	info, err := groove.GrowTrack(&preview, rand.New(rand.NewPCG(seed1, seed2)), full)
	if err != nil {
		u.showError(err)
		return
	}
	u.edit("Grow into a track · "+info, func(p *groove.Project) error {
		_, err := groove.GrowTrack(p, rand.New(rand.NewPCG(seed1, seed2)), full)
		return err
	})
	u.player.SetSongMode(true)
	u.selectedSection = ""
	if len(u.session.Project.Song) > 0 {
		u.selectedSection = u.session.Project.Song[0].ID
	}
	for i, tab := range u.editorTabs.Items {
		if tab.Text == "Song" {
			u.editorTabs.SelectIndex(i)
		}
	}
	u.updatePlaybackLabel()
	u.refresh()
}
