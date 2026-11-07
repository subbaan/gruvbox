package config

import (
	"os"
	"path/filepath"
)

const (
	// Version is embedded at compile time
	Version = "3.0.0-alpha.52"
)

var (
	// BaseDir is the base directory for all Gruvbox data
	BaseDir string

	// ProjectsDir is where project files are saved
	ProjectsDir string

	// SamplesDir is where sample files are stored
	SamplesDir string

	// RecordingsDir is where recorded samples are saved
	RecordingsDir string

	// TrimmedDir is where trimmed samples are saved
	TrimmedDir string
)

// init initializes default paths
func init() {
	// Get user's home directory
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home not available
		home = "."
	}

	// Set default base directory
	BaseDir = filepath.Join(home, "Documents", "gruvbox")

	// Set subdirectories
	ProjectsDir = filepath.Join(BaseDir, "projects")
	SamplesDir = filepath.Join(BaseDir, "samples")
	RecordingsDir = filepath.Join(SamplesDir, "recordings")
	TrimmedDir = filepath.Join(SamplesDir, "trimmed")
}

// EnsureDirectories creates all required directories if they don't exist
func EnsureDirectories() error {
	dirs := []string{
		BaseDir,
		ProjectsDir,
		SamplesDir,
		RecordingsDir,
		TrimmedDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// SetBaseDir allows customizing the base directory (for future config panel)
func SetBaseDir(path string) error {
	BaseDir = path
	ProjectsDir = filepath.Join(BaseDir, "projects")
	SamplesDir = filepath.Join(BaseDir, "samples")
	RecordingsDir = filepath.Join(SamplesDir, "recordings")
	TrimmedDir = filepath.Join(SamplesDir, "trimmed")

	return EnsureDirectories()
}
