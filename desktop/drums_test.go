package desktop

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
)

func TestAddedDrumsAvailableFromDropdownWithLiveControls(t *testing.T) {
	for _, kind := range []groove.Kind{groove.Clap, groove.Rimshot, groove.Tom, groove.Crash} {
		t.Run(string(kind), func(t *testing.T) {
			a := test.NewApp()
			defer a.Quit()
			w := a.NewWindow("drum")
			u := newUI(a, w, theme.Library{Dir: t.TempDir()})
			w.SetContent(u.content())
			test.Tap(u.addTrackButton)
			overlays := w.Canvas().Overlays().List()
			var popup *widget.PopUpMenu
			var walk func(fyne.CanvasObject)
			walk = func(obj fyne.CanvasObject) {
				switch o := obj.(type) {
				case *widget.PopUpMenu:
					popup = o
				case *fyne.Container:
					for _, child := range o.Objects {
						walk(child)
					}
				case fyne.Widget:
					for _, child := range test.WidgetRenderer(o).Objects() {
						walk(child)
					}
				}
			}
			walk(overlays[len(overlays)-1])
			if popup == nil {
				t.Fatal("dropdown missing")
			}
			for index, k := range groove.Kinds {
				if k == kind {
					test.Tap(popup.Menu.Items[index].(fyne.Tappable))
					break
				}
			}
			if u.selectedTrack().Kind != kind || len(u.panels) != 0 {
				t.Fatal("direct drum creation failed")
			}
			var sliders []*widget.Slider
			var find func(fyne.CanvasObject)
			find = func(obj fyne.CanvasObject) {
				if slider, ok := obj.(*widget.Slider); ok {
					sliders = append(sliders, slider)
				}
				if c, ok := obj.(*fyne.Container); ok {
					for _, child := range c.Objects {
						find(child)
					}
				}
			}
			find(u.inspectorTabs.Items[0].Content)
			if len(sliders) != len(groove.Parameters(kind)) {
				t.Fatal("drum sound controls missing")
			}
			param := groove.Parameters(kind)[0]
			before := u.selectedTrack().Parameters[param.ID]
			slider := sliders[0]
			slider.Resize(fyne.NewSize(300, 36))
			slider.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(245, 18)}, Dragged: fyne.NewDelta(100, 0)})
			if u.selectedTrack().Parameters[param.ID] != slider.Value || slider.Value == before {
				t.Fatal("drum parameter is not live")
			}
			slider.DragEnd()
			u.undo()
			if u.selectedTrack().Parameters[param.ID] != before {
				t.Fatal("drum slider undo failed")
			}
			u.undo()
			if len(u.session.Project.Tracks) != 6 {
				t.Fatal("add drum undo failed")
			}
		})
	}
}
