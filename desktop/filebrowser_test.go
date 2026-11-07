package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fileTree makes a folder with sub-folders, hidden entries and files of a
// few sizes and ages.
func fileTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"beats", "Archive", ".cache"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	for i, name := range []string{"zebra.gruvbox3", "Acid.gruvbox3", "notes.txt", ".secret", "kick.wav"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(strings.Repeat("x", 100*(i+1))), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(path, old.Add(time.Duration(i)*time.Hour), old.Add(time.Duration(i)*time.Hour))
	}
	return root
}

func testBrowser(mode fileMode, dir string, kinds ...fileKind) *fileBrowser {
	b := newFileBrowser(fileBrowserInit{mode: mode, dir: dir, kinds: append(kinds, allFiles)})
	b.read = func(dir string) {
		entries, err := readFolder(dir)
		b.loaded(dir, entries, err)
	}
	b.reload()
	return b
}

func names(b *fileBrowser) []string {
	out := []string{}
	for _, e := range b.visible() {
		out = append(out, e.Name)
	}
	return out
}

func TestFileBrowserFoldersFirstHiddenOffAndSorting(t *testing.T) {
	b := testBrowser(openFile, fileTree(t))
	b.kind = 1 // All files
	if got := strings.Join(names(b), " "); got != "Archive beats Acid.gruvbox3 kick.wav notes.txt zebra.gruvbox3" {
		t.Fatalf("listing: %s", got)
	}
	b.toggleHidden()
	if got := names(b); got[0] != ".cache" || len(got) != 8 {
		t.Fatalf("hidden: %v", got)
	}
	b.toggleHidden()
	b.sortBy(sortSize) // biggest first, folders still first
	if got := strings.Join(names(b), " "); got != "Archive beats kick.wav notes.txt Acid.gruvbox3 zebra.gruvbox3" {
		t.Fatalf("by size: %s", got)
	}
	b.sortBy(sortSize)
	if got := names(b); got[2] != "zebra.gruvbox3" {
		t.Fatalf("by size reversed: %v", got)
	}
	b.sortBy(sortModified) // newest first
	if got := names(b); got[2] != "kick.wav" {
		t.Fatalf("by date: %v", got)
	}
	b.sortBy(sortName)
}

func TestFileBrowserTypeFilterKeepsFolders(t *testing.T) {
	b := testBrowser(openFile, fileTree(t), grooveFiles)
	if got := strings.Join(names(b), " "); got != "Archive beats Acid.gruvbox3 zebra.gruvbox3" {
		t.Fatalf("projects only: %s", got)
	}
	b.setKind(1)
	if len(names(b)) != 6 {
		t.Fatalf("all files: %v", names(b))
	}
}

func TestFileBrowserTypingFiltersAndEnterOpensTheMatch(t *testing.T) {
	root := fileTree(t)
	b := testBrowser(openFile, root, grooveFiles)
	b.setName("ze")
	if got := names(b); len(got) != 1 || got[0] != "zebra.gruvbox3" {
		t.Fatalf("filter: %v", got)
	}
	if o := b.accept(); o.kind != chosen || o.path != filepath.Join(root, "zebra.gruvbox3") {
		t.Fatalf("enter: %+v", o)
	}
	// "ac" matches "Acid" by its start; the folder "Archive" holds no "ac".
	b.setName("ac")
	if e, _ := b.selected(); e.Name != "Acid.gruvbox3" {
		t.Fatalf("type-ahead picked %s", e.Name)
	}
	b.setName("nothing")
	if o := b.accept(); o.kind != warn {
		t.Fatalf("no match: %+v", o)
	}
}

