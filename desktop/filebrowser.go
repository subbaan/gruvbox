package desktop

// The Open / Save dialog's model (alpha.49), after the one in plica: places
// (recent files, home, the user's folders, bookmarks), a sortable folder
// listing, a name field that filters or takes a path, and back / forward /
// up history. It does no drawing; filedialog.go draws it.
//
// Recent files and bookmarks are never stat'ed while listing them: they may
// be on sleeping drives or automounts, and are touched only when chosen.
// Folders are read through read, which the dialog runs off the UI thread.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type fileMode int

const (
	openFile fileMode = iota
	saveFile
)

type fileSort int

const (
	sortName fileSort = iota
	sortSize
	sortModified
)

// fileKind is one choice of the type filter: a name and the extensions it
// shows (lower case, with the dot). No extensions shows every file.
type fileKind struct {
	Name string
	Exts []string
}

var allFiles = fileKind{Name: "All files"}

func (k fileKind) matches(name string) bool {
	if len(k.Exts) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	return slices.Contains(k.Exts, ext)
}

// label is the kind as the filter shows it: "Gruvbox projects (.gruvbox3)".
func (k fileKind) label() string {
	if len(k.Exts) == 0 {
		return k.Name
	}
	return k.Name + " (" + strings.Join(k.Exts, " ") + ")"
}

type fileEntry struct {
	Name, Path string
	Dir        bool
	Special    bool // not a regular file or folder (FIFO, socket, device)
	Size       int64
	HasSize    bool
	Modified   time.Time
	Folder     string // recent files: the folder, shown in place of the size
}

// filePrefs are view choices kept between dialogs for the session.
type filePrefs struct {
	hidden, preview bool
	sort            fileSort
	desc            bool
	dirs            map[string]string // the folder each dialog was last left in
}

func newFilePrefs() *filePrefs { return &filePrefs{preview: true, dirs: map[string]string{}} }

type placeKind int

const (
	placeRecent placeKind = iota
	placeDir
	placeBookmark
)

type place struct {
	label string
	kind  placeKind
	dir   string
}

type outcomeKind int

const (
	stay outcomeKind = iota
	cancelled
	chosen // path: the file to open or save
	askNewFolder
	info
	warn
)

type fileOutcome struct {
	kind outcomeKind
	path string
	msg  string
}

var none = fileOutcome{}

type fileBrowser struct {
	mode     fileMode
	dir      string
	recentOn bool
	entries  []fileEntry
	recent   []fileEntry // nil: no Recent place
	loading  bool
	err      string
	name     string // the name field
	sel      int    // index into visible()
	places   []place
	placeSel int
	// placesFocus: Up / Down / Enter move through the places, not the list.
	placesFocus   bool
	prefs         *filePrefs
	kinds         []fileKind
	kind          int
	back, forward []string
	bookmarks     []string
	bookmarksFile string
	extra         []place // folders this dialog adds (the project's, Gruvbox's)
	home          string
	pending       string // a path to select once its folder's listing arrives
	// read asks for dir's listing, delivered by loaded (at once in tests,
	// off the UI thread in the dialog).
	read func(dir string)
}

// fileBrowserInit is what a browser starts with.
type fileBrowserInit struct {
	mode          fileMode
	dir, name     string
	kinds         []fileKind
	recent        []recentFile // nil: no Recent place
	bookmarksFile string
	extra         []place
	prefs         *filePrefs
	read          func(dir string)
}

func newFileBrowser(init fileBrowserInit) *fileBrowser {
	home, _ := os.UserHomeDir()
	b := &fileBrowser{mode: init.mode, dir: filepath.Clean(init.dir), name: init.name, kinds: init.kinds, bookmarksFile: init.bookmarksFile, extra: init.extra, prefs: init.prefs, home: home, read: init.read}
	if b.prefs == nil {
		b.prefs = newFilePrefs()
	}
	if init.recent != nil {
		b.recent = []fileEntry{}
		for _, r := range init.recent {
			b.recent = append(b.recent, fileEntry{Name: filepath.Base(r.Path), Path: r.Path, Folder: tilde(filepath.Dir(r.Path), home), Modified: time.Unix(r.Used, 0)})
		}
	}
	b.bookmarks = loadBookmarks(b.bookmarksFile)
	b.buildPlaces()
	b.afterMove()
	b.name = init.name
	b.loading = true
	return b // the caller starts the first listing with reload
}

