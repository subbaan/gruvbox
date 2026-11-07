package desktop

import (
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	fynecanvas "fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/config"
)

// fileSpec describes one use of the file dialog.
type fileSpec struct {
	title, accept string // "Open groove", "Open"
	mode          fileMode
	kinds         []fileKind // the type filter; opening adds All files
	dir, name     string     // where to start, and (saving) the suggested name
	recent        string     // recent-files list ("projects", "samples"), or ""
	canvas        fyne.Canvas
}

// fileReadAsync reads folders and previews off the UI thread; tests turn
// it off so results arrive at once.
var fileReadAsync = true

// fileWork runs file work (opening, saving, loading a sample) off the UI
// thread, or at once when tests turn fileReadAsync off.
func fileWork(f func()) {
	if !fileReadAsync {
		f()
		return
	}
	go f()
}

const fileDialogHint = "Tab places · Alt+↑ up · Alt+←/→ back/forward · Ctrl+D bookmark · Ctrl+N new folder · Alt+. hidden · Alt+P preview"

type fileDialog struct {
	u      *UI
	b      *fileBrowser
	spec   fileSpec
	canvas fyne.Canvas
	m      *modal
	done   func(string)

	name                              *nameEntry
	list                              *widget.List
	places                            *fyne.Container
	crumbs                            *fyne.Container
	back, forward, up                 *widget.Button
	bookmark, newFolder               *widget.Button
	sortName, sortSize, sortWhen      *widget.Button
	columns                           *columnLayout
	preview, previewBox               *fyne.Container
	previewTitle                      *widget.Label
	previewBody                       *widget.Label
	swatches                          *fyne.Container
	hidden, showPreview               *widget.Check
	kind                              *widget.Select
	message                           *widget.Label
	acceptButton                      *widget.Button
	syncing                           bool
	previews                          map[string]filePreview
	lastClick                         time.Time
	lastRow                           int
	messageGeneration                 int
	placeRows                         []*placeRow
	cardWidth, placesWidth, sideWidth float32
}

// dataRoot is the folder Gruvbox keeps its own files in (themes, sounds,
// favourites, bookmarks).
func (u *UI) dataRoot() string { return filepath.Dir(u.library.Dir) }

func (u *UI) bookmarksFile() string   { return filepath.Join(u.dataRoot(), "bookmarks.json") }
func (u *UI) recentFilesFile() string { return filepath.Join(u.dataRoot(), "recent-files.json") }

// rememberFile puts a file that was opened or saved in a recent list.
func (u *UI) rememberFile(key, path string) {
	if err := addRecent(u.recentFilesFile(), key, path, time.Now()); err != nil {
		u.status.SetText("Could not update recent files: " + err.Error())
	}
}

