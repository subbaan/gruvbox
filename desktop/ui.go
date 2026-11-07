package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/config"
	"github.com/subbaan/gruvbox/groove"
	"github.com/subbaan/gruvbox/theme"
)

type UI struct {
	filePrefs *filePrefs // the file dialog's view choices and last folders, this session
	// bind is the binder receiving sync functions while a panel is built.
	bind                            *binder
	inspectorBinder, songBinder     binder
	mixerBinder                     binder
	inspectorKey, songKey, mixerKey string
	inspectorBuilds                 int
	// lockMode makes Sound/FX controls write locks for the selected step.
	lockMode                           bool
	generatorRefresh                   map[string]func()
	recordingRefresh                   func()
	dualAccordion                      *widget.Accordion
	dualAccordionTrack                 string
	dualSections                       map[string][]bool
	soundLibrary                       groove.SoundLibrary
	soundLibraryRefresh                func()
	activeDrawer                       *drawer
	songTimeline                       *songTimeline
	songScroll                         *container.Scroll
	selectedSection                    string
	stepClip                           *groove.Step
	patternClip                        *groove.Pattern
	fxFolded                           map[string]bool // effect IDs whose cards are collapsed (view state only)
	rollScroll                         *container.Scroll
	root                               *fyne.Container
	modals                             map[string]*modal
	modalOrder                         []*modal
	toast                              *fyne.Container
	toastLabel                         *widget.Label
	toastBackground                    *canvas.Rectangle
	toastGeneration                    int
	drawerHost                         *fyne.Container
	sidePane                           fyne.CanvasObject
	workspace                          *container.Split
	splitOffset                        float64
	effectModulationRefresh            map[string]func()
	performanceLabel                   *widget.Label
	performanceActions                 []*widget.Button
	performanceCancel, performanceKeep *widget.Button
	jam                                jamMode
	history                            grooveHistory
	sampleViews                        map[string]sampleView
	liveEdit                           bool
	liveSlider                         *widget.Slider
	liveSession                        *groove.Session
	songPanel                          *fyne.Container
	playbackLabel                      *widget.Label
	followSession                      *groove.Session
	followLaunch                       uint64
	app                                fyne.App
	window                             fyne.Window
	session                            *groove.Session
	player                             *groove.Player
	library                            theme.Library
	scheme                             theme.Scheme
	selected                           string
	selectedStep, page                 int
	grid, roll                         *sequencerGrid
	inspector                          *fyne.Container
	inspectorTabs                      *container.AppTabs
	status, projectLabel               *widget.Label
	play                               *widget.Button
	addTrackButton                     *widget.Button
	bpm                                *commitEntry
	patternPads, pageButtons           []*widget.Button
	bankButton                         *widget.Button
	bankView, bankFollows              int // the bank the pads show; the pattern it last followed
	patternEmpty                       [groove.PatternCount]bool
	pageFilled                         [4]bool
	pageCount, playheadPage            int
	brand, versionText                 *canvas.Text
	swingLabel                         *widget.Label
	patternMode, songMode, menuButton  *widget.Button
	editorTabs                         *container.AppTabs
	mixer                              *fyne.Container
	swing                              *widget.Slider
	path                               string
	audioError                         error
	panels                             map[string]fyne.Window
}

