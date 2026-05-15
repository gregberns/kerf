package beads

import "testing"

func mkBead(id string, labels ...string) Bead {
	return Bead{ID: id, Labels: labels}
}

func TestDetectFilterPrefix_ZeroCodenames(t *testing.T) {
	// Beads exist, but no codenames — cannot match anything, so matchScore is 0.
	// We still return top-by-count so the fallback prompt has data.
	all := []Bead{
		mkBead("a", "subsystem:auth"),
		mkBead("b", "subsystem:db"),
		mkBead("c", "subsystem:api"),
	}
	prefix, score, top := DetectFilterPrefix(all, nil)
	if prefix != "" {
		t.Errorf("expected no winning prefix, got %q", prefix)
	}
	if score != 0 {
		t.Errorf("expected score 0, got %v", score)
	}
	if len(top) != 1 || top[0].Prefix != "subsystem" || top[0].Count != 3 {
		t.Errorf("unexpected top-by-count: %+v", top)
	}
}

func TestDetectFilterPrefix_BelowThreshold(t *testing.T) {
	// Fewer than 3 beads carry each prefix — no candidates emerge.
	all := []Bead{
		mkBead("a", "subsystem:auth"),
		mkBead("b", "epic:foo"),
	}
	prefix, score, top := DetectFilterPrefix(all, []string{"auth", "foo"})
	if prefix != "" || score != 0 || top != nil {
		t.Errorf("expected empty result, got prefix=%q score=%v top=%+v", prefix, score, top)
	}
}

func TestDetectFilterPrefix_DominantPrefixWins(t *testing.T) {
	all := []Bead{
		mkBead("1", "subsystem:auth"),
		mkBead("2", "subsystem:auth"),
		mkBead("3", "subsystem:db"),
		mkBead("4", "subsystem:db"),
		mkBead("5", "subsystem:nothing"), // does not map to a codename
	}
	codenames := []string{"auth", "db"}

	prefix, score, top := DetectFilterPrefix(all, codenames)
	if prefix != "subsystem" {
		t.Errorf("expected subsystem, got %q", prefix)
	}
	if score < 0.79 || score > 0.81 {
		t.Errorf("expected score ~0.8, got %v", score)
	}
	if len(top) == 0 || top[0].Prefix != "subsystem" {
		t.Errorf("expected subsystem in top, got %+v", top)
	}
}

func TestDetectFilterPrefix_AmbiguousFallsBackToTop(t *testing.T) {
	// Two prefixes, each with 3 beads, neither correlated with codenames.
	all := []Bead{
		mkBead("1", "subsystem:x"),
		mkBead("2", "subsystem:y"),
		mkBead("3", "subsystem:z"),
		mkBead("4", "epic:a"),
		mkBead("5", "epic:b"),
		mkBead("6", "epic:c"),
		mkBead("7", "epic:d"),
	}
	codenames := []string{"auth"} // matches none of the labels

	prefix, score, top := DetectFilterPrefix(all, codenames)
	if prefix != "" {
		t.Errorf("expected no winner, got %q (score=%v)", prefix, score)
	}
	if len(top) != 2 {
		t.Fatalf("expected 2 prefixes in top, got %+v", top)
	}
	// epic has more beads (4) than subsystem (3) — should be first.
	if top[0].Prefix != "epic" || top[0].Count != 4 {
		t.Errorf("expected epic first with 4, got %+v", top[0])
	}
	if top[1].Prefix != "subsystem" || top[1].Count != 3 {
		t.Errorf("expected subsystem second with 3, got %+v", top[1])
	}
}

func TestDetectFilterPrefix_CaseSensitive(t *testing.T) {
	// "Subsystem" and "subsystem" are different prefixes.
	all := []Bead{
		mkBead("1", "Subsystem:auth"),
		mkBead("2", "Subsystem:db"),
		mkBead("3", "subsystem:auth"),
		mkBead("4", "subsystem:db"),
		mkBead("5", "subsystem:api"),
	}
	codenames := []string{"auth", "db", "api"}

	prefix, score, _ := DetectFilterPrefix(all, codenames)
	// Lowercase "subsystem" has 3 beads, all match codenames → score 1.0.
	// Uppercase "Subsystem" has only 2 beads → below threshold.
	if prefix != "subsystem" {
		t.Errorf("expected subsystem, got %q", prefix)
	}
	if score != 1.0 {
		t.Errorf("expected score 1.0, got %v", score)
	}
}

func TestDetectFilterPrefix_DedupePerBead(t *testing.T) {
	// A bead with two "subsystem:*" labels still only counts once.
	all := []Bead{
		{ID: "1", Labels: []string{"subsystem:auth", "subsystem:extra"}},
		mkBead("2", "subsystem:db"),
		mkBead("3", "subsystem:api"),
	}
	codenames := []string{"auth", "db", "api"}
	_, score, top := DetectFilterPrefix(all, codenames)
	if len(top) == 0 || top[0].Count != 3 {
		t.Errorf("expected count 3, got %+v", top)
	}
	if score != 1.0 {
		t.Errorf("expected score 1.0, got %v", score)
	}
}
