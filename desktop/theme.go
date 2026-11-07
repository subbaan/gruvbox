package desktop

import (
	"fmt"
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	ftheme "fyne.io/fyne/v2/theme"
	"github.com/subbaan/gruvbox/theme"
)

type paletteTheme struct{ scheme theme.Scheme }

func colour(hex string) color.Color {
	n, _ := strconv.ParseUint(hex[1:], 16, 32)
	return color.NRGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}
}

// focusTint mixes the accent into the panel (or background, whichever the
// text reads better on) as strongly as it can, up to half, while the text on
// it keeps a contrast of at least 4.5:1, the usual minimum for reading. If no
// tint gets there, the most readable one is used.
func focusTint(r theme.Roles) string {
	return readableMix(r, r.Focus, .5)
}

// readableFill keeps a fill the text is drawn on (hover, selection) if the
// text reads on it, otherwise moves it towards the panel or background
// until it does.
func readableFill(r theme.Roles, fill string) string {
	if theme.Contrast(r.Text, fill) >= 4.5 {
		return fill
	}
	return readableMix(r, fill, 1)
}

// readableMix mixes colour into a base (panel or background) by the largest
// share up to most at which the text reaches 4.5:1, or else the share and
// base that read best.
func readableMix(r theme.Roles, colour string, most float64) string {
	best, score := r.Panel, 0.
	for _, base := range []string{r.Panel, r.Background} {
		for t := most; t > .1; t -= .05 {
			tint := mixHex(base, colour, t)
			contrast := theme.Contrast(r.Text, tint)
			if contrast >= 4.5 {
				return tint
			}
			if contrast > score {
				best, score = tint, contrast
			}
		}
	}
	return best
}

// mixHex blends a toward b by t (0 = a, 1 = b) and returns a #rrggbb colour.
func mixHex(a, b string, t float64) string {
	x, _ := strconv.ParseUint(a[1:], 16, 32)
	y, _ := strconv.ParseUint(b[1:], 16, 32)
	channel := func(shift uint) uint64 {
		from, to := float64((x>>shift)&0xff), float64((y>>shift)&0xff)
		return uint64(from + (to-from)*t + .5)
	}
	return fmt.Sprintf("#%02x%02x%02x", channel(16), channel(8), channel(0))
}

// translucent returns hex at the given alpha (0–255).
func translucent(hex string, alpha uint8) color.Color {
	c := colour(hex).(color.NRGBA)
	c.A = alpha
	return c
}

func (t paletteTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	r := t.scheme.Roles()
	value := r.Text
	switch name {
	case ftheme.ColorNameBackground:
		value = r.Background
	case ftheme.ColorNameButton, ftheme.ColorNameDisabledButton, ftheme.ColorNameInputBackground, ftheme.ColorNameMenuBackground, ftheme.ColorNameHeaderBackground:
		value = r.Panel
	case ftheme.ColorNameDisabled, ftheme.ColorNamePlaceHolder:
		value = r.Secondary
	case ftheme.ColorNameError:
		value = r.Error
	case ftheme.ColorNameSuccess:
		value = r.Success
	case ftheme.ColorNameWarning:
		value = r.Warning
	case ftheme.ColorNameFocus:
		// Fyne draws normal text over the focus colour (a focused select,
		// the highlighted menu item), so it is a tint of the accent that
		// keeps the text readable, not the accent itself.
		value = focusTint(r)
	case ftheme.ColorNamePrimary, ftheme.ColorNameHyperlink:
		value = r.Focus
	case ftheme.ColorNameForegroundOnPrimary:
		value = r.SelectedText
	case ftheme.ColorNameForegroundOnError:
		value = t.scheme.TextOn(r.Error)
	case ftheme.ColorNameForegroundOnSuccess:
		value = t.scheme.TextOn(r.Success)
	case ftheme.ColorNameForegroundOnWarning:
		value = t.scheme.TextOn(r.Warning)
	case ftheme.ColorNameInputBorder, ftheme.ColorNameSeparator, ftheme.ColorNameScrollBar:
		value = r.Border
	case ftheme.ColorNameHover, ftheme.ColorNamePressed, ftheme.ColorNameSelection:
		value = readableFill(r, r.Selection)
	case ftheme.ColorNameScrollBarBackground:
		value = r.Panel
	case ftheme.ColorNameShadow:
		return color.Transparent
	case ftheme.ColorNameOverlayBackground:
		c := colour(r.Background).(color.NRGBA)
		c.A = 210
		return c
	}
	return colour(value)
}
func (t paletteTheme) Font(style fyne.TextStyle) fyne.Resource {
	return ftheme.DefaultTheme().Font(style)
}
func (t paletteTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return ftheme.DefaultTheme().Icon(name)
}
func (t paletteTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case ftheme.SizeNameText:
		return 13
	case ftheme.SizeNamePadding:
		return 6
	case ftheme.SizeNameInnerPadding:
		return 6
	case ftheme.SizeNameInputRadius, ftheme.SizeNameSelectionRadius:
		return 2
	}
	return ftheme.DefaultTheme().Size(name)
}