func Run(themeDir string, noAudio bool) error {
	a := app.NewWithID("io.subbass.gruvbox")
	w := a.NewWindow("Gruvbox 3")
	w.SetMaster()
	u := newUI(a, w, theme.Library{Dir: themeDir})
	if !noAudio {
		u.audioError = u.player.Open()
	}
	defer u.player.Close()
	w.Resize(fyne.NewSize(1320, 780))
	w.SetContent(u.content())
	w.SetCloseIntercept(func() { u.discardConfirm(func() { w.SetCloseIntercept(nil); w.Close() }) })
	done := make(chan struct{})
	w.SetOnClosed(func() {
		u.closeDrawer(u.activeDrawer)
		for _, panel := range u.panels {
			panel.Close()
		}
		close(done)
	})
	go func() {
		ticker := time.NewTicker(playheadInterval)
		defer ticker.Stop()
		var pending atomic.Bool
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !pending.CompareAndSwap(false, true) {
					continue
				}
				fyne.Do(func() {
					defer pending.Store(false)
					u.updatePlaybackLabel()
					u.jamTick()
					if u.recordingRefresh != nil {
						u.recordingRefresh()
					}
					if u.player.Playing() {
						u.grid.advancePlayhead()
						u.roll.advancePlayhead()
					}
				})
			}
		}
	}()
	w.ShowAndRun()
	return nil
}
func newUI(a fyne.App, w fyne.Window, library theme.Library) *UI {
	p, info := startupProject()
	entry, err := library.Selected()
	if err != nil {
		entry, _ = library.Load("warm-dark")
	}
	u := &UI{sampleViews: map[string]sampleView{}, fxFolded: map[string]bool{}, panels: map[string]fyne.Window{}, app: a, window: w, session: groove.NewSession(p), player: groove.NewPlayer(), library: library, scheme: entry.Scheme, selected: p.Tracks[0].ID}
	u.session.LastAction = info
	u.recordGroove(info)
	// Use the same data root as the theme library, including custom theme dirs.
	u.soundLibrary = groove.SoundLibrary{Dir: filepath.Join(filepath.Dir(library.Dir), "sounds")}
	a.Settings().SetTheme(paletteTheme{u.scheme})
	u.grid, u.roll = newGrid(u, false), newGrid(u, true)
	u.player.Publish(p)
	return u
}
func (u *UI) selectedTrack() *groove.Track { return u.session.Project.Track(u.selected) }
func (u *UI) edit(label string, fn func(*groove.Project) error) {
	if !u.liveEdit {
		u.finishLiveSlider()
	}
	if err := u.session.Do(label, fn); err != nil {
		u.showError(err)
		return
	}
	u.player.Publish(u.session.Project)
	if !u.liveEdit {
		u.refresh()
	} else {
		u.updateProjectTitle()
	}
}
func (u *UI) content() fyne.CanvasObject {
	brand := canvas.NewText("GRUVBOX 3", colour(u.scheme.Roles().Text))
	brand.TextSize = 23
	brand.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	u.brand = brand
	version := canvas.NewText("v"+config.Version, colour(u.scheme.Roles().Secondary))
	version.TextSize = 11
	version.TextStyle = fyne.TextStyle{Monospace: true}
	u.versionText = version
	u.play = widget.NewButtonWithIcon("Play", fynetheme.MediaPlayIcon(), u.togglePlay)
	u.bpm = newCommitEntry(fmt.Sprintf("%.0f", u.session.Project.BPM), func(value string) {
		bpm, err := strconv.ParseFloat(value, 64)
		if err != nil || bpm < 40 || bpm > 300 {
			u.bpm.SetValue(fmt.Sprintf("%.0f", u.session.Project.BPM))
			return
		}
		u.edit("Tempo", func(p *groove.Project) error { p.BPM = bpm; return nil })
	})
	bpmBox := container.NewGridWrap(fyne.NewSize(65, 34), u.bpm)
	swing := widget.NewSlider(0, 50)
	swing.Step = 1
	u.swing = swing
	u.swingLabel = widget.NewLabel("Swing 0%")
	u.bindLiveSlider(swing, func(v float64) {
		u.swingLabel.SetText(fmt.Sprintf("Swing %.0f%%", v))
		u.edit("Swing", func(p *groove.Project) error { p.Swing = v / 100; return nil })
	})
	swingBox := container.NewGridWrap(fyne.NewSize(110, 32), swing)
	u.bankButton = widget.NewButtonWithIcon("A", fynetheme.MenuDropDownIcon(), nil)
	u.bankButton.IconPlacement = widget.ButtonIconTrailingText
	u.bankButton.OnTapped = func() { u.showBankMenu(u.bankButton) }
	pads := container.NewHBox(widget.NewLabel("Pattern"), u.bankButton)
	u.patternPads = nil
	for pad := 0; pad < groove.BankSize; pad++ {
		pad := pad
		button := widget.NewButton(strconv.Itoa(pad+1), func() { u.selectPattern(u.bankView*groove.BankSize + pad) })
		u.patternPads = append(u.patternPads, button)
		pads.Add(button)
	}
	u.patternMode = widget.NewButton("Pattern", func() { u.setSongMode(false) })
	u.songMode = widget.NewButton("Song", func() { u.setSongMode(true) })
	randomAll := widget.NewButtonWithIcon("Randomise all", fynetheme.ViewRefreshIcon(), u.randomiseGroove)
	randomParts := widget.NewButtonWithIcon("", fynetheme.MenuDropDownIcon(), nil)
	randomParts.OnTapped = func() { u.showRandomiseMenu(randomParts) }
	undo := widget.NewButtonWithIcon("", fynetheme.ContentUndoIcon(), u.undo)
	redo := widget.NewButtonWithIcon("", fynetheme.ContentRedoIcon(), u.redo)
	u.menuButton = widget.NewButtonWithIcon("", fynetheme.MenuIcon(), u.showMainMenu)
	brandBox := container.NewVBox(layout.NewSpacer(), brand, version, layout.NewSpacer())
	transport := container.NewHBox(brandBox, u.play, widget.NewLabel("BPM"), bpmBox, u.swingLabel, swingBox, widget.NewSeparator(), pads, widget.NewSeparator(), container.NewHBox(u.patternMode, u.songMode), layout.NewSpacer(), container.NewHBox(randomAll, randomParts), undo, redo, u.menuButton, u.helpButton(shortcutHelp))
	u.projectLabel = widget.NewLabel("")
	u.playbackLabel = widget.NewLabel("")
	u.addTrackButton = widget.NewButton("+ Track", u.addTrack)
	patternTools := widget.NewButton("Patterns", nil)
	patternTools.OnTapped = func() { u.patternTools(patternTools) }
	pages := container.NewHBox(widget.NewLabel("Steps"))
	u.pageButtons = nil
	for page := 0; page < 4; page++ {
		page := page
		button := widget.NewButton(pageLabel(page, false), func() { u.setPage(page) })
		u.pageButtons = append(u.pageButtons, button)
		pages.Add(button)
	}
	trackActions := container.NewHBox(u.addTrackButton, patternTools, layout.NewSpacer(), pages)
	u.songPanel = container.NewVBox()
	u.mixer = container.NewHBox()
	u.rollScroll = container.NewVScroll(u.roll)
	editorTabs := container.NewAppTabs(container.NewTabItem("Steps", container.NewVScroll(u.grid)), container.NewTabItem("Piano roll", container.NewBorder(container.NewHBox(widget.NewButton("Octave −", func() { u.roll.shiftOctave(-1) }), widget.NewButton("Octave +", func() { u.roll.shiftOctave(1) }), widget.NewLabel("Shift-click selects · drag edge for length")), u.roll.selectionTools(), nil, nil, u.rollScroll)))
	u.songTimeline = newSongTimeline(u)
	u.songScroll = container.NewVScroll(u.songPanel)
	editorTabs.Append(container.NewTabItem("Song", container.NewBorder(container.NewVBox(container.NewHScroll(u.songTimeline), widget.NewSeparator()), nil, nil, nil, u.songScroll)))
	editorTabs.Append(container.NewTabItem("Mixer", container.NewScroll(u.mixer)))
	editorTabs.Append(container.NewTabItem("Perform", container.NewVScroll(u.performancePanel())))
	u.editorTabs = editorTabs
	editorTabs.OnSelected = func(*container.TabItem) { u.followRoll() }
	u.inspector = container.NewVBox()
	u.sidePane = container.NewVScroll(u.inspector)
	u.drawerHost = container.NewStack()
	u.drawerHost.Hide()
	workspace := container.NewHSplit(container.NewBorder(trackActions, nil, nil, nil, editorTabs), container.NewStack(u.sidePane, u.drawerHost))
	workspace.Offset = .74
	u.workspace = workspace
	u.status = widget.NewLabel("")
	u.status.Truncation = fyne.TextTruncateEllipsis
	foot := container.NewVBox(u.errorToast(), widget.NewSeparator(), container.NewBorder(nil, nil, container.NewHBox(u.projectLabel, u.playbackLabel), nil, u.status))
	u.window.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) { u.key(e.Name) })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { u.undo() })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}, func(fyne.Shortcut) { u.redo() })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyD, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { u.duplicatePattern(false) })
	// The desktop driver reports Ctrl+Z, Ctrl+Y, Ctrl+C and Ctrl+V as Fyne's
	// standard shortcuts rather than custom ones, so bind those names too.
	u.window.Canvas().AddShortcut(&fyne.ShortcutUndo{}, func(fyne.Shortcut) { u.undo() })
	u.window.Canvas().AddShortcut(&fyne.ShortcutRedo{}, func(fyne.Shortcut) { u.redo() })
	u.window.Canvas().AddShortcut(&fyne.ShortcutCopy{}, func(fyne.Shortcut) { u.copyStep() })
	u.window.Canvas().AddShortcut(&fyne.ShortcutPaste{}, func(fyne.Shortcut) { u.pasteStep() })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyC, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}, func(fyne.Shortcut) { u.copyPattern() })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyV, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}, func(fyne.Shortcut) { u.pastePattern() })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyTab, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { u.cycleEditorTab(1) })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyTab, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}, func(fyne.Shortcut) { u.cycleEditorTab(-1) })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { u.save() })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { u.randomiseGroove() })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}, func(fyne.Shortcut) { u.randomiseParts(groove.DrumParts) })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyG, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { u.growTrack() })
	for bank := 0; bank < groove.BankCount; bank++ {
		bank := bank
		u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyName(strconv.Itoa(bank + 1)), Modifier: fyne.KeyModifierAlt}, func(fyne.Shortcut) { u.showBank(bank) })
	}
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyH, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { u.showHistory() })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl | fyne.KeyModifierAlt}, func(fyne.Shortcut) { u.randomiseParts(groove.SynthParts) })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}, func(fyne.Shortcut) { u.saveAs() })
	u.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyP, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}, func(fyne.Shortcut) { u.storePatternState() })
	u.refresh()
	u.root = container.NewBorder(container.NewVBox(transport, widget.NewSeparator()), foot, nil, nil, workspace)
	return container.NewPadded(u.root)
}