// ---- places ------------------------------------------------------------

func (b *fileBrowser) buildPlaces() {
	list := []place{}
	if b.mode == openFile && b.recent != nil {
		list = append(list, place{label: "Recent", kind: placeRecent})
	}
	has := func(dir string) bool {
		for _, p := range list {
			if p.kind != placeRecent && p.dir == dir {
				return true
			}
		}
		return false
	}
	if b.home != "" {
		list = append(list, place{label: "Home", kind: placeDir, dir: b.home})
		for _, d := range []struct{ key, fallback string }{{"DESKTOP", "Desktop"}, {"DOCUMENTS", "Documents"}} {
			dir := userDir(b.home, d.key)
			if dir == "" {
				dir = filepath.Join(b.home, d.fallback)
			}
			if info, err := os.Stat(dir); err == nil && info.IsDir() && dir != b.home {
				list = append(list, place{label: d.fallback, kind: placeDir, dir: dir})
			}
		}
	}
	for _, p := range b.extra {
		if p.dir != "" && !has(p.dir) {
			list = append(list, p)
		}
	}
	list = append(list, place{label: "File system", kind: placeDir, dir: "/"})
	for _, dir := range b.bookmarks {
		list = append(list, place{label: baseName(dir), kind: placeBookmark, dir: dir})
	}
	b.places = list
	b.placeSel = min(b.placeSel, max(0, len(list)-1))
}

// currentPlace is the place showing now, or -1.
func (b *fileBrowser) currentPlace() int {
	for i, p := range b.places {
		if b.recentOn && p.kind == placeRecent || !b.recentOn && p.kind != placeRecent && p.dir == b.dir {
			return i
		}
	}
	return -1
}

func (b *fileBrowser) isBookmarked() bool { return slices.Contains(b.bookmarks, b.dir) }

func (b *fileBrowser) goPlace(i int) {
	if i < 0 || i >= len(b.places) {
		return
	}
	b.placeSel = i
	if p := b.places[i]; p.kind == placeRecent {
		b.recentOn = true
		b.afterMove()
	} else {
		b.goTo(p.dir)
	}
}

// toggleBookmark bookmarks the folder shown, or removes its bookmark. The
// file is read again first so another dialog's changes are kept.
func (b *fileBrowser) toggleBookmark() fileOutcome {
	if b.recentOn {
		return fileOutcome{kind: warn, msg: "Choose a folder to bookmark"}
	}
	return b.setBookmark(b.dir, !b.isBookmarked())
}

func (b *fileBrowser) setBookmark(dir string, on bool) fileOutcome {
	if b.bookmarksFile == "" {
		return fileOutcome{kind: warn, msg: "No folder to keep bookmarks in"}
	}
	list := slices.DeleteFunc(loadBookmarks(b.bookmarksFile), func(p string) bool { return p == dir })
	if on {
		list = append(list, dir)
	}
	if err := saveBookmarks(b.bookmarksFile, list); err != nil {
		return fileOutcome{kind: warn, msg: "Could not save bookmarks: " + err.Error()}
	}
	b.bookmarks = list
	b.buildPlaces()
	if on {
		return fileOutcome{kind: info, msg: "Bookmarked " + baseName(dir)}
	}
	return fileOutcome{kind: info, msg: "Removed the bookmark for " + baseName(dir)}
}

// ---- folders -----------------------------------------------------------

func (b *fileBrowser) reload() {
	b.entries, b.err, b.loading = nil, "", true
	if b.read != nil {
		b.read(b.dir)
	}
}

