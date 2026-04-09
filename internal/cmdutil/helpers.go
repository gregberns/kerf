package cmdutil

// ResolveProject resolves the project ID from the --project flag,
// .kerf/project-identifier, or config default_project.
func ResolveProject(flagValue string) (string, error) {
	// TODO: implement project resolution chain
	return "", nil
}

// LoadWorkWithChecks loads a work's spec.yaml and runs cross-cutting checks:
// stale session warning, jig version mismatch warning, and interval snapshot check.
func LoadWorkWithChecks(projectID, codename string) error {
	// TODO: implement cross-cutting checks
	return nil
}
