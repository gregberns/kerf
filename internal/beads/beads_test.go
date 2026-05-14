package beads

import (
	"testing"
)

// --- test fixtures ---

func sampleBeads() []Bead {
	return []Bead{
		{ID: "coord-001", Title: "Setup", Status: "closed", Epic: "infra", Labels: []string{"work:alpha"}, DependsOn: nil},
		{ID: "coord-002", Title: "Design", Status: "done", Epic: "infra", Labels: []string{"work:alpha"}, DependsOn: []string{"coord-001"}},
		{ID: "coord-003", Title: "Implement API", Status: "in-progress", Epic: "api", Labels: []string{"work:alpha", "backend"}, DependsOn: []string{"coord-002"}},
		{ID: "coord-004", Title: "Implement UI", Status: "open", Epic: "ui", Labels: []string{"work:beta"}, DependsOn: []string{"coord-002"}},
		{ID: "coord-005", Title: "Integration", Status: "blocked", Epic: "api", Labels: []string{"work:alpha"}, DependsOn: []string{"coord-003", "coord-004"}},
		{ID: "coord-006", Title: "Deploy", Status: "open", Epic: "infra", Labels: []string{"work:beta"}, DependsOn: []string{"coord-005"}},
	}
}

func TestIsAvailable(t *testing.T) {
	// Just verify it does not panic. The result depends on the environment.
	_ = IsAvailable()
}

func TestParseJSON(t *testing.T) {
	input := `[
		{"id":"b-1","title":"First","status":"open","epic":"e1","labels":["work:foo"],"depends_on":["b-0"]},
		{"id":"b-2","title":"Second","status":"closed","epic":"e1","labels":[],"depends_on":[]}
	]`
	beads, err := ParseJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseJSON failed: %v", err)
	}
	if len(beads) != 2 {
		t.Fatalf("expected 2 beads, got %d", len(beads))
	}
	if beads[0].ID != "b-1" {
		t.Errorf("expected id b-1, got %s", beads[0].ID)
	}
	if beads[1].Status != "closed" {
		t.Errorf("expected status closed, got %s", beads[1].Status)
	}
	if len(beads[0].Labels) != 1 || beads[0].Labels[0] != "work:foo" {
		t.Errorf("unexpected labels: %v", beads[0].Labels)
	}
	if len(beads[0].DependsOn) != 1 || beads[0].DependsOn[0] != "b-0" {
		t.Errorf("unexpected depends_on: %v", beads[0].DependsOn)
	}
}

func TestParseJSON_Empty(t *testing.T) {
	beads, err := ParseJSON([]byte("[]"))
	if err != nil {
		t.Fatalf("ParseJSON failed: %v", err)
	}
	if len(beads) != 0 {
		t.Errorf("expected 0 beads, got %d", len(beads))
	}
}

func TestParseJSON_MissingFields(t *testing.T) {
	// br output might omit optional fields; parsing should still succeed.
	input := `[{"id":"b-1","title":"Minimal","status":"open"}]`
	beads, err := ParseJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseJSON failed: %v", err)
	}
	if beads[0].Epic != "" {
		t.Errorf("expected empty epic, got %q", beads[0].Epic)
	}
	if beads[0].Labels != nil {
		t.Errorf("expected nil labels, got %v", beads[0].Labels)
	}
}

