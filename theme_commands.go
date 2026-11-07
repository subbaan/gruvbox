package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/subbaan/gruvbox/config"
	"github.com/subbaan/gruvbox/theme"
	"github.com/subbaan/gruvbox/ui"
)

type themeFlags struct {
	dir, selectID, importPath, download *string
	list                                *bool
}

func registerThemeFlags() themeFlags {
	return themeFlags{
		dir:        flag.String("theme-dir", filepath.Join(config.BaseDir, "themes"), "Theme library and appearance settings directory"),
		selectID:   flag.String("theme", "", "Save a theme selection by ID and exit"),
		importPath: flag.String("theme-import", "", "Import a Base16 YAML file and exit"),
		download:   flag.String("theme-download", "", "Download a named Tinted Theming Base16 scheme from GitHub and exit"),
		list:       flag.Bool("theme-list", false, "List bundled and installed themes and exit"),
	}
}

func configureThemes(opts themeFlags) (bool, error) {
	library := theme.Library{Dir: *opts.dir}
	actions := 0
	for _, enabled := range []bool{*opts.list, *opts.selectID != "", *opts.importPath != "", *opts.download != ""} {
		if enabled {
			actions++
		}
	}
	if actions > 1 {
		return true, fmt.Errorf("use one theme action at a time")
	}
	switch {
	case *opts.list:
		entries, err := library.List()
		if err != nil {
			return true, err
		}
		for _, entry := range entries {
			fmt.Printf("%-32s %-6s %s\n", entry.ID, entry.Scheme.Variant, entry.Scheme.Name)
		}
		return true, nil
	case *opts.selectID != "":
		if err := library.Select(*opts.selectID); err != nil {
			return true, err
		}
		fmt.Printf("Selected %s. It will be used next time Gruvbox starts.\n", *opts.selectID)
		return true, nil
	case *opts.importPath != "":
		name := filepath.Base(*opts.importPath)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		id := regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(strings.ToLower(name), "-")
		id = strings.Trim(id, "-_")
		entry, err := library.Import(id, *opts.importPath)
		if err != nil {
			return true, err
		}
		fmt.Printf("Imported %s (%s). Select with --theme %s\n", entry.Scheme.Name, entry.Scheme.Variant, entry.ID)
		return true, nil
	case *opts.download != "":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		entry, err := library.Download(ctx, &http.Client{Timeout: 30 * time.Second}, *opts.download)
		if err != nil {
			return true, err
		}
		fmt.Printf("Downloaded %s (%s). Select with --theme %s\n", entry.Scheme.Name, entry.Scheme.Variant, entry.ID)
		return true, nil
	}
	entry, err := library.Selected()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot load selected theme: %v; using Warm Dark.\n", err)
		entry, _ = library.Load("warm-dark")
	}
	ui.ApplyTheme(entry.Scheme)
	return false, nil
}
