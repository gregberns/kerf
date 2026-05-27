package scenarios

import (
	"strings"
	"testing"

	"github.com/gregberns/kerf/internal/sim/scenario"
)

var cannedScenarios = []string{
	"small-linear.yaml",
	"wide-fanout.yaml",
	"rework-heavy.yaml",
}

func TestEmbedFSListsExactlyThreeYAMLFiles(t *testing.T) {
	entries, err := fs.ReadDir(".")
	if err != nil {
		t.Fatalf("read embedded FS: %v", err)
	}
	var yamls []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".yaml") {
			yamls = append(yamls, e.Name())
		}
	}
	if len(yamls) != 3 {
		t.Fatalf("expected exactly 3 .yaml files in embed.FS, got %d: %v", len(yamls), yamls)
	}
	want := map[string]bool{
		"small-linear.yaml":  true,
		"wide-fanout.yaml":   true,
		"rework-heavy.yaml":  true,
	}
	for _, name := range yamls {
		if !want[name] {
			t.Errorf("unexpected embedded scenario file: %q", name)
		}
	}
}

func TestCannedScenariosLoadValidateAndShape(t *testing.T) {
	for _, name := range cannedScenarios {
		name := name
		t.Run(name, func(t *testing.T) {
			b, err := fs.ReadFile(name)
			if err != nil {
				t.Fatalf("read embedded %s: %v", name, err)
			}
			s, err := scenario.LoadBytes(b)
			if err != nil {
				t.Fatalf("scenario.LoadBytes(%s): %v", name, err)
			}
			if err := s.Validate(); err != nil {
				t.Fatalf("scenario.Validate(%s): %v", name, err)
			}
			// Unique codenames within the scenario.
			seen := make(map[string]struct{}, len(s.Works))
			for _, w := range s.Works {
				if _, dup := seen[w.Codename]; dup {
					t.Fatalf("%s: duplicate codename %q", name, w.Codename)
				}
				seen[w.Codename] = struct{}{}
			}
			// Duration kind must be lognormal per spec / bead spec.
			if s.AgentModel.Duration.Kind != scenario.DurationKindLogNormal {
				t.Fatalf("%s: duration.kind = %q, want %q", name, s.AgentModel.Duration.Kind, scenario.DurationKindLogNormal)
			}
		})
	}
}
