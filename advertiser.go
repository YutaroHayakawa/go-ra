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
	"sync"
	"time"

	"github.com/mdlayher/ndp"
	"github.com/sethvargo/go-retry"
	"golang.org/x/sys/unix"
)

type advertiser struct {
	logger *slog.Logger

	initialConfig *InterfaceConfig

	// We use mutex-based synchronization instead of channels because
	// ifaceStatus must be reported even when the main loop is hanging.
	ifaceStatus     *InterfaceStatus
	ifaceStatusLock sync.RWMutex

	reloadCh   chan *InterfaceConfig
	stopCh     chan any
	sock       socket
	socketCtor socketCtor
}

// An internal structure to represent RS
type rsMsg struct {
	rs   *ndp.RouterSolicitation
	from netip.Addr
}

func newAdvertiser(initialConfig *InterfaceConfig, ctor socketCtor, logger *slog.Logger) *advertiser {
	return &advertiser{
		logger:        logger.With(slog.String("interface", initialConfig.Name)),
		initialConfig: initialConfig,
		ifaceStatus:   &InterfaceStatus{Name: initialConfig.Name, State: "Unknown"},
		reloadCh:      make(chan *InterfaceConfig),
		stopCh:        make(chan any),
		socketCtor:    ctor,
	}
}

func (s *advertiser) createRAMsg(config *InterfaceConfig) *ndp.RouterAdvertisement {
	msg := &ndp.RouterAdvertisement{
		CurrentHopLimit:           uint8(config.CurrentHopLimit),
		ManagedConfiguration:      config.Managed,
		OtherConfiguration:        config.Other,
		RouterSelectionPreference: s.toNDPPreference(config.Preference),
		RouterLifetime:            time.Duration(config.RouterLifetimeSeconds) * time.Second,
		ReachableTime:             time.Duration(config.ReachableTimeMilliseconds) * time.Millisecond,
		RetransmitTimer:           time.Duration(config.RetransmitTimeMilliseconds) * time.Millisecond,
		Options:                   s.createOptions(config),
	}

	for _, prefix := range config.Prefixes {
		// At this point, we should have validated the
		// configuration. If we haven't, it's a bug.
		p := netip.MustParsePrefix(prefix.Prefix)
		msg.Options = append(msg.Options, &ndp.PrefixInformation{
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
		msg.Options = append(msg.Options, &ndp.RouteInformation{
			PrefixLength:  uint8(p.Bits()),
			Preference:    s.toNDPPreference(route.Preference),
			RouteLifetime: time.Second * time.Duration(route.LifetimeSeconds),
			Prefix:        p.Addr(),
		})
	}

	return msg
}

func (s *advertiser) createOptions(config *InterfaceConfig) []ndp.Option {
	options := []ndp.Option{
		&ndp.LinkLayerAddress{
			Direction: ndp.Source,
			Addr:      s.sock.hardwareAddr(),
		},
	}

	if config.MTU > 0 {
		options = append(options, &ndp.MTU{
			MTU: uint32(config.MTU),
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

	// Set a timestamp for the first "update"
	s.setLastUpdate()

	// Create the socket
	err := retry.Constant(ctx, time.Second, func(ctx context.Context) error {
		var err error

		s.sock, err = s.socketCtor(config.Name)
		if err != nil {
			// These are the unrecoverable errors we're aware of now.
			if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EINVAL) {
				return fmt.Errorf("cannot create socket: %w", err)
			}

			s.reportFailing(err)

			return retry.RetryableError(err)
		}

		return nil
	})
	if err != nil {
		s.reportStopped(err)
		return
	}

	// Launch the RS receiver
	rsCh := make(chan *rsMsg)
	go func() {
		for {
			rs, addr, err := s.sock.recvRS(ctx)
			if err != nil {
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
		msg := s.createRAMsg(config)

		// For unsolicited RA
		ticker := time.NewTicker(time.Duration(config.RAIntervalMilliseconds) * time.Millisecond)

		for {
			select {
			case rs := <-rsCh:
				// Reply to RS
				//
				// TODO: Rate limit this to mitigate RS flooding attack
				err := s.sock.sendRA(ctx, rs.from, msg)
				if err != nil {
					s.reportFailing(err)
					continue
				}
				s.incTxStat(true)
				s.reportRunning()
			case <-ticker.C:
				// Send unsolicited RA
				err := s.sock.sendRA(ctx, netip.IPv6LinkLocalAllNodes(), msg)
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
			case <-ctx.Done():
				s.reportStopped(ctx.Err())
				break reload
			case <-s.stopCh:
				s.reportStopped(nil)
				break reload
			}
		}

	}

	s.sock.close()
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
