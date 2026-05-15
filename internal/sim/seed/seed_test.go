package seed

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

// TestDerive_Determinism: identical inputs produce identical outputs,
// invocation after invocation.
func TestDerive_Determinism(t *testing.T) {
	top := []byte{0, 0, 0, 0, 0, 0, 0, 42}
	a := Derive(top, "gen")
	b := Derive(top, "gen")
	c := Derive(top, "gen")
	if a != b || b != c {
		t.Fatalf("Derive not deterministic: %d %d %d", a, b, c)
	}
}

// TestDerive_DifferentNamesDiffer: distinct names produce distinct
// sub-seeds under a fixed top seed. (Probabilistic, but with SHA256
// truncated to 64 bits the chance of an unintended collision among the
// Phase 1 names is negligible.)
func TestDerive_DifferentNamesDiffer(t *testing.T) {
	top := []byte{0, 0, 0, 0, 0, 0, 0, 42}
	names := []string{Gen, Dur, Events, Tiebreak, BaselineRandom}
	seen := make(map[uint64]string, len(names))
	for _, n := range names {
		v := Derive(top, n)
		if other, ok := seen[v]; ok {
			t.Fatalf("collision between %q and %q -> %d", n, other, v)
		}
		seen[v] = n
	}
}

// TestDerive_DifferentTopsDiffer: same name, distinct top seeds produce
// distinct sub-seeds.
func TestDerive_DifferentTopsDiffer(t *testing.T) {
	tops := []uint64{0, 1, 2, 42, 1 << 32, ^uint64(0)}
	seen := make(map[uint64]uint64, len(tops))
	for _, ts := range tops {
		v := From(ts)(Gen)
		if other, ok := seen[v]; ok {
			t.Fatalf("collision: top=%d and top=%d both yield %d", ts, other, v)
		}
		seen[v] = ts
	}
}

// TestFrom_BigEndianTopSeed: From encodes the top seed in big-endian
// before hashing. Confirm by re-deriving with a hand-built big-endian
// byte slice.
func TestFrom_BigEndianTopSeed(t *testing.T) {
	const top uint64 = 0x0123456789ABCDEF
	want := Derive(
		[]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF},
		Gen,
	)
	got := From(top)(Gen)
	if got != want {
		t.Fatalf("From did not big-endian-encode top seed: got %#016x want %#016x", got, want)
	}

	// Sanity: little-endian encoding of the same uint64 must NOT match
	// (the value 0x0123456789ABCDEF is non-palindromic byte-wise).
	leBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(leBytes, top)
	if le := Derive(leBytes, Gen); le == want {
		t.Fatalf("little-endian and big-endian encodings happen to agree; test is degenerate")
	}
}

// TestDerive_BigEndianOutput: the truncated hash is interpreted as a
// big-endian uint64. Confirm by hashing manually and reading the first
// 8 bytes with binary.BigEndian.
func TestDerive_BigEndianOutput(t *testing.T) {
	top := []byte{0, 0, 0, 0, 0, 0, 0, 42}
	h := sha256.Sum256(append(append([]byte{}, top...), []byte(Gen)...))
	want := binary.BigEndian.Uint64(h[:8])
	got := Derive(top, Gen)
	if got != want {
		t.Fatalf("Derive output not big-endian: got %#016x want %#016x", got, want)
	}
	// And confirm it differs from a little-endian interpretation.
	if le := binary.LittleEndian.Uint64(h[:8]); got == le {
		t.Fatalf("hash prefix is byte-palindromic; test is degenerate (want big-endian semantics)")
	}
}

// TestDerive_KnownVectors: spot-check two hand-computed SHA256 values
// to lock the formula. These were computed independently (Python +
// hashlib) and must not change across simulator versions.
func TestDerive_KnownVectors(t *testing.T) {
	cases := []struct {
		name string
		top  []byte
		in   string
		want uint64
	}{
		// top = uint64(0) big-endian = 8 zero bytes; name = "" →
		// SHA256(8 zero bytes)[:8] big-endian.
		{
			name: "top=0,name=empty",
			top:  []byte{0, 0, 0, 0, 0, 0, 0, 0},
			in:   "",
			want: 0xaf5570f5a1810b7a,
		},
		// top = uint64(1) big-endian; name = "x".
		{
			name: "top=1,name=x",
			top:  []byte{0, 0, 0, 0, 0, 0, 0, 1},
			in:   "x",
			want: 0x028f0df5b8596fbc,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Derive(tc.top, tc.in)
			if got != tc.want {
				t.Fatalf("Derive(%x, %q) = %#016x; want %#016x", tc.top, tc.in, got, tc.want)
			}
		})
	}
}

// TestNamedSubSeeds_Regression: each Phase-1 named constant maps to a
// stable, distinct sub-seed under top seed 42 (the example from
// specs/simulator.md). These values are part of the determinism
// contract — if any change, the simulator's output is no longer
// reproducible across versions, which is a spec violation.
func TestNamedSubSeeds_Regression(t *testing.T) {
	derive := From(42)
	cases := []struct {
		name string
		want uint64
	}{
		{Gen, 0xa8ae71b0a5ebfde7},
		{Dur, 0xad3dfa57a28b62ce},
		{Events, 0x63f35fa5926e91d1},
		{Tiebreak, 0x068a6fa926251411},
		{BaselineRandom, 0xe3e7ec9bdd73218b},
	}
	seen := make(map[uint64]string, len(cases))
	for _, tc := range cases {
		got := derive(tc.name)
		if got != tc.want {
			t.Errorf("From(42)(%q) = %#016x; want %#016x", tc.name, got, tc.want)
		}
		if other, ok := seen[got]; ok {
			t.Errorf("named sub-seeds collide: %q and %q both -> %#016x", tc.name, other, got)
		}
		seen[got] = tc.name
	}
}

// TestFrom_ClosureIsolation: a closure captures its top seed at
// construction time — later mutation of the caller's variable must not
// affect derived sub-seeds.
func TestFrom_ClosureIsolation(t *testing.T) {
	top := uint64(42)
	d := From(top)
	before := d(Gen)
	top = 99
	_ = top
	after := d(Gen)
	if before != after {
		t.Fatalf("closure leaked top-seed mutation: before=%#x after=%#x", before, after)
	}
}