// showMainMenu opens the ≡ menu: project files, export and appearance.
func (u *UI) showMainMenu() {
	newProject := fyne.NewMenuItem("New", nil)
	newProject.ChildMenu = fyne.NewMenu("New project", u.templateItems()...)
	save := fyne.NewMenuItem("Save", u.save)
	save.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl}
	saveAs := fyne.NewMenuItem("Save as…", u.saveAs)
	saveAs.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}
	export := fyne.NewMenuItem("Export WAV", nil)
	export.ChildMenu = fyne.NewMenu("Export WAV", u.exportItems()...)
	randomAll := fyne.NewMenuItem("Randomise", nil)
	randomAll.ChildMenu = fyne.NewMenu("Randomise", u.randomiseItems()...)
	menu := fyne.NewMenu("Gruvbox", newProject, randomAll, fyne.NewMenuItem("Open…", u.open), save, saveAs, fyne.NewMenuItemSeparator(), export, fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Appearance…", u.settings))
	pop := widget.NewPopUpMenu(menu, u.window.Canvas())
	button := u.menuButton
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(button).Add(fyne.NewPos(button.Size().Width-pop.MinSize().Width, button.Size().Height))
	pop.ShowAtPosition(fyne.NewPos(max(0, pos.X), pos.Y))
}

