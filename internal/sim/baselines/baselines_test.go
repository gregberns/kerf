package baselines

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/sim/store"
)

// seedBytes returns the canonical 8-big-endian-byte encoding of u, matching
// what seed.From produces internally so test seeds compose cleanly.
func seedBytes(u uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], u)
	return buf[:]
}

func makeWork(codename string, created time.Time) *spec.SpecYAML {
	return &spec.SpecYAML{
		Codename:     codename,
		Type:         "feature",
		Status:       "in-progress",
		StatusValues: []string{"design", "in-progress", "review", "complete"},
		Created:      created,
		Updated:      created,
		Areas:        []string{"area-" + codename},
	}
}

func makeBead(id, workCode string, deps ...string) beads.Bead {
	return beads.Bead{
		ID:        id,
		Title:     id,
		Status:    "open",
		Epic:      workCode,
		Labels:    []string{"work:" + workCode},
		DependsOn: deps,
	}
}

// twoWorkWorld returns a deterministic small world:
//   - alpha (created t0), beta (created t0+1h)
//   - a1, a2 in alpha; b1 in beta
//   - all arrive at tick 0 (via From), so we can re-arrive selectively.
func twoWorkWorld() *store.GeneratedWorld {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &store.GeneratedWorld{
		Works: []*spec.SpecYAML{
			makeWork("alpha", t0),
			makeWork("beta", t0.Add(time.Hour)),
		},
		InitialBeads: []beads.Bead{
			makeBead("a1", "alpha"),
			makeBead("a2", "alpha"),
			makeBead("b1", "beta"),
		},
	}
}

func TestFIFOBead_OrderingByArrivalThenID(t *testing.T) {
	s := store.New()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.AddWork(makeWork("alpha", t0))

	// Arrive in non-id order at distinct ticks.
	s.Arrive(makeBead("z", "alpha"), 5)
	s.Arrive(makeBead("a", "alpha"), 10)
	s.Arrive(makeBead("m", "alpha"), 5) // ties with z on arrival tick

	p := NewFIFOBead()
	if got := p.Next(s); got != "m" {
		// arrival 5: {z, m}, tiebreaker bead_id ascending -> "m" < "z"
		t.Fatalf("Next = %q, want %q", got, "m")
	}
	// Idempotent / deterministic: same call twice returns same result.
	if got := p.Next(s); got != "m" {
		t.Fatalf("Next (second call) = %q, want %q", got, "m")
	}
}

func TestFIFOWork_OrderingByWorkCreatedThenArrivalThenID(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := store.New()
	s.AddWork(makeWork("beta", t0.Add(time.Hour)))  // newer
	s.AddWork(makeWork("alpha", t0))                // older
	s.AddWork(makeWork("acme", t0))                 // ties with alpha on Created

	// Beads:
	//   beta:  b1 arrives at 0
	//   alpha: a2 arrives at 10, a1 arrives at 5
	//   acme:  c1 arrives at 0
	s.Arrive(makeBead("b1", "beta"), 0)
	s.Arrive(makeBead("a2", "alpha"), 10)
	s.Arrive(makeBead("a1", "alpha"), 5)
	s.Arrive(makeBead("c1", "acme"), 0)

	p := NewFIFOWork()
	// Oldest work tier = {alpha, acme} (both at t0). Tiebreak: codename
	// ascending -> acme wins.
	if got := p.Next(s); got != "c1" {
		t.Fatalf("Next #1 = %q, want %q (acme < alpha by codename)", got, "c1")
	}

	// Close c1; now alpha is the only t0 work with open beads.
	s.Dispatch("c1", 1, 1)
	s.Complete("c1", 2)
	// Within alpha: a1 arrived at 5, a2 at 10 → a1 wins.
	if got := p.Next(s); got != "a1" {
		t.Fatalf("Next #2 = %q, want %q (alpha: a1 arrival 5 < a2 arrival 10)", got, "a1")
	}
}

