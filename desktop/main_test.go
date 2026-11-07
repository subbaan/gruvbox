package desktop

import (
	"os"
	"testing"

	"github.com/subbaan/gruvbox/config"
	"github.com/subbaan/gruvbox/groove"
)

// TestMain points the data folders at a temporary folder and pins the launch groove to the classic layout with default sounds
// and no FX, which the UI tests were written against. The random groove
// itself is tested in the groove package and by TestRandomGrooveMenu.
func TestMain(m *testing.M) {
	random := startupProject
	randomStartup = random
	startupProject = func() (groove.Project, string) {
		p := groove.NewProject()
		for i, action := range []string{"Acid", "Sparse", "4 Floor", "2 & 4", "8ths", "Clear"} {
			groove.GenerateIn(&p, &p.Tracks[i], 0, action)
		}
		return p, ""
	}
	// Tests never touch the real data folders (~/Documents/gruvbox): the
	// file dialog lists and writes them.
	base, err := os.MkdirTemp("", "gruvbox-desktop-test-")
	if err != nil {
		panic(err)
	}
	if err := config.SetBaseDir(base); err != nil {
		panic(err)
	}
	// The test driver runs fyne.Do on the calling goroutine, so background
	// folder reads would race the test; read in place instead.
	fileReadAsync = false
	code := m.Run()
	os.RemoveAll(base) // only the folder made above
	os.Exit(code)
}

var randomStartup func() (groove.Project, string)