// loaded takes a listing of dir; a listing for a folder no longer shown is
// dropped.
func (b *fileBrowser) loaded(dir string, entries []fileEntry, err error) {
	if dir != b.dir {
		return
	}
	keep := b.selectedPath()
	b.entries, b.loading, b.err = entries, false, ""
	if err != nil {
		b.err = err.Error()
	}
	b.sortEntries()
	b.sel = 0
	b.selectPath(keep)
	b.nameChanged()
}

// readFolder lists dir. Each entry is stat'ed once here (symlinks followed),
// never while drawing.
func readFolder(dir string) ([]fileEntry, error) {
	items, err := os.ReadDir(dir)
	entries := make([]fileEntry, 0, len(items))
	for _, item := range items {
		path := filepath.Join(dir, item.Name())
		var info os.FileInfo
		if item.Type()&os.ModeSymlink != 0 {
			info, _ = os.Stat(path)
		} else {
			info, _ = item.Info()
		}
		e := fileEntry{Name: item.Name(), Path: path}
		if info != nil {
			e.Dir = info.IsDir()
			e.Special = !e.Dir && !info.Mode().IsRegular()
			e.Modified = info.ModTime()
			if info.Mode().IsRegular() {
				e.Size, e.HasSize = info.Size(), true
			}
		}
		entries = append(entries, e)
	}
	return entries, err
}