func TestFileBrowserSlashGoesIntoFoldersAndUpComesBack(t *testing.T) {
	root := fileTree(t)
	b := testBrowser(openFile, root)
	b.setName("beats/")
	if b.dir != filepath.Join(root, "beats") || b.name != "" {
		t.Fatalf("slash: %s %q", b.dir, b.name)
	}
	b.goUp()
	if b.dir != root {
		t.Fatalf("up: %s", b.dir)
	}
	if e, _ := b.selected(); e.Name != "beats" {
		t.Fatalf("up should select the folder we came from, got %s", e.Name)
	}
	if !b.canBack() {
		t.Fatal("no history")
	}
	b.goBack()
	if b.dir != filepath.Join(root, "beats") || !b.canForward() {
		t.Fatalf("back: %s", b.dir)
	}
	b.goForward()
	if b.dir != root {
		t.Fatalf("forward: %s", b.dir)
	}
	// A typed path is resolved against the folder shown.
	b.setName("../" + filepath.Base(root) + "/Archive")
	if o := b.accept(); o.kind != none.kind || b.dir != filepath.Join(root, "Archive") {
		t.Fatalf("typed folder: %+v %s", o, b.dir)
	}
}

func TestFileBrowserSaveKeepsTheNameAndClicksFillIt(t *testing.T) {
	root := fileTree(t)
	b := testBrowser(saveFile, root, grooveFiles)
	b.selectPath(filepath.Join(root, "zebra.gruvbox3"))
	b.setName("")
	if o := b.accept(); o.kind != warn {
		t.Fatalf("empty name: %+v", o)
	}
	// With a folder selected, Enter on an empty name goes into it.
	b.selectPath(filepath.Join(root, "beats"))
	if b.accept(); b.dir != filepath.Join(root, "beats") {
		t.Fatalf("enter on a folder: %s", b.dir)
	}
	b.goUp()
	b.setName("new-groove")
	if len(names(b)) != 4 {
		t.Fatal("saving should not filter the list")
	}
	b.goTo(filepath.Join(root, "beats"))
	if b.name != "new-groove" {
		t.Fatal("the name was lost moving folder")
	}
	b.goUp()
	for i, e := range b.visible() {
		if e.Name == "zebra.gruvbox3" {
			b.clickRow(i)
		}
	}
	if b.name != "zebra.gruvbox3" {
		t.Fatalf("click did not fill the name: %q", b.name)
	}
	if o := b.accept(); o.kind != chosen || o.path != filepath.Join(root, "zebra.gruvbox3") {
		t.Fatalf("save: %+v", o)
	}
}

func TestFileBrowserCrumbsUseTildeAndSelectTheWayBack(t *testing.T) {
	root := fileTree(t)
	b := testBrowser(openFile, filepath.Join(root, "beats"))
	b.home = root
	list := b.crumbs()
	if len(list) != 2 || list[0].label != "~" || list[1].label != "beats" {
		t.Fatalf("crumbs: %+v", list)
	}
	b.goCrumb(0)
	if e, _ := b.selected(); b.dir != root || e.Name != "beats" {
		t.Fatalf("crumb: %s %s", b.dir, e.Name)
	}
	b.home = "/nowhere"
	if list := b.crumbs(); list[0].label != "/" {
		t.Fatalf("outside home: %+v", list)
	}
}

func TestFileBrowserBookmarksRoundTripAndShowAsPlaces(t *testing.T) {
	root := fileTree(t)
	file := filepath.Join(t.TempDir(), "bookmarks.json")
	b := testBrowser(openFile, filepath.Join(root, "beats"))
	b.bookmarksFile = file
	if o := b.toggleBookmark(); o.kind != info || !b.isBookmarked() {
		t.Fatalf("bookmark: %+v", o)
	}
	if got := loadBookmarks(file); len(got) != 1 || got[0] != b.dir {
		t.Fatalf("saved: %v", got)
	}
	last := b.places[len(b.places)-1]
	if last.kind != placeBookmark || last.label != "beats" || b.currentPlace() != len(b.places)-1 {
		t.Fatalf("place: %+v", last)
	}
	b.goTo(root)
	b.goPlace(len(b.places) - 1)
	if b.dir != filepath.Join(root, "beats") {
		t.Fatal("bookmark place did not go there")
	}
	b.toggleBookmark()
	if b.isBookmarked() || len(loadBookmarks(file)) != 0 {
		t.Fatal("bookmark not removed")
	}
}

