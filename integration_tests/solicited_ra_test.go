// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package integration_tests

import (
	"context"
	"testing"
	"time"

	"github.com/YutaroHayakawa/go-ra"
	"github.com/lorenzosaino/go-sysctl"
	"github.com/osrg/gobgp/v3/pkg/config/oc"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

func TestSolicitedRA(t *testing.T) {
	veth0Name := vethPair1[0]
	veth1Name := vethPair1[1]

	// Create veth pair
	err := netlink.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:      veth0Name,
			OperState: netlink.OperUp,
		},
		PeerName: veth1Name,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		t.Log("Cleaning up veth pair")
		netlink.LinkDel(&netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{
				Name: veth0Name,
			},
		})
	})

	link0, err := netlink.LinkByName(veth0Name)
	require.NoError(t, err)

	link1, err := netlink.LinkByName(veth1Name)
	require.NoError(t, err)

	t.Log("Created veth pair. Setting sysctl.")

	sysctlClient, err := sysctl.NewClient(sysctl.DefaultPath)
	require.NoError(t, err)

	err = sysctlClient.Set("net.ipv6.conf."+veth0Name+".forwarding", "1")
	require.NoError(t, err)

	err = sysctlClient.Set("net.ipv6.conf."+veth0Name+".accept_ra", "2")
	require.NoError(t, err)

	err = sysctlClient.Set("net.ipv6.conf."+veth1Name+".forwarding", "1")
	require.NoError(t, err)

	err = sysctlClient.Set("net.ipv6.conf."+veth1Name+".accept_ra", "2")
	require.NoError(t, err)

	err = netlink.LinkSetUp(link0)
	require.NoError(t, err)

	err = netlink.LinkSetUp(link1)
	require.NoError(t, err)

	// Start rad
	t.Log("Starting rad")

	ctx := context.Background()

	// Start rad on veth0
	rad0, err := ra.NewDaemon(&ra.Config{
		Interfaces: []*ra.InterfaceConfig{
			{
				Name: veth0Name,
				// Set this to super long to avoid sending unsolicited RAs.
				RAIntervalMilliseconds: 1800000,
			},
		},
	})
	require.NoError(t, err)

	go rad0.Run(ctx)

	// Wait until the RA sender is ready
	require.Eventually(t, func() bool {
		status := rad0.Status()
		return status.Interfaces[0].State == ra.Running
	}, time.Second*10, 100*time.Millisecond)

	t.Logf("rad is ready. Down -> Up %s to send RS", veth1Name)

	// Down and up the link to trigger an RS
	err = netlink.LinkSetDown(link1)
	require.NoError(t, err)

	err = netlink.LinkSetUp(link1)
	require.NoError(t, err)

	// Ensure the neighbor entry is created
	require.Eventually(t, func() bool {
		_, err := oc.GetIPv6LinkLocalNeighborAddress(veth1Name)
		status := rad0.Status()
		return err == nil && status.Interfaces[0].TxSolicitedRA > 0
	}, time.Second*10, 100*time.Millisecond)

	t.Log("Neighbor entry created. Done.")
}
