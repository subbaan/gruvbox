package theme

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestCatalogueOfflineCacheAndFailedRefresh(t *testing.T) {
	library := Library{Dir: t.TempDir()}
	body := `{"sha":"revision","tree":[{"type":"blob","path":"base16/warm.yaml"},{"type":"blob","path":"base24/ignore.yaml"},{"type":"blob","path":"base16/../invalid.yaml"},{"type":"blob","path":"base16/light.yaml"}]}`
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/repos/tinted-theming/schemes/git/trees/spec-0.11" || req.URL.Query().Get("recursive") != "1" {
			t.Fatalf("unexpected catalogue endpoint: %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	catalogue, err := library.RefreshCatalogue(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(catalogue.Schemes, []string{"light", "warm"}) {
		t.Fatal("catalogue did not filter invalid or non-Base16 files")
	}
	body = `{"sha":"new","truncated":true}`
	if _, err := library.RefreshCatalogue(context.Background(), client); err == nil {
		t.Fatal("incomplete catalogue accepted")
	}
	cached, err := library.CachedCatalogue()
	if err != nil || !reflect.DeepEqual(cached, catalogue) {
		t.Fatal("failed refresh changed offline catalogue")
	}
	if entries, err := library.List(); err != nil || len(entries) != 2 {
		t.Fatal("catalogue cache polluted installed theme list")
	}
}