func TestRecentFilesAreKeptPerKindAndNeverStatted(t *testing.T) {
	file := filepath.Join(t.TempDir(), "recent.json")
	now := time.Now()
	for i := 0; i < maxRecent+5; i++ {
		addRecent(file, "projects", fmt.Sprintf("/gone/%02d.gruvbox3", i), now.Add(time.Duration(i)*time.Minute))
	}
	addRecent(file, "projects", "/gone/03.gruvbox3", now.Add(time.Hour))
	addRecent(file, "samples", "/gone/kick.wav", now)
	list := loadRecent(file, "projects")
	if len(list) != maxRecent || list[0].Path != "/gone/03.gruvbox3" || list[1].Path != "/gone/24.gruvbox3" {
		t.Fatalf("recent: %d, %v", len(list), list[:2])
	}
	if s := loadRecent(file, "samples"); len(s) != 1 {
		t.Fatalf("samples: %v", s)
	}
	// The files do not exist; the Recent place lists them anyway.
	b := newFileBrowser(fileBrowserInit{mode: openFile, dir: t.TempDir(), recent: list, kinds: []fileKind{grooveFiles, allFiles}})
	b.read = func(string) {}
	if b.places[0].kind != placeRecent {
		t.Fatal("no Recent place")
	}
	b.goPlace(0)
	if !b.recentOn || len(b.visible()) != maxRecent {
		t.Fatalf("recent list: %d", len(b.visible()))
	}
	b.setName("24")
	if o := b.accept(); o.kind != chosen || o.path != "/gone/24.gruvbox3" {
		t.Fatalf("recent pick: %+v", o)
	}
	// Saving has no Recent place.
	s := newFileBrowser(fileBrowserInit{mode: saveFile, dir: t.TempDir(), recent: list})
	if s.places[0].kind == placeRecent {
		t.Fatal("save dialog shows Recent")
	}
}

func TestFileBrowserNewFolderIsMadeAndSelected(t *testing.T) {
	root := fileTree(t)
	b := testBrowser(openFile, root)
	if o := b.createFolder("loops"); o.kind != info {
		t.Fatalf("make: %+v", o)
	}
	if e, _ := b.selected(); e.Name != "loops" || !e.Dir {
		t.Fatalf("selected %s", e.Name)
	}
	for _, bad := range []string{"a/b", "..", "loops"} {
		if o := b.createFolder(bad); o.kind != warn {
			t.Fatalf("%q: %+v", bad, o)
		}
	}
}

func TestFileBrowserDropsStaleListings(t *testing.T) {
	root := fileTree(t)
	b := testBrowser(openFile, root)
	before := len(b.entries)
	b.loaded("/elsewhere", nil, nil)
	if len(b.entries) != before {
		t.Fatal("a listing for another folder replaced this one")
	}
}

func TestFileBrowserFormats(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 999: "999 B", 1000: "1.0 kB", 6100: "6.1 kB", 18000: "18 kB", 2_300_000: "2.3 MB"} {
		if got := humanSize(n); got != want {
			t.Errorf("%d: %s, want %s", n, got, want)
		}
	}
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.Local)
	for at, want := range map[time.Time]string{
		now.Add(-time.Hour):   "Today 11:00",
		now.AddDate(0, 0, -1): "Yesterday",
		now.AddDate(0, -1, 0): "7 Oct",
		now.AddDate(-1, 0, 0): "7 Nov 2025",
		time.Time{}:           "",
	} {
		if got := when(at, now); got != want {
			t.Errorf("%v: %q, want %q", at, got, want)
		}
	}
	text := "# comment\nXDG_DESKTOP_DIR=\"$HOME/Desk top\"\nXDG_DOCUMENTS_DIR=\"/data/docs\"\n"
	if got := parseUserDir(text, "/home/me", "DESKTOP"); got != "/home/me/Desk top" {
		t.Errorf("desktop: %s", got)
	}
	if got := parseUserDir(text, "/home/me", "DOCUMENTS"); got != "/data/docs" {
		t.Errorf("documents: %s", got)
	}
}
