package codename

import (
	_ "embed"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
)

//go:embed adjectives.txt
var adjectivesRaw string

//go:embed nouns.txt
var nounsRaw string

var (
	adjectives []string
	nouns      []string
	nameRegex  = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

func init() {
	adjectives = parseWordList(adjectivesRaw)
	nouns = parseWordList(nounsRaw)
}

func parseWordList(raw string) []string {
	var words []string
	for _, line := range strings.Split(raw, "\n") {
		w := strings.TrimSpace(line)
		if w != "" {
			words = append(words, w)
		}
	}
	return words
}

// Generate returns a random adjective-noun slug (e.g., "bold-crane").
func Generate() string {
	adj := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	return adj + "-" + noun
}

// Validate checks that a codename matches [a-z0-9]+(-[a-z0-9]+)*.
func Validate(name string) error {
	if name == "" {
		return fmt.Errorf("codename cannot be empty")
	}
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("invalid codename %q: must match [a-z0-9]+(-[a-z0-9]+)*", name)
	}
	return nil
}
