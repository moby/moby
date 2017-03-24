// +build !windows

package config

import (
	"testing"

	"github.com/docker/docker/api/types"
)

func TestCommonUnixValidateConfigurationErrors(t *testing.T) {
	testCases := []struct {
		config *Config
	}{
		// Can't override the stock runtime
		{
			config: &Config{
				CommonUnixConfig: CommonUnixConfig{
					Runtimes: map[string]types.Runtime{
						StockRuntimeName: {},
					},
				},
			},
		},
		// Default runtime should be present in runtimes
		{
			config: &Config{
				CommonUnixConfig: CommonUnixConfig{
					Runtimes: map[string]types.Runtime{
						"foo": {},
					},
					DefaultRuntime: "bar",
				},
			},
		},
	}
	for _, tc := range testCases {
		err := Validate(tc.config)
		if err == nil {
			t.Fatalf("expected error, got nil for config %v", tc.config)
		}
	}
}
