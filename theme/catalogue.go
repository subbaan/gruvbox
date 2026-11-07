package theme

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
)

type Catalogue struct {
	Revision string   `json:"revision"`
	Schemes  []string `json:"schemes"`
}

func (l Library) CachedCatalogue() (Catalogue, error) {
	data, err := readLimited(filepath.Join(l.Dir, "catalogue", "index.json"), 4*1024*1024)
	if err != nil {
		return Catalogue{}, err
	}
	var catalogue Catalogue
	err = json.Unmarshal(data, &catalogue)
	return catalogue, err
}
func (l Library) RefreshCatalogue(ctx context.Context, client *http.Client) (Catalogue, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/tinted-theming/schemes/git/trees/spec-0.11?recursive=1", nil)
	if err != nil {
		return Catalogue{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Gruvbox-Theme-Library")
	response, err := client.Do(req)
	if err != nil {
		return Catalogue{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Catalogue{}, fmt.Errorf("GitHub catalogue returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
	if err != nil {
		return Catalogue{}, err
	}
	if len(data) > 4*1024*1024 {
		return Catalogue{}, fmt.Errorf("catalogue response is too large")
	}
	var tree struct {
		SHA       string `json:"sha"`
		Truncated bool   `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(data, &tree); err != nil {
		return Catalogue{}, err
	}
	if tree.Truncated || tree.SHA == "" {
		return Catalogue{}, fmt.Errorf("GitHub returned an incomplete catalogue")
	}
	catalogue := Catalogue{Revision: tree.SHA}
	for _, entry := range tree.Tree {
		if entry.Type != "blob" || !strings.HasPrefix(entry.Path, "base16/") || !strings.HasSuffix(entry.Path, ".yaml") {
			continue
		}
		slug := strings.TrimSuffix(strings.TrimPrefix(entry.Path, "base16/"), ".yaml")
		if validID.MatchString(slug) && len(slug) <= 73 {
			catalogue.Schemes = append(catalogue.Schemes, slug)
		}
	}
	if len(catalogue.Schemes) == 0 {
		return Catalogue{}, fmt.Errorf("catalogue contains no Base16 schemes")
	}
	sort.Strings(catalogue.Schemes)
	if err := ctx.Err(); err != nil {
		return Catalogue{}, err
	}
	return catalogue, writeJSON(filepath.Join(l.Dir, "catalogue", "index.json"), catalogue)
}
