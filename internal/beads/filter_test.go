package beads

import (
	"strings"
	"testing"
)

// --- ParseFilterClause ------------------------------------------------------

func TestParseFilterClause_Label(t *testing.T) {
	f, err := ParseFilterClause("label=subsystem:bridge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Label != "subsystem:bridge" || f.IDPrefix != "" || len(f.Any) != 0 {
		t.Fatalf("unexpected filter: %+v", f)
	}
}

func TestParseFilterClause_IDPrefix(t *testing.T) {
	f, err := ParseFilterClause("id_prefix=hk-cb-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.IDPrefix != "hk-cb-" || f.Label != "" {
		t.Fatalf("unexpected filter: %+v", f)
	}
}

func TestParseFilterClause_ValueMayContainEquals(t *testing.T) {
	f, err := ParseFilterClause("label=key=value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Label != "key=value" {
		t.Fatalf("expected value to retain trailing '=value', got %q", f.Label)
	}
}

// kerf-2km: the `any:` union input form mirrors what bootstrap-filters
// writes into spec.yaml, so an agent can copy a proposed filter and feed it
// back through `kerf work edit --bead-filter-add`. The parser returns a
// Filter with Any populated; callers (internal/spec.AddBeadFilterClause)
// expand it into the canonical mapping shape on write.
func TestParseFilterClause_AnyUnion(t *testing.T) {
	f, err := ParseFilterClause("any:label=codename:foo,label=spec:foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Label != "" || f.IDPrefix != "" {
		t.Fatalf("expected no leaf fields on union, got %+v", f)
	}
	if len(f.Any) != 2 {
		t.Fatalf("expected 2 union members, got %d (%+v)", len(f.Any), f.Any)
	}
	if f.Any[0].Label != "codename:foo" || f.Any[1].Label != "spec:foo" {
		t.Fatalf("unexpected member labels: %+v", f.Any)
	}
	// The returned union must Validate clean — it's the same shape as the
	// `any:` mapping that ends up in spec.yaml.
	if err := f.Validate(); err != nil {
		t.Fatalf("expected valid any: union, got %v", err)
	}
}

func TestParseFilterClause_AnyUnion_MixedLeafKinds(t *testing.T) {
	f, err := ParseFilterClause("any:label=subsystem:bridge,id_prefix=alpha-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Any) != 2 {
		t.Fatalf("expected 2 members, got %d", len(f.Any))
	}
	if f.Any[0].Label != "subsystem:bridge" {
		t.Fatalf("expected first to be label leaf, got %+v", f.Any[0])
	}
	if f.Any[1].IDPrefix != "alpha-" {
		t.Fatalf("expected second to be id_prefix leaf, got %+v", f.Any[1])
	}
}

func TestParseFilterClause_AnyUnion_TrimsWhitespace(t *testing.T) {
	// Whitespace around leaves and after the `any:` prefix is tolerated;
	// values themselves are kept verbatim.
	f, err := ParseFilterClause("any: label=foo , label=bar ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Any) != 2 || f.Any[0].Label != "foo" || f.Any[1].Label != "bar" {
		t.Fatalf("unexpected parse: %+v", f)
	}
}

func TestParseFilterClause_AnyUnion_Rejects(t *testing.T) {
	cases := []struct {
		in     string
		errSub string
	}{
		{"any:", "empty any: union"},
		{"any:   ", "empty any: union"},
		{"any:label=foo", "fewer than two members"},
		{"any:label=foo,", "empty leaf"},
		{"any:label=foo,bogus=bar", "unknown key"},
		{"any:label=,label=foo", "empty value"},
	}
	for _, c := range cases {
		_, err := ParseFilterClause(c.in)
		if err == nil {
			t.Errorf("expected error for %q", c.in)
			continue
		}
		if !strings.Contains(err.Error(), c.errSub) {
			t.Errorf("input %q: expected error containing %q, got %v", c.in, c.errSub, err)
		}
	}
}

