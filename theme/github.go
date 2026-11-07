package theme

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const GitHubSource = "https://github.com/tinted-theming/schemes"

// Download fetches a named scheme from the verified Base16 source. Callers
// supply a timeout/cancellable context; this is control-thread work only.
func (l Library) Download(ctx context.Context, client *http.Client, slug string) (Entry, error) {
	if !validID.MatchString(slug) || len(slug) > 73 {
		return Entry{}, fmt.Errorf("invalid GitHub scheme name %q", slug)
	}
	url := "https://api.github.com/repos/tinted-theming/schemes/contents/base16/" + slug + ".yaml?ref=spec-0.11"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Entry{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Gruvbox-Theme-Library")
	resp, err := client.Do(req)
	if err != nil {
		return Entry{}, fmt.Errorf("download theme: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Entry{}, fmt.Errorf("GitHub returned HTTP %d for %s (check name, connectivity or API rate limit)", resp.StatusCode, slug)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024+1))
	if err != nil {
		return Entry{}, err
	}
	if len(data) > 256*1024 {
		return Entry{}, fmt.Errorf("GitHub response is too large")
	}
	var file struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		SHA      string `json:"sha"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return Entry{}, err
	}
	if file.Encoding != "base64" || file.SHA == "" {
		return Entry{}, fmt.Errorf("GitHub did not return a versioned theme file")
	}
	yaml, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
	if err != nil {
		return Entry{}, err
	}
	scheme, err := Parse(yaml)
	if err != nil {
		return Entry{}, err
	}
	entry := Entry{ID: "github-" + slug, Scheme: scheme, Source: GitHubSource + "/blob/spec-0.11/base16/" + slug + ".yaml", Revision: file.SHA}
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	return entry, l.Save(entry)
}
