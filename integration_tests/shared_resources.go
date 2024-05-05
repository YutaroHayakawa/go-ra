package integration_tests

// This file manages the assignment of the shared resources used by the
// integration tests that may run concurrently.
var (
	// Assigned to the TestGoBGPUnnumbered
	vethPair0 = []string{"go-ra0", "go-ra1"}

	// Assigned to the TestSolictedRA
	vethPair1 = []string{"go-ra2", "go-ra3"}
)
