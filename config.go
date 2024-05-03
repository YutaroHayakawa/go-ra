package radv

import (
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

// Config represents the configuration of the daemon
type Config struct {
	// Interface-specific configuration parameters. The Name field must be
	// unique within the slice. The slice itself and elements must not be
	// nil.
	Interfaces []*InterfaceConfig `yaml:"interfaces" json:"interfaces" validate:"required,non_nil_and_unique_name,dive,required"`
}

// InterfaceConfig represents the interface-specific configuration parameters
type InterfaceConfig struct {
	// Required: Network interface name. Must be unique within the configuration.
	Name string `yaml:"name" json:"name" validate:"required"`
	// Interval between sending unsolicited RA. Must be >= 70 and <= 1800000. Default is 600000.
	// The upper bound is chosen to be compliant with RFC4861. The lower bound is intentionally
	// chosen to be lower than RFC4861 for faster convergence. If you don't wish to overwhelm the
	// network, and wish to be compliant with RFC4861, set to higher than 3000 as RFC4861 suggests.
	RAIntervalMilliseconds int `yaml:"raIntervalMilliseconds" json:"raIntervalMilliseconds" validate:"required,gte=70,lte=1800000" default:"600000"`
}

type ValidationErrors = validator.ValidationErrors

func (c *Config) defaultAndValidate() error {
	if err := defaults.Set(c); err != nil {
		panic("BUG (Please report 🙏): Defaulting failed: " + err.Error())
	}

	validate := validator.New(validator.WithRequiredStructEnabled())

	// Adhoc custom validator which validates the slice elements are not
	// nil AND the Name field is unique. As far as I know, there is no way
	// to validate the uniqueness of struct fields in the nil-able slice of
	// struct pointerrs with validator's built-in constraints.
	validate.RegisterValidation("non_nil_and_unique_name", func(fl validator.FieldLevel) bool {
		names := make(map[string]struct{})

		ifaceSlice := fl.Field()
		for i := 0; i < fl.Field().Len(); i++ {
			ifacep := ifaceSlice.Index(i)
			if ifacep.IsNil() {
				return false
			}

			if ifacep.IsNil() {
				return false
			}

			iface := ifacep.Elem()

			name := iface.FieldByName("Name")
			if _, ok := names[name.String()]; ok {
				return false
			} else {
				names[name.String()] = struct{}{}
			}
		}

		return true
	})

	if err := validate.Struct(c); err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			panic("BUG (Please report 🙏): Invalid validation: " + err.Error())
		}

		var verrs ValidationErrors
		if errors.As(err, &verrs) {
			return verrs
		}

		// This is impossible, according to the validator's documentation
		// https://pkg.go.dev/github.com/go-playground/validator/v10#hdr-Validation_Functions_Return_Type_error
		return err
	}

	return nil
}

func ParseConfigJSON(r io.Reader) (*Config, error) {
	var c Config

	if err := json.NewDecoder(r).Decode(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

func ParseConfigYAML(r io.Reader) (*Config, error) {
	var c Config

	if err := yaml.NewDecoder(r).Decode(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

func ParseConfigFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseConfigYAML(f)
}
