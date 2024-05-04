package radv

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

type raSender struct {
	logger *slog.Logger

	initialConfig *InterfaceConfig

	// We use mutex-based synchronization instead of channels because
	// status must be reported even when the main loop is hanging.
	status     *InterfaceStatus
	statusLock sync.RWMutex

	reloadCh   chan *InterfaceConfig
	stopCh     chan any
	sock       rAdvSocket
	socketCtor rAdvSocketCtor
}

type raSenderStatusReq struct {
	ctx   context.Context
	resCh chan *InterfaceStatus
}

func newRASender(initialConfig *InterfaceConfig, ctor rAdvSocketCtor, logger *slog.Logger) *raSender {
	return &raSender{
		logger:        logger.With(slog.String("interface", initialConfig.Name)),
		initialConfig: initialConfig,
		status:        &InterfaceStatus{Name: initialConfig.Name, State: "Unknown"},
		reloadCh:      make(chan *InterfaceConfig),
		stopCh:        make(chan any),
		socketCtor:    ctor,
	}
}

func (s *raSender) reportRunning() {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status.State = Running
	s.status.Message = ""
}

func (s *raSender) reportReloading() {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status.State = Reloading
	s.status.Message = ""
}

func (s *raSender) reportFailing(err error) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status.State = Failing
	if err == nil {
		s.status.Message = ""
	} else {
		s.status.Message = err.Error()
	}
}

func (s *raSender) reportStopped(err error) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status.State = Stopped
	if err == nil {
		s.status.Message = ""
	} else {
		s.status.Message = err.Error()
	}
}

func (s *raSender) incTxStat() {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status.TxUnsolicitedRA++
}

func (s *raSender) setLastUpdate() {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status.LastUpdate = time.Now().Unix()
}

func (s *raSender) resetStat() {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status.TxUnsolicitedRA = 0
}

func (s *raSender) run(ctx context.Context) {
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

	s.reportRunning()

reload:
	for {
		msg := &ndp.RouterAdvertisement{
			Options: []ndp.Option{
				&ndp.LinkLayerAddress{
					Direction: ndp.Source,
					Addr:      s.sock.hardwareAddr(),
				},
			},
		}

		// For unsolicited RA
		ticker := time.NewTicker(time.Duration(config.RAIntervalMilliseconds) * time.Millisecond)

		for {
			select {
			case <-ticker.C:
				err := s.sock.sendRA(ctx, netip.IPv6LinkLocalAllNodes(), msg)
				if err != nil {
					s.reportFailing(err)
					continue
				}
				s.incTxStat()
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

func (s *raSender) getStatus() *InterfaceStatus {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	return s.status.DeepCopy()
}

func (s *raSender) reload(ctx context.Context, newConfig *InterfaceConfig) error {
	select {
	case s.reloadCh <- newConfig:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (s *raSender) stop() {
	close(s.stopCh)
}
