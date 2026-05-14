package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSymlink_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := EnsureSymlink(link, target); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("link is not a symlink")
	}
}

func TestEnsureSymlink_IdempotentWhenMatching(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := EnsureSymlink(link, target); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSymlink(link, target); err != nil {
		t.Fatalf("second EnsureSymlink should be no-op: %v", err)
	}
}

func TestEnsureSymlink_ErrorWhenDifferentTarget(t *testing.T) {
	dir := t.TempDir()
	t1 := filepath.Join(dir, "t1")
	t2 := filepath.Join(dir, "t2")
	for _, p := range []string{t1, t2} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(dir, "link")
	if err := EnsureSymlink(link, t1); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSymlink(link, t2); err == nil {
		t.Fatal("expected error pointing to different target")
	}
}

func TestEnsureSymlink_ErrorWhenPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.MkdirAll(link, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSymlink(link, target); err == nil {
		t.Fatal("expected error when path is a real directory")
	}
}

func TestIsStaleSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "gone")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := EnsureSymlink(link, target); err != nil {
		t.Fatal(err)
	}

	stale, err := IsStaleSymlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Fatal("expected fresh symlink")
	}

	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}
	stale, err = IsStaleSymlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Fatal("expected stale after target removed")
	}
}
