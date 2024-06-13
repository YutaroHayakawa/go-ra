// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package ra

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/mdlayher/ndp"
	"golang.org/x/sys/unix"
)

type advertiser struct {
	logger *slog.Logger

	initialConfig *InterfaceConfig

	// We use mutex-based synchronization instead of channels because
	// ifaceStatus must be reported even when the main loop is hanging.
	ifaceStatus     *InterfaceStatus
	ifaceStatusLock sync.RWMutex

	reloadCh      chan *InterfaceConfig
	stopCh        chan any
	socketCtor    socketCtor
	deviceWatcher deviceWatcher
}

// An internal structure to represent RS
type rsMsg struct {
	rs   *ndp.RouterSolicitation
	from netip.Addr
}

func newAdvertiser(initialConfig *InterfaceConfig, ctor socketCtor, devWatcher deviceWatcher, logger *slog.Logger) *advertiser {
	return &advertiser{
		logger:        logger.With(slog.String("interface", initialConfig.Name)),
		initialConfig: initialConfig,
		ifaceStatus:   &InterfaceStatus{Name: initialConfig.Name, State: "Unknown"},
		reloadCh:      make(chan *InterfaceConfig),
		stopCh:        make(chan any),
		socketCtor:    ctor,
		deviceWatcher: devWatcher,
	}
}

func (s *advertiser) createRAMsg(config *InterfaceConfig, deviceState *deviceState) *ndp.RouterAdvertisement {
	return &ndp.RouterAdvertisement{
		CurrentHopLimit:           uint8(config.CurrentHopLimit),
		ManagedConfiguration:      config.Managed,
		OtherConfiguration:        config.Other,
		RouterSelectionPreference: s.toNDPPreference(config.Preference),
		RouterLifetime:            time.Duration(config.RouterLifetimeSeconds) * time.Second,
		ReachableTime:             time.Duration(config.ReachableTimeMilliseconds) * time.Millisecond,
		RetransmitTimer:           time.Duration(config.RetransmitTimeMilliseconds) * time.Millisecond,
		Options:                   s.createOptions(config, deviceState),
	}
}

func (s *advertiser) createOptions(config *InterfaceConfig, deviceState *deviceState) []ndp.Option {
	options := []ndp.Option{
		&ndp.LinkLayerAddress{
			Direction: ndp.Source,
			Addr:      deviceState.addr,
		},
	}

	if config.MTU > 0 {
		options = append(options, &ndp.MTU{
			MTU: uint32(config.MTU),
		})
	}

	for _, prefix := range config.Prefixes {
		// At this point, we should have validated the
		// configuration. If we haven't, it's a bug.
		p := netip.MustParsePrefix(prefix.Prefix)
		options = append(options, &ndp.PrefixInformation{
			PrefixLength:                   uint8(p.Bits()),
			OnLink:                         prefix.OnLink,
			AutonomousAddressConfiguration: prefix.Autonomous,
			ValidLifetime:                  time.Second * time.Duration(*prefix.ValidLifetimeSeconds),
			PreferredLifetime:              time.Second * time.Duration(*prefix.PreferredLifetimeSeconds),
			Prefix:                         p.Addr(),
		})
	}

	for _, route := range config.Routes {
		// At this point, we should have validated the
		// configuration. If we haven't, it's a bug.
		p := netip.MustParsePrefix(route.Prefix)
		options = append(options, &ndp.RouteInformation{
			PrefixLength:  uint8(p.Bits()),
			Preference:    s.toNDPPreference(route.Preference),
			RouteLifetime: time.Second * time.Duration(route.LifetimeSeconds),
			Prefix:        p.Addr(),
		})
	}

	for _, rdnss := range config.RDNSSes {
		addresses := []netip.Addr{}
		for _, addr := range rdnss.Addresses {
			// At this point, we should have validated the
			// configuration. If we haven't, it's a bug.
			addresses = append(addresses, netip.MustParseAddr(addr))
		}
		options = append(options, &ndp.RecursiveDNSServer{
			Lifetime: time.Second * time.Duration(rdnss.LifetimeSeconds),
			Servers:  addresses,
		})
	}

	for _, dnssl := range config.DNSSLs {
		options = append(options, &ndp.DNSSearchList{
			Lifetime:    time.Second * time.Duration(dnssl.LifetimeSeconds),
			DomainNames: dnssl.DomainNames,
		})
	}

	for _, nat64prefix := range config.NAT64Prefixes {
		options = append(options, &ndp.PREF64{
			Lifetime: time.Second * time.Duration(*nat64prefix.LifetimeSeconds),
			Prefix:   netip.MustParsePrefix(nat64prefix.Prefix),
		})
	}

	return options
}

func (s *advertiser) toNDPPreference(preference string) ndp.Preference {
	switch preference {
	case "low":
		return ndp.Low
	case "medium":
		return ndp.Medium
	case "high":
		return ndp.High
	default:
		s.logger.Warn("Unknown router preference. Using medium.", "preference", preference)
		return ndp.Medium
	}
}

func (s *advertiser) reportRunning() {
	s.ifaceStatusLock.Lock()
	defer s.ifaceStatusLock.Unlock()
	s.ifaceStatus.State = Running
	s.ifaceStatus.Message = ""
}

