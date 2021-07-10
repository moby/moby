// +build !windows

package config // import "github.com/docker/docker/daemon/config"

import (
	"testing"

	"github.com/docker/docker/api/types"
)

func TestUnixValidateConfigurationErrors(t *testing.T) {
	testCases := []struct {
		config *Config
	}{
		// Can't override the stock runtime
		{
			config: &Config{
				Runtimes: map[string]types.Runtime{
					StockRuntimeName: {},
				},
			},
		},
		// Default runtime should be present in runtimes
		{
			config: &Config{
				Runtimes: map[string]types.Runtime{
					"foo": {},
				},
				DefaultRuntime: "bar",
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

func TestUnixGetInitPath(t *testing.T) {
	testCases := []struct {
		config           *Config
		expectedInitPath string
	}{
		{
			config: &Config{
				InitPath: "some-init-path",
			},
			expectedInitPath: "some-init-path",
		},
		{
			config: &Config{
				DefaultInitBinary: "foo-init-bin",
			},
			expectedInitPath: "foo-init-bin",
		},
		{
			config: &Config{
				InitPath:          "init-path-A",
				DefaultInitBinary: "init-path-B",
			},
			expectedInitPath: "init-path-A",
		},
		{
			config:           &Config{},
			expectedInitPath: "docker-init",
		},
	}
	for _, tc := range testCases {
		initPath := tc.config.GetInitPath()
		if initPath != tc.expectedInitPath {
			t.Fatalf("expected initPath to be %v, got %v", tc.expectedInitPath, initPath)
		}
	}
}
