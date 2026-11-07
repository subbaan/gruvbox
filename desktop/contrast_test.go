package desktop

import (
	"fmt"
	"image/color"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	ftheme "fyne.io/fyne/v2/theme"
	"github.com/subbaan/gruvbox/theme"
)

func hexOf(c color.Color) string {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return fmt.Sprintf("#%02x%02x%02x", n.R, n.G, n.B)
}

// Fyne draws ordinary text on the focus colour (a focused select, the
// highlighted menu item) and on hover and selection, and the on-primary
// colour on primary buttons. Every pair must stay readable in every theme.
func TestThemeTextStaysReadableOnHighlights(t *testing.T) {
	schemes := theme.Bundled()
	if data, err := os.ReadFile("../theme/testdata/light.yaml"); err == nil {
		if s, err := theme.Parse(data); err == nil {
			schemes["testdata-light"] = s
		}
	}
	// A dim, low-contrast dark theme like some downloaded ones.
	dim := schemes["gruvbox-dark"]
	dim.Palette[1], dim.Palette[2], dim.Palette[5], dim.Palette[10] = "#3b4252", "#4c566a", "#9aa3b5", "#9aa3b5"
	schemes["dim"] = dim
	for name, scheme := range schemes {
		th := paletteTheme{scheme}
		text := hexOf(th.Color(ftheme.ColorNameForeground, 0))
		for _, pair := range []struct {
			fg, bg fyne.ThemeColorName
			min    float64
		}{
			{ftheme.ColorNameForeground, ftheme.ColorNameFocus, 4.5},
			{ftheme.ColorNameForeground, ftheme.ColorNameHover, 4.5},
			{ftheme.ColorNameForeground, ftheme.ColorNameSelection, 4.5},
			{ftheme.ColorNameForegroundOnPrimary, ftheme.ColorNamePrimary, 4.5},
		} {
			fg, bg := hexOf(th.Color(pair.fg, 0)), hexOf(th.Color(pair.bg, 0))
			if c := theme.Contrast(fg, bg); c < pair.min {
				t.Errorf("%s: %s on %s is %.1f:1 (text %s)", name, pair.fg, pair.bg, c, text)
			}
		}
	}
}
