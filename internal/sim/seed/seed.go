// Package seed implements deterministic sub-seed derivation for the
// kerfsim queue simulator.
//
// The simulator's determinism guarantee (specs/simulator.md §Determinism)
// requires that a single top-level seed split into named sub-seeds, each
// consumed by exactly one subsystem. The derivation is fixed across
// simulator versions and across OS/arch:
//
//	sub_seed = SHA256(top_seed_bytes || name_bytes)[:8]   // big-endian uint64
//
// where top_seed_bytes is the top-level seed encoded as 8 big-endian
// bytes and name_bytes is the ASCII name of the sub-seed.
package seed

import (
	"crypto/sha256"
	"encoding/binary"
)

// Phase 1 named sub-seeds, per specs/simulator.md §Determinism.
const (
	Gen            = "gen"             // synthetic scenario generator
	Dur            = "dur"             // duration pre-rolling
	Events         = "events"          // probabilistic events (arrivals, etc.)
	Tiebreak       = "tiebreak"        // score-tie resolution
	BaselineRandom = "baseline_random" // the "random" baseline's selection draws
)

// Derive returns a sub-seed for the named consumer.
//
// Formula: SHA256(topSeed || name)[:8] interpreted as a big-endian
// uint64. topSeed is the raw top-level seed byte representation
// (canonically 8 big-endian bytes when the top seed is a uint64; see
// From). name is the ASCII identifier of the consumer.
//
// Identical (topSeed, name) inputs always produce identical outputs
// across runs and across OS/arch.
func Derive(topSeed []byte, name string) uint64 {
	h := sha256.New()
	h.Write(topSeed)
	h.Write([]byte(name))
	sum := h.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8])
}

// From returns a closure that derives sub-seeds from the given uint64
// top seed. The top seed is encoded as 8 big-endian bytes before
// hashing — this is the canonical encoding used by the simulator.
func From(topSeed uint64) func(name string) uint64 {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], topSeed)
	frozen := buf // closure captures a fixed snapshot
	return func(name string) uint64 {
		return Derive(frozen[:], name)
	}
}
