package desktop

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// findSlider returns the slider in the sliderRow whose name starts with prefix.
// It searches by content rather than container shape so layout changes to
// sliderRow do not break tests.
func findSlider(root fyne.CanvasObject, prefix string) *widget.Slider {
	var found *widget.Slider
	var walk func(fyne.CanvasObject) (bool, *widget.Slider)
	// walk reports whether obj contains a matching label and the first slider in it.
	walk = func(obj fyne.CanvasObject) (bool, *widget.Slider) {
		switch o := obj.(type) {
		case *widget.Label:
			return strings.HasPrefix(o.Text, prefix), nil
		case *paramName:
			return strings.HasPrefix(o.Text, prefix), nil
		case *widget.Slider:
			return false, o
		case *container.Scroll:
			return walk(o.Content)
		case *fyne.Container:
			labelled, matches := false, 0
			var slider *widget.Slider
			for _, child := range o.Objects {
				l, s := walk(child)
				if found != nil {
					return false, nil
				}
				if l {
					labelled = true
					matches++
				}
				if s != nil && slider == nil {
					slider = s
				}
			}
			if labelled && slider != nil && matches == 1 {
				found = slider
				return false, nil
			}
			return labelled, slider
		}
		return false, nil
	}
	walk(root)
	return found
}