// setSongMode switches between looping the selected pattern and playing the
// song arrangement.
func (u *UI) setSongMode(on bool) {
	if on && len(u.session.Project.Song) == 0 {
		u.status.SetText("Add song sections in the Song tab first")
		return
	}
	u.player.SetSongMode(on)
	u.updatePlaybackLabel()
}
func (u *UI) updateModeButtons() {
	song := u.player.SongMode()
	for _, b := range []struct {
		button *widget.Button
		on     bool
	}{{u.patternMode, !song}, {u.songMode, song}} {
		importance := widget.LowImportance
		if b.on {
			importance = widget.HighImportance
		}
		if b.button.Importance != importance {
			b.button.Importance = importance
			b.button.Refresh()
		}
	}
	setButtonEnabled(u.songMode, song || len(u.session.Project.Song) > 0)
}
func (u *UI) refresh() {
	if u.status == nil {
		return
	}
	p := &u.session.Project
	if u.selectedTrack() == nil && len(p.Tracks) > 0 {
		u.selected = p.Tracks[0].ID
	}
	if t := u.selectedTrack(); t != nil {
		u.selectedStep = min(u.selectedStep, len(t.Patterns[p.Pattern].Steps)-1)
	}
	u.pageCount = 1
	for _, t := range p.Tracks {
		u.pageCount = max(u.pageCount, (len(t.Patterns[p.Pattern].Steps)+15)/16)
	}
	u.page = min(u.page, u.pageCount-1)
	for slot := range u.patternEmpty {
		u.patternEmpty[slot] = groove.PatternEmpty(*p, slot)
	}
	u.pageFilled = [4]bool{}
	for _, t := range p.Tracks {
		for i, step := range t.Patterns[p.Pattern].Steps {
			if step.Active && i/16 < len(u.pageFilled) {
				u.pageFilled[i/16] = true
			}
		}
	}
	u.grid.Refresh()
	u.roll.fitNotes()
	u.roll.Refresh()
	u.followRoll()
	u.refreshInspector()
	u.refreshMixer()
	u.refreshSong()
	for _, refresh := range u.effectModulationRefresh {
		refresh()
	}
	if u.soundLibraryRefresh != nil {
		u.soundLibraryRefresh()
	}
	if u.recordingRefresh != nil {
		u.recordingRefresh()
	}
	for _, refresh := range u.generatorRefresh {
		refresh()
	}
	u.refreshTransport()
}