func TestParseFilterClause_Rejects(t *testing.T) {
	cases := []struct {
		in     string
		errSub string
	}{
		{"", "empty input"},
		{"label", "does not parse"},
		{"all=foo", "unknown key"},
		{"=value", "unknown key"},
		{"label=", "empty value"},
		{"   ", "empty input"},
	}
	for _, c := range cases {
		_, err := ParseFilterClause(c.in)
		if err == nil {
			t.Errorf("expected error for %q", c.in)
			continue
		}
		if !strings.Contains(err.Error(), c.errSub) {
			t.Errorf("input %q: expected error containing %q, got %v", c.in, c.errSub, err)
		}
	}
}

func TestFilterClauseEquals(t *testing.T) {
	a, _ := ParseFilterClause("label=foo")
	b, _ := ParseFilterClause("label=foo")
	if !FilterClauseEquals(a, b) {
		t.Fatal("expected equal label clauses")
	}
	c, _ := ParseFilterClause("label=bar")
	if FilterClauseEquals(a, c) {
		t.Fatal("expected unequal label clauses")
	}
	d, _ := ParseFilterClause("id_prefix=foo")
	if FilterClauseEquals(a, d) {
		t.Fatal("expected label vs id_prefix to be unequal")
	}
}

// --- Validate ---------------------------------------------------------------

func TestFilter_Validate_EmptyRejected(t *testing.T) {
	f := &Filter{}
	if err := f.Validate(); err == nil {
		t.Fatal("expected empty filter to be rejected per spec")
	}
}

func TestFilter_Validate_NilRejected(t *testing.T) {
	var f *Filter
	if err := f.Validate(); err == nil {
		t.Fatal("expected nil filter to be rejected")
	}
}

func TestFilter_Validate_MixedDirectAndAnyRejected(t *testing.T) {
	f := &Filter{Label: "work:{codename}", Any: []Filter{{Label: "x"}}}
	if err := f.Validate(); err == nil {
		t.Fatal("expected direct+any mix to be rejected")
	}
	f2 := &Filter{IDPrefix: "foo-", Any: []Filter{{Label: "x"}}}
	if err := f2.Validate(); err == nil {
		t.Fatal("expected id_prefix+any mix to be rejected")
	}
}

func TestFilter_Validate_LabelAndIDPrefixMixRejected(t *testing.T) {
	f := &Filter{Label: "foo", IDPrefix: "bar-"}
	if err := f.Validate(); err == nil {
		t.Fatal("expected label+id_prefix in same clause to be rejected")
	}
}

func TestFilter_Validate_NestedAnyEmptyRejected(t *testing.T) {
	f := &Filter{Any: []Filter{{}}}
	if err := f.Validate(); err == nil {
		t.Fatal("expected nested empty clause to be rejected")
	}
}

func TestFilter_Validate_GoodCases(t *testing.T) {
	cases := []*Filter{
		{Label: "work:{codename}"},
		{IDPrefix: "{codename}-"},
		{Any: []Filter{{Label: "a"}, {IDPrefix: "b-"}}},
	}
	for i, f := range cases {
		if err := f.Validate(); err != nil {
			t.Errorf("case %d: expected valid, got %v", i, err)
		}
	}
}

// --- Match: label clause ----------------------------------------------------

func TestFilter_Match_LabelHit(t *testing.T) {
	f := &Filter{Label: "work:alpha"}
	b := Bead{ID: "x-1", Labels: []string{"work:alpha", "other"}}
	if !f.Match(b, "alpha") {
		t.Fatal("expected label match")
	}
}

func TestFilter_Match_LabelMiss(t *testing.T) {
	f := &Filter{Label: "work:alpha"}
	b := Bead{ID: "x-1", Labels: []string{"work:beta"}}
	if f.Match(b, "alpha") {
		t.Fatal("expected label miss")
	}
}

func TestFilter_Match_LabelCaseSensitive_perSpec(t *testing.T) {
	// Spec: matching is case-sensitive. "Subsystem:bridge" must NOT match
	// "subsystem:bridge".
	f := &Filter{Label: "subsystem:bridge"}
	b := Bead{ID: "x-1", Labels: []string{"Subsystem:bridge"}}
	if f.Match(b, "ignored") {
		t.Fatal("expected case-sensitive miss")
	}
	b2 := Bead{ID: "x-2", Labels: []string{"subsystem:bridge"}}
	if !f.Match(b2, "ignored") {
		t.Fatal("expected exact-case hit")
	}
}

// --- Match: id_prefix clause ------------------------------------------------

