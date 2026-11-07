package desktop

import (
	"net/url"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/config"
)

func TestAboutCardShowsVersionAndOpensKofi(t *testing.T) {
	u, _ := redesignUI(t)
	u.showAbout()
	m := u.modals[aboutTitle]
	if m == nil {
		t.Fatal("About card did not open")
	}
	var labels []string
	var support *widget.Button
	var link *widget.Hyperlink
	buttons := 0
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch o := obj.(type) {
		case *widget.Label:
			labels = append(labels, o.Text)
		case *widget.Button:
			buttons++
			if strings.Contains(o.Text, "Ko-fi") {
				support = o
			}
		case *widget.Hyperlink:
			link = o
		case *fyne.Container:
			for _, child := range o.Objects {
				walk(child)
			}
		}
	}
	walk(m.pop.Content)
	if !strings.Contains(strings.Join(labels, "\n"), "v"+config.Version) {
		t.Fatalf("About card does not show the version: %q", labels)
	}
	if support == nil || link == nil || link.URL.String() != KofiURL {
		t.Fatalf("About card lacks the Ko-fi button or link (link %v)", link)
	}
	if buttons != 2 {
		t.Fatalf("an information card should have Support and Close only, got %d buttons", buttons)
	}
	var opened []string
	saved := openURL
	openURL = func(link *url.URL) error { opened = append(opened, link.String()); return nil }
	defer func() { openURL = saved }()
	u.key(fyne.KeyReturn) // Enter closes; it must not open a browser
	if len(opened) != 0 || u.modals[aboutTitle] != nil || len(u.modalOrder) != 0 {
		t.Fatalf("Enter should only close the About card (opened %v)", opened)
	}
	u.showAbout()
	support = nil
	walk(u.modals[aboutTitle].pop.Content)
	test.Tap(support)
	if len(opened) != 1 || opened[0] != KofiURL {
		t.Fatalf("Support button opened %v, want %s", opened, KofiURL)
	}
	u.key(fyne.KeyEscape)
	if u.modals[aboutTitle] != nil {
		t.Fatal("Escape did not close the About card")
	}
}
