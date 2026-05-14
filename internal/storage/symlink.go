package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureSymlink creates a symlink at linkPath pointing at target. If linkPath
// already exists and is a symlink, the existing target is checked: if it
// matches, no-op; if it differs, an error is returned. If linkPath exists and
// is not a symlink, an error is returned.
func EnsureSymlink(linkPath, target string) error {
	target, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolving target: %w", err)
	}
	if info, err := os.Lstat(linkPath); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%s exists and is not a symlink", linkPath)
		}
		existing, err := os.Readlink(linkPath)
		if err != nil {
			return fmt.Errorf("reading existing symlink: %w", err)
		}
		existingAbs, _ := filepath.Abs(existing)
		if existingAbs == target {
			return nil
		}
		return fmt.Errorf("symlink %s points to %s, expected %s", linkPath, existing, target)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking symlink path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("creating symlink (symlinks may be unsupported on this platform): %w", err)
	}
	return nil
}

// IsSymlink returns true if path exists and is a symlink.
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// IsStaleSymlink returns true if path is a symlink whose target does not
// exist. Returns (false, nil) if path is not a symlink, or the target exists.
func IsStaleSymlink(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	target, err := os.Readlink(path)
	if err != nil {
		return false, err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// SymlinkTarget returns the symlink target, or empty string if not a symlink.
func SymlinkTarget(path string) string {
	if !IsSymlink(path) {
		return ""
	}
	t, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return t
}
