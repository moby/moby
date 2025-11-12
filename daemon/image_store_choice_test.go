package daemon

import (
	"runtime"
	"testing"

	"github.com/moby/moby/v2/daemon/config"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDetermineImageStoreChoice(t *testing.T) {
	str := func(s string) *string {
		return &s
	}

	tests := []struct {
		name                  string
		envDockerDriver       *string
		envTestUseGraphDriver *string
		cfg                   *config.Config
		expectedChoice        imageStoreChoice
		expectError           bool
		skipPlatform          string
		onlyPlatform          string
	}{
		{
			name: "default containerd on non-Windows",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceContainerd,
			skipPlatform:   "windows",
		},
		{
			name: "default graphdriver on Windows",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriver,
			onlyPlatform:   "windows",
		},
		{
			name: "containerd-snapshotter feature enabled",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{
						"containerd-snapshotter": true,
					},
				},
			},
			expectedChoice: imageStoreChoiceContainerdExplicit,
		},
		{
			name: "containerd-snapshotter feature disabled",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{
						"containerd-snapshotter": false,
					},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
		},
		{
			name:                  "TEST_INTEGRATION_USE_GRAPHDRIVER env var set",
			envTestUseGraphDriver: str("1"),
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
		},
		{
			name: "vfs driver in config",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					GraphDriver: "vfs",
					Features:    map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
			skipPlatform:   "windows",
		},
		{
			name: "overlay2 driver in config",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					GraphDriver: "overlay2",
					Features:    map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
			skipPlatform:   "windows",
		},
		{
			name: "btrfs driver in config",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					GraphDriver: "btrfs",
					Features:    map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
			skipPlatform:   "windows",
		},
		{
			name:            "DOCKER_DRIVER env var set to overlay2",
			envDockerDriver: str("overlay2"),
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
			skipPlatform:   "windows",
		},
		{
			name:            "custom snapshotter",
			envDockerDriver: str("my-custom-snapshotter"),
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceContainerdExplicit,
			skipPlatform:   "windows",
		},
		{
			name:            "windows driver on Windows",
			envDockerDriver: str("windows"),
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceContainerdExplicit,
			onlyPlatform:   "windows",
		},
		{
			name:            "windowsfilter driver on Windows",
			envDockerDriver: str("windowsfilter"),
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
			onlyPlatform:   "windows",
		},
		{
			name:                  "TEST_INTEGRATION_USE_GRAPHDRIVER takes precedence over feature flag",
			envTestUseGraphDriver: str("1"),
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{
						"containerd-snapshotter": true,
					},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
		},
		{
			name: "native driver in config",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					GraphDriver: "native",
					Features:    map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceContainerdExplicit,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipPlatform != "" && runtime.GOOS == tc.skipPlatform {
				t.Skip("Skipping on " + tc.skipPlatform)
			}
			if tc.onlyPlatform != "" && runtime.GOOS != tc.onlyPlatform {
				t.Skip("Skipping on " + tc.onlyPlatform)
			}

			if tc.envDockerDriver != nil {
				t.Setenv("DOCKER_DRIVER", *tc.envDockerDriver)
			} else {
				t.Setenv("DOCKER_DRIVER", "")
			}
			if tc.envTestUseGraphDriver != nil {
				t.Setenv("TEST_INTEGRATION_USE_GRAPHDRIVER", *tc.envTestUseGraphDriver)
			} else {
				t.Setenv("TEST_INTEGRATION_USE_GRAPHDRIVER", "")
			}

			choice, err := determineImageStoreChoice(tc.cfg)
			if tc.expectError {
				assert.Error(t, err, "expected an error but got none")
			} else {
				assert.NilError(t, err)
			}

			assert.Check(t, is.Equal(tc.expectedChoice, choice))
		})
	}
}
