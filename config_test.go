package radv

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigParsers(t *testing.T) {
	conf := `
interfaces:
  - name: net0
    raIntervalMilliseconds: 1000
  - name: net1
    raIntervalMilliseconds: 1000
`

	t.Run("ParseConfigFile", func(t *testing.T) {
		f, err := os.CreateTemp(".", "radv-test")
		require.NoError(t, err)
		defer os.Remove(f.Name())
		_, err = f.Write([]byte(conf))
		require.NoError(t, err)
		c, err := ParseConfigFile(f.Name())
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
		name          string
		config        *Config
		expectedError error
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
			expectedError: nil,
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
			expectedError: ErrDuplicateInterfaceName,
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
			expectedError: &ParameterError{"RAIntervalMilliseconds", "must be >= 70 and <= 1800000"},
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
			expectedError: &ParameterError{"RAIntervalMilliseconds", "must be >= 70 and <= 1800000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.expectedError == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.expectedError)
		})
	}
}
