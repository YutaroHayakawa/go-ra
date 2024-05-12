// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package integration_tests

import (
	"context"
	"testing"
	"time"

	"github.com/YutaroHayakawa/go-ra"

	apipb "github.com/osrg/gobgp/v3/api"
	"github.com/osrg/gobgp/v3/pkg/config/oc"
	"github.com/osrg/gobgp/v3/pkg/server"
	"github.com/stretchr/testify/require"
)

func TestGoBGPUnnumbered(t *testing.T) {
	f := newFixture(t, fixtureParam{vethPair: vethPair0})
	veth0Name := f.veth0.Attrs().Name
	veth1Name := f.veth1.Attrs().Name

	t.Log("Starting rad")

	ctx := context.Background()

	// Start rad
	rad0, err := ra.NewDaemon(&ra.Config{
		Interfaces: []*ra.InterfaceConfig{
			{
				Name:                   veth0Name,
				RAIntervalMilliseconds: 1000,
			},
		},
	})
	require.NoError(t, err)

	rad1, err := ra.NewDaemon(&ra.Config{
		Interfaces: []*ra.InterfaceConfig{
			{
				Name:                   veth1Name,
				RAIntervalMilliseconds: 1000,
			},
		},
	})
	require.NoError(t, err)

	go rad0.Run(ctx)
	go rad1.Run(ctx)

	t.Log("Started rad. Waiting for RAs to be sent.")

	// Wait at least for 2 RAs to be sent
	require.Eventually(t, func() bool {
		status0 := rad0.Status()
		status1 := rad1.Status()
		return status0 != nil && status1 != nil &&
			status0.Interfaces[0].State == ra.Running &&
			status1.Interfaces[0].State == ra.Running
	}, time.Second*10, time.Millisecond*500)

	t.Log("RAs are being sent. Starting BGP.")

	// Start bgpd
	timeout, cancel := context.WithTimeout(ctx, time.Second*1)
	bgpd0 := server.NewBgpServer()
	go bgpd0.Serve()

	err = bgpd0.StartBgp(timeout, &apipb.StartBgpRequest{
		Global: &apipb.Global{
			Asn:        64512,
			RouterId:   "10.0.0.0",
			ListenPort: 10179,
		},
	})
	require.NoError(t, err)
	cancel()

	timeout, cancel = context.WithTimeout(ctx, time.Second*1)
	bgpd1 := server.NewBgpServer()
	go bgpd1.Serve()

	err = bgpd1.StartBgp(timeout, &apipb.StartBgpRequest{
		Global: &apipb.Global{
			Asn:        64512,
			RouterId:   "10.0.0.1",
			ListenPort: 11179,
		},
	})
	require.NoError(t, err)
	cancel()

	t.Log("Started BGP. Adding peers.")

	lladdr0, err := oc.GetIPv6LinkLocalNeighborAddress(veth0Name)
	require.NoError(t, err)

	lladdr1, err := oc.GetIPv6LinkLocalNeighborAddress(veth1Name)
	require.NoError(t, err)

	// Set up unnumbered peer
	err = bgpd0.AddPeer(ctx, &apipb.AddPeerRequest{
		Peer: &apipb.Peer{
			Conf: &apipb.PeerConf{
				PeerAsn:           64512,
				NeighborAddress:   lladdr0,
				NeighborInterface: veth0Name,
			},
			Transport: &apipb.Transport{
				RemotePort: 11179,
			},
			Timers: &apipb.Timers{
				Config: &apipb.TimersConfig{
					ConnectRetry: 1,
				},
			},
		},
	})
	require.NoError(t, err)

	err = bgpd1.AddPeer(ctx, &apipb.AddPeerRequest{
		Peer: &apipb.Peer{
			Conf: &apipb.PeerConf{
				PeerAsn:           64512,
				NeighborAddress:   lladdr1,
				NeighborInterface: veth1Name,
			},
			Transport: &apipb.Transport{
				RemotePort: 10179,
			},
			Timers: &apipb.Timers{
				Config: &apipb.TimersConfig{
					ConnectRetry: 1,
				},
			},
		},
	})
	require.NoError(t, err)

	t.Log("Peers added. Waiting for session to be established.")

	require.Eventually(t, func() bool {
		var peer0, peer1 *apipb.Peer

		if err := bgpd0.ListPeer(ctx, &apipb.ListPeerRequest{}, func(p *apipb.Peer) {
			if p.Conf.NeighborInterface == veth0Name {
				peer0 = p
			}
		}); err != nil {
			return false
		}

		if err := bgpd1.ListPeer(ctx, &apipb.ListPeerRequest{}, func(p *apipb.Peer) {
			if p.Conf.NeighborInterface == veth1Name {
				peer1 = p
			}
		}); err != nil {
			return false
		}

		return peer0 != nil && peer1 != nil &&
			peer0.State.SessionState == apipb.PeerState_ESTABLISHED &&
			peer1.State.SessionState == apipb.PeerState_ESTABLISHED
	}, time.Second*10, time.Millisecond*500)

	t.Log("Session established. All done.")
}
