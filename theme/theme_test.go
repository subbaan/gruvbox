package theme

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func schemeYAML(s Scheme, modern bool) string {
	header, indent := "scheme: Test\nauthor: Tester\n", ""
	if modern {
		header, indent = "system: base16\nname: Test\nvariant: "+s.Variant+"\npalette:\n", "  "
	}
	for i, colour := range s.Palette {
		header += fmt.Sprintf("%sbase%02X: '%s'\n", indent, i, colour)
	}
	return header
}

func TestParseModernLegacyAndLight(t *testing.T) {
	for _, modern := range []bool{false, true} {
		for _, variant := range []string{"dark", "light"} {
			s := Bundled()["warm-dark"]
			if variant == "light" {
				for i := 0; i < 4; i++ {
					s.Palette[i], s.Palette[7-i] = s.Palette[7-i], s.Palette[i]
				}
				s.Variant = "light"
			}
			parsed, err := Parse([]byte(schemeYAML(s, modern)))
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Palette != s.Palette || parsed.Variant != variant {
				t.Fatalf("modern=%v variant=%s: incorrect palette or inferred variant", modern, variant)
			}
			if parsed.TextOn(s.Palette[0]) != s.Palette[7] {
				t.Fatal("background text did not use highest-contrast neutral")
			}
		}
	}
}

func TestRejectInvalidSchemes(t *testing.T) {
	valid := schemeYAML(Bundled()["warm-dark"], true)
	for _, data := range []string{
		"name: Incomplete\npalette: {}\n",
		strings.Replace(valid, "#22211f", "#nope00", 1),
		strings.Replace(valid, "system: base16", "system: base24", 1),
		strings.Replace(valid, "variant: dark", "variant: unknown", 1),
		valid + "---\nname: Second document\n",
		strings.Repeat("x", MaxSchemeBytes+1),
	} {
		if _, err := Parse([]byte(data)); err == nil {
			t.Fatal("accepted invalid scheme")
		}
	}
}

func TestLibraryPreservesSelectionAndValidImport(t *testing.T) {
	dir := t.TempDir()
	library := Library{Dir: filepath.Join(dir, "themes")}
	path := filepath.Join(dir, "example.yaml")
	if err := os.WriteFile(path, []byte(schemeYAML(Bundled()["warm-dark"], true)), 0600); err != nil {
		t.Fatal(err)
	}
	entry, err := library.Import("example", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := library.Select(entry.ID); err != nil {
		t.Fatal(err)
	}
	before, err := library.Selected()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("name: broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := library.Import("example", path); err == nil {
		t.Fatal("invalid import replaced good theme")
	}
	if err := library.Select("missing"); err == nil {
		t.Fatal("accepted missing selection")
	}
	after, err := (Library{Dir: library.Dir}).Selected()
	if err != nil || before != after {
		t.Fatalf("selection or palette changed: %v", err)
	}
	if _, err := library.Load("../outside"); err == nil {
		t.Fatal("accepted path traversal")
	}
	if err := library.Save(Entry{ID: "warm-dark", Scheme: before.Scheme}); err == nil {
		t.Fatal("allowed overwriting a bundled theme")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadRevisionAndFailedUpdate(t *testing.T) {
	library := Library{Dir: t.TempDir()}
	status := http.StatusOK
	content := schemeYAML(Bundled()["warm-dark"], true)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "api.github.com" || req.URL.Query().Get("ref") != "spec-0.11" {
			t.Fatalf("unexpected GitHub source: %s", req.URL)
		}
		body, _ := json.Marshal(map[string]string{"encoding": "base64", "sha": "file-revision", "content": base64.StdEncoding.EncodeToString([]byte(content))})
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})}
	entry, err := library.Download(context.Background(), client, "example")
	if err != nil || entry.Revision != "file-revision" || entry.ID != "github-example" {
		t.Fatalf("download or provenance: %v", err)
	}
	if err := library.Select(entry.ID); err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"http", "yaml"} {
		if failure == "http" {
			status = http.StatusForbidden
		} else {
			status, content = http.StatusOK, "name: Incomplete"
		}
		if _, err := library.Download(context.Background(), client, "example"); err == nil {
			t.Fatal("failed update was accepted")
		}
		saved, err := library.Selected()
		if err != nil || saved != entry {
			t.Fatalf("failed download changed installed theme: %v", err)
		}
	}
	if _, err := library.Download(context.Background(), client, "../example"); err == nil {
		t.Fatal("accepted invalid download name")
	}
}
