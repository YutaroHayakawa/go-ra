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
	reloadCh      chan *InterfaceConfig
	stopCh        any
	sock          rAdvSocket

	childWg       *sync.WaitGroup
	childReloadCh []chan *InterfaceConfig
	childStopCh   []chan any

	socketCtor rAdvSocketCtor
}

func newRASender(initialConfig *InterfaceConfig, ctor rAdvSocketCtor, logger *slog.Logger) *raSender {
	return &raSender{
		logger:        logger.With(slog.String("interface", initialConfig.Name)),
		initialConfig: initialConfig,
		reloadCh:      make(chan *InterfaceConfig),
		stopCh:        make(chan any),
		childWg:       &sync.WaitGroup{},
		childReloadCh: []chan *InterfaceConfig{},
		childStopCh:   []chan any{},
		socketCtor:    ctor,
	}
}

func (s *raSender) run(ctx context.Context) {
	// Create the socket
	err := retry.Constant(ctx, time.Second, func(ctx context.Context) error {
		var err error

		s.sock, err = s.socketCtor(s.initialConfig.Name)
		if err != nil {
			// These are the unrecoverable errors we're aware of now.
			if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EINVAL) {
				return fmt.Errorf("cannot create socket: %w", err)
			}

			return retry.RetryableError(err)
		}

		return nil
	})
	if err != nil {
		return
	}

	s.spawnChild(ctx, s.runUnsolicitedRASender)
	s.childWg.Wait()
	s.sock.close()
}

func (s *raSender) reload(ctx context.Context, newConfig *InterfaceConfig) error {
	for _, ch := range s.childReloadCh {
		select {
		case ch <- newConfig:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *raSender) stop() {
	for _, ch := range s.childStopCh {
		close(ch)
	}
}

func (s *raSender) spawnChild(ctx context.Context, f func(context.Context, chan *InterfaceConfig, chan any)) {
	s.childWg.Add(1)
	reloadCh := make(chan *InterfaceConfig)
	stopCh := make(chan any)
	s.childReloadCh = append(s.childReloadCh, reloadCh)
	s.childStopCh = append(s.childStopCh, stopCh)
	go f(ctx, reloadCh, stopCh)
}

func (s *raSender) runUnsolicitedRASender(ctx context.Context, reloadCh chan *InterfaceConfig, stopCh chan any) {
	defer s.childWg.Done()

	// The current desired configuration
	config := s.initialConfig

reload:
	for {
		msg := &ndp.RouterAdvertisement{
			// TODO: Make this configurable
			RouterLifetime: 1800 * time.Second,
		}

		ticker := time.NewTicker(time.Duration(config.RAIntervalMilliseconds) * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				err := s.sock.sendRA(ctx, netip.IPv6LinkLocalAllNodes(), msg)
				if err != nil {
					continue
				}
			case newConfig := <-reloadCh:
				if reflect.DeepEqual(config, newConfig) {
					s.logger.Info("No configuration change. Skip reloading.")
					continue
				}
				config = newConfig
				s.logger.Info("Configuration changed. Reloading.")
				continue reload
			case <-ctx.Done():
				s.logger.Info("Context is done. Stopping.")
				return
			case <-stopCh:
				s.logger.Info("Stop event received. Stopping.")
				return
			}
		}
	}
}