func (b *fileBrowser) sortEntries() {
	by, desc := b.prefs.sort, b.prefs.desc
	slices.SortStableFunc(b.entries, func(x, y fileEntry) int {
		// Folders first, whichever way the files go.
		if x.Dir != y.Dir {
			if x.Dir {
				return -1
			}
			return 1
		}
		key := func(e fileEntry) string { return strings.ToLower(e.Name) }
		if x.Dir && by == sortSize {
			// Folders have no size worth sorting by: keep them A to Z.
			if o := strings.Compare(key(x), key(y)); o != 0 {
				return o
			}
			return strings.Compare(x.Name, y.Name)
		}
		o := 0
		switch by {
		case sortSize:
			o = cmpInt64(x.Size, y.Size)
		case sortModified:
			o = x.Modified.Compare(y.Modified)
		}
		if o == 0 {
			o = strings.Compare(key(x), key(y))
		}
		if o == 0 {
			o = strings.Compare(x.Name, y.Name)
		}
		if desc {
			o = -o
		}
		return o
	})
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// sortBy sorts by a column; the same column again reverses the order. Size
// and date start biggest and newest first.
func (b *fileBrowser) sortBy(s fileSort) {
	if b.recentOn {
		return
	}
	keep := b.selectedPath()
	if b.prefs.sort == s {
		b.prefs.desc = !b.prefs.desc
	} else {
		b.prefs.sort, b.prefs.desc = s, s != sortName
	}
	b.sortEntries()
	b.selectPath(keep)
}

func (b *fileBrowser) goTo(dir string) {
	dir = filepath.Clean(dir)
	if dir != b.dir {
		b.back = append(b.back, b.dir)
		b.forward = nil
		b.dir = dir
	}
	b.recentOn = false
	b.afterMove()
	b.reload()
}

// goUp goes to the parent folder, with the folder we came from selected.
func (b *fileBrowser) goUp() {
	if b.recentOn {
		b.recentOn = false
		b.afterMove()
		b.reload()
		return
	}
	child := b.dir
	if parent := filepath.Dir(child); parent != child {
		b.pending = child
		b.goTo(parent)
	}
}

func (b *fileBrowser) goBack() {
	if b.recentOn {
		b.goUp()
		return
	}
	if n := len(b.back); n > 0 {
		b.forward = append(b.forward, b.dir)
		b.dir = b.back[n-1]
		b.back = b.back[:n-1]
		b.afterMove()
		b.reload()
	}
}

func (b *fileBrowser) goForward() {
	if n := len(b.forward); n > 0 {
		b.back = append(b.back, b.dir)
		b.dir = b.forward[n-1]
		b.forward = b.forward[:n-1]
		b.recentOn = false
		b.afterMove()
		b.reload()
	}
}

func (b *fileBrowser) canBack() bool    { return b.recentOn || len(b.back) > 0 }
func (b *fileBrowser) canForward() bool { return len(b.forward) > 0 }

func (b *fileBrowser) afterMove() {
	b.sel = 0
	if b.mode == openFile {
		b.name = ""
	}
	if i := b.currentPlace(); i >= 0 {
		b.placeSel = i
	}
}

// crumb is one segment of the path bar.
type crumb struct{ label, dir string }

// crumbs runs from ~ or / down to the folder shown.
func (b *fileBrowser) crumbs() []crumb {
	dir := b.dir
	list := []crumb{{"/", "/"}}
	rest := strings.TrimPrefix(dir, "/")
	if b.home != "" && b.home != "/" && (dir == b.home || strings.HasPrefix(dir, b.home+"/")) {
		list = []crumb{{"~", b.home}}
		rest = strings.TrimPrefix(strings.TrimPrefix(dir, b.home), "/")
	}
	at := list[0].dir
	for _, part := range strings.Split(rest, "/") {
		if part == "" {
			continue
		}
		at = filepath.Join(at, part)
		list = append(list, crumb{part, at})
	}
	return list
}

// goCrumb goes to a path bar segment, selecting the folder we came from.
func (b *fileBrowser) goCrumb(i int) {
	list := b.crumbs()
	if i < 0 || i >= len(list) {
		return
	}
	// The listing may arrive later; remember what to select.
	b.pending = nextComponent(list[i].dir, b.dir)
	b.goTo(list[i].dir)
}

// nextComponent is the child of dir on the way to path.
func nextComponent(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.Join(dir, strings.Split(rel, string(filepath.Separator))[0])
}

// createFolder makes a folder in the one shown and selects it.
func (b *fileBrowser) createFolder(name string) fileOutcome {
	name = strings.TrimSpace(name)
	if name == "" {
		return none
	}
	if strings.Contains(name, "/") || name == "." || name == ".." {
		return fileOutcome{kind: warn, msg: "“" + name + "” isn't a folder name"}
	}
	path := filepath.Join(b.dir, name)
	if err := os.Mkdir(path, 0o755); err != nil {
		return fileOutcome{kind: warn, msg: "Could not make " + name + ": " + err.Error()}
	}
	if strings.HasPrefix(name, ".") {
		b.prefs.hidden = true
	}
	b.pending = path
	b.reload()
	return fileOutcome{kind: info, msg: "Made folder " + name}
}

// ---- the list ----------------------------------------------------------

func (b *fileBrowser) list() []fileEntry {
	if b.recentOn && b.recent != nil {
		return b.recent
	}
	return b.entries
}

// filter is the name field as a filter: when opening, unless it is a path.
func (b *fileBrowser) filter() string {
	t := strings.TrimSpace(b.name)
	if b.mode != openFile || t == "" || isPathText(t) {
		return ""
	}
	return strings.ToLower(t)
}

// visible lists the entries shown, in order.
func (b *fileBrowser) visible() []fileEntry {
	f := b.filter()
	kind := b.currentKind()
	out := []fileEntry{}
	for _, e := range b.list() {
		if b.recentOn {
			if f == "" || strings.Contains(strings.ToLower(e.Name+" "+e.Folder), f) {
				out = append(out, e)
			}
			continue
		}
		if !b.prefs.hidden && strings.HasPrefix(e.Name, ".") {
			continue
		}
		if !e.Dir && !kind.matches(e.Name) {
			continue
		}
		if f != "" && !strings.Contains(strings.ToLower(e.Name), f) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (b *fileBrowser) currentKind() fileKind {
	if b.kind >= 0 && b.kind < len(b.kinds) {
		return b.kinds[b.kind]
	}
	return allFiles
}

func (b *fileBrowser) selected() (fileEntry, bool) {
	v := b.visible()
	if b.sel >= 0 && b.sel < len(v) {
		return v[b.sel], true
	}
	return fileEntry{}, false
}

func (b *fileBrowser) selectedPath() string {
	if e, ok := b.selected(); ok {
		return e.Path
	}
	return ""
}

func (b *fileBrowser) selectPath(path string) {
	if path == "" {
		return
	}
	for i, e := range b.visible() {
		if e.Path == path {
			b.sel = i
			return
		}
	}
}

// setName is typing in the name field.
func (b *fileBrowser) setName(text string) {
	b.name = text
	b.nameChanged()
}

// nameChanged selects the first name starting with the filter (else the
// first containing it), and goes into a folder typed with a final "/".
func (b *fileBrowser) nameChanged() {
	if b.pending != "" && !b.loading {
		b.selectPath(b.pending)
		b.pending = ""
		b.copySelectedName()
		return
	}
	t := b.name
	if strings.HasSuffix(t, "/") && strings.TrimSpace(t) == t {
		if target := b.resolve(t); isDir(target) {
			b.name = ""
			b.goTo(target)
			return
		}
	}
	if f := b.filter(); f != "" {
		v := b.visible()
		b.sel = 0
		for i, e := range v {
			if strings.HasPrefix(strings.ToLower(e.Name), f) {
				b.sel = i
				break
			}
		}
	}
}

// resolve reads the field's text as a path: ~ expanded, relative to the
// folder shown.
func (b *fileBrowser) resolve(t string) string {
	switch {
	case t == "~":
		t = b.home
	case strings.HasPrefix(t, "~/"):
		t = filepath.Join(b.home, t[2:])
	}
	if !filepath.IsAbs(t) {
		t = filepath.Join(b.dir, t)
	}
	return filepath.Clean(t)
}

func (b *fileBrowser) moveSel(delta int) {
	n := len(b.visible())
	b.sel = max(0, min(n-1, b.sel+delta))
	b.copySelectedName()
}

// copySelectedName: when saving, a chosen file's name goes into the field.
func (b *fileBrowser) copySelectedName() {
	if e, ok := b.selected(); ok && b.mode == saveFile && !e.Dir {
		b.name = e.Name
	}
}

// activate is Enter or a double click on the selected entry: into a folder,
// or open / save a file.
func (b *fileBrowser) activate() fileOutcome {
	e, ok := b.selected()
	if !ok {
		return none
	}
	if e.Dir {
		b.goTo(e.Path)
		return none
	}
	if b.mode == saveFile {
		b.name = e.Name
	}
	return fileOutcome{kind: chosen, path: e.Path}
}

// accept is the Open / Save button, or Enter in the name field.
func (b *fileBrowser) accept() fileOutcome {
	t := strings.TrimSpace(b.name)
	if t == "" {
		if b.mode == saveFile {
			if e, ok := b.selected(); !ok || !e.Dir {
				return fileOutcome{kind: warn, msg: "Type a name to save as"}
			}
		}
		return b.activate()
	}
	if b.recentOn && !isPathText(t) {
		return b.activate()
	}
	path := b.resolve(t)
	if isDir(path) {
		b.name = ""
		b.goTo(path)
		return none
	}
	if b.mode == openFile {
		// A name that isn't there picks the selected match (type-ahead).
		if _, err := os.Stat(path); err != nil && !isPathText(t) {
			if _, ok := b.selected(); ok {
				return b.activate()
			}
			return fileOutcome{kind: warn, msg: "No file called " + t}
		}
	}
	return fileOutcome{kind: chosen, path: path}
}

// clickRow selects a row; a double click activates it (the dialog reports
// those separately).
func (b *fileBrowser) clickRow(i int) {
	if i < 0 || i >= len(b.visible()) {
		return
	}
	b.sel = i
	b.placesFocus = false
	b.copySelectedName()
}

func (b *fileBrowser) toggleHidden() {
	keep := b.selectedPath()
	b.prefs.hidden = !b.prefs.hidden
	b.sel = 0
	b.selectPath(keep)
}

func (b *fileBrowser) setKind(i int) {
	keep := b.selectedPath()
	b.kind = i
	b.sel = 0
	b.selectPath(keep)
}

// ---- helpers -----------------------------------------------------------

// isPathText: text that names a path rather than filters the list.
func isPathText(t string) bool { return strings.Contains(t, "/") || t == "~" || t == ".." }

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func baseName(p string) string {
	if p == "/" {
		return "/"
	}
	return filepath.Base(p)
}

func tilde(p, home string) string {
	if home != "" && home != "/" && (p == home || strings.HasPrefix(p, home+"/")) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

// userDir reads a folder from user-dirs.dirs (XDG_DESKTOP_DIR="$HOME/Desktop").
func userDir(home, key string) string {
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	data, err := os.ReadFile(filepath.Join(config, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	return parseUserDir(string(data), home, key)
}

func parseUserDir(text, home, key string) string {
	want := "XDG_" + key + "_DIR="
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, want) {
			continue
		}
		v := strings.Trim(strings.TrimSpace(line[len(want):]), `"`)
		if rest, ok := strings.CutPrefix(v, "$HOME"); ok {
			return filepath.Join(home, strings.TrimPrefix(rest, "/"))
		}
		if strings.HasPrefix(v, "/") {
			return v
		}
		return ""
	}
	return ""
}

// humanSize: "512 B", "6.1 kB", "18 kB", "2.3 MB" (decimal, like file managers).
func humanSize(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d B", n)
	}
	v := float64(n) / 1000
	for _, unit := range []string{"kB", "MB", "GB", "TB"} {
		if v < 999.5 || unit == "TB" {
			if v < 9.95 {
				return fmt.Sprintf("%.1f %s", v, unit)
			}
			return fmt.Sprintf("%.0f %s", v, unit)
		}
		v /= 1000
	}
	return ""
}

// when: "Today 11:40", "Yesterday", "12 Oct", "3 Sep 2024".
func when(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	t, now = t.Local(), now.Local()
	day := func(x time.Time) (int, time.Month, int) { return x.Date() }
	ty, tm, td := day(t)
	ny, nm, nd := day(now)
	yy, ym, yd := day(now.AddDate(0, 0, -1))
	switch {
	case ty == ny && tm == nm && td == nd:
		return t.Format("Today 15:04")
	case ty == yy && tm == ym && td == yd:
		return "Yesterday"
	case ty == ny && t.Before(now):
		return t.Format("2 Jan")
	}
	return t.Format("2 Jan 2006")
}

// ---- bookmarks and recent files ---------------------------------------

// Bookmarks are a JSON list of folders in the data root (bookmarks.json).
func loadBookmarks(file string) []string {
	if file == "" {
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var list []string
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return slices.DeleteFunc(list, func(p string) bool { return !filepath.IsAbs(p) })
}

func saveBookmarks(file string, list []string) error {
	return writeJSONFile(file, list)
}

// recentFile is a file used recently, newest first in its list.
type recentFile struct {
	Path string `json:"path"`
	Used int64  `json:"used"` // unix seconds
}

const maxRecent = 20

// Recent files are kept per kind ("projects", "samples") in
// recent-files.json in the data root.
func loadRecent(file, key string) []recentFile {
	all := map[string][]recentFile{}
	if data, err := os.ReadFile(file); err == nil {
		_ = json.Unmarshal(data, &all)
	}
	if all[key] == nil {
		return []recentFile{}
	}
	return all[key]
}

// addRecent puts path first in key's list.
func addRecent(file, key, path string, now time.Time) error {
	if file == "" || key == "" {
		return nil
	}
	all := map[string][]recentFile{}
	if data, err := os.ReadFile(file); err == nil {
		_ = json.Unmarshal(data, &all)
	}
	list := slices.DeleteFunc(all[key], func(r recentFile) bool { return r.Path == path })
	list = append([]recentFile{{Path: path, Used: now.Unix()}}, list...)
	if len(list) > maxRecent {
		list = list[:maxRecent]
	}
	all[key] = list
	return writeJSONFile(file, all)
}

// writeJSONFile writes atomically: a temporary file, then a rename.
func writeJSONFile(file string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(file), ".tmp-*.json")
	if err != nil {
		return err
	}
	_, werr := f.Write(append(data, '\n'))
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), file)
}
