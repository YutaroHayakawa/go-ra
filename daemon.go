package radv

import (
	"context"
	"log/slog"
	"time"
)

// Daemon is the main struct for the radv daemon
type Daemon struct {
	initialConfig     Config
	reloadCh          chan Config
	stopCh            any
	logger            *slog.Logger
	socketConstructor rAdvSocketCtor
}

// New creates a new Daemon instance with the provided configuration and options
func New(c Config, opts ...DaemonOption) (*Daemon, error) {
	// Validate the configuration first
	if err := c.validate(); err != nil {
		return nil, err
	}

	d := &Daemon{
		initialConfig: c,
		reloadCh:      make(chan Config),
		logger:        slog.Default(),
	}

	for _, opt := range opts {
		if err := opt(d); err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (d *Daemon) Run(ctx context.Context) error {
	// Current desired configuration
	config := d.initialConfig

	// Interface to raSender mapping
	raSenders := map[string]*raSender{}

	// Main loop
	for {
		var (
			toAdd    []InterfaceConfig
			toUpdate []*raSender
			toRemove []*raSender
		)

		// Cache the interface => config mapping for later use
		ifaceConfigs := map[string]InterfaceConfig{}

		// Find out which raSender to add, update and remove
		for _, c := range config.Interfaces {
			if raSender, ok := raSenders[c.Name]; !ok {
				toAdd = append(toAdd, c)
			} else {
				toUpdate = append(toUpdate, raSender)
			}
			ifaceConfigs[c.Name] = c
		}
		for name, raSender := range raSenders {
			if _, ok := ifaceConfigs[name]; !ok {
				toRemove = append(toRemove, raSender)
			}
		}

		// Add new per-interface jobs
		for _, c := range toAdd {
			d.logger.Info("Adding new RA sender", slog.String("interface", c.Name))
			sender := newRASender(c, d.socketConstructor, d.logger)
			go sender.run(ctx)
			raSenders[c.Name] = sender
		}

		// Update (reload) existing workers
		for _, raSender := range toUpdate {
			iface := raSender.initialConfig.Name
			d.logger.Info("Updating RA sender", slog.String("interface", iface))
			// Set timeout to guarantee progress
			timeout, cancel := context.WithTimeout(ctx, time.Second*3)
			raSender.reload(timeout, ifaceConfigs[iface])
			cancel()
		}

		// Remove unnecessary workers
		for _, raSender := range toRemove {
			iface := raSender.initialConfig.Name
			d.logger.Info("Deleting RA sender", slog.String("interface", iface))
			raSender.stop()
			delete(raSenders, iface)
		}

		// Wait for the events
		select {
		case newConfig := <-d.reloadCh:
			d.logger.Info("Reloading configuration")
			config = newConfig
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Reload reloads the configuration of the daemon. The context passed to this
// function is used to cancel the potentially long-running operations during
// the reload process. Currently, the result of the unsucecssful or cancelled
// reload is undefined and the daemon may be running with either the old or the
// new configuration or both.
func (d *Daemon) Reload(ctx context.Context, newConfig Config) error {
	select {
	case d.reloadCh <- newConfig:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// DaemonOption is an optional parameter for the Daemon constructor
type DaemonOption func(*Daemon) error

// WithLogger overrides the default logger with the provided one.
func WithLogger(l *slog.Logger) DaemonOption {
	return func(d *Daemon) error {
		d.logger = l
		return nil
	}
}

// withSocketConstructor overrides the default socket constructor with the
// provided one. For testing purposes only.
func withSocketConstructor(c rAdvSocketCtor) DaemonOption {
	return func(d *Daemon) error {
		d.socketConstructor = c
		return nil
	}
}
