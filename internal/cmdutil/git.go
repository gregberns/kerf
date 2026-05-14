package cmdutil

import (
	"os/exec"
	"strings"
)

// HasUncommittedChanges reports whether the working tree at repoRoot has
// uncommitted changes (modified, added, deleted, or untracked files).
// Returns (false, nil) silently if git is unavailable, the path is not a
// repo, or the check fails for any reason — callers treat any error as
// "no signal" so the retrofit hint is non-blocking.
func HasUncommittedChanges(repoRoot string) (bool, error) {
	if repoRoot == "" {
		return false, nil
	}
	cmd := exec.Command("git", "-C", repoRoot, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// repoRootFromCwd returns the git repo root for cwd, or "" if not in a repo
// or git is unavailable. Soft failure.
func repoRootFromCwd(cwd string) string {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// RetrofitHintText is the message emitted when uncommitted changes are
// detected outside any active kerf work.
const RetrofitHintText = "Detected uncommitted changes not tracked by any active work.\nConsider: kerf new --jig retrofit"

// MaybeRetrofitHint returns the retrofit hint string if cwd is a git repo
// with uncommitted changes, otherwise "". Always soft: returns "" on any
// error or missing git. If requireActiveWork is true, the caller has
// already determined there is an active work; this function does not
// re-check.
func MaybeRetrofitHint(cwd string) string {
	root := repoRootFromCwd(cwd)
	if root == "" {
		return ""
	}
	dirty, err := HasUncommittedChanges(root)
	if err != nil || !dirty {
		return ""
	}
	return RetrofitHintText
}