func TestParseJSON_InvalidJSON(t *testing.T) {
	_, err := ParseJSON([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCountByStatus(t *testing.T) {
	counts := CountByStatus(sampleBeads())
	if counts["closed"] != 1 {
		t.Errorf("expected 1 closed, got %d", counts["closed"])
	}
	if counts["done"] != 1 {
		t.Errorf("expected 1 done, got %d", counts["done"])
	}
	if counts["in-progress"] != 1 {
		t.Errorf("expected 1 in-progress, got %d", counts["in-progress"])
	}
	if counts["open"] != 2 {
		t.Errorf("expected 2 open, got %d", counts["open"])
	}
	if counts["blocked"] != 1 {
		t.Errorf("expected 1 blocked, got %d", counts["blocked"])
	}
}

func TestCountByStatus_Empty(t *testing.T) {
	counts := CountByStatus(nil)
	if len(counts) != 0 {
		t.Errorf("expected empty map, got %v", counts)
	}
}

func TestCountByEpic(t *testing.T) {
	epics := CountByEpic(sampleBeads())

	infra := epics["infra"]
	if infra.Total != 3 {
		t.Errorf("infra: expected total 3, got %d", infra.Total)
	}
	if infra.Complete != 2 {
		t.Errorf("infra: expected complete 2, got %d", infra.Complete)
	}

	api := epics["api"]
	if api.Total != 2 {
		t.Errorf("api: expected total 2, got %d", api.Total)
	}
	if api.InProgress != 1 {
		t.Errorf("api: expected in-progress 1, got %d", api.InProgress)
	}
	if api.Blocked != 1 {
		t.Errorf("api: expected blocked 1, got %d", api.Blocked)
	}

	ui := epics["ui"]
	if ui.Total != 1 {
		t.Errorf("ui: expected total 1, got %d", ui.Total)
	}
}

func TestAvailable(t *testing.T) {
	avail := Available(sampleBeads())
	// Should exclude: closed (coord-001), done (coord-002), blocked (coord-005)
	// Should include: in-progress (coord-003), open (coord-004), open (coord-006)
	if len(avail) != 3 {
		t.Fatalf("expected 3 available beads, got %d", len(avail))
	}
	ids := make(map[string]bool)
	for _, b := range avail {
		ids[b.ID] = true
	}
	for _, want := range []string{"coord-003", "coord-004", "coord-006"} {
		if !ids[want] {
			t.Errorf("expected %s in available set", want)
		}
	}
}

func TestAvailable_Empty(t *testing.T) {
	avail := Available(nil)
	if avail != nil {
		t.Errorf("expected nil for nil input, got %v", avail)
	}
}

func TestAvailable_AllComplete(t *testing.T) {
	beads := []Bead{
		{ID: "1", Status: "closed"},
		{ID: "2", Status: "done"},
		{ID: "3", Status: "complete"},
	}
	avail := Available(beads)
	if avail != nil {
		t.Errorf("expected nil when all complete, got %v", avail)
	}
}

func TestForWork(t *testing.T) {
	alpha := ForWork(sampleBeads(), "alpha")
	if len(alpha) != 4 {
		t.Fatalf("expected 4 beads for work:alpha, got %d", len(alpha))
	}
	ids := make(map[string]bool)
	for _, b := range alpha {
		ids[b.ID] = true
	}
	for _, want := range []string{"coord-001", "coord-002", "coord-003", "coord-005"} {
		if !ids[want] {
			t.Errorf("expected %s in work:alpha set", want)
		}
	}

	beta := ForWork(sampleBeads(), "beta")
	if len(beta) != 2 {
		t.Fatalf("expected 2 beads for work:beta, got %d", len(beta))
	}
}

func TestForWork_CaseInsensitive(t *testing.T) {
	beads := []Bead{
		{ID: "1", Labels: []string{"Work:Alpha"}},
		{ID: "2", Labels: []string{"WORK:ALPHA"}},
	}
	result := ForWork(beads, "alpha")
	// Our comparison lowercases both sides via EqualFold, but the target is
	// constructed as "work:alpha". EqualFold handles the label side.
	if len(result) != 2 {
		t.Errorf("expected 2 case-insensitive matches, got %d", len(result))
	}
}

func TestForWork_NoMatch(t *testing.T) {
	result := ForWork(sampleBeads(), "nonexistent")
	if result != nil {
		t.Errorf("expected nil for no matches, got %v", result)
	}
}

func TestForWork_Empty(t *testing.T) {
	result := ForWork(nil, "anything")
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestIsRework(t *testing.T) {
	tests := []struct {
		name string
		b    Bead
		want bool
	}{
		{"no labels", Bead{ID: "1"}, false},
		{"empty labels", Bead{ID: "1", Labels: []string{}}, false},
		{"unrelated labels", Bead{ID: "1", Labels: []string{"work:alpha", "backend"}}, false},
		{"pure rework label", Bead{ID: "1", Labels: []string{"rework:true"}}, true},
		{"finding prefix only", Bead{ID: "1", Labels: []string{"finding:"}}, true},
		{"finding with origin", Bead{ID: "1", Labels: []string{"finding:work-a"}}, true},
		{"mixed labels with rework", Bead{ID: "1", Labels: []string{"work:alpha", "rework:true", "backend"}}, true},
		{"mixed labels with finding", Bead{ID: "1", Labels: []string{"work:alpha", "finding:work-b"}}, true},
		{"case insensitive rework", Bead{ID: "1", Labels: []string{"ReWork:True"}}, true},
		{"case insensitive finding prefix", Bead{ID: "1", Labels: []string{"FINDING:Work-A"}}, true},
		{"finding substring not prefix", Bead{ID: "1", Labels: []string{"prefinding:foo"}}, false},
		{"rework:false is not rework", Bead{ID: "1", Labels: []string{"rework:false"}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsRework(tc.b); got != tc.want {
				t.Errorf("IsRework(%v) = %v, want %v", tc.b.Labels, got, tc.want)
			}
		})
	}
}

func TestReworkCount(t *testing.T) {
	beads := []Bead{
		{ID: "1", Labels: []string{"work:alpha"}},
		{ID: "2", Labels: []string{"rework:true"}},
		{ID: "3", Labels: []string{"finding:work-a"}},
		{ID: "4", Labels: []string{"FINDING:work-b", "extra"}},
		{ID: "5", Labels: nil},
	}
	if got := ReworkCount(beads); got != 3 {
		t.Errorf("ReworkCount = %d, want 3", got)
	}
	if got := ReworkCount(nil); got != 0 {
		t.Errorf("ReworkCount(nil) = %d, want 0", got)
	}
}
