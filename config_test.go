package radv

import (
	"bytes"
	"os"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

func TestConfigParsers(t *testing.T) {
	yamlConf := `
interfaces:
  - name: net0
    raIntervalMilliseconds: 1000
  - name: net1
    raIntervalMilliseconds: 1000
`

	t.Run("ParseConfigYAMLFile", func(t *testing.T) {
		f, err := os.CreateTemp(".", "radv-test")
		require.NoError(t, err)
		defer os.Remove(f.Name())
		_, err = f.Write([]byte(yamlConf))
		require.NoError(t, err)
		c, err := ParseConfigYAMLFile(f.Name())
		require.NoError(t, err)
		require.NotNil(t, c)
		require.Len(t, c.Interfaces, 2)
		require.Equal(t, "net0", c.Interfaces[0].Name)
		require.Equal(t, 1000, c.Interfaces[0].RAIntervalMilliseconds)
		require.Equal(t, "net1", c.Interfaces[1].Name)
		require.Equal(t, 1000, c.Interfaces[1].RAIntervalMilliseconds)
	})

	jsonConf := `
{
	"interfaces": [
		{
			"name": "net0",
			"raIntervalMilliseconds": 1000
		},
		{
			"name": "net1",
			"raIntervalMilliseconds": 1000
		}
	]
}
`

	t.Run("ParseConfigJSON", func(t *testing.T) {
		c, err := ParseConfigJSON(bytes.NewBuffer([]byte(jsonConf)))
		require.NoError(t, err)
		require.NotNil(t, c)
		require.Len(t, c.Interfaces, 2)
		require.Equal(t, "net0", c.Interfaces[0].Name)
		require.Equal(t, 1000, c.Interfaces[0].RAIntervalMilliseconds)
		require.Equal(t, "net1", c.Interfaces[1].Name)
		require.Equal(t, 1000, c.Interfaces[1].RAIntervalMilliseconds)
	})

}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorField  string
		errorTag    string
	}{
		{
			name: "Valid Config",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
					},
					{
						Name:                   "net1",
						RAIntervalMilliseconds: 1000,
					},
				},
			},
			expectError: false,
		},
		{
			name: "Nil InterfaceConig",
			config: &Config{
				Interfaces: nil,
			},
			expectError: true,
			errorField:  "Interfaces",
			errorTag:    "required",
		},
		{
			name: "Empty InterfaceConig",
			config: &Config{
				Interfaces: []*InterfaceConfig{},
			},
			expectError: false,
		},
		{
			name: "Nil InterfaceConig Element",
			config: &Config{
				Interfaces: []*InterfaceConfig{nil},
			},
			expectError: true,
			errorField:  "Interfaces",
			errorTag:    "non_nil_and_unique_name",
		},
		{
			name: "Duplicated Interface Name",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
					},
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
					},
				},
			},
			expectError: true,
			errorField:  "Interfaces",
			errorTag:    "non_nil_and_unique_name",
		},
		{
			name: "RAIntervalMilliseconds < 70",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 69,
					},
				},
			},
			expectError: true,
			errorField:  "RAIntervalMilliseconds",
			errorTag:    "gte",
		},
		{
			name: "RAIntervalMilliseconds > 1800000",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1800001,
					},
				},
			},
			expectError: true,
			errorField:  "RAIntervalMilliseconds",
			errorTag:    "lte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.defaultAndValidate()
			if !tt.expectError {
				require.NoError(t, err)
				return
			}
			var verr validator.ValidationErrors
			require.ErrorAs(t, err, &verr)
			require.Len(t, verr, 1)
			require.Equal(t, tt.errorField, verr[0].Field())
			require.Equal(t, tt.errorTag, verr[0].Tag())
		})
	}
}
