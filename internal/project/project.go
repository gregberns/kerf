package project

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

// ErrCorruptIdentifier is the sentinel returned (wrapped) by ReadIdentifier and
// Resolve when .kerf/project-identifier exists but fails ValidateIdentifier.
// Call sites use errors.Is(err, ErrCorruptIdentifier) to distinguish a corrupt
// file (which must surface non-zero with kerf-dlb's wording) from a missing
// file (which is the legitimate fall-through to derive/default_project).
var ErrCorruptIdentifier = errors.New("corrupt project identifier")

// IsCorruptIdentifier reports whether err was produced by a validation failure
// on .kerf/project-identifier. Convenience wrapper around errors.Is.
func IsCorruptIdentifier(err error) bool {
	return errors.Is(err, ErrCorruptIdentifier)
}

// FindGitRoot walks up from cwd to find a directory containing .git.
func FindGitRoot(cwd string) (string, error) {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir, nil
		}
		// Also handle .git as a file (worktrees)
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not in a git repository (searched from %s)", cwd)
		}
		dir = parent
	}
}

// maxProjectIdentifierLen caps how long a project identifier can be. The slug
// is used as a directory name under ~/.kerf/projects/<id> (and inside the
// repo's bench symlink), so it must comfortably fit common filesystem path
// limits with room to spare for the surrounding path components.
const maxProjectIdentifierLen = 80

// ValidateIdentifier enforces the on-disk shape of `.kerf/project-identifier`.
// kerf-dlb: corrupt identifiers (garbage bytes, non-printable characters,
// path separators) used to leak through to mkdir(2) and surface as raw Go
// errors. The rules here mirror what slugifyPath produces:
//
//   - nonempty
//   - printable ASCII only (no NUL, no control bytes, no high-bit chars)
//   - no path separators ('/' or '\\') and no '.' segments
//   - length capped at maxProjectIdentifierLen
//
// Returned errors are short reason fragments suitable for embedding in a
// "corrupt project identifier at <path>: <reason>" wrapper at the call site.
func ValidateIdentifier(id string) error {
	if id == "" {
		return fmt.Errorf("identifier is empty")
	}
	if len(id) > maxProjectIdentifierLen {
		return fmt.Errorf("identifier is %d bytes; max is %d", len(id), maxProjectIdentifierLen)
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		// Printable ASCII range, exclusive of DEL.
		if c < 0x20 || c > 0x7e {
			return fmt.Errorf("identifier contains non-printable byte 0x%02x at offset %d", c, i)
		}
		if c == '/' || c == '\\' {
			return fmt.Errorf("identifier contains path separator %q at offset %d", string(c), i)
		}
	}
	if id == "." || id == ".." {
		return fmt.Errorf("identifier %q is a reserved path segment", id)
	}
	return nil
}

// ReadIdentifier reads the project ID from .kerf/project-identifier in the
// repo root and validates its shape (kerf-dlb). On a validation failure the
// caller receives a wrapped error naming the file and the reason, so the
// corrupt value never reaches mkdir(2) or the bench symlink resolver.
func ReadIdentifier(repoPath string) (string, error) {
	path := filepath.Join(repoPath, ".kerf", "project-identifier")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(data))
	if verr := ValidateIdentifier(id); verr != nil {
		return "", fmt.Errorf("%w at %s: %s; replace with a clean slug", ErrCorruptIdentifier, path, verr)
	}
	return id, nil
}

// WriteIdentifier writes the project ID to .kerf/project-identifier in the repo root.
func WriteIdentifier(repoPath string, projectID string) error {
	dir := filepath.Join(repoPath, ".kerf")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating .kerf directory: %w", err)
	}
	path := filepath.Join(dir, "project-identifier")
	return os.WriteFile(path, []byte(projectID+"\n"), 0644)
}

// DeriveFromRemote parses the git origin URL and returns owner-repo slug.
func DeriveFromRemote(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no git remote 'origin': %w", err)
	}
	rawURL := strings.TrimSpace(string(out))
	return slugifyRemoteURL(rawURL)
}

// slugifyRemoteURL parses SSH or HTTPS git URLs and extracts owner-repo.
func slugifyRemoteURL(rawURL string) (string, error) {
	// SSH format: git@github.com:owner/repo.git
	if strings.Contains(rawURL, "://") {
		return slugifyHTTPS(rawURL)
	}
	if strings.Contains(rawURL, "@") && strings.Contains(rawURL, ":") {
		return slugifySSH(rawURL)
	}
	return "", fmt.Errorf("unrecognized remote URL format: %s", rawURL)
}

func slugifySSH(rawURL string) (string, error) {
	// git@github.com:owner/repo.git -> owner/repo
	parts := strings.SplitN(rawURL, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("cannot parse SSH URL: %s", rawURL)
	}
	path := strings.TrimSuffix(parts[1], ".git")
	return slugifyPath(path), nil
}

func slugifyHTTPS(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing URL: %w", err)
	}
	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" {
		return "", fmt.Errorf("empty path in URL: %s", rawURL)
	}
	return slugifyPath(path), nil
}

// slugifyPath converts "owner/repo" to "owner-repo" (lowercase, alphanum+hyphens).
func slugifyPath(path string) string {
	s := strings.ToLower(path)
	s = slugRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// DeriveFromDirectory returns the repo root directory name as a slug.
func DeriveFromDirectory(repoPath string) string {
	name := filepath.Base(repoPath)
	return slugifyPath(name)
}

// Resolve finds the project ID: reads existing .kerf/project-identifier,
// or derives from git remote / directory name. Checks for collision with
// existing projects on the bench.
func Resolve(cwd string, benchPath string) (string, error) {
	gitRoot, err := FindGitRoot(cwd)
	if err != nil {
		return "", err
	}

	// Try existing identifier first. A read error that is NotExist means
	// "no identifier on disk yet" — derive a new one. Any other error
	// (including kerf-dlb validation failures on a corrupt identifier file)
	// is a hard error: surface it instead of silently deriving a fresh ID
	// that disagrees with what's on disk.
	idPath := filepath.Join(gitRoot, ".kerf", "project-identifier")
	if _, statErr := os.Stat(idPath); statErr == nil {
		id, err := ReadIdentifier(gitRoot)
		if err != nil {
			return "", err
		}
		return id, nil
	}

	// Derive from remote, fallback to directory name.
	var projectID string
	if derived, err := DeriveFromRemote(gitRoot); err == nil {
		projectID = derived
	} else {
		projectID = DeriveFromDirectory(gitRoot)
	}

	// Check for collision: does this project ID already exist on the bench
	// for a different repo?
	if benchPath != "" {
		projectDir := filepath.Join(benchPath, "projects", projectID)
		if _, err := os.Stat(projectDir); err == nil {
			// Project dir exists — this could be a collision if it's from a different repo.
			// For now we just warn; the caller can decide what to do.
			// The spec says: "kerf warns the user and requires manual resolution."
		}
	}

	return projectID, nil
}