func TestFilter_Match_IDPrefixHit(t *testing.T) {
	f := &Filter{IDPrefix: "alpha-"}
	b := Bead{ID: "alpha-001"}
	if !f.Match(b, "alpha") {
		t.Fatal("expected id_prefix hit")
	}
}

func TestFilter_Match_IDPrefixMiss(t *testing.T) {
	f := &Filter{IDPrefix: "alpha-"}
	b := Bead{ID: "beta-001"}
	if f.Match(b, "alpha") {
		t.Fatal("expected id_prefix miss")
	}
}

func TestFilter_Match_IDPrefixCaseSensitive(t *testing.T) {
	f := &Filter{IDPrefix: "Alpha-"}
	b := Bead{ID: "alpha-001"}
	if f.Match(b, "alpha") {
		t.Fatal("expected case-sensitive id_prefix miss")
	}
}

// --- Match: {codename} template substitution --------------------------------

func TestFilter_Match_LabelTemplate(t *testing.T) {
	f := &Filter{Label: "work:{codename}"}
	b := Bead{ID: "x-1", Labels: []string{"work:gamma"}}
	if !f.Match(b, "gamma") {
		t.Fatal("expected template-substituted label match")
	}
	if f.Match(b, "delta") {
		t.Fatal("expected template substitution to bind to provided codename")
	}
}

func TestFilter_Match_IDPrefixTemplate(t *testing.T) {
	f := &Filter{IDPrefix: "{codename}-"}
	b := Bead{ID: "gamma-007"}
	if !f.Match(b, "gamma") {
		t.Fatal("expected template-substituted id_prefix match")
	}
	if f.Match(b, "delta") {
		t.Fatal("expected non-matching codename to miss")
	}
}

// --- Match: any: union ------------------------------------------------------

func TestFilter_Match_AnyUnion_OneMatches(t *testing.T) {
	f := &Filter{Any: []Filter{
		{Label: "work:{codename}"},
		{IDPrefix: "{codename}-"},
	}}
	// Matches via id_prefix even though label doesn't match.
	b := Bead{ID: "alpha-001", Labels: []string{"unrelated"}}
	if !f.Match(b, "alpha") {
		t.Fatal("expected any: to match when one clause matches (id_prefix)")
	}
	// Matches via label even though id_prefix doesn't match.
	b2 := Bead{ID: "zzz-001", Labels: []string{"work:alpha"}}
	if !f.Match(b2, "alpha") {
		t.Fatal("expected any: to match when one clause matches (label)")
	}
}

func TestFilter_Match_AnyUnion_NoneMatch(t *testing.T) {
	f := &Filter{Any: []Filter{
		{Label: "work:{codename}"},
		{IDPrefix: "{codename}-"},
	}}
	b := Bead{ID: "zzz-001", Labels: []string{"unrelated"}}
	if f.Match(b, "alpha") {
		t.Fatal("expected any: to miss when no clause matches")
	}
}

// --- Match: empty filter matches nothing -----------------------------------

func TestFilter_Match_EmptyMatchesNothing(t *testing.T) {
	// Per spec, an empty filter is invalid; if one slips past Validate,
	// Match must not match anything.
	f := &Filter{}
	b := Bead{ID: "x-1", Labels: []string{"work:alpha"}}
	if f.Match(b, "alpha") {
		t.Fatal("expected empty filter to match nothing")
	}
}

func TestFilter_Match_NilReceiverMatchesNothing(t *testing.T) {
	var f *Filter
	b := Bead{ID: "x-1", Labels: []string{"work:alpha"}}
	if f.Match(b, "alpha") {
		t.Fatal("expected nil filter to match nothing")
	}
}

// --- Resolve precedence -----------------------------------------------------

func TestResolve_PerWorkWinsOverProject(t *testing.T) {
	pw := &Filter{Label: "per-work"}
	pr := &Filter{Label: "project"}
	got := Resolve(pw, pr)
	if got != pw {
		t.Fatal("expected per-work filter to win")
	}
}

func TestResolve_ProjectWinsOverDefault(t *testing.T) {
	pr := &Filter{Label: "project"}
	got := Resolve(nil, pr)
	if got != pr {
		t.Fatal("expected project filter when no per-work is set")
	}
}

