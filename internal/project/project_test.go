package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSlugifyRemoteURL_SSH(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"git@github.com:acme/webapp.git", "acme-webapp"},
		{"git@github.com:acme/webapp", "acme-webapp"},
		{"git@gitlab.com:org/sub/repo.git", "org-sub-repo"},
	}
	for _, tt := range tests {
		got, err := slugifyRemoteURL(tt.url)
		if err != nil {
			t.Errorf("slugifyRemoteURL(%q) error: %v", tt.url, err)
			continue
		}
		if got != tt.want {
			t.Errorf("slugifyRemoteURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestSlugifyRemoteURL_HTTPS(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/acme/webapp.git", "acme-webapp"},
		{"https://github.com/acme/webapp", "acme-webapp"},
		{"https://gitlab.com/org/sub/repo.git", "org-sub-repo"},
	}
	for _, tt := range tests {
		got, err := slugifyRemoteURL(tt.url)
		if err != nil {
			t.Errorf("slugifyRemoteURL(%q) error: %v", tt.url, err)
			continue
		}
		if got != tt.want {
			t.Errorf("slugifyRemoteURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestDeriveFromDirectory(t *testing.T) {
	got := DeriveFromDirectory("/home/user/My Repo")
	if got != "my-repo" {
		t.Errorf("DeriveFromDirectory = %q, want %q", got, "my-repo")
	}
}

func TestFindGitRoot(t *testing.T) {
	// Create a temp git repo.
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Subdir should still find root.
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := FindGitRoot(sub)
	if err != nil {
		t.Fatalf("FindGitRoot(%q) error: %v", sub, err)
	}
	if got != dir {
		t.Errorf("FindGitRoot(%q) = %q, want %q", sub, got, dir)
	}
}

func TestFindGitRoot_NotInRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := FindGitRoot(dir)
	if err == nil {
		t.Error("FindGitRoot in non-repo should error")
	}
}

func TestReadWriteIdentifier(t *testing.T) {
	dir := t.TempDir()

	// Write and read back.
	if err := WriteIdentifier(dir, "acme-webapp"); err != nil {
		t.Fatal(err)
	}
	got, err := ReadIdentifier(dir)
	if err != nil {
		t.Fatalf("ReadIdentifier: %v", err)
	}
	if got != "acme-webapp" {
		t.Errorf("ReadIdentifier = %q, want %q", got, "acme-webapp")
	}
}

func TestReadIdentifier_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadIdentifier(dir)
	if err == nil {
		t.Error("ReadIdentifier on missing file should error")
	}
}

func TestResolve_ExistingIdentifier(t *testing.T) {
	dir := t.TempDir()
	// Make it a git repo.
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatal(err)
	}
	// Write identifier.
	if err := WriteIdentifier(dir, "my-project"); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "my-project" {
		t.Errorf("Resolve = %q, want %q", got, "my-project")
	}
}

func TestResolve_DeriveFromRemote(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:acme/webapp.git").Run(); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "acme-webapp" {
		t.Errorf("Resolve = %q, want %q", got, "acme-webapp")
	}
}

func TestResolve_FallbackToDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatal(err)
	}
	// No remote — should fall back to directory name.

	got, err := Resolve(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	expected := DeriveFromDirectory(dir)
	if got != expected {
		t.Errorf("Resolve = %q, want %q", got, expected)
	}
}

func TestResolve_NotInGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := Resolve(dir, "")
	if err == nil {
		t.Error("Resolve outside git repo should error")
	}
}

func TestResolve_CollisionDetection(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:acme/webapp.git").Run(); err != nil {
		t.Fatal(err)
	}

	// Create bench with existing project dir.
	bench := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bench, "projects", "acme-webapp"), 0755); err != nil {
		t.Fatal(err)
	}

	// Should still resolve (collision is a warning, not an error per spec).
	got, err := Resolve(dir, bench)
	if err != nil {
		t.Fatal(err)
	}
	if got != "acme-webapp" {
		t.Errorf("Resolve = %q, want %q", got, "acme-webapp")
	}
}