func (s *advertiser) reportReloading() {
	s.ifaceStatusLock.Lock()
	defer s.ifaceStatusLock.Unlock()
	s.ifaceStatus.State = Reloading
	s.ifaceStatus.Message = ""
}

func (s *advertiser) reportFailing(err error) {
	s.ifaceStatusLock.Lock()
	defer s.ifaceStatusLock.Unlock()
	s.ifaceStatus.State = Failing
	if err == nil {
		s.ifaceStatus.Message = ""
	} else {
		s.ifaceStatus.Message = err.Error()
	}
}

func (s *advertiser) reportStopped(err error) {
	s.ifaceStatusLock.Lock()
	defer s.ifaceStatusLock.Unlock()
	s.ifaceStatus.State = Stopped
	if err == nil {
		s.ifaceStatus.Message = ""
	} else {
		s.ifaceStatus.Message = err.Error()
	}
}

func (s *advertiser) incTxStat(solicited bool) {
	s.ifaceStatusLock.Lock()
	defer s.ifaceStatusLock.Unlock()
	if solicited {
		s.ifaceStatus.TxSolicitedRA++
	} else {
		s.ifaceStatus.TxUnsolicitedRA++
	}
}

func (s *advertiser) setLastUpdate() {
	s.ifaceStatusLock.Lock()
	defer s.ifaceStatusLock.Unlock()
	s.ifaceStatus.LastUpdate = time.Now().Unix()
}

func (s *advertiser) run(ctx context.Context) {
	// The current desired configuration
	config := s.initialConfig

	// The current device state
	devState := deviceState{}

	// Set a timestamp for the first "update"
	s.setLastUpdate()

	// Watch the device state
	devCh, err := s.deviceWatcher.watch(ctx, config.Name)
	if err != nil {
		s.reportStopped(err)
		return
	}

waitDevice:
	// Wait for the device to be present and up and the addresses are assigned
	for {
		select {
		case <-ctx.Done():
			s.reportStopped(ctx.Err())
			return
		case dev := <-devCh:
			// Update the device state
			devState = dev

			// If the device is up, mac and link-local address are
			// assigned, we can proceed with the socket creation
			if dev.isUp || len(dev.addr) > 0 || dev.v6LLAddrAssigned {
				break waitDevice
			}
		}
	}

	// Create the socket
	sock, err := s.socketCtor(config.Name)
	if err != nil {
		// These are the unrecoverable errors we're aware of now.
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EINVAL) {
			s.reportStopped(fmt.Errorf("cannot create socket: %w", err))
			return
		}
		// Otherwise, we'll retry
		s.reportFailing(err)
		goto waitDevice
	}

	// Launch the RS receiver
	rsCh := make(chan *rsMsg)
	receiverCtx, cancelReceiver := context.WithCancel(ctx)
	go func() {
		for {
			rs, addr, err := sock.recvRS(receiverCtx)
			if err != nil {
				if receiverCtx.Err() != nil {
					return
				}
				s.reportFailing(err)
				continue
			}
			rsCh <- &rsMsg{rs: rs, from: addr}
		}
	}()

	s.reportRunning()

reload:
	for {
		// RA message
		msg := s.createRAMsg(config, &devState)

		// For unsolicited RA
		ticker := time.NewTicker(time.Duration(config.RAIntervalMilliseconds) * time.Millisecond)

		for {
			select {
			case rs := <-rsCh:
				// Reply to RS
				//
				// TODO: Rate limit this to mitigate RS flooding attack
				err := sock.sendRA(ctx, rs.from, msg)
				if err != nil {
					s.reportFailing(err)
					continue
				}
				s.incTxStat(true)
				s.reportRunning()
			case <-ticker.C:
				// Send unsolicited RA
				err := sock.sendRA(ctx, netip.IPv6LinkLocalAllNodes(), msg)
				if err != nil {
					s.reportFailing(err)
					continue
				}
				s.incTxStat(false)
				s.reportRunning()
			case newConfig := <-s.reloadCh:
				if reflect.DeepEqual(config, newConfig) {
					s.logger.Info("No configuration change. Skip reloading.")
					continue
				}
				config = newConfig
				s.reportReloading()
				s.setLastUpdate()
				continue reload
			case dev := <-devCh:
				// Save the old address for comparison
				oldAddr := devState.addr

				// Update the device state
				devState = dev

				// Device is stopped. Stop the advertisement
				// and wait for the device to be up again.
				if !devState.isUp {
					cancelReceiver()
					s.reportFailing(fmt.Errorf("device is down"))
					goto waitDevice
				}

				// Device address has changed. We need to
				// change the Link Layer Address option in the
				// RA message. Reload internally.
				if !slices.Equal(oldAddr, dev.addr) {
					s.reportReloading()
					continue reload
				}
			case <-ctx.Done():
				s.reportStopped(ctx.Err())
				break reload
			case <-s.stopCh:
				s.reportStopped(nil)
				break reload
			}
		}
	}

	cancelReceiver()
	sock.close()
}

func (s *advertiser) status() *InterfaceStatus {
	s.ifaceStatusLock.RLock()
	defer s.ifaceStatusLock.RUnlock()
	return s.ifaceStatus.deepCopy()
}

func (s *advertiser) reload(ctx context.Context, newConfig *InterfaceConfig) error {
	select {
	case s.reloadCh <- newConfig:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (s *advertiser) stop() {
	close(s.stopCh)
}
