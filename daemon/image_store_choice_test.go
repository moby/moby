package daemon

import (
	"slices"
	"testing"

	"github.com/moby/moby/v2/daemon/config"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDetermineImageStoreChoice(t *testing.T) {
	str := func(s string) *string {
		return &s
	}

	type testCase struct {
		name                  string
		envDockerDriver       *string
		envTestUseGraphDriver *string
		priorGraphDriver      bool
		cfg                   *config.Config
		expectedChoice        imageStoreChoice
		expectError           bool
		skipPlatform          string
		onlyPlatform          string
	}

	tests := []testCase{
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
		{
			name: "vfs driver in config",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					GraphDriver: "vfs",
					Features:    map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
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
		},
	}

	nonWindows := []testCase{
		{
			name: "default containerd on non-Windows",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceContainerd,
		},
	}

	for _, gd := range []string{"fuse-overlayfs", "overlay2", "btrfs", "zfs"} {
		nonWindows = append(nonWindows, testCase{
			name: gd + " driver in config",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					GraphDriver: gd,
					Features:    map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
		})

		nonWindows = append(nonWindows, testCase{
			name: gd + " driver in config with prior data",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					GraphDriver: gd,
					Features:    map[string]bool{},
				},
			},
			priorGraphDriver: true,
			expectedChoice:   imageStoreChoiceGraphdriverPrior,
		})

		nonWindows = append(nonWindows, testCase{
			name: gd + " driver in config with containerd snapshotter feature enabled",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					GraphDriver: gd,
					Features: map[string]bool{
						"containerd-snapshotter": true,
					},
				},
			},
			expectedChoice: imageStoreChoiceContainerdExplicit,
		})

		nonWindows = append(nonWindows, testCase{
			name: gd + " driver in config with containerd snapshotter feature disabled",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					GraphDriver: gd,
					Features: map[string]bool{
						"containerd-snapshotter": false,
					},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
		})

		nonWindows = append(nonWindows, testCase{
			name:                  gd + " driver in config with TEST_INTEGRATION_USE_GRAPHDRIVER env var set",
			envTestUseGraphDriver: str("1"),
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					GraphDriver: gd,
					Features:    map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriverExplicit,
		})
	}

	windows := []testCase{
		{
			name: "default graphdriver on Windows",
			cfg: &config.Config{
				CommonConfig: config.CommonConfig{
					Features: map[string]bool{},
				},
			},
			expectedChoice: imageStoreChoiceGraphdriver,
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
		},
	}

	for i := range nonWindows {
		nonWindows[i].skipPlatform = "windows"
	}
	tests = append(tests, nonWindows...)

	for i := range windows {
		windows[i].onlyPlatform = "windows"
	}
	tests = append(tests, windows...)

	registeredDrivers := []string{"fuse-overlayfs", "overlay2", "btrfs", "zfs", "vfs"}
	windowsRegisteredDrivers := []string{"vfs", "windowsfilter"}

	for _, os := range []string{"linux", "windows"} {
		for _, tc := range tests {
			if tc.skipPlatform != "" && os == tc.skipPlatform {
				continue
			}
			if tc.onlyPlatform != "" && os != tc.onlyPlatform {
				continue
			}

			t.Run(os+"/"+tc.name, func(t *testing.T) {
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

				choice, err := determineImageStoreChoice(tc.cfg, determineImageStoreChoiceOptions{
					runtimeOS: os,
					hasPriorDriver: func(root string) bool {
						return tc.priorGraphDriver
					},
					isRegisteredGraphdriver: func(driverName string) bool {
						if os == "windows" {
							return slices.Contains(windowsRegisteredDrivers, driverName)
						}
						return slices.Contains(registeredDrivers, driverName)
					},
				})
				if tc.expectError {
					assert.Error(t, err, "expected an error but got none")
				} else {
					assert.NilError(t, err)
				}

				assert.Check(t, is.Equal(tc.expectedChoice, choice))
			})
		}
	}
}