// refreshTransport updates the top bar, pads, pages and status line.
func (u *UI) refreshTransport() {
	p := &u.session.Project
	u.updatePlaybackLabel()
	u.updateProjectTitle()
	setEntry(u.bpm, fmt.Sprintf("%.0f", p.BPM))
	if u.liveSlider != u.swing {
		setSlider(u.swing, p.Swing*100)
		setLabel(u.swingLabel, fmt.Sprintf("Swing %.0f%%", u.swing.Value))
	}
	u.brand.Color = colour(u.scheme.Roles().Text)
	u.brand.Refresh()
	u.versionText.Color = colour(u.scheme.Roles().Secondary)
	u.versionText.Refresh()
	u.updatePatternPads()
	u.updatePageButtons()
	if u.player.Playing() {
		u.play.SetText("Stop")
		u.play.SetIcon(fynetheme.MediaStopIcon())
		u.play.Importance = widget.HighImportance
	} else {
		u.play.SetText("Play")
		u.play.SetIcon(fynetheme.MediaPlayIcon())
		u.play.Importance = widget.MediumImportance
	}
	u.play.Refresh()
	// Play state is shown by playbackLabel beside the status line.
	var parts []string
	if u.lockMode {
		parts = append(parts, fmt.Sprintf("LOCK · step %02d", u.selectedStep+1))
	}
	if u.audioError != nil {
		parts = append(parts, "Audio unavailable: "+u.audioError.Error())
	}
	stored, edited := groove.PatternStateStatus(*p, p.Pattern)
	snapshot := "State: live"
	if stored > 0 {
		snapshot = fmt.Sprintf("State: stored %d/%d", stored, len(p.Tracks))
		if edited > 0 {
			snapshot += " · edited"
		}
	}
	parts = append(parts, fmt.Sprintf("%d tracks", len(p.Tracks)), fmt.Sprintf("Steps %02d–%02d", u.page*16+1, u.page*16+16), snapshot)
	if u.session.LastAction != "" {
		parts = append(parts, u.session.LastAction)
	}
	u.status.SetText(strings.Join(parts, "  /  "))
}
func (u *UI) togglePlay() {
	if u.audioError != nil {
		u.showError(u.audioError)
		return
	}
	u.player.SetPlaying(!u.player.Playing())
	u.refresh()
}
func (u *UI) undo() {
	u.finishLiveSlider()
	if u.session.Undo() {
		u.player.Publish(u.session.Project)
		u.refresh()
	}
}
func (u *UI) redo() {
	u.finishLiveSlider()
	if u.session.Redo() {
		u.player.Publish(u.session.Project)
		u.refresh()
	}
}
func (u *UI) generate(action string) {
	id := u.selected
	if u.selectedTrack() == nil {
		return
	}
	u.edit(action, func(p *groove.Project) error {
		if action == "Variation" {
			next := groove.NextInBank(p.Pattern)
			if err := groove.CopyPattern(p, p.Pattern, next, ""); err != nil {
				return err
			}
		}
		groove.GenerateIn(p, p.Track(id), p.Pattern, action)
		if action == "Variation" {
			return groove.SelectPattern(p, groove.NextInBank(p.Pattern))
		}
		return nil
	})
}
func (u *UI) toggleMute() { u.toggleTrackMute(u.selected) }

// toggleTrackHold marks a track to keep when randomising the groove.
func (u *UI) toggleTrackHold(id string) {
	if u.session.Project.Track(id) == nil {
		return
	}
	u.edit("Hold", func(p *groove.Project) error { t := p.Track(id); t.Hold = !t.Hold; return nil })
}
func (u *UI) toggleSolo() { u.toggleTrackSolo(u.selected) }
func (u *UI) toggleTrackMute(id string) {
	if u.session.Project.Track(id) == nil {
		return
	}
	u.edit("Mute", func(p *groove.Project) error { t := p.Track(id); t.Mute = !t.Mute; return nil })
}
func (u *UI) toggleTrackSolo(id string) {
	if u.session.Project.Track(id) == nil {
		return
	}
	u.edit("Solo", func(p *groove.Project) error { t := p.Track(id); t.Solo = !t.Solo; return nil })
}
func (u *UI) addTrack() {
	items := make([]*fyne.MenuItem, 0, len(groove.Kinds))
	for _, kind := range groove.Kinds {
		kind := kind
		items = append(items, fyne.NewMenuItem(kind.Name(), func() {
			u.edit("Add track", func(p *groove.Project) error {
				t := groove.NewTrack(kind, len(p.Tracks))
				p.Tracks = append(p.Tracks, t)
				u.selected = t.ID
				return nil
			})
		}))
	}
	menu := widget.NewPopUpMenu(fyne.NewMenu("", items...), u.window.Canvas())
	menu.ShowAtRelativePosition(fyne.NewPos(0, u.addTrackButton.Size().Height), u.addTrackButton)
}

