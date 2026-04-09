package codename

import (
	"testing"
)

func FuzzValidate(f *testing.F) {
	// Seed corpus.
	f.Add("")
	f.Add("bold-crane")
	f.Add("a")
	f.Add("auth-rewrite")
	f.Add("abc123")
	f.Add("-leading")
	f.Add("trailing-")
	f.Add("double--hyphen")
	f.Add("UPPER")
	f.Add("has space")
	f.Add("has/slash")
	f.Add("../../../etc/passwd")
	f.Add("foo\x00bar")
	f.Add("café")
	f.Add("日本語")
	f.Add("a-b-c-d-e-f-g")

	f.Fuzz(func(t *testing.T, input string) {
		// Validate should never panic.
		err := Validate(input)

		// If validation passes, verify the name actually matches the format.
		if err == nil {
			if input == "" {
				t.Error("empty string should not validate")
			}
			// Re-check: must only contain [a-z0-9-] and match the pattern.
			for _, c := range input {
				if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
					t.Errorf("validated string %q contains invalid char %q", input, string(c))
				}
			}
			if input[0] == '-' || input[len(input)-1] == '-' {
				t.Errorf("validated string %q starts or ends with hyphen", input)
			}
		}
	})
}
