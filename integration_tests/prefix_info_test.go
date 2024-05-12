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
	"k8s.io/utils/ptr"
)

func TestPrefixInfo(t *testing.T) {
	f := newFixture(t, fixtureParam{vethPair: vethPair2})
	veth0Name := f.veth0.Attrs().Name

	config := &ra.Config{
		Interfaces: []*ra.InterfaceConfig{
			{
				Name:                   veth0Name,
				RAIntervalMilliseconds: 70, // Fastest possible
				Prefixes: []*ra.PrefixConfig{
					{
						Prefix:                   "fd00:0:0:0::/64",
						OnLink:                   true,
						Autonomous:               true,
						ValidLifetimeSeconds:     ptr.To(60),
						PreferredLifetimeSeconds: ptr.To(30),
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

	// Check the address generation
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		addrs, err := netlink.AddrList(f.veth1, unix.AF_INET6)
		require.NoError(ct, err)

		for _, addr := range addrs {
			if !addr.IP.IsGlobalUnicast() {
				continue
			}
			// Extract /64 from the address
			assert.Equal(ct, []byte{0xfd, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, []byte(addr.IP[0:8]), "Prefix mismatch")
			assert.InDelta(ct, 60, addr.ValidLft, 3, "Valid lifetime mismatch")
			assert.InDelta(ct, 30, addr.PreferedLft, 3, "Preferred lifetime mismatch")
		}
	}, 3*time.Second, time.Millisecond*100)
}
