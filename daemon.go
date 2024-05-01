package radv

import (
	"context"
	"log/slog"
)

// Daemon is the main struct for the radv daemon
type Daemon struct {
	config *Config
	logger *slog.Logger
}

// New creates a new Daemon instance with the provided configuration and options
func New(c *Config, opts ...DaemonOption) (*Daemon, error) {
	// Validate the configuration first
	if err := c.validate(); err != nil {
		return nil, err
	}

	d := &Daemon{
		config: c,
		logger: slog.Default(),
	}

	for _, opt := range opts {
		if err := opt(d); err != nil {
			return nil, err
		}
	}

	return &Daemon{
		config: c,
	}, nil
}

// Run starts the daemon and blocks until the context is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	return nil
}

// Reload reloads the configuration of the daemon. The context passed to this
// function is used to cancel the potentially long-running operations during
// the reload process. Currently, the result of the unsucecssful or cancelled
// reload is undefined and the daemon may be running with either the old or the
// new configuration or both.
func (d *Daemon) Reload(ctx context.Context) error {
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