func (u *UI) duplicateTrack() {
	if u.selectedTrack() == nil {
		return
	}
	id := u.selected
	u.edit("Duplicate track", func(p *groove.Project) error {
		source := p.Track(id)
		copy := groove.NewTrack(source.Kind, source.Colour)
		newID := copy.ID
		copy = *source
		copy.ID = newID
		copy.Name += " copy"
		p.Tracks = append(p.Tracks, copy)
		u.selected = newID
		return nil
	})
}
func (u *UI) removeTrack() {
	id := u.selected
	u.edit("Remove track", func(p *groove.Project) error {
		for i, t := range p.Tracks {
			if t.ID == id {
				for n := range p.Song {
					delete(p.Song[n].Mutes, id)
				}
				p.Tracks = append(p.Tracks[:i], p.Tracks[i+1:]...)
				break
			}
		}
		return nil
	})
}
func (u *UI) moveTrack(delta int) {
	id := u.selected
	u.edit("Reorder track", func(p *groove.Project) error {
		for i, t := range p.Tracks {
			if t.ID == id {
				j := i + delta
				if j >= 0 && j < len(p.Tracks) {
					p.Tracks[i], p.Tracks[j] = p.Tracks[j], p.Tracks[i]
				}
				break
			}
		}
		return nil
	})
}
func (u *UI) key(key fyne.KeyName) {
	if u.modalKey(key) {
		return
	}
	t := u.selectedTrack()
	switch key {
	case fyne.KeySpace:
		u.togglePlay()
		return
	case fyne.KeyF:
		if u.player.Playing() {
			u.queuePerformance(groove.DrumFill)
		} else {
			u.status.SetText("Start playback to queue a fill")
		}
		return
	case fyne.KeyG:
		if t != nil {
			u.generate(groove.Actions(t.Kind)[0])
		}
		return
	case fyne.KeyR:
		u.randomiseSound()
		return
	case fyne.KeyM:
		u.toggleMute()
		return
	case fyne.KeyH:
		u.toggleTrackHold(u.selected)
		return
	case fyne.KeyJ:
		u.setJam(!u.jam.on)
		return
	case fyne.KeyS:
		u.toggleSolo()
		return
	case fyne.Key1, fyne.Key2, fyne.Key3, fyne.Key4, fyne.Key5, fyne.Key6, fyne.Key7, fyne.Key8:
		u.selectPattern(u.bankView*groove.BankSize + int(key[0]-'1'))
		return
	case fyne.KeyK:
		u.setLockMode(!u.lockMode)
		return
	case fyne.KeyEscape:
		if u.activeDrawer != nil {
			u.activeDrawer.Close()
			return
		}
		u.setLockMode(false)
		return
	case fyne.KeyComma:
		u.nudgeVelocity(-.1)
		return
	case fyne.KeyPeriod:
		u.nudgeVelocity(.1)
		return
	case fyne.KeyLeftBracket:
		u.setPage(u.page - 1)
		return
	case fyne.KeyRightBracket:
		u.setPage(u.page + 1)
		return
	}
	if t == nil {
		return
	}
	steps := len(t.Patterns[u.session.Project.Pattern].Steps)
	switch key {
	case fyne.KeyLeft:
		u.selectedStep = (u.selectedStep + steps - 1) % steps
	case fyne.KeyRight:
		u.selectedStep = (u.selectedStep + 1) % steps
	case fyne.KeyUp, fyne.KeyDown:
		for i, track := range u.session.Project.Tracks {
			if track.ID == u.selected {
				delta := 1
				if key == fyne.KeyUp {
					delta = -1
				}
				u.selected = u.session.Project.Tracks[(i+delta+len(u.session.Project.Tracks))%len(u.session.Project.Tracks)].ID
				break
			}
		}
	case fyne.KeyReturn, fyne.KeyDelete, fyne.KeyA, fyne.KeyL:
		id, index := t.ID, u.selectedStep
		u.edit("Edit step", func(p *groove.Project) error {
			s := &p.Track(id).Patterns[p.Pattern].Steps[index]
			switch key {
			case fyne.KeyReturn:
				s.Active = !s.Active
			case fyne.KeyDelete:
				*s = groove.DefaultStep()
			case fyne.KeyA:
				s.Accent = !s.Accent
			case fyne.KeyL:
				if t.Kind.Capabilities().Slide {
					s.Slide = !s.Slide
				}
			}
			return nil
		})
		return
	default:
		return
	}
	u.page = u.selectedStep / 16
	u.refresh()
}
func (u *UI) discardConfirm(action func()) {
	if !u.session.Dirty() {
		action()
		return
	}
	u.confirm("Unsaved groove", "Discard", "Cancel", widget.NewLabel("Discard the current unsaved changes?"), func(ok bool) {
		if ok {
			action()
		}
	})
}

