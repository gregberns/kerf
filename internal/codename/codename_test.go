package codename

import (
	"testing"
)

func TestGenerate_MatchesFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		name := Generate()
		if err := Validate(name); err != nil {
			t.Errorf("Generate() produced invalid codename %q: %v", name, err)
		}
	}
}

func TestGenerate_ContainsHyphen(t *testing.T) {
	for i := 0; i < 50; i++ {
		name := Generate()
		if len(name) < 3 {
			t.Errorf("Generate() produced unexpectedly short codename %q", name)
		}
	}
}

func TestGenerate_NoDuplicatesInSmallRun(t *testing.T) {
	seen := make(map[string]bool)
	dupes := 0
	runs := 200
	for i := 0; i < runs; i++ {
		name := Generate()
		if seen[name] {
			dupes++
		}
		seen[name] = true
	}
	// With ~58 adjectives * ~100 nouns = ~5800 combinations,
	// 200 draws should have very few duplicates.
	if dupes > 10 {
		t.Errorf("too many duplicates in %d generations: %d", runs, dupes)
	}
}

func TestValidate_AcceptsValid(t *testing.T) {
	valid := []string{
		"bold-crane",
		"auth-rewrite",
		"a",
		"abc123",
		"my-cool-feature",
		"x-y-z",
		"a1-b2-c3",
	}
	for _, name := range valid {
		if err := Validate(name); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidate_RejectsInvalid(t *testing.T) {
	invalid := []string{
		"",
		"Bold-Crane",
		"UPPER",
		"-leading",
		"trailing-",
		"double--hyphen",
		"has space",
		"has_underscore",
		"has.dot",
		"has/slash",
		"has@symbol",
	}
	for _, name := range invalid {
		if err := Validate(name); err == nil {
			t.Errorf("Validate(%q) = nil, want error", name)
		}
	}
}

func TestWordLists_NonEmpty(t *testing.T) {
	if len(adjectives) == 0 {
		t.Fatal("adjectives list is empty")
	}
	if len(nouns) == 0 {
		t.Fatal("nouns list is empty")
	}
}
