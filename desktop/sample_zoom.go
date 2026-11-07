package desktop

type sampleView struct{ start, end float64 }

func (w *sampleWave) bounds() sampleView {
	if w.view.end <= w.view.start {
		return sampleView{0, 1}
	}
	return w.view
}
func (w *sampleWave) setView(view sampleView) {
	if view.end <= view.start {
		view = sampleView{0, 1}
	}
	minimum := min(1, 32/float64(w.sample.Frames()))
	span := max(minimum, min(1, view.end-view.start))
	center := (view.start + view.end) / 2
	start := max(0, min(1-span, center-span/2))
	w.view = sampleView{start, start + span}
	w.peaks = w.sample.PeaksRange(w.view.start, w.view.end)
	if w.ui.sampleViews != nil {
		w.ui.sampleViews[w.viewID] = w.view
	}
}
func (w *sampleWave) zoom(factor float64) {
	view := w.bounds()
	center := (view.start + view.end) / 2
	if factor < 1 && view.start == 0 && view.end == 1 {
		center = (w.start + w.end) / 2
	}
	span := (view.end - view.start) * factor
	w.setView(sampleView{center - span/2, center + span/2})
	w.Refresh()
}
func (w *sampleWave) pan(direction float64) {
	view := w.bounds()
	offset := (view.end - view.start) * direction * .5
	w.setView(sampleView{view.start + offset, view.end + offset})
	w.Refresh()
}
