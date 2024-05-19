// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package integration_tests

import (
	"context"
	"testing"
	"time"

	"github.com/YutaroHayakawa/go-ra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestRouteInfo(t *testing.T) {
	f := newFixture(t, fixtureParam{vethPair: vethPair3})
	veth0Name := f.veth0.Attrs().Name

	config := &ra.Config{
		Interfaces: []*ra.InterfaceConfig{
			{
				Name:                   veth0Name,
				RAIntervalMilliseconds: 70, // Fastest possible
				Routes: []*ra.RouteConfig{
					{
						Prefix:          "2001:db8::/64",
						LifetimeSeconds: 10,
						Preference:      "low",
					},
					{
						Prefix:          "2001:db8:1::/64",
						LifetimeSeconds: 10,
						Preference:      "high",
					},
				},
			},
		},
	}

	daemon, err := ra.NewDaemon(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go daemon.Run(ctx)
	require.NoError(t, err)

	// Wait for the daemon to start
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		status := daemon.Status()
		for _, iface := range status.Interfaces {
			assert.Equal(ct, ra.Running, iface.State)
		}
	}, 3*time.Second, time.Millisecond*100)

	// Check the routing table entries
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		routes, err := netlink.RouteList(nil, unix.AF_INET6)
		require.NoError(ct, err)

		var found0, found1 bool
		for _, route := range routes {
			if route.Dst.String() == "2001:db8::/64" {
				found0 = true
			}
			if route.Dst.String() == "2001:db8:1::/64" {
				found1 = true
			}
		}
		assert.True(ct, found0, "route 2001:db8::/64 not found")
		assert.True(ct, found1, "route 2001:db8:1::/64 not found")
	}, 3*time.Second, time.Millisecond*100)
}
