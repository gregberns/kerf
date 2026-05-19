package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// kerf-dlb: corrupt project-identifier files must be rejected at read time,
// before the value can reach mkdir(2) or the bench symlink resolver. Each
// case covers a distinct corruption mode the validator must catch.
func TestValidateIdentifier_Rejects(t *testing.T) {
	cases := []struct {
		name string
		id   string
		want string
	}{
		{"empty", "", "empty"},
		{"nul byte", "ok\x00bad", "non-printable"},
		{"control char", "ok\x07bad", "non-printable"},
		{"high bit", "café", "non-printable"},
		{"unix sep", "owner/repo", "path separator"},
		{"windows sep", "owner\\repo", "path separator"},
		{"dot", ".", "reserved path segment"},
		{"dotdot", "..", "reserved path segment"},
		{"too long", strings.Repeat("a", maxProjectIdentifierLen+1), "max is"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIdentifier(tc.id)
			if err == nil {
				t.Fatalf("ValidateIdentifier(%q) = nil; want error containing %q", tc.id, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q missing %q", err.Error(), tc.want)
			}
		})
	}
}

func TestValidateIdentifier_Accepts(t *testing.T) {
	good := []string{
		"a",
		"owner-repo",
		"a1b2c3",
		"my-project-2026",
		strings.Repeat("a", maxProjectIdentifierLen),
	}
	for _, id := range good {
		if err := ValidateIdentifier(id); err != nil {
			t.Errorf("ValidateIdentifier(%q) = %v; want nil", id, err)
		}
	}
}

// ReadIdentifier wraps the validation failure with the file path and the
// "replace with a clean slug" hint so callers can surface a usable error.
func TestReadIdentifier_CorruptRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Garbage bytes including a NUL and a path separator.
	garbage := []byte("owner/\x00bad-id\n")
	if err := os.WriteFile(filepath.Join(dir, ".kerf", "project-identifier"), garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadIdentifier(dir)
	if err == nil {
		t.Fatal("ReadIdentifier on corrupt file should error")
	}
	msg := err.Error()
	for _, want := range []string{"corrupt project identifier at", "project-identifier", "replace with a clean slug"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

// Resolve must surface the corruption rather than silently deriving a fresh
// identifier from the git remote — that would produce a value disagreeing
// with what is on disk.
func TestResolve_CorruptIdentifier_Errors(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".kerf", "project-identifier"), []byte("bad\x01id"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve(dir, t.TempDir())
	if err == nil {
		t.Fatal("Resolve with corrupt identifier should error")
	}
	if !strings.Contains(err.Error(), "corrupt project identifier") {
		t.Errorf("Resolve error %q should mention corrupt identifier", err.Error())
	}
}
