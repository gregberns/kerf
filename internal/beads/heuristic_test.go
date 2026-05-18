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

// kerf-yxl: empty store returns ConfidenceNone — the detector stays silent.
func TestDetectFilterPrefixConfidence_EmptyCorpus(t *testing.T) {
	prefix, score, top, conf := DetectFilterPrefixConfidence(nil, []string{"auth"})
	if conf != ConfidenceNone {
		t.Errorf("empty corpus: want ConfidenceNone, got %v", conf)
	}
	if prefix != "" || score != 0 || top != nil {
		t.Errorf("empty corpus: want zero result, got prefix=%q score=%v top=%+v", prefix, score, top)
	}
}

// kerf-yxl: a single bead (even at 100%) must NOT clear the count floor —
// Plan 016 Open Q 2, silent path.
func TestDetectFilterPrefixConfidence_OneBeadNotConfident(t *testing.T) {
	all := []Bead{mkBead("a", "subsystem:auth")}
	prefix, _, _, conf := DetectFilterPrefixConfidence(all, []string{"auth"})
	if conf == ConfidenceConfident {
		t.Errorf("1-bead 100%% corpus must not be confident; got %v prefix=%q", conf, prefix)
	}
	if prefix != "" {
		t.Errorf("1-bead corpus: want empty prefix, got %q", prefix)
	}
}

// kerf-yxl: a mixed-prefix corpus with no dominant codename correlation
// produces ConfidenceLow (count floor met, score floor not met).
func TestDetectFilterPrefixConfidence_MixedNoMatchIsLow(t *testing.T) {
	all := []Bead{
		mkBead("1", "subsystem:x"),
		mkBead("2", "subsystem:y"),
		mkBead("3", "subsystem:z"),
	}
	prefix, _, top, conf := DetectFilterPrefixConfidence(all, []string{"unrelated"})
	if conf != ConfidenceLow {
		t.Errorf("mixed-prefix no-match: want ConfidenceLow, got %v", conf)
	}
	if prefix != "" {
		t.Errorf("expected silent (empty prefix), got %q", prefix)
	}
	if len(top) == 0 {
		t.Errorf("expected top-by-count to be populated for diagnostics")
	}
}

// kerf-yxl: a dominant prefix above both floors is reported as confident.
func TestDetectFilterPrefixConfidence_DominantIsConfident(t *testing.T) {
	all := []Bead{
		mkBead("1", "subsystem:auth"),
		mkBead("2", "subsystem:db"),
		mkBead("3", "subsystem:api"),
	}
	prefix, _, _, conf := DetectFilterPrefixConfidence(all, []string{"auth", "db", "api"})
	if conf != ConfidenceConfident {
		t.Errorf("dominant: want ConfidenceConfident, got %v", conf)
	}
	if prefix != "subsystem" {
		t.Errorf("want subsystem, got %q", prefix)
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
