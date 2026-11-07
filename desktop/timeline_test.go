package desktop

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

func songWithSections(t *testing.T) (*UI, fyne.Window) {
	u, w := redesignUI(t)
	u.edit("Song", func(p *groove.Project) error {
		for i, bars := range []int{4, 8, 2} {
			s := groove.NewSongSection(i)
			s.Repeats = bars
			s.Name = []string{"Intro", "Drop", "Break"}[i]
			p.Song = append(p.Song, s)
		}
		return nil
	})
	u.editorTabs.SelectIndex(2)
	return u, w
}

func TestTimelineSelectsSections(t *testing.T) {
	u, _ := songWithSections(t)
	song := u.session.Project.Song
	if u.selectedSection != song[0].ID {
		t.Fatal("first section should be selected by default")
	}
	bar := u.songTimeline.barWidth(u.songTimeline.Size().Width)
	// The second block starts after the first section's four bars.
	test.TapAt(u.songTimeline, fyne.NewPos(4+4*bar+bar/2, 20))
	if u.selectedSection != song[1].ID {
		t.Fatal("clicking the second block did not select it")
	}
	number := u.songPanel.Objects[2].(*fyne.Container).Objects[0].(*widget.Button)
	if number.Importance != widget.HighImportance {
		t.Fatal("selected section row not highlighted")
	}
	if got := u.songTimeline.sectionAt(4 + 12*bar + bar/2); got != 2 {
		t.Fatalf("x in the third block resolved to section %d", got)
	}
	if u.songTimeline.sectionAt(4+14*bar+1) != -1 {
		t.Fatal("space after the song should not hit a section")
	}
	// Removing the selected section falls back to the first.
	id := song[1].ID
	u.edit("Remove", func(p *groove.Project) error {
		p.Song = append(p.Song[:1], p.Song[2:]...)
		return nil
	})
	if u.selectedSection == id || u.selectedSection != u.session.Project.Song[0].ID {
		t.Fatal("selection not reset after its section was removed")
	}
}

func TestTimelineShowsPlayingSection(t *testing.T) {
	u, _ := songWithSections(t)
	u.player.Publish(u.session.Project)
	u.setSongMode(true)
	u.player.SetPlaying(true)
	var block [735 * 2]float32
	for i := 0; i < 10; i++ {
		u.player.Render(block[:])
	}
	u.songTimeline.advance()
	r := u.songTimeline.renderer
	if !r.lit.Visible() || !r.marker.Visible() {
		t.Fatal("playing section not lit")
	}
	u.player.SetPlaying(false)
	u.songTimeline.advance()
	if r.lit.Visible() {
		t.Fatal("lit section remained after stop")
	}
}
