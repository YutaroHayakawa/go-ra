package radv

import (
	"context"
	"log/slog"
	"time"
)

// Daemon is the main struct for the radv daemon
type Daemon struct {
	initialConfig     *Config
	reloadCh          chan *Config
	stopCh            any
	statusCh          chan chan *Status
	logger            *slog.Logger
	socketConstructor rAdvSocketCtor
}

// NewDaemon creates a new Daemon instance with the provided configuration and options
func NewDaemon(config *Config, opts ...DaemonOption) (*Daemon, error) {
	// Take a copy of the new configuration. c.validate() will modify it to
	// set default values.
	c := config.DeepCopy()

	// Validate the configuration first
	if err := c.defaultAndValidate(); err != nil {
		return nil, err
	}

	d := &Daemon{
		initialConfig:     c,
		reloadCh:          make(chan *Config),
		statusCh:          make(chan chan *Status),
		logger:            slog.Default(),
		socketConstructor: newRAdvSocket,
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

	// Interface to raSender mapping
	raSenders := map[string]*raSender{}

reload:
	// Main loop
	for {
		var (
			toAdd    []*InterfaceConfig
			toUpdate []*raSender
			toRemove []*raSender
		)

		// Cache the interface => config mapping for later use
		ifaceConfigs := map[string]*InterfaceConfig{}

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
			timeout, cancelTimeout := context.WithTimeout(ctx, time.Second*3)
			raSender.reload(timeout, ifaceConfigs[iface])
			cancelTimeout()
		}

		// Remove unnecessary workers
		for _, raSender := range toRemove {
			iface := raSender.initialConfig.Name
			d.logger.Info("Deleting RA sender", slog.String("interface", iface))
			raSender.stop()
			delete(raSenders, iface)
		}

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
			case resCh := <-d.statusCh:
				ifaceStatus := d.raSenderStatus(ctx, raSenders)
				resCh <- &Status{Interfaces: ifaceStatus}
			}
		}
	}
}

func (d *Daemon) raSenderStatus(ctx context.Context, raSenders map[string]*raSender) []*InterfaceStatus {
	ifaceStatus := []*InterfaceStatus{}
	for _, raSender := range raSenders {
		ifaceStatus = append(ifaceStatus, raSender.status())
	}
	return ifaceStatus
}

// Reload reloads the configuration of the daemon. The context passed to this
// function is used to cancel the potentially long-running operations during
// the reload process. Currently, the result of the unsucecssful or cancelled
// reload is undefined and the daemon may be running with either the old or the
// new configuration or both.
func (d *Daemon) Reload(ctx context.Context, newConfig *Config) error {
	// Take a copy of the new configuration. c.validate() will modify it to
	// set default values.
	c := newConfig.DeepCopy()

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
	resCh := make(chan *Status)
	d.statusCh <- resCh
	return <-resCh
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
func withSocketConstructor(c rAdvSocketCtor) DaemonOption {
	return func(d *Daemon) {
		d.socketConstructor = c
	}
}