func TestResolve_DefaultWhenBothNil(t *testing.T) {
	got := Resolve(nil, nil)
	if got == nil || got.Label != "work:{codename}" {
		t.Fatalf("expected built-in default work:{codename}, got %+v", got)
	}
}

// Zero-match resolution: project filter that matches no beads is returned,
// not walked to the default.
func TestResolve_ProjectZeroMatchDoesNotFallThrough(t *testing.T) {
	project := &Filter{Label: "subsystem:bridge"}
	got := Resolve(nil, project)
	if got != project {
		t.Fatal("expected zero-matching project filter to be returned unchanged")
	}
	beads := []Bead{
		{ID: "x-1", Labels: []string{"work:alpha"}},
		{ID: "x-2", Labels: []string{"unrelated"}},
	}
	out := ForWorkWithFilter(beads, "alpha", got)
	if len(out) != 0 {
		t.Fatalf("expected zero matches, got %d", len(out))
	}
}

// --- ForWorkWithFilter ------------------------------------------------------

func TestForWorkWithFilter_AppliesResolved(t *testing.T) {
	all := []Bead{
		{ID: "alpha-001", Labels: []string{"work:alpha"}},
		{ID: "beta-001", Labels: []string{"work:beta"}},
		{ID: "gamma-001", Labels: []string{"work:gamma", "subsystem:bridge"}},
	}
	f := &Filter{Label: "subsystem:bridge"}
	got := ForWorkWithFilter(all, "alpha", f)
	if len(got) != 1 || got[0].ID != "gamma-001" {
		t.Fatalf("expected 1 match via configured filter, got %+v", got)
	}
}

func TestForWorkWithFilter_NilFilterMatchesNone(t *testing.T) {
	all := []Bead{{ID: "x", Labels: []string{"work:alpha"}}}
	got := ForWorkWithFilter(all, "alpha", nil)
	if got != nil {
		t.Fatalf("expected nil for nil filter, got %+v", got)
	}
}

// Multi-work matches: a single bead that matches N filters appears in each
// N output sets (each call independent). Demonstrates a bead "counts for
// each" matching work.
func TestForWorkWithFilter_MultiWorkMatches(t *testing.T) {
	shared := Bead{ID: "x-1", Labels: []string{"work:alpha", "work:beta"}}
	all := []Bead{shared}

	fa := &Filter{Label: "work:{codename}"}
	gotA := ForWorkWithFilter(all, "alpha", fa)
	if len(gotA) != 1 {
		t.Fatalf("expected shared bead in work:alpha, got %d", len(gotA))
	}
	gotB := ForWorkWithFilter(all, "beta", fa)
	if len(gotB) != 1 {
		t.Fatalf("expected shared bead in work:beta, got %d", len(gotB))
	}
}

// --- ForWork back-compat ----------------------------------------------------

// The back-compat wrapper's default-filter behavior matches the legacy
// implementation (case-insensitive against "work:<codename>"). Spec-conformant
// case-sensitive matching is available via ForWorkWithFilter + DefaultFilter.
func TestForWork_BackCompat_DefaultBehavior(t *testing.T) {
	all := []Bead{
		{ID: "x-1", Labels: []string{"work:alpha"}},
		{ID: "x-2", Labels: []string{"Work:Alpha"}}, // case-insensitive hit (legacy)
		{ID: "x-3", Labels: []string{"work:beta"}},
		{ID: "x-4", Labels: []string{"unrelated"}},
	}
	got := ForWork(all, "alpha")
	if len(got) != 2 {
		t.Fatalf("expected 2 hits (legacy case-insensitive), got %d", len(got))
	}
}

func TestForWorkWithFilter_DefaultFilter_IsCaseSensitivePerSpec(t *testing.T) {
	// When callers explicitly use the resolved default filter, the spec's
	// case-sensitive rule applies (the legacy back-compat wrapper is the
	// only intentional exception).
	all := []Bead{
		{ID: "x-1", Labels: []string{"work:alpha"}},
		{ID: "x-2", Labels: []string{"Work:Alpha"}},
	}
	got := ForWorkWithFilter(all, "alpha", Resolve(nil, nil))
	if len(got) != 1 || got[0].ID != "x-1" {
		t.Fatalf("expected only case-exact hit, got %+v", got)
	}
}
