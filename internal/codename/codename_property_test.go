package codename

import (
	"strings"
	"testing"
	"testing/quick"
	"unicode/utf8"
)

func TestProperty_GeneratedNamesAlwaysValid(t *testing.T) {
	f := func() bool {
		name := Generate()
		return Validate(name) == nil
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Error(err)
	}
}

func TestProperty_GeneratedNamesAlwaysContainHyphen(t *testing.T) {
	f := func() bool {
		name := Generate()
		return strings.Contains(name, "-")
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Error(err)
	}
}

func TestProperty_ValidateRejectsUnicode(t *testing.T) {
	// Any string containing non-ASCII should be rejected.
	f := func(s string) bool {
		if !utf8.ValidString(s) {
			return true // skip invalid UTF-8
		}
		hasNonASCII := false
		for _, r := range s {
			if r > 127 {
				hasNonASCII = true
				break
			}
		}
		if !hasNonASCII {
			return true // skip pure ASCII — tested elsewhere
		}
		return Validate(s) != nil
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Error(err)
	}
}

func TestProperty_ValidateRejectsPathTraversal(t *testing.T) {
	traversals := []string{
		"../etc/passwd",
		"..%2f..%2f",
		"foo/../bar",
		"./foo",
		"foo/bar",
		"foo\\bar",
		"..",
		".",
	}
	for _, s := range traversals {
		if err := Validate(s); err == nil {
			t.Errorf("Validate(%q) should reject path traversal", s)
		}
	}
}

func TestProperty_ValidateRejectsSpecialChars(t *testing.T) {
	specials := []string{
		"foo bar",
		"foo\tbar",
		"foo\nbar",
		"foo\x00bar",
		"foo@bar",
		"foo#bar",
		"foo$bar",
		"foo%bar",
		"foo&bar",
		"foo*bar",
		"foo+bar",
		"foo=bar",
		"foo!bar",
		"foo?bar",
		"foo,bar",
		"foo;bar",
		"foo:bar",
		"foo'bar",
		"foo\"bar",
		"foo`bar",
		"foo~bar",
		"foo|bar",
		"foo<bar",
		"foo>bar",
		"foo{bar",
		"foo}bar",
		"foo[bar",
		"foo]bar",
		"foo(bar",
		"foo)bar",
	}
	for _, s := range specials {
		if err := Validate(s); err == nil {
			t.Errorf("Validate(%q) should reject special character", s)
		}
	}
}

func TestProperty_ValidateRejectsLengthExtremes(t *testing.T) {
	// Empty
	if err := Validate(""); err == nil {
		t.Error("empty string should be rejected")
	}

	// Very long string (valid chars but extreme length) — should still be accepted
	long := strings.Repeat("a", 1000)
	if err := Validate(long); err != nil {
		t.Errorf("long valid string should be accepted: %v", err)
	}
}
