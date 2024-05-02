package radv

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var (
	RAIntervalMillisecondsDefault = 600000 // 600 seconds

	ErrDuplicateInterfaceName = errors.New("duplicate interface name")
)

// ParameterError represents an error in a configuration parameter
type ParameterError struct {
	// The name of the problematic parameter
	Parameter string
	// The error message
	Message string
}

var _ error = (*ParameterError)(nil)

func (a *ParameterError) Is(b error) bool {
	var e *ParameterError
	if !errors.As(b, &e) {
		return false
	}
	return a.Parameter == e.Parameter && a.Message == e.Message
}

func (e *ParameterError) Error() string {
	return fmt.Sprintf("%s: %s", e.Parameter, e.Message)
}

// Config represents the configuration of the daemon
type Config struct {
	// Interface-specific configuration parameters
	Interfaces []InterfaceConfig `yaml:"interfaces"`
}

func (c *Config) validate() error {
	if c == nil {
		// No configuration is a valid configuration
		return nil
	}

	ifaces := map[string]struct{}{}
	for _, p := range c.Interfaces {
		if _, ok := ifaces[p.Name]; ok {
			return ErrDuplicateInterfaceName
		}

		if err := p.validate(); err != nil {
			return fmt.Errorf("interface %s has an invalid parameter: %w", p.Name, err)
		}

		ifaces[p.Name] = struct{}{}
	}

	return nil
}

// InterfaceConfig represents the interface-specific configuration parameters
type InterfaceConfig struct {
	// Interface name. Must be unique within the configuration.
	Name string `yaml:"name"`
	// Interval between sending unsolicited RA. Must be >= 70 and <= 1800000.
	RAIntervalMilliseconds int `yaml:"raIntervalMilliseconds"`
}

// applyDefaults applies the default values to the missing parameters
func (p *InterfaceConfig) applyDefaults() {
	if p == nil {
		return
	}
	if p.RAIntervalMilliseconds == 0 {
		p.RAIntervalMilliseconds = RAIntervalMillisecondsDefault
	}
}

func (p *InterfaceConfig) validate() error {
	if p == nil {
		return fmt.Errorf("nil parameter")
	}

	// Fill missing parameters before validation
	p.applyDefaults()

	if p.Name == "" {
		return &ParameterError{"Name", "must be a valid interface name"}
	}

	// The lower bound violates RFC 4861, but like FRRouting does, we allow
	// it for the fast convergence in the BGP Unnumbered use case. The
	// upper bound comes from RFC 4861.
	if p.RAIntervalMilliseconds < 70 || p.RAIntervalMilliseconds > 1800000 {
		return &ParameterError{"RAIntervalMilliseconds", "must be >= 70 and <= 1800000"}
	}

	return nil
}

func ParseConfigFile(path string) (*Config, error) {
	var c Config

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open config file: %w", err)
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(&c); err != nil {
		return nil, fmt.Errorf("cannot parse config file: %w", err)
	}

	return &c, nil
}
