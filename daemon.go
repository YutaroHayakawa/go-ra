// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package ra

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// Daemon is the main struct for the ra daemon
type Daemon struct {
	initialConfig     *Config
	reloadCh          chan *Config
	logger            *slog.Logger
	socketConstructor socketCtor

	advertisers     map[string]*advertiser
	advertisersLock sync.RWMutex
}

// NewDaemon creates a new Daemon instance with the provided configuration and
// options. It returns ValidationErrors if the configuration is invalid.
func NewDaemon(config *Config, opts ...DaemonOption) (*Daemon, error) {
	// Take a copy of the new configuration. c.validate() will modify it to
	// set default values.
	c := config.deepCopy()

	// Validate the configuration first
	if err := c.defaultAndValidate(); err != nil {
		return nil, err
	}

	d := &Daemon{
		initialConfig:     c,
		reloadCh:          make(chan *Config),
		logger:            slog.Default(),
		socketConstructor: newSocket,
		advertisers:       map[string]*advertiser{},
	}

	for _, opt := range opts {
		opt(d)
	}

	return d, nil
}

// Run starts the daemon and blocks until the context is cancelled
func (d *Daemon) Run(ctx context.Context) {
	d.logger.Info("Starting daemon")

	// Current desired configuration
	config := d.initialConfig

reload:
	// Main loop
	for {
		var (
			toAdd    []*InterfaceConfig
			toUpdate []*advertiser
			toRemove []*advertiser
		)

		// We may modify the advertiser map from now
		d.advertisersLock.Lock()

		// Cache the interface => config mapping for later use
		ifaceConfigs := map[string]*InterfaceConfig{}

		// Find out which advertiser to add, update and remove
		for _, c := range config.Interfaces {
			if advertiser, ok := d.advertisers[c.Name]; !ok {
				toAdd = append(toAdd, c)
			} else {
				toUpdate = append(toUpdate, advertiser)
			}
			ifaceConfigs[c.Name] = c
		}
		for name, advertiser := range d.advertisers {
			if _, ok := ifaceConfigs[name]; !ok {
				toRemove = append(toRemove, advertiser)
			}
		}

		// Add new per-interface jobs
		for _, c := range toAdd {
			d.logger.Info("Adding new RA sender", slog.String("interface", c.Name))
			sender := newAdvertiser(c, d.socketConstructor, d.logger)
			go sender.run(ctx)
			d.advertisers[c.Name] = sender
		}

		// Update (reload) existing workers
		for _, advertiser := range toUpdate {
			iface := advertiser.initialConfig.Name
			d.logger.Info("Updating RA sender", slog.String("interface", iface))
			// Set timeout to guarantee progress
			timeout, cancelTimeout := context.WithTimeout(ctx, time.Second*3)
			advertiser.reload(timeout, ifaceConfigs[iface])
			cancelTimeout()
		}

		// Remove unnecessary workers
		for _, advertiser := range toRemove {
			iface := advertiser.initialConfig.Name
			d.logger.Info("Deleting RA sender", slog.String("interface", iface))
			advertiser.stop()
			delete(d.advertisers, iface)
		}

		d.advertisersLock.Unlock()

		// Wait for the events
		for {
			select {
			case newConfig := <-d.reloadCh:
				d.logger.Info("Reloading configuration")
				config = newConfig
				continue reload
			case <-ctx.Done():
				d.logger.Info("Shutting down daemon")
				return
			}
		}
	}
}

// Reload reloads the configuration of the daemon. The context passed to this
// function is used to cancel the potentially long-running operations during
// the reload process. Currently, the result of the unsucecssful or cancelled
// reload is undefined and the daemon may be running with either the old or the
// new configuration or both. It returns ValidationErrors if the configuration
// is invalid.
func (d *Daemon) Reload(ctx context.Context, newConfig *Config) error {
	// Take a copy of the new configuration. c.validate() will modify it to
	// set default values.
	c := newConfig.deepCopy()

	if err := c.defaultAndValidate(); err != nil {
		return err
	}

	select {
	case d.reloadCh <- c:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// Status returns the current status of the daemon
func (d *Daemon) Status() *Status {
	d.advertisersLock.RLock()

	ifaceStatus := []*InterfaceStatus{}
	for _, advertiser := range d.advertisers {
		ifaceStatus = append(ifaceStatus, advertiser.status())
	}

	d.advertisersLock.RUnlock()

	sort.Slice(ifaceStatus, func(i, j int) bool {
		return ifaceStatus[i].Name < ifaceStatus[j].Name
	})

	return &Status{Interfaces: ifaceStatus}
}

// DaemonOption is an optional parameter for the Daemon constructor
type DaemonOption func(*Daemon)

// WithLogger overrides the default logger with the provided one.
func WithLogger(l *slog.Logger) DaemonOption {
	return func(d *Daemon) {
		d.logger = l
	}
}

// withSocketConstructor overrides the default socket constructor with the
// provided one. For testing purposes only.
func withSocketConstructor(c socketCtor) DaemonOption {
	return func(d *Daemon) {
		d.socketConstructor = c
	}
}
