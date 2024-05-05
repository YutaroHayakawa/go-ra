package integration_tests

// This file manages the assignment of the shared resources used by the
// integration tests that may run concurrently.
var (
	// Assigned to the TestGoBGPUnnumbered
	vethPair0 = []string{"go-radv0", "go-radv1"}
)