// chooseFile shows the Open / Save dialog over spec.canvas (the main window
// if nil) and calls done with the chosen path. Saving adds the type's
// extension when it is missing and asks before replacing a file.
func (u *UI) chooseFile(spec fileSpec, done func(path string)) {
	if u.modals == nil {
		u.modals = map[string]*modal{}
	}
	if u.modals[spec.title] != nil {
		return
	}
	if u.filePrefs == nil {
		u.filePrefs = newFilePrefs()
	}
	canvas := spec.canvas
	if canvas == nil {
		canvas = u.window.Canvas()
	}
	d := &fileDialog{u: u, spec: spec, canvas: canvas, done: done, previews: map[string]filePreview{}, lastRow: -1}
	kinds := slices.Clone(spec.kinds)
	if spec.mode == openFile || len(kinds) == 0 {
		kinds = append(kinds, allFiles)
	}
	dir := spec.dir
	if last := u.filePrefs.dirs[spec.title]; last != "" {
		dir = last
	}
	// Start in the nearest folder that exists (a new projects folder may not
	// have been made yet).
	for dir != "/" && dir != "." && !isDir(dir) {
		dir = filepath.Dir(dir)
	}
	var recent []recentFile
	if spec.recent != "" {
		recent = loadRecent(u.recentFilesFile(), spec.recent)
	}
	extra := []place{{label: "Gruvbox", kind: placeDir, dir: config.BaseDir}}
	if spec.dir != "" && spec.dir != config.BaseDir {
		extra = append(extra, place{label: titleCase(baseName(spec.dir)), kind: placeDir, dir: spec.dir})
	}
	if u.path != "" {
		extra = append(extra, place{label: "This project's folder", kind: placeDir, dir: filepath.Dir(u.path)})
	}
	d.build()
	d.b = newFileBrowser(fileBrowserInit{mode: spec.mode, dir: dir, name: spec.name, kinds: kinds, recent: recent, bookmarksFile: u.bookmarksFile(), extra: extra, prefs: u.filePrefs, read: d.read})
	labels := []string{}
	for _, k := range kinds {
		labels = append(labels, k.label())
	}
	d.kind.Options = labels
	d.kind.SetSelectedIndex(0)
	d.kind.OnChanged = func(string) {
		d.b.setKind(d.kind.SelectedIndex())
		d.refresh()
	}

	m := &modal{}
	finished := false
	m.finish = func(ok bool) {
		if finished {
			return
		}
		finished = true
		m.pop.Hide()
		delete(u.modals, spec.title)
		u.modalOrder = deleteModal(u.modalOrder, m)
		u.filePrefs.dirs[spec.title] = d.b.dir
	}
	m.key = func(key fyne.KeyName) { d.key(key) }
	m.file = d
	d.m = m
	roles := u.scheme.Roles()
	background := fynecanvas.NewRectangle(colour(roles.Panel))
	background.StrokeColor, background.StrokeWidth, background.CornerRadius = colour(roles.Border), 1, 4
	m.pop = widget.NewModalPopUp(container.NewStack(background, container.NewPadded(d.content())), canvas)
	// As large as suits a file list, but never bigger than the window.
	size := canvas.Size()
	width := min(1040, max(min(720, size.Width-16), size.Width*.92))
	height := min(700, max(min(460, size.Height-16), size.Height*.9))
	m.pop.Resize(fyne.NewSize(width, height))
	u.modals[spec.title] = m
	if canvas == u.window.Canvas() {
		u.modalOrder = append(u.modalOrder, m)
	}
	d.b.reload()
	d.refresh()
	m.pop.Show()
	canvas.Focus(d.name)
	d.name.CursorColumn = len([]rune(d.name.Text))
	d.name.Refresh()
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// read lists a folder off the UI thread and hands it to the browser.
func (d *fileDialog) read(dir string) {
	deliver := func(entries []fileEntry, err error) {
		d.b.loaded(dir, entries, err)
		d.refresh()
	}
	if !fileReadAsync {
		deliver(readFolder(dir))
		return
	}
	go func() {
		entries, err := readFolder(dir)
		fyne.Do(func() { deliver(entries, err) })
	}()
}

func (d *fileDialog) build() {
	d.name = newNameEntry(d)
	d.name.SetPlaceHolder("type to filter, or a path")
	if d.spec.mode == saveFile {
		d.name.SetPlaceHolder("file name, or a path")
	}
	d.name.OnChanged = func(text string) {
		if d.syncing {
			return
		}
		d.b.setName(text)
		d.refresh()
	}
	d.back = widget.NewButtonWithIcon("", fynetheme.NavigateBackIcon(), func() { d.b.goBack(); d.after() })
	d.forward = widget.NewButtonWithIcon("", fynetheme.NavigateNextIcon(), func() { d.b.goForward(); d.after() })
	d.up = widget.NewButtonWithIcon("", fynetheme.MoveUpIcon(), func() { d.b.goUp(); d.after() })
	for _, b := range []*widget.Button{d.back, d.forward, d.up} {
		b.Importance = widget.LowImportance
	}
	d.crumbs = container.New(&crumbLayout{})
	d.bookmark = widget.NewButton("Bookmark", func() { d.report(d.b.toggleBookmark()); d.after() })
	d.newFolder = widget.NewButton("New folder", d.askNewFolder)
	d.columns = &columnLayout{}
	header := func(label string, s fileSort) *widget.Button {
		b := widget.NewButton(label, func() { d.b.sortBy(s); d.after() })
		b.Importance, b.Alignment = widget.LowImportance, widget.ButtonAlignLeading
		return b
	}
	d.sortName, d.sortSize, d.sortWhen = header("Name", sortName), header("Size", sortSize), header("Modified", sortModified)
	d.list = widget.NewList(
		func() int { return len(d.b.visible()) },
		func() fyne.CanvasObject { return newFileRow(d) },
		func(id widget.ListItemID, obj fyne.CanvasObject) { obj.(*fileRow).show(id) },
	)
	d.places = container.NewVBox()
	d.previewTitle = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	d.previewTitle.Wrapping = fyne.TextWrapWord
	d.previewBody = widget.NewLabel("")
	d.previewBody.Wrapping = fyne.TextWrapWord
	d.swatches = container.NewGridWithColumns(8)
	d.previewBox = container.NewVBox(d.previewTitle, d.previewBody, d.swatches)
	d.hidden = widget.NewCheck("Hidden", func(on bool) {
		if d.syncing {
			return
		}
		d.b.toggleHidden()
		d.after()
	})
	d.showPreview = widget.NewCheck("Preview", func(on bool) {
		if d.syncing {
			return
		}
		d.b.prefs.preview = on
		d.after()
	})
	d.kind = widget.NewSelect(nil, nil)
	d.message = widget.NewLabel("")
	d.message.Truncation = fyne.TextTruncateEllipsis
	d.acceptButton = widget.NewButton(d.spec.accept, func() { d.handle(d.b.accept()) })
	d.acceptButton.Importance = widget.HighImportance
}

func (d *fileDialog) content() fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(d.spec.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	nav := container.NewHBox(d.back, d.forward, d.up)
	top := container.NewVBox(heading, container.NewBorder(nil, nil, nav, container.NewHBox(d.bookmark, d.newFolder), d.crumbs), widget.NewSeparator())
	headerRow := container.New(d.columns, d.sortName, d.sortSize, d.sortWhen)
	listPane := container.NewBorder(headerRow, nil, nil, nil, d.list)
	placesPane := container.New(&fixedWidth{width: 150}, container.NewVScroll(d.places))
	d.preview = container.NewHBox(widget.NewSeparator(), container.New(&fixedWidth{width: 230}, container.NewVScroll(container.NewPadded(d.previewBox))))
	middle := container.NewBorder(nil, nil, container.NewHBox(placesPane, widget.NewSeparator()), d.preview, listPane)
	nameRow := container.NewBorder(nil, nil, widget.NewLabelWithStyle("Name:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), nil, d.name)
	options := container.NewBorder(nil, nil, container.NewHBox(d.hidden, d.showPreview, container.New(&fixedWidth{width: 210}, d.kind)), nil, d.message)
	cancel := widget.NewButton("Cancel", func() { d.m.finish(false) })
	hint := widget.NewLabel(fileDialogHint)
	hint.Importance = widget.LowImportance
	hint.Truncation = fyne.TextTruncateEllipsis
	buttons := container.NewBorder(nil, nil, nil, container.NewHBox(cancel, d.acceptButton), hint)
	bottom := container.NewVBox(widget.NewSeparator(), nameRow, options, buttons)
	return container.NewBorder(top, bottom, nil, nil, middle)
}

// after a move or option change: show it and keep typing in the name field.
func (d *fileDialog) after() {
	d.refresh()
	d.canvas.Focus(d.name)
}

func (d *fileDialog) refresh() {
	b := d.b
	if b == nil || d.list == nil {
		return
	}
	d.syncing = true
	defer func() { d.syncing = false }()
	if d.name.Text != b.name {
		d.name.SetText(b.name)
		d.name.CursorColumn = len([]rune(b.name))
		d.name.Refresh()
	}
	d.hidden.SetChecked(b.prefs.hidden)
	d.showPreview.SetChecked(b.prefs.preview)
	setEnabled(d.back, b.canBack())
	setEnabled(d.forward, b.canForward())
	setEnabled(d.up, b.recentOn || b.dir != "/")
	setEnabled(d.bookmark, !b.recentOn)
	setEnabled(d.newFolder, !b.recentOn)
	if b.isBookmarked() {
		d.bookmark.SetText("Remove bookmark")
	} else {
		d.bookmark.SetText("Bookmark")
	}

	// Path bar. The first object is the "…" shown when leading crumbs
	// don't fit.
	more := widget.NewLabel("…")
	more.Importance = widget.LowImportance
	d.crumbs.Objects = []fyne.CanvasObject{more}
	if b.recentOn {
		d.crumbs.Add(widget.NewLabelWithStyle("Recent", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	} else {
		list := b.crumbs()
		for i, c := range list {
			if i > 0 {
				sep := widget.NewLabel("›")
				sep.Importance = widget.LowImportance
				d.crumbs.Add(sep)
			}
			i := i
			if i == len(list)-1 {
				current := widget.NewLabelWithStyle(c.label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				current.Truncation = fyne.TextTruncateEllipsis
				d.crumbs.Add(current)
				continue
			}
			button := widget.NewButton(c.label, func() { d.b.goCrumb(i); d.after() })
			button.Importance = widget.LowImportance
			d.crumbs.Add(button)
		}
	}
	d.crumbs.Refresh()

	// Column headers show the sort.
	arrow := map[bool]string{false: " ▲", true: " ▼"}[b.prefs.desc]
	labels := map[fileSort]string{sortName: "Name", sortSize: "Size", sortModified: "Modified"}
	if b.recentOn {
		d.sortName.SetText("Name")
		d.sortSize.SetText("Folder")
		d.sortWhen.SetText("Used")
	} else {
		for s, button := range map[fileSort]*widget.Button{sortName: d.sortName, sortSize: d.sortSize, sortModified: d.sortWhen} {
			text := labels[s]
			if b.prefs.sort == s {
				text += arrow
			}
			button.SetText(text)
		}
	}
	d.columns.wide = b.recentOn
	d.list.Refresh()
	if n := len(b.visible()); n > 0 {
		d.list.ScrollTo(min(b.sel, n-1))
	}

	// Places.
	current := b.currentPlace()
	if len(d.placeRows) != len(b.places) {
		d.placeRows = nil
		d.places.Objects = nil
		for i := range b.places {
			row := newPlaceRow(d, i)
			d.placeRows = append(d.placeRows, row)
			d.places.Add(row)
		}
	}
	for i, row := range d.placeRows {
		row.set(b.places[i].label, i == current, b.placesFocus && i == b.placeSel)
	}
	d.places.Refresh()

	// Message: loading, an error, or nothing to show.
	switch {
	case b.loading:
		d.setMessage("Reading " + tilde(b.dir, b.home) + "…")
	case b.err != "" && !b.recentOn:
		d.setMessage("Can't read this folder: " + b.err)
	case len(b.visible()) == 0 && b.filter() != "":
		d.setMessage("Nothing matches “" + strings.TrimSpace(b.name) + "”")
	case len(b.visible()) == 0 && b.recentOn:
		d.setMessage("No recent files yet")
	case d.messageGeneration == 0:
		d.setMessage("")
	}
	d.refreshPreview()
}

func setEnabled(b *widget.Button, on bool) {
	if on && b.Disabled() {
		b.Enable()
	} else if !on && !b.Disabled() {
		b.Disable()
	}
}

func (d *fileDialog) setMessage(text string) {
	if d.message.Text != text {
		d.message.SetText(text)
	}
}

// report shows an outcome's message for a few seconds.
func (d *fileDialog) report(o fileOutcome) {
	if o.kind != info && o.kind != warn {
		return
	}
	d.message.Importance = map[bool]widget.Importance{true: widget.DangerImportance, false: widget.MediumImportance}[o.kind == warn]
	d.message.SetText(o.msg)
	d.messageGeneration++
	generation := d.messageGeneration
	time.AfterFunc(4*time.Second, func() {
		fyne.Do(func() {
			if d.messageGeneration == generation {
				d.messageGeneration = 0
				d.message.Importance = widget.MediumImportance
				d.refresh()
			}
		})
	})
}

func (d *fileDialog) refreshPreview() {
	on := d.b.prefs.preview
	if on && !d.preview.Visible() {
		d.preview.Show()
	} else if !on && d.preview.Visible() {
		d.preview.Hide()
	}
	if !on {
		return
	}
	e, ok := d.b.selected()
	if !ok {
		d.showPreviewOf(filePreview{})
		return
	}
	if d.b.recentOn {
		// Recent files may be on a sleeping drive: describe them without
		// touching the disk.
		d.showPreviewOf(filePreview{title: e.Name, lines: []string{e.Folder, "", "Used " + when(e.Modified, time.Now())}})
		return
	}
	if p, ok := d.previews[e.Path]; ok {
		d.showPreviewOf(p)
		return
	}
	d.showPreviewOf(filePreview{title: e.Name, lines: []string{"…"}})
	path := e.Path
	work := func() {
		p := previewFile(e)
		fyne.Do(func() {
			d.previews[path] = p
			if sel, ok := d.b.selected(); ok && sel.Path == path {
				d.showPreviewOf(p)
			}
		})
	}
	if fileReadAsync {
		go work()
	} else {
		p := previewFile(e)
		d.previews[path] = p
		d.showPreviewOf(p)
	}
}

func (d *fileDialog) showPreviewOf(p filePreview) {
	d.previewTitle.SetText(p.title)
	d.previewBody.SetText(strings.Join(p.lines, "\n"))
	d.swatches.Objects = nil
	for _, hex := range p.swatches {
		if len(hex) == 7 && hex[0] == '#' {
			swatch := fynecanvas.NewRectangle(colour(hex))
			swatch.SetMinSize(fyne.NewSize(18, 18))
			d.swatches.Add(swatch)
		}
	}
	d.swatches.Refresh()
}

// handle runs a browser outcome.
func (d *fileDialog) handle(o fileOutcome) {
	switch o.kind {
	case cancelled:
		d.m.finish(false)
	case chosen:
		d.choose(o.path)
	case askNewFolder:
		d.askNewFolder()
	case info, warn:
		d.report(o)
	}
	if o.kind != cancelled && o.kind != chosen {
		d.after()
	}
}

// choose finishes with path; saving adds the type's extension if missing
// and asks before replacing a file.
func (d *fileDialog) choose(path string) {
	if d.spec.mode == saveFile {
		kind := d.b.currentKind()
		if len(kind.Exts) == 0 && len(d.spec.kinds) > 0 {
			kind = d.spec.kinds[0]
		}
		if len(kind.Exts) > 0 && !kind.matches(path) {
			path += kind.Exts[0]
		}
		if info, err := os.Stat(path); err == nil {
			if info.IsDir() {
				d.b.goTo(path)
				d.after()
				return
			}
			label := widget.NewLabel("Replace " + filepath.Base(path) + "? It is in " + tilde(filepath.Dir(path), d.b.home) + ".")
			label.Wrapping = fyne.TextWrapWord
			d.u.confirmOn(d.canvas, "Replace file", "Replace", "Cancel", label, func(ok bool) {
				if ok {
					d.finish(path)
				} else {
					d.canvas.Focus(d.name)
				}
			})
			return
		}
	}
	d.finish(path)
}

func (d *fileDialog) finish(path string) {
	d.m.finish(true)
	d.u.filePrefs.dirs[d.spec.title] = filepath.Dir(path)
	d.done(path)
}

func (d *fileDialog) askNewFolder() {
	if d.b.recentOn {
		return
	}
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Folder name")
	d.u.confirmOn(d.canvas, "New folder", "Make", "Cancel", container.NewVBox(widget.NewLabel("Make a folder in "+tilde(d.b.dir, d.b.home)), entry), func(ok bool) {
		if ok {
			d.report(d.b.createFolder(entry.Text))
		}
		d.after()
	})
}

// key handles a key for the dialog (from the name field, or the window
// when nothing is focused). It reports whether the key was used.
func (d *fileDialog) key(name fyne.KeyName) bool {
	b := d.b
	page := 10
	switch name {
	case fyne.KeyEscape:
		d.m.finish(false)
		return true
	case fyne.KeyTab:
		b.placesFocus = !b.placesFocus
		if b.placesFocus {
			if i := b.currentPlace(); i >= 0 {
				b.placeSel = i
			}
		}
	case fyne.KeyReturn, fyne.KeyEnter:
		if b.placesFocus {
			b.goPlace(b.placeSel)
			b.placesFocus = false
		} else {
			d.handle(b.accept())
			return true
		}
	case fyne.KeyUp, fyne.KeyDown, fyne.KeyPageUp, fyne.KeyPageDown:
		delta := map[fyne.KeyName]int{fyne.KeyUp: -1, fyne.KeyDown: 1, fyne.KeyPageUp: -page, fyne.KeyPageDown: page}[name]
		if b.placesFocus {
			b.placeSel = max(0, min(len(b.places)-1, b.placeSel+delta))
		} else {
			b.moveSel(delta)
		}
	case fyne.KeyDelete:
		if !b.placesFocus || b.placeSel >= len(b.places) || b.places[b.placeSel].kind != placeBookmark {
			return false
		}
		d.report(b.setBookmark(b.places[b.placeSel].dir, false))
	case fyne.KeyBackspace:
		if b.name != "" {
			return false
		}
		b.goUp()
	default:
		return false
	}
	d.refresh()
	return true
}

// shortcut handles the dialog's modified keys.
func (d *fileDialog) shortcut(s fyne.Shortcut) bool {
	c, ok := s.(*desktop.CustomShortcut)
	if !ok {
		return false
	}
	b := d.b
	alt, ctrl := c.Modifier == fyne.KeyModifierAlt, c.Modifier == fyne.KeyModifierControl
	switch {
	case alt && c.KeyName == fyne.KeyUp:
		b.goUp()
	case alt && c.KeyName == fyne.KeyLeft:
		b.goBack()
	case alt && c.KeyName == fyne.KeyRight:
		b.goForward()
	case alt && c.KeyName == fyne.KeyHome:
		if b.home != "" {
			b.goTo(b.home)
		}
	case alt && c.KeyName == fyne.KeyPeriod, ctrl && c.KeyName == fyne.KeyH:
		b.toggleHidden()
	case alt && c.KeyName == fyne.KeyP:
		b.prefs.preview = !b.prefs.preview
	case ctrl && c.KeyName == fyne.KeyD:
		d.report(b.toggleBookmark())
	case ctrl && c.KeyName == fyne.KeyN:
		d.askNewFolder()
		return true
	default:
		return false
	}
	d.after()
	return true
}

// clickRow selects a row; a second click on it soon after opens it, as a
// double click does.
func (d *fileDialog) clickRow(i int) {
	now := time.Now()
	double := d.lastRow == i && now.Sub(d.lastClick) < 400*time.Millisecond
	d.lastClick, d.lastRow = now, i
	d.b.clickRow(i)
	if double {
		d.lastRow = -1
		d.handle(d.b.activate())
		return
	}
	d.after()
}

// ---- the name field ----------------------------------------------------

// nameEntry is the name field: typing filters or names a path, and the
// dialog's keys are caught before the entry sees them.
type nameEntry struct {
	widget.Entry
	d *fileDialog
}

func newNameEntry(d *fileDialog) *nameEntry {
	e := &nameEntry{d: d}
	e.ExtendBaseWidget(e)
	return e
}

func (e *nameEntry) TypedKey(k *fyne.KeyEvent) {
	if e.d.key(k.Name) {
		return
	}
	e.Entry.TypedKey(k)
}

func (e *nameEntry) TypedShortcut(s fyne.Shortcut) {
	if e.d.shortcut(s) {
		return
	}
	e.Entry.TypedShortcut(s)
}

// AcceptsTab keeps Tab for the dialog (it moves between the list and the
// places) instead of moving focus.
func (e *nameEntry) AcceptsTab() bool { return true }

// ---- rows --------------------------------------------------------------

type fileRow struct {
	widget.BaseWidget
	d                *fileDialog
	index            int
	bg               *fynecanvas.Rectangle
	name, size, when *widget.Label
}

func newFileRow(d *fileDialog) *fileRow {
	r := &fileRow{d: d, bg: fynecanvas.NewRectangle(color.Transparent), name: widget.NewLabel(""), size: widget.NewLabel(""), when: widget.NewLabel("")}
	r.name.Truncation = fyne.TextTruncateEllipsis
	r.size.Truncation = fyne.TextTruncateEllipsis
	r.size.Alignment = fyne.TextAlignTrailing
	r.bg.CornerRadius = 2
	r.ExtendBaseWidget(r)
	return r
}

func (r *fileRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(r.bg, container.New(r.d.columns, r.name, r.size, r.when)))
}

func (r *fileRow) show(i int) {
	r.index = i
	v := r.d.b.visible()
	if i >= len(v) {
		return
	}
	e := v[i]
	name := e.Name
	if e.Dir {
		name += "/"
	}
	r.name.TextStyle.Bold = e.Dir
	r.name.SetText(name)
	switch {
	case e.Folder != "":
		r.size.Alignment = fyne.TextAlignLeading
		r.size.SetText(e.Folder)
	case e.HasSize:
		r.size.Alignment = fyne.TextAlignTrailing
		r.size.SetText(humanSize(e.Size))
	default:
		r.size.SetText("")
	}
	r.when.SetText(when(e.Modified, time.Now()))
	th := fyne.CurrentApp().Settings().Theme()
	v2 := fyne.CurrentApp().Settings().ThemeVariant()
	if i == r.d.b.sel && !r.d.b.placesFocus {
		r.bg.FillColor = th.Color(fynetheme.ColorNameFocus, v2)
	} else if i == r.d.b.sel {
		r.bg.FillColor = th.Color(fynetheme.ColorNameHover, v2)
	} else {
		r.bg.FillColor = color.Transparent
	}
	r.bg.Refresh()
}

func (r *fileRow) Tapped(*fyne.PointEvent) { r.d.clickRow(r.index) }

// columnLayout lays out name, size and date; Recent files show the folder
// in a wider middle column.
type columnLayout struct{ wide bool }

const sizeColumn, whenColumn = 90, 118

func (c *columnLayout) widths(total float32) (float32, float32, float32) {
	middle := float32(sizeColumn)
	if c.wide {
		middle = total * .42
	}
	name := max(40, total-middle-whenColumn)
	return name, middle, whenColumn
}

func (c *columnLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 3 {
		return
	}
	a, b, w := c.widths(size.Width)
	x := float32(0)
	for i, width := range []float32{a, b, w} {
		objects[i].Move(fyne.NewPos(x, 0))
		objects[i].Resize(fyne.NewSize(width, size.Height))
		x += width
	}
}

func (c *columnLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objects {
		h = max(h, o.MinSize().Height)
	}
	return fyne.NewSize(40+sizeColumn+whenColumn, h)
}

// crumbLayout lays the path bar out left to right. When it is too narrow
// it keeps the trailing crumbs (the current folder matters most) and shows
// its first object, a "…", in place of the leading ones.
type crumbLayout struct{}

func (c *crumbLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	more, items := objects[0], objects[1:]
	total := float32(0)
	for _, o := range items {
		total += naturalWidth(o)
	}
	first := 0
	if total > size.Width {
		// Drop crumbs (and the separator after each) from the front until
		// the rest and the "…" fit; always keep the last one.
		room := size.Width - more.MinSize().Width
		for first < len(items)-1 && total > room {
			total -= naturalWidth(items[first])
			first++
		}
		// Don't start on a separator.
		if first < len(items)-1 && first%2 == 1 {
			total -= naturalWidth(items[first])
			first++
		}
	}
	x := float32(0)
	place := func(o fyne.CanvasObject) {
		h := o.MinSize().Height
		w := min(naturalWidth(o), max(0, size.Width-x))
		if o == items[len(items)-1] {
			w = max(0, size.Width-x) // the current folder takes what's left
		}
		o.Move(fyne.NewPos(x, (size.Height-h)/2))
		o.Resize(fyne.NewSize(w, h))
		o.Show()
		x += w
	}
	if first > 0 {
		place(more)
	} else {
		more.Hide()
	}
	for i, o := range items {
		if i < first {
			o.Hide()
			continue
		}
		place(o)
	}
}

func (c *crumbLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h, w := float32(0), float32(0)
	for _, o := range objects {
		h = max(h, o.MinSize().Height)
	}
	if len(objects) > 1 {
		w = objects[0].MinSize().Width + min(naturalWidth(objects[len(objects)-1]), 160)
	}
	return fyne.NewSize(w, h)
}

// naturalWidth is an object's width untruncated (a truncating label's
// MinSize is only its ellipsis).
func naturalWidth(o fyne.CanvasObject) float32 {
	if l, ok := o.(*widget.Label); ok && l.Truncation != fyne.TextTruncateOff {
		return widget.NewLabelWithStyle(l.Text, l.Alignment, l.TextStyle).MinSize().Width
	}
	return o.MinSize().Width
}

// fixedWidth gives its content a set width.
type fixedWidth struct{ width float32 }

func (f *fixedWidth) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

func (f *fixedWidth) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objects {
		h = max(h, o.MinSize().Height)
	}
	return fyne.NewSize(f.width, h)
}

// ---- places ------------------------------------------------------------

type placeRow struct {
	widget.BaseWidget
	d     *fileDialog
	index int
	bg    *fynecanvas.Rectangle
	bar   *fynecanvas.Rectangle
	label *widget.Label
}

func newPlaceRow(d *fileDialog, i int) *placeRow {
	r := &placeRow{d: d, index: i, bg: fynecanvas.NewRectangle(color.Transparent), bar: fynecanvas.NewRectangle(color.Transparent), label: widget.NewLabel("")}
	r.label.Truncation = fyne.TextTruncateEllipsis
	r.bg.CornerRadius = 2
	r.ExtendBaseWidget(r)
	return r
}

func (r *placeRow) CreateRenderer() fyne.WidgetRenderer {
	bar := container.New(&fixedWidth{width: 3}, r.bar)
	return widget.NewSimpleRenderer(container.NewStack(r.bg, container.NewBorder(nil, nil, bar, nil, r.label)))
}

func (r *placeRow) set(label string, current, focused bool) {
	r.label.TextStyle.Bold = current
	r.label.SetText(label)
	th := fyne.CurrentApp().Settings().Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()
	r.bg.FillColor = color.Transparent
	if focused {
		r.bg.FillColor = th.Color(fynetheme.ColorNameFocus, v)
	}
	r.bar.FillColor = color.Transparent
	if current {
		r.bar.FillColor = th.Color(fynetheme.ColorNamePrimary, v)
	}
	r.bg.Refresh()
	r.bar.Refresh()
}

func (r *placeRow) Tapped(*fyne.PointEvent) {
	r.d.b.placesFocus = false
	r.d.b.goPlace(r.index)
	r.d.after()
}

// TappedSecondary offers to remove a bookmark.
func (r *placeRow) TappedSecondary(e *fyne.PointEvent) {
	b := r.d.b
	if r.index >= len(b.places) || b.places[r.index].kind != placeBookmark {
		return
	}
	dir := b.places[r.index].dir
	menu := fyne.NewMenu("", fyne.NewMenuItem("Remove bookmark", func() {
		r.d.report(b.setBookmark(dir, false))
		r.d.after()
	}))
	widget.ShowPopUpMenuAtPosition(menu, r.d.canvas, e.AbsolutePosition)
}