// grooveFiles is the type filter for project files.
var grooveFiles = fileKind{Name: "Gruvbox projects", Exts: []string{".gruvbox3"}}

// projectDir is where project dialogs start: the open project's folder, or
// the projects folder.
func (u *UI) projectDir() string {
	if u.path != "" {
		return filepath.Dir(u.path)
	}
	return config.ProjectsDir
}

func (u *UI) open() {
	u.chooseFile(fileSpec{title: "Open groove", accept: "Open", mode: openFile, kinds: []fileKind{grooveFiles}, dir: u.projectDir(), recent: "projects"}, u.openProject)
}

// openProject reads a project and, once it is ready, replaces the current
// one (confirming any unsaved work first).
func (u *UI) openProject(path string) {
	fileWork(func() {
		f, err := os.Open(path)
		var p groove.Project
		if err == nil {
			p, err = groove.Decode(f)
			f.Close()
		}
		fyne.Do(func() {
			if err != nil {
				u.showError(fmt.Errorf("could not open %s: %w", filepath.Base(path), err))
				return
			}
			u.discardConfirm(func() {
				u.player.SetPlaying(false)
				u.player.SetSongMode(false)
				u.player.StopAudition()
				u.player.ClearPerformanceTake()
				u.sampleViews = map[string]sampleView{}
				u.session = groove.NewSession(p)
				u.path = path
				u.selected = ""
				u.selectedStep, u.page = 0, 0
				u.player.Publish(p)
				u.refresh()
				u.rememberFile("projects", path)
				u.status.SetText("Opened " + path)
			})
		})
	})
}

// save writes to the project's known path; a new project asks for one first.
func (u *UI) save() {
	if u.path == "" {
		u.saveAs()
		return
	}
	u.writeProject(u.path)
}
func (u *UI) saveAs() {
	name := "untitled.gruvbox3"
	if u.path != "" {
		name = filepath.Base(u.path)
	}
	u.chooseFile(fileSpec{title: "Save groove as", accept: "Save", mode: saveFile, kinds: []fileKind{grooveFiles}, dir: u.projectDir(), name: name}, u.writeProject)
}
func (u *UI) writeProject(destination string) {
	snapshot := u.session.Project.Clone()
	session := u.session
	fileWork(func() {
		err := os.MkdirAll(filepath.Dir(destination), 0755)
		if err == nil {
			err = groove.Save(destination, snapshot)
		}
		fyne.Do(func() {
			if err != nil {
				u.showError(err)
				return
			}
			if u.session == session {
				u.path = destination
				u.session.MarkSavedProject(snapshot)
				u.refresh()
				u.rememberFile("projects", destination)
				u.status.SetText("Saved " + destination)
			}
		})
	})
}

// selectPattern switches the edited pattern; while playing, the switch is
// queued for the next bar and song playback returns to manual.
func (u *UI) selectPattern(slot int) {
	if slot < 0 || slot >= groove.PatternCount || slot == u.session.Project.Pattern {
		return
	}
	u.player.CancelPerformance()
	u.player.SetSongMode(false)
	u.selectedStep, u.page = 0, 0
	u.edit("Select pattern", func(p *groove.Project) error { return groove.SelectPattern(p, slot) })
}
func (u *UI) setPage(page int) {
	page = max(0, min(page, u.pageCount-1))
	if page == u.page {
		return
	}
	u.page = page
	if t := u.selectedTrack(); t != nil && u.selectedStep/16 != page {
		u.selectedStep = min(page*16, len(t.Patterns[u.session.Project.Pattern].Steps)-1)
	}
	u.refresh()
}
func pageLabel(page int, playing bool) string {
	label := fmt.Sprintf("%d–%d", page*16+1, page*16+16)
	if playing {
		label = "▸" + label
	}
	return label
}

// updatePatternPads shows the viewed bank: the edited pattern, empty slots
// and a queued switch. The view follows the edited pattern when it changes
// (song playback, history, Grow), and otherwise stays where it was put.
func (u *UI) updatePatternPads() {
	p := &u.session.Project
	if p.Pattern != u.bankFollows {
		u.bankFollows, u.bankView = p.Pattern, groove.BankOf(p.Pattern)
	}
	bank := string(rune('A' + u.bankView))
	if u.bankButton.Text != bank {
		u.bankButton.SetText(bank)
	}
	queued := u.player.QueuedPattern()
	for n, pad := range u.patternPads {
		slot := u.bankView*groove.BankSize + n
		importance := widget.MediumImportance
		if slot == p.Pattern {
			importance = widget.HighImportance
		} else if u.patternEmpty[slot] {
			importance = widget.LowImportance
		}
		text := strconv.Itoa(slot%groove.BankSize + 1)
		if slot == queued && u.player.Playing() {
			text = "→" + text
		}
		if pad.Importance != importance || pad.Text != text {
			pad.Importance = importance
			pad.Text = text
			pad.Refresh()
		}
	}
}

