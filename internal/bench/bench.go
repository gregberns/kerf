// Package bench manages the kerf bench directory (~/.kerf/).
package bench

import (
	"fmt"
	"os"
	"path/filepath"
)

// BenchPath returns the path to the bench directory (~/.kerf/).
func BenchPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".kerf"), nil
}

// EnsureBench creates the bench directory and projects/ subdirectory if missing.
func EnsureBench() (string, error) {
	bp, err := BenchPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(bp, "projects"), 0755); err != nil {
		return "", fmt.Errorf("creating bench: %w", err)
	}
	return bp, nil
}

// Exists returns true if the bench directory exists.
func Exists() bool {
	bp, err := BenchPath()
	if err != nil {
		return false
	}
	info, err := os.Stat(bp)
	return err == nil && info.IsDir()
}

// WorkDir returns the path to a work directory.
func WorkDir(benchPath, projectID, codename string) string {
	return filepath.Join(benchPath, "projects", projectID, codename)
}

// ArchiveDir returns the path to an archived work directory.
func ArchiveDir(benchPath, projectID, codename string) string {
	return filepath.Join(benchPath, "archive", projectID, codename)
}

// CreateWork creates the work directory.
func CreateWork(benchPath, projectID, codename string) error {
	dir := WorkDir(benchPath, projectID, codename)
	return os.MkdirAll(dir, 0755)
}

// ListWorks returns codenames of all works in a project.
func ListWorks(benchPath, projectID string) ([]string, error) {
	projectDir := filepath.Join(benchPath, "projects", projectID)
	return listDirs(projectDir)
}

// ListArchivedWorks returns codenames of archived works in a project.
func ListArchivedWorks(benchPath, projectID string) ([]string, error) {
	archiveDir := filepath.Join(benchPath, "archive", projectID)
	return listDirs(archiveDir)
}

// WorkExists returns true if a work directory exists.
func WorkExists(benchPath, projectID, codename string) bool {
	info, err := os.Stat(WorkDir(benchPath, projectID, codename))
	return err == nil && info.IsDir()
}

// IsArchived returns true if the work is in the archive.
func IsArchived(benchPath, projectID, codename string) bool {
	info, err := os.Stat(ArchiveDir(benchPath, projectID, codename))
	return err == nil && info.IsDir()
}

// MoveToArchive moves a work from projects/ to archive/.
func MoveToArchive(benchPath, projectID, codename string) error {
	src := WorkDir(benchPath, projectID, codename)
	dst := ArchiveDir(benchPath, projectID, codename)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating archive directory: %w", err)
	}
	return os.Rename(src, dst)
}

// DeleteWork removes the work directory entirely.
func DeleteWork(benchPath, projectID, codename string) error {
	dir := WorkDir(benchPath, projectID, codename)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Try archive
		dir = ArchiveDir(benchPath, projectID, codename)
	}
	return os.RemoveAll(dir)
}

// ListAllProjects returns all project IDs on the bench.
func ListAllProjects(benchPath string) ([]string, error) {
	return listDirs(filepath.Join(benchPath, "projects"))
}

func listDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
