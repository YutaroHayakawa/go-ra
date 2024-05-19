// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package integration_tests

// This file manages the assignment of the shared resources used by the
// integration tests that may run concurrently.
var (
	// Assigned to the TestGoBGPUnnumbered
	vethPair0 = []string{"go-ra0", "go-ra1"}

	// Assigned to the TestSolictedRA
	vethPair1 = []string{"go-ra2", "go-ra3"}

	// Assigned to the TestPrefixInfo
	vethPair2 = []string{"go-ra4", "go-ra5"}

	// Assigned to the TestRouteInfo
	vethPair3 = []string{"go-ra6", "go-ra7"}
)
