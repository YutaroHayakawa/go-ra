// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package ra

import (
	"bytes"
	"os"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
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
		f, err := os.CreateTemp(".", "ra-test")
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
		{
			name: "CurrentHopLimit < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						CurrentHopLimit:        -1,
					},
				},
			},
			expectError: true,
			errorField:  "CurrentHopLimit",
			errorTag:    "gte",
		},
		{
			name: "CurrentHopLimit > 255",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						CurrentHopLimit:        256,
					},
				},
			},
			expectError: true,
			errorField:  "CurrentHopLimit",
			errorTag:    "lte",
		},
		{
			name: "RouterLifetimeSeconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RouterLifetimeSeconds:  -1,
					},
				},
			},
			expectError: true,
			errorField:  "RouterLifetimeSeconds",
			errorTag:    "gte",
		},
		{
			name: "RouterLifetimeSeconds > 65535",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RouterLifetimeSeconds:  65536,
					},
				},
			},
			expectError: true,
			errorField:  "RouterLifetimeSeconds",
			errorTag:    "lte",
		},
		{
			name: "ReachableTimeMilliseconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                      "net0",
						RAIntervalMilliseconds:    1000,
						ReachableTimeMilliseconds: -1,
					},
				},
			},
			expectError: true,
			errorField:  "ReachableTimeMilliseconds",
			errorTag:    "gte",
		},
		{
			name: "ReachableTimeMilliseconds > 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                      "net0",
						RAIntervalMilliseconds:    1000,
						ReachableTimeMilliseconds: 4294967296,
					},
				},
			},
			expectError: true,
			errorField:  "ReachableTimeMilliseconds",
			errorTag:    "lte",
		},
		{
			name: "RetransmitTimeMilliseconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                       "net0",
						RAIntervalMilliseconds:     1000,
						RetransmitTimeMilliseconds: -1,
					},
				},
			},
			expectError: true,
			errorField:  "RetransmitTimeMilliseconds",
			errorTag:    "gte",
		},
		{
			name: "RetransmitTimeMilliseconds > 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                       "net0",
						RAIntervalMilliseconds:     1000,
						RetransmitTimeMilliseconds: 4294967296,
					},
				},
			},
			expectError: true,
			errorField:  "RetransmitTimeMilliseconds",
			errorTag:    "lte",
		},
		{
			name: "MTU > 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						MTU:                    -1,
					},
				},
			},
			expectError: true,
			errorField:  "MTU",
			errorTag:    "gte",
		},
		{
			name: "MTU > 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						MTU:                    4294967296,
					},
				},
			},
			expectError: true,
			errorField:  "MTU",
			errorTag:    "lte",
		},
		{
			name: "Nil PrefixConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes:               nil,
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty PrefixConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes:               []*PrefixConfig{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Nil PrefixConfig Element",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes:               []*PrefixConfig{nil},
					},
				},
			},
			expectError: true,
			errorField:  "Prefixes",
			errorTag:    "non_nil_and_non_overlapping_prefix",
		},
		{
			name: "No Prefix",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								OnLink: true,
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Prefix",
			errorTag:    "required",
		},
		{
			name: "Overlapping Prefix",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix: "2001:db8::/32",
							},
							{
								Prefix: "2001:db8::/64",
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Prefixes",
			errorTag:    "non_nil_and_non_overlapping_prefix",
		},
		{
			name: "ValidLifetimeSeconds = 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:               "2001:db8::/64",
								ValidLifetimeSeconds: ptr.To(4294967295),
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "ValidLifetimeSeconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:               "2001:db8::/64",
								ValidLifetimeSeconds: ptr.To(-1),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "ValidLifetimeSeconds",
			errorTag:    "gte",
		},
		{
			name: "ValidLifetimeSeconds > 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:               "2001:db8::/64",
								ValidLifetimeSeconds: ptr.To(4294967296),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "ValidLifetimeSeconds",
			errorTag:    "lte",
		},
		{
			name: "PreferredLifetimeSeconds = 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:                   "2001:db8::/64",
								ValidLifetimeSeconds:     ptr.To(4294967295), // PreferredLifetimeSeconds must be less than ValidLifetimeSeconds
								PreferredLifetimeSeconds: ptr.To(4294967295),
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "PreferredLifetimeSeconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:                   "2001:db8::/64",
								PreferredLifetimeSeconds: ptr.To(-1),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "PreferredLifetimeSeconds",
			errorTag:    "gte",
		},
		{
			name: "PreferredLifetimeSeconds > 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:                   "2001:db8::/64",
								ValidLifetimeSeconds:     ptr.To(4294967296),
								PreferredLifetimeSeconds: ptr.To(4294967296),
							},
						},
					},
				},
			},
			expectError: true,
			// PreferredLifetimeSeconds must be less than
			// ValidLifetimeSeconds, but ValdateLifetimeSeconds
			// must be <= 4294967295, so it's impossible to specify
			// PreferredLifetimeSeconds > 4294967295 actually.
			errorField: "ValidLifetimeSeconds",
			errorTag:   "lte",
		},
		{
			name: "ValidLifetimeSeconds < PreferredLifetimeSeconds",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:                   "2001:db8::/64",
								ValidLifetimeSeconds:     ptr.To(100),
								PreferredLifetimeSeconds: ptr.To(101),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "PreferredLifetimeSeconds",
			errorTag:    "ltefield",
		},
		{
			name: "Preference low && RouterLifetimeSeconds != 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Preference:             "low",
						RouterLifetimeSeconds:  1,
					},
				},
			},
		},
		{
			name: "Preference medium && RouterLifetimeSeconds != 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Preference:             "medium",
						RouterLifetimeSeconds:  1,
					},
				},
			},
		},
		{
			name: "Preference high && RouterLifetimeSeconds != 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Preference:             "high",
						RouterLifetimeSeconds:  1,
					},
				},
			},
		},
		{
			name: "Preference foo && RouterLifetimeSeconds != 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Preference:             "foo",
						RouterLifetimeSeconds:  1,
					},
				},
			},
			expectError: true,
			errorField:  "Preference",
			errorTag:    "oneof",
		},
		{
			name: "Preference == low && RouterLifetimeSeconds == 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Preference:             "low",
						RouterLifetimeSeconds:  0,
					},
				},
			},
			expectError: true,
			errorField:  "Preference",
			errorTag:    "eq_if medium RouterLifetimeSeconds 0",
		},
		{
			name: "Preference == <empty> && RouterLifetimeSeconds == 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RouterLifetimeSeconds:  0,
					},
				},
			},
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

			// Find the target error and we can ignore the rest.
			for _, v := range verr {
				if v.Field() == tt.errorField && v.Tag() == tt.errorTag {
					return
				}
			}

			require.Failf(t, "expected error not found", verr.Error())
		})
	}
}
