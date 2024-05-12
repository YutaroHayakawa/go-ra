// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package integration_tests

import (
	"testing"

	"github.com/lorenzosaino/go-sysctl"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

type fixture struct {
	veth0 netlink.Link
	veth1 netlink.Link
}

type fixtureParam struct {
	vethPair []string
}

func newFixture(t testing.TB, param fixtureParam) *fixture {
	t.Helper()

	veth0Name := param.vethPair[0]
	veth1Name := param.vethPair[1]

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

	t.Log("Set sysctl. Setting up links.")

	err = netlink.LinkSetUp(link0)
	require.NoError(t, err)

	err = netlink.LinkSetUp(link1)
	require.NoError(t, err)

	return &fixture{
		veth0: link0,
		veth1: link1,
	}
}
