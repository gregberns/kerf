package cmdutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	run("config", "commit.gpgsign", "false")
	// Seed commit so HEAD exists.
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README")
	run("commit", "-q", "-m", "init")
}

func TestHasUncommittedChanges_Clean(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	initRepo(t, dir)
	dirty, err := HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dirty {
		t.Errorf("expected clean tree, got dirty")
	}
}

func TestHasUncommittedChanges_Dirty(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	initRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	dirty, err := HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dirty {
		t.Errorf("expected dirty tree, got clean")
	}
}

func TestHasUncommittedChanges_NotARepo(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	// Not a git repo — should soft-fail (return false with an error;
	// callers ignore both).
	dirty, _ := HasUncommittedChanges(dir)
	if dirty {
		t.Errorf("expected non-dirty for non-repo, got dirty")
	}
}

func TestHasUncommittedChanges_EmptyPath(t *testing.T) {
	dirty, err := HasUncommittedChanges("")
	if err != nil {
		t.Errorf("expected no error for empty path, got %v", err)
	}
	if dirty {
		t.Errorf("expected non-dirty for empty path, got dirty")
	}
}

func TestMaybeRetrofitHint_DirtyRepo(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	initRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	hint := MaybeRetrofitHint(dir)
	if hint == "" {
		t.Errorf("expected non-empty hint for dirty repo")
	}
}

func TestMaybeRetrofitHint_CleanRepo(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	initRepo(t, dir)
	hint := MaybeRetrofitHint(dir)
	if hint != "" {
		t.Errorf("expected empty hint for clean repo, got %q", hint)
	}
}

func TestMaybeRetrofitHint_NonRepo(t *testing.T) {
	dir := t.TempDir()
	hint := MaybeRetrofitHint(dir)
	if hint != "" {
		t.Errorf("expected empty hint for non-repo, got %q", hint)
	}
}
