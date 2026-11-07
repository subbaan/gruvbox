package desktop

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/theme"
)

func (u *UI) applyScheme(scheme theme.Scheme) {
	u.scheme = scheme
	u.app.Settings().SetTheme(paletteTheme{scheme})
	u.refresh()
}

// themeFiles is the type filter for colour themes (Base16 / Tinted YAML,
// or Gruvbox's own JSON).
var themeFiles = fileKind{Name: "Colour themes", Exts: []string{".yaml", ".yml", ".json"}}

func (u *UI) settings() {
	if w := u.panels["Conf / Appearance"]; w != nil {
		w.Show()
		w.RequestFocus()
		return
	}
	original := u.scheme
	chosen := ""
	applied := false
	closed := false
	description := widget.NewLabel("")
	description.Wrapping = fyne.TextWrapWord
	preview := container.NewGridWithColumns(16)
	selector := widget.NewSelect(nil, nil)
	refresh := func() {
		entries, err := u.library.List()
		if err != nil {
			u.showError(err)
			return
		}
		var ids []string
		for _, entry := range entries {
			ids = append(ids, entry.ID)
		}
		selector.Options = ids
		selector.Refresh()
	}
	selector.OnChanged = func(id string) {
		entry, err := u.library.Load(id)
		if err != nil {
			u.showError(err)
			return
		}
		chosen = id
		u.applyScheme(entry.Scheme)
		description.SetText(fmt.Sprintf("%s · %s\n%s", entry.Scheme.Name, entry.Scheme.Variant, entry.Scheme.Author))
		preview.Objects = nil
		for _, c := range entry.Scheme.Palette {
			rect := canvas.NewRectangle(colour(c))
			rect.SetMinSize(fyne.NewSize(20, 28))
			preview.Add(rect)
		}
		preview.Refresh()
	}
	refresh()
	if selected, err := u.library.Selected(); err == nil {
		selector.SetSelected(selected.ID)
	}
	importTheme := func() {
		conf := u.panels["Conf / Appearance"]
		if conf == nil {
			return
		}
		// The dialog needs more room than the settings window has.
		if size := conf.Canvas().Size(); size.Width < 900 || size.Height < 600 {
			conf.Resize(fyne.NewSize(max(900, size.Width), max(600, size.Height)))
		}
		u.chooseFile(fileSpec{title: "Import theme", accept: "Import", mode: openFile, kinds: []fileKind{themeFiles}, dir: u.library.Dir, canvas: conf.Canvas()}, func(path string) {
			go func() {
				name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				name = regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(strings.ToLower(name), "-")
				entry, err := u.library.Import(strings.Trim(name, "-_"), path)
				fyne.Do(func() {
					if closed {
						return
					}
					if err != nil {
						u.showError(err)
						return
					}
					refresh()
					selector.SetSelected(entry.ID)
				})
			}()
		})
	}
	content := container.NewVBox(widget.NewLabel("APPEARANCE"), selector, description, preview, widget.NewLabel("Selecting a theme previews the whole application."), container.NewHBox(widget.NewButton("Download themes", func() {
		u.catalogue(func(id string) {
			if closed {
				return
			}
			refresh()
			selector.SetSelected(id)
		})
	}), widget.NewButton("Import theme", importTheme), widget.NewButton("Reset to default", func() { selector.SetSelected("warm-dark") })))
	w, _ := u.panel("Conf / Appearance", fyne.NewSize(650, 400), func() {
		closed = true
		if !applied {
			u.applyScheme(original)
		}
	})
	apply := func() {
		if err := u.library.Select(chosen); err != nil {
			u.showError(err)
			return
		}
		applied = true
		w.Close()
	}
	w.SetContent(container.NewPadded(container.NewBorder(nil, container.NewHBox(widget.NewButton("Cancel", w.Close), widget.NewButton("Apply", apply)), nil, nil, content)))
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name == fyne.KeyEscape {
			w.Close()
		}
	})
	w.Show()
}

func (u *UI) catalogue(installed func(string)) {
	if w := u.panels["Download themes"]; w != nil {
		w.Show()
		w.RequestFocus()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{Timeout: 30 * time.Second}
	all, filtered := []string{}, []string{}
	selected := ""
	closed := false
	status := widget.NewLabel("Loading GitHub catalogue…")
	status.Wrapping = fyne.TextWrapWord
	search := widget.NewEntry()
	search.SetPlaceHolder("Search Base16 schemes")
	list := widget.NewList(func() int { return len(filtered) }, func() fyne.CanvasObject { return widget.NewLabel("") }, func(id widget.ListItemID, obj fyne.CanvasObject) { obj.(*widget.Label).SetText(filtered[id]) })
	download := widget.NewButton("Download selected", nil)
	download.Disable()
	list.OnSelected = func(id widget.ListItemID) { selected = filtered[id]; download.Enable() }
	filter := func() {
		filtered = nil
		query := strings.ToLower(search.Text)
		for _, name := range all {
			if strings.Contains(name, query) {
				filtered = append(filtered, name)
			}
		}
		list.UnselectAll()
		selected = ""
		download.Disable()
		list.Refresh()
	}
	search.OnChanged = func(string) { filter() }
	refresh := func() {
		status.SetText("Refreshing from GitHub…")
		go func() {
			catalogue, err := u.library.RefreshCatalogue(ctx, client)
			fyne.Do(func() {
				if closed {
					return
				}
				if err != nil {
					status.SetText("Refresh failed; cached themes remain available. " + err.Error())
					return
				}
				all = catalogue.Schemes
				filter()
				status.SetText(fmt.Sprintf("%d Base16 schemes · tinted-theming/schemes", len(all)))
			})
		}()
	}
	download.OnTapped = func() {
		if selected == "" {
			return
		}
		slug := selected
		download.Disable()
		status.SetText("Downloading " + slug + "…")
		go func() {
			entry, err := u.library.Download(ctx, client, slug)
			fyne.Do(func() {
				if closed {
					return
				}
				download.Enable()
				if err != nil {
					status.SetText(err.Error())
					return
				}
				status.SetText("Installed " + entry.Scheme.Name)
				installed(entry.ID)
			})
		}()
	}
	content := container.NewBorder(container.NewVBox(search, status), container.NewHBox(download, widget.NewButton("Refresh catalogue", refresh)), nil, nil, list)
	w, _ := u.panel("Download themes", fyne.NewSize(650, 520), func() { closed = true; cancel() })
	w.SetContent(container.NewPadded(container.NewBorder(nil, widget.NewButton("Close", w.Close), nil, nil, content)))
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name == fyne.KeyEscape {
			w.Close()
		}
	})
	w.Show()
	if cached, err := u.library.CachedCatalogue(); err == nil {
		all = cached.Schemes
		filter()
		status.SetText(fmt.Sprintf("%d cached schemes · Refresh to check GitHub", len(all)))
	} else {
		refresh()
	}
}
