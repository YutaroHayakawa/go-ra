package radv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"reflect"
	"time"

	"github.com/mdlayher/ndp"
	"github.com/sethvargo/go-retry"
	"golang.org/x/sys/unix"
)

type raSender struct {
	logger *slog.Logger

	initialConfig *InterfaceConfig
	reloadCh      chan *InterfaceConfig
	stopCh        chan any
	sock          rAdvSocket
	socketCtor    rAdvSocketCtor
}

func newRASender(initialConfig *InterfaceConfig, ctor rAdvSocketCtor, logger *slog.Logger) *raSender {
	return &raSender{
		logger:        logger.With(slog.String("interface", initialConfig.Name)),
		initialConfig: initialConfig,
		reloadCh:      make(chan *InterfaceConfig),
		stopCh:        make(chan any),
		socketCtor:    ctor,
	}
}

func (s *raSender) run(ctx context.Context) {
	// The current desired configuration
	config := s.initialConfig

	// Create the socket
	err := retry.Constant(ctx, time.Second, func(ctx context.Context) error {
		var err error

		s.sock, err = s.socketCtor(config.Name)
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

reload:
	for {
		msg := &ndp.RouterAdvertisement{
			// TODO: Make this configurable
			RouterLifetime: 1800 * time.Second,
		}

		// For unsolicited RA
		ticker := time.NewTicker(time.Duration(config.RAIntervalMilliseconds) * time.Millisecond)

		for {
			select {
			case <-ticker.C:
				err := s.sock.sendRA(ctx, netip.IPv6LinkLocalAllNodes(), msg)
				if err != nil {
					continue
				}
			case newConfig := <-s.reloadCh:
				if reflect.DeepEqual(config, newConfig) {
					s.logger.Info("No configuration change. Skip reloading.")
					continue
				}
				config = newConfig
				s.logger.Info("Configuration changed. Reloading.")
				continue reload
			case <-ctx.Done():
				s.logger.Info("Context is done. Stopping.")
				break reload
			case <-s.stopCh:
				s.logger.Info("Stop event received. Stopping.")
				break reload
			}
		}
	}

	s.sock.close()
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
