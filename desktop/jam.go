package desktop

import (
	"fmt"
	"math"
	"math/rand/v2"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// jamMode is the UI side of Jam: groove.Jam picks each move; this decides
// when, from the render's bar clock.
type jamMode struct {
	on     bool
	every  int     // bars between moves
	next   float64 // bar clock of the next move; 0 = not scheduled
	state  groove.JamState
	rand   *rand.Rand
	button *widget.Button
	amount *widget.Select
	label  *widget.Label
}

// jamLead is how early, in bars, a move is applied so it is in place for the
// next downbeat rather than landing just after it.
const jamLead = 1. / 8

var jamChoices = []int{4, 8, 16}

func (u *UI) jamPanel() fyne.CanvasObject {
	if u.jam.every == 0 {
		u.jam.every = 8
		u.jam.state.Amount = groove.JamEvolving
	}
	u.jam.button = widget.NewButton("Jam", func() { u.setJam(!u.jam.on) })
	options := []string{}
	for _, bars := range jamChoices {
		options = append(options, fmt.Sprintf("every %d bars", bars))
	}
	every := widget.NewSelect(options, nil)
	for i, bars := range jamChoices {
		if bars == u.jam.every {
			every.SetSelectedIndex(i)
		}
	}
	every.OnChanged = func(string) {
		u.jam.every = jamChoices[every.SelectedIndex()]
		u.jam.next = 0
		u.updateJam()
	}
	amount := widget.NewSelect(groove.JamAmounts, nil)
	amount.SetSelectedIndex(int(u.jam.state.Amount))
	amount.OnChanged = func(string) { u.jam.state.Amount = groove.JamAmount(amount.SelectedIndex()) }
	u.jam.amount = amount
	u.jam.label = widget.NewLabel("")
	u.jam.label.Wrapping = fyne.TextWrapWord
	help := u.helpButton("Jam changes the groove every few bars while a pattern loops.\n\nGentle: one small move: a couple of steps of one part, a part dropped out or brought back, or a 303's cutoff, resonance, decay or envelope nudged.\n\nEvolving: one or two moves, which can also give a drum a new pattern or sound, write a new phrase into part of a 303 line (in key, keeping its opening), bring in a silent track (clap, rimshot, tom, crash…) and ride echo and reverb mixes. It builds for six changes (parts come in, filters open) then releases for six (parts drop out, it gets wetter), so it rises and falls.\n\nWild: two or three moves at a time and bigger nudges.\n\nEach change lands just before a bar and is a normal edit, so undo steps back through them. Held tracks (H) are never touched, and Jam waits while Song mode plays or you are dragging a control.")
	u.updateJam()
	return container.NewVBox(container.NewBorder(nil, nil, container.NewHBox(u.jam.button, every, amount), help, u.jam.label))
}

func (u *UI) setJam(on bool) {
	u.jam.on, u.jam.next = on, 0
	if on && u.jam.rand == nil {
		u.jam.rand = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	if on {
		u.status.SetText(fmt.Sprintf("Jam on (%s): a change every %d bars while playing", groove.JamAmounts[u.jam.state.Amount], u.jam.every))
	} else {
		u.status.SetText("Jam off")
	}
	u.updateJam()
}

func (u *UI) updateJam() {
	if u.jam.button == nil {
		return
	}
	importance := widget.MediumImportance
	text := "Off · J toggles"
	if u.jam.on {
		importance = widget.HighImportance
		switch {
		case u.player.SongMode():
			text = "On · waits while Song mode plays (choose Pattern)"
		case !u.player.Playing():
			text = "On · starts when you play"
		case u.jam.next > 0:
			bars := int(math.Ceil(u.jam.next - u.player.BarClock()))
			text = fmt.Sprintf("On · next change in %d bar%s", bars, plural(bars))
			if u.jam.state.Amount != groove.JamGentle {
				text += map[bool]string{true: " · building", false: " · releasing"}[u.jam.state.Rising()]
			}
		default:
			text = "On"
		}
	}
	if u.jam.button.Importance != importance {
		u.jam.button.Importance = importance
		u.jam.button.Refresh()
	}
	if u.jam.label.Text != text {
		u.jam.label.SetText(text)
	}
}

// jamTick runs on the UI ticker. Moves fall on multiples of every bars from
// Play, applied jamLead bars early.
func (u *UI) jamTick() {
	if !u.jam.on {
		return
	}
	defer u.updateJam()
	if !u.player.Playing() || u.player.SongMode() {
		u.jam.next = 0
		return
	}
	every := float64(u.jam.every)
	clock := u.player.BarClock()
	if u.jam.next == 0 || clock < u.jam.next-every-1 {
		// First move, or playback restarted: the next multiple of every.
		u.jam.next = (math.Floor((clock+jamLead)/every) + 1) * every
		return
	}
	if clock < u.jam.next-jamLead {
		return
	}
	u.jam.next += every
	if u.liveSlider != nil || len(u.modals) > 0 {
		return // never edit under a drag or an open card
	}
	u.jamMove()
}

// jamMove applies one move as an undoable edit. The move is worked out on a
// copy first so the edit, its label and the jam's memory agree.
func (u *UI) jamMove() {
	preview := u.session.Project.Clone()
	state := u.jam.state.Clone()
	seed1, seed2 := u.jam.rand.Uint64(), u.jam.rand.Uint64()
	info := groove.Jam(&preview, rand.New(rand.NewPCG(seed1, seed2)), &state)
	if info == "" {
		return
	}
	u.edit("Jam · "+info, func(p *groove.Project) error {
		*p = preview
		return nil
	})
	u.jam.state = state
}