func TestRandom_Deterministic_SameSeed(t *testing.T) {
	s1 := store.From(twoWorkWorld())
	s2 := store.From(twoWorkWorld())

	seedA := seedBytes(42)
	p1 := NewRandom(seedA)
	p2 := NewRandom(seedA)

	// Same seed → same draw on identical store state.
	if a, b := p1.Next(s1), p2.Next(s2); a != b {
		t.Fatalf("same seed produced different picks: %q vs %q", a, b)
	}
}

func TestRandom_DifferentSeeds_MayDiffer(t *testing.T) {
	// We can't assert "always differ" (two seeds could collide on a 3-bead
	// world). Instead, sweep a handful of seeds and assert at least one
	// disagrees with seed 0 — extremely likely for any non-pathological
	// derivation.
	world := twoWorkWorld()
	base := NewRandom(seedBytes(0)).Next(store.From(world))

	differed := false
	for _, u := range []uint64{1, 2, 3, 17, 99} {
		s := store.From(world)
		pick := NewRandom(seedBytes(u)).Next(s)
		if pick != base {
			differed = true
			break
		}
	}
	if !differed {
		t.Fatalf("no other seed produced a different pick from seed 0 (pick = %q); seeds may be collapsing", base)
	}
}

func TestRandom_PicksFromDispatchableOnly(t *testing.T) {
	// With one work and three open beads, the picked ID must be one of them.
	s := store.New()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.AddWork(makeWork("alpha", t0))
	s.Arrive(makeBead("a1", "alpha"), 0)
	s.Arrive(makeBead("a2", "alpha"), 0)
	s.Arrive(makeBead("a3", "alpha"), 0)

	p := NewRandom(seedBytes(7))
	got := p.Next(s)
	if got != "a1" && got != "a2" && got != "a3" {
		t.Fatalf("Next = %q, want one of a1/a2/a3", got)
	}
}

func TestAll_EmptyStoreReturnsEmpty(t *testing.T) {
	s := store.New()
	for _, p := range []interface {
		Next(*store.Store) string
		Name() string
	}{
		NewRandom(seedBytes(1)),
		NewFIFOBead(),
		NewFIFOWork(),
	} {
		if got := p.Next(s); got != "" {
			t.Fatalf("%s.Next on empty store = %q, want \"\"", p.Name(), got)
		}
	}
}

func TestAll_NoDispatchableReturnsEmpty(t *testing.T) {
	// One work, all beads closed -> nothing dispatchable.
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := store.New()
	s.AddWork(makeWork("alpha", t0))
	s.Arrive(makeBead("a1", "alpha"), 0)
	s.Dispatch("a1", 1, 1)
	s.Complete("a1", 2)

	for _, p := range []interface {
		Next(*store.Store) string
		Name() string
	}{
		NewRandom(seedBytes(1)),
		NewFIFOBead(),
		NewFIFOWork(),
	} {
		if got := p.Next(s); got != "" {
			t.Fatalf("%s.Next on all-closed store = %q, want \"\"", p.Name(), got)
		}
	}
}

func TestNames(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{NewRandom(seedBytes(1)).Name(), "random"},
		{NewFIFOBead().Name(), "fifo-bead"},
		{NewFIFOWork().Name(), "fifo-work"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("Name = %q, want %q", c.got, c.want)
		}
	}
}

func TestFIFOBead_RespectsBeadLevelDeps(t *testing.T) {
	// a2 depends on a1; until a1 is closed, fifo-bead must skip a2.
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := store.New()
	s.AddWork(makeWork("alpha", t0))
	s.Arrive(makeBead("a1", "alpha"), 5)
	s.Arrive(makeBead("a2", "alpha", "a1"), 1) // arrives first but blocked

	p := NewFIFOBead()
	if got := p.Next(s); got != "a1" {
		t.Fatalf("Next = %q, want %q (a2 blocked by dep on a1)", got, "a1")
	}
	s.Dispatch("a1", 1, 6)
	s.Complete("a1", 7)
	if got := p.Next(s); got != "a2" {
		t.Fatalf("Next after a1 closed = %q, want %q", got, "a2")
	}
}
