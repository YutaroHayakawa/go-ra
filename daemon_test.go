// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package ra

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/mdlayher/ndp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// We use a common parameter for most of the Eventually.
func eventully(t *testing.T, f func() bool) {
	require.Eventually(t, f, time.Second*1, time.Millisecond*10)
}

func assertRAInterval(t *testing.T, sock *fakeSock, interval time.Duration) bool {
	// wait until we get 3 RAs
	timeout, cancel := context.WithTimeout(context.Background(), time.Second*1)

	ras := []fakeRA{}
outer:
	for {
		select {
		case <-timeout.Done():
			cancel()
			return assert.Fail(t, "couldn't get 3 RAs in time")
		case ra := <-sock.txMulticastCh():
			ras = append(ras, ra)
			if len(ras) == 3 {
				cancel()
				break outer
			}
		}
	}

	// Ensure the interval is correct. We let 5ms of error margin.
	mergin := float64(5 * time.Millisecond)
	diff0 := ras[1].tstamp.Sub(ras[0].tstamp)
	diff1 := ras[2].tstamp.Sub(ras[1].tstamp)

	return assert.InDelta(t, interval, diff0, mergin) && assert.InDelta(t, interval, diff1, mergin)
}

func TestDaemonHappyPath(t *testing.T) {
	config := &Config{
		Interfaces: []*InterfaceConfig{
			{
				Name:                   "net0",
				RAIntervalMilliseconds: 100,
			},
			{
				Name:                   "net1",
				RAIntervalMilliseconds: 100,
			},
		},
	}

	reg := newFakeSockRegistry()

	d, err := NewDaemon(config, withSocketConstructor(reg.newSock))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go d.Run(ctx)
	t.Cleanup(func() {
		if t.Failed() {
			cancel()
		}
	})

	t.Run("Ensure socket is created", func(t *testing.T) {
		eventully(t, func() bool {
			_, err0 := reg.getSock("net0")
			_, err1 := reg.getSock("net1")
			return assert.NoError(t, err0) && assert.NoError(t, err1)
		})
	})

	t.Run("Ensure unsolicited RA is sent with the specified interval", func(t *testing.T) {
		sock, err := reg.getSock("net0")
		require.NoError(t, err)
		require.True(t, assertRAInterval(t, sock, time.Millisecond*100))

		sock, err = reg.getSock("net1")
		require.NoError(t, err)
		require.True(t, assertRAInterval(t, sock, time.Millisecond*100))
	})

	t.Run("Ensure the status is running and the result is ordered by name", func(t *testing.T) {
		status := d.Status()
		require.NoError(t, err)
		require.Len(t, status.Interfaces, 2)
		assert.Equal(t, "net0", status.Interfaces[0].Name)
		assert.Equal(t, "net1", status.Interfaces[1].Name)
		assert.Equal(t, Running, status.Interfaces[0].State)
		assert.Equal(t, Running, status.Interfaces[1].State)
	})

	t.Run("Ensure unsolicited RA interval is updated after reload", func(t *testing.T) {
		// Update the interval of net1. net0 should remain the same.
		config.Interfaces[1].RAIntervalMilliseconds = 200

		// Reload
		timeout, cancelTimeout := context.WithTimeout(context.Background(), time.Second*1)
		err := d.Reload(timeout, config)
		require.NoError(t, err)
		cancelTimeout()

		eventully(t, func() bool {
			sock0, err := reg.getSock("net0")
			if !assert.NoError(t, err) {
				return false
			}
			sock1, err := reg.getSock("net1")
			if !assert.NoError(t, err) {
				return false
			}
			return assertRAInterval(t, sock0, time.Millisecond*100) &&
				assertRAInterval(t, sock1, time.Millisecond*200)
		})
	})

	t.Run("Ensure RS is replied with unicast RA", func(t *testing.T) {
		sock, err := reg.getSock("net0")
		require.NoError(t, err)

		from := netip.MustParseAddr("fe80::1%net0")

		rs := &ndp.RouterSolicitation{}

		// Send RS
		sock.rxCh() <- fakeRS{msg: rs, from: from}

		// Wait for solicited RA
		timeout, cancelTimeout := context.WithTimeout(context.Background(), time.Second*1)
		select {
		case ra := <-sock.txLLUnicastCh():
			require.Equal(t, ra.to, from)
		case <-timeout.Done():
			require.Fail(t, "timeout waiting for RA")
		}
		cancelTimeout()
	})

	t.Run("Ensure unsolicited RA is stopped after removing configuration", func(t *testing.T) {
		// Remove net1
		config.Interfaces = config.Interfaces[:1]

		// Reload
		timeout, cancelTimeout := context.WithTimeout(context.Background(), time.Second*1)
		err := d.Reload(timeout, config)
		require.NoError(t, err)
		cancelTimeout()

		eventully(t, func() bool {
			sock0, err := reg.getSock("net0")
			if !assert.NoError(t, err) {
				return false
			}
			sock1, err := reg.getSock("net1")
			if !assert.NoError(t, err) {
				return false
			}
			return assertRAInterval(t, sock0, time.Millisecond*100) && assert.True(t, sock1.isClosed())
		})
	})

	t.Run("Ensure unsolicited RA is stopped after stopping the daemon", func(t *testing.T) {
		// Cancel the daemon's context
		cancel()
		eventully(t, func() bool {
			sock0, err := reg.getSock("net0")
			if !assert.NoError(t, err) {
				return false
			}
			sock1, err := reg.getSock("net1")
			if !assert.NoError(t, err) {
				return false
			}
			return assert.True(t, sock0.isClosed()) && assert.True(t, sock1.isClosed())
		})
	})
}
