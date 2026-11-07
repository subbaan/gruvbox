package desktop

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/groove"
)

// Draft sliders update previews only, without opening project undo gestures.
func draftSlider(name string, value, low, high, step float64, changed func(float64)) (*fyne.Container, *widget.Slider) {
	format := func(v, increment float64) string {
		precision := 2
		if increment >= 1 {
			precision = 0
		} else if increment < .01 {
			precision = 3
		}
		return fmt.Sprintf("%s  %.*f", name, precision, v)
	}
	label := widget.NewLabel(format(value, step))
	slider := widget.NewSlider(low, high)
	slider.Step = step
	slider.SetValue(value)
	slider.OnChanged = func(v float64) { label.SetText(format(v, slider.Step)); changed(v) }
	return container.NewVBox(label, slider), slider
}

type generatorPreview struct {
	widget.BaseWidget
	ui        *UI
	pattern   groove.Pattern
	parameter string
	notes     bool
	low, high float64
}

func newGeneratorPreview(u *UI) *generatorPreview {
	p := &generatorPreview{ui: u}
	p.ExtendBaseWidget(p)
	return p
}
func (p *generatorPreview) CreateRenderer() fyne.WidgetRenderer {
	r := &generatorPreviewRenderer{view: p, content: container.NewWithoutLayout()}
	r.Refresh()
	return r
}

type generatorPreviewRenderer struct {
	view    *generatorPreview
	content *fyne.Container
}

func (r *generatorPreviewRenderer) MinSize() fyne.Size {
	return fyne.NewSize(480, float32(max(1, (len(r.view.pattern.Steps)+15)/16))*48)
}
func (r *generatorPreviewRenderer) Layout(size fyne.Size) { r.content.Resize(size); r.draw(size) }
func (r *generatorPreviewRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}
func (r *generatorPreviewRenderer) Destroy() {}
func (r *generatorPreviewRenderer) Refresh() { r.draw(r.view.Size()); r.content.Refresh() }
func (r *generatorPreviewRenderer) draw(size fyne.Size) {
	p := r.view
	roles := p.ui.scheme.Roles()
	r.content.Objects = nil
	width := max(480, size.Width) / 16
	for i, step := range p.pattern.Steps {
		x, y := float32(i%16)*width, float32(i/16)*48
		label := canvas.NewText(fmt.Sprintf("%02d", i+1), colour(roles.Secondary))
		label.TextSize = 10
		label.Move(fyne.NewPos(x+3, y))
		r.content.Add(label)
		fill := roles.Panel
		if step.Active {
			fill = roles.Accent
		}
		cell := canvas.NewRectangle(colour(fill))
		cell.Move(fyne.NewPos(x+2, y+15))
		cell.Resize(fyne.NewSize(width-4, 28))
		r.content.Add(cell)
		if p.notes && step.Active {
			ink := colour(p.ui.scheme.TextOn(fill))
			note := canvas.NewText(noteName(step.Note), ink)
			note.TextSize = 10
			note.Move(fyne.NewPos(x+4, y+16))
			r.content.Add(note)
			marks := ""
			if step.Accent {
				marks += ">"
			}
			if step.Slide {
				marks += "~"
			}
			text := canvas.NewText(marks, ink)
			text.TextSize = 9
			text.Move(fyne.NewPos(x+4, y+30))
			r.content.Add(text)
		}
		if p.parameter != "" {
			cell.FillColor = colour(roles.Panel)
			if step.Active {
				cell.StrokeColor = colour(roles.Accent)
				cell.StrokeWidth = 1
			}
			if value, ok := step.Locks[p.parameter]; ok {
				fraction := max(0, min(1, (value-p.low)/max(1e-12, p.high-p.low)))
				bar := canvas.NewRectangle(colour(roles.Accent))
				if !step.Active {
					bar.FillColor = colour(roles.Secondary)
				}
				height := max(2, float32(fraction)*24)
				bar.Move(fyne.NewPos(x+5, y+41-height))
				bar.Resize(fyne.NewSize(width-10, height))
				r.content.Add(bar)
			}
		}
	}
}
