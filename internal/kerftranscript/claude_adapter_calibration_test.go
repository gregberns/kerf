package kerftranscript

// Calibration test for the real-Claude-Code adapter (bead kerf-ek21).
//
// Unlike the synthetic-line tests in claude_adapter_test.go, this test
// runs the full Parse pipeline against an anonymised slice of a real
// production transcript (testdata/claude_real_slice.jsonl, sampled from
// ~/.claude/projects/-Users-gb-github-kerf/bea7e962-…jsonl lines 70–78
// with absolute home paths scrubbed). The slice was chosen because it
// contains:
//
//   - at least one assistant line with an Agent tool_use (dispatch),
//   - at least one user line with a tool_result block,
//   - and a mix of irrelevant types (attachment, last-prompt, ai-title,
//     permission-mode) that should silently produce zero events.
//
// The test guards against regressions of the original kerf-ek21 bug
// (parser rejecting 100% of real lines). It asserts that the slice
// produces non-zero dispatch and tool_result events and that a known
// bead ID survives the path through Parse + ExtractBeadIDs.

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestParse_realClaudeSlice(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "claude_real_slice.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected parse errors (%d): %+v", len(res.Errors), res.Errors)
	}
	if len(res.Events) == 0 {
		t.Fatal("got zero events from real-transcript slice — the kerf-ek21 bug has returned")
	}

	var dispatches, toolResults int
	for _, ev := range res.Events {
		switch ev.Kind {
		case EventDispatch:
			dispatches++
		case EventToolResult:
			toolResults++
		}
	}
	if dispatches == 0 {
		t.Errorf("expected >=1 dispatch event from the slice, got 0")
	}
	if toolResults == 0 {
		t.Errorf("expected >=1 tool_result event from the slice, got 0")
	}

	// Bead-ID extraction must work end-to-end: at least one event's
	// Text contains a bead-ID-shaped substring under the kerf pattern.
	pat := regexp.MustCompile(`kerf-[a-z0-9]+`)
	enriched := ExtractBeadIDs(res.Events, pat)
	var withBead int
	for _, ev := range enriched {
		if ev.BeadID != "" {
			withBead++
		}
	}
	if withBead == 0 {
		t.Errorf("ExtractBeadIDs populated 0 events — pattern did not match any dispatch/tool_result Text")
	}

	// The slice was sampled from a kerf-6iiw dispatch+result pair;
	// confirm the bead id round-trips for the visible Text.
	foundKerf6iiw := false
	for _, ev := range enriched {
		if strings.Contains(ev.Text, "kerf-6iiw") {
			foundKerf6iiw = true
			break
		}
	}
	if !foundKerf6iiw {
		t.Errorf("expected the kerf-6iiw bead id to appear in the slice's event text; slice may have drifted")
	}
}
