package cmd

import "testing"

// TestDoctorCmd_SilenceUsageOnError pins kerf-jy2i: when runDoctor returns
// an error (e.g. scaffolding failure on an unreadable project), cobra must
// NOT dump the usage block. Mirrors TestNextCmd_SilenceUsageOnError;
// symmetry with kerf next (kerf-1d6).
func TestDoctorCmd_SilenceUsageOnError(t *testing.T) {
	if !doctorCmd.SilenceUsage {
		t.Fatalf("doctorCmd.SilenceUsage must be true so runDoctor errors do not trigger a usage dump (kerf-jy2i)")
	}
}