// updatePageButtons hides pages past the longest pattern and marks the page
// that holds the selected track's playhead. It runs on the playhead tick, so
// it only touches widgets whose state changed.
func (u *UI) updatePageButtons() {
	playing := -1
	if t := u.selectedTrack(); t != nil && u.player.Playing() && u.player.CurrentPattern() == u.session.Project.Pattern {
		playing = u.player.Position(t.ID) / 16
	}
	u.playheadPage = playing
	for page, button := range u.pageButtons {
		if page >= u.pageCount {
			if button.Visible() {
				button.Hide()
			}
			continue
		}
		if !button.Visible() {
			button.Show()
		}
		importance := widget.MediumImportance
		if page == u.page {
			importance = widget.HighImportance
		} else if page < len(u.pageFilled) && !u.pageFilled[page] {
			importance = widget.LowImportance
		}
		text := pageLabel(page, page == playing)
		if button.Importance != importance || button.Text != text {
			button.Importance = importance
			button.Text = text
			button.Refresh()
		}
	}
}

const shortcutHelp = `Keyboard
Space  Play / stop
1–8  Select pattern in the bank shown
Alt+1–8  Show bank A–H
[ ]  Previous / next step page
← →  Move step     ↑ ↓  Select track
Enter  Toggle step     Delete  Clear step
A  Accent     L  Slide     , .  Velocity down / up
Ctrl+C / Ctrl+V  Copy / paste step (paste moves on)
Ctrl+Shift+C / V  Copy / paste the track's pattern
K  Lock mode (controls write step locks)
G  Generate (first action)     R  Randomise sound
Ctrl+R  Randomise all: a fresh groove in this pattern
Ctrl+Shift+R  Randomise drums only     Ctrl+Alt+R  Randomise 303s only
H  Hold the track: randomising leaves it alone
Ctrl+G  Grow this pattern into a whole track (song)
J  Jam: a small change every few bars while a pattern plays
Ctrl+H  Groove history: recent randomised grooves and favourites
F  Queue drum fill (while playing)
M  Mute     S  Solo
Ctrl+Z / Ctrl+Shift+Z (or Ctrl+Y)  Undo / redo
Ctrl+Tab / Ctrl+Shift+Tab  Next / previous editor tab
Esc  Close drawer or card, then leave lock mode
Ctrl+D  Duplicate groove to empty slot
Ctrl+Shift+P  Store pattern state
Ctrl+S  Save     Ctrl+Shift+S  Save as

Mouse
Click a step to toggle it; right-click to select it only
Right-click a track name  Rename, duplicate, colour, move, remove
H / M / S chips  Hold / mute / solo`

// showBankMenu lists banks A–H; ● marks banks holding patterns and the
// current bank is ticked.
func (u *UI) showBankMenu(button fyne.CanvasObject) {
	items := []*fyne.MenuItem{}
	for bank := 0; bank < groove.BankCount; bank++ {
		bank := bank
		used := 0
		for pad := 0; pad < groove.BankSize; pad++ {
			if !u.patternEmpty[bank*groove.BankSize+pad] {
				used++
			}
		}
		label := fmt.Sprintf("Bank %c   ○", 'A'+bank)
		if used > 0 {
			label = fmt.Sprintf("Bank %c   ● %d", 'A'+bank, used)
		}
		item := fyne.NewMenuItem(label, func() { u.showBank(bank) })
		item.Checked = bank == u.bankView
		item.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyName(strconv.Itoa(bank + 1)), Modifier: fyne.KeyModifierAlt}
		items = append(items, item)
	}
	canvas := u.window.Canvas()
	widget.ShowPopUpMenuAtPosition(fyne.NewMenu("Banks", items...), canvas, fyne.CurrentApp().Driver().AbsolutePositionForObject(button).Add(fyne.NewPos(0, button.Size().Height)))
}

// showBank shows a bank's pads without changing the pattern.
func (u *UI) showBank(bank int) {
	if bank < 0 || bank >= groove.BankCount {
		return
	}
	u.bankView = bank
	u.updatePatternPads()
}
