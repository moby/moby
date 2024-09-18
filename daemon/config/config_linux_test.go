package config // import "github.com/docker/docker/daemon/config"

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	dopts "github.com/docker/docker/internal/opts"
	"github.com/docker/docker/opts"
	"github.com/spf13/pflag"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestGetConflictFreeConfiguration(t *testing.T) {
	configFile := makeConfigFile(t, `
		{
			"debug": true,
			"default-ulimits": {
				"nofile": {
					"Name": "nofile",
					"Hard": 2048,
					"Soft": 1024
				}
			},
			"log-opts": {
				"tag": "test_tag"
			},
			"default-network-opts": {
				"overlay": {
					"com.docker.network.driver.mtu": "1337"
				}
			}
		}`)

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	var debug bool
	flags.BoolVarP(&debug, "debug", "D", false, "")
	flags.Var(opts.NewNamedUlimitOpt("default-ulimits", nil), "default-ulimit", "")
	flags.Var(opts.NewNamedMapOpts("log-opts", nil, nil), "log-opt", "")
	flags.Var(opts.NewNamedMapMapOpts("default-network-opts", nil, nil), "default-network-opt", "")

	cc, err := getConflictFreeConfiguration(configFile, flags)
	assert.NilError(t, err)

	assert.Check(t, cc.Debug)

	expectedUlimits := map[string]*container.Ulimit{
		"nofile": {
			Name: "nofile",
			Hard: 2048,
			Soft: 1024,
		},
	}

	assert.Check(t, is.DeepEqual(expectedUlimits, cc.Ulimits))
}

func TestDaemonConfigurationMerge(t *testing.T) {
	configFile := makeConfigFile(t, `
		{
			"debug": true,
			"default-ulimits": {
				"nofile": {
					"Name": "nofile",
					"Hard": 2048,
					"Soft": 1024
				}
			}
		}`)

	conf, err := New()
	assert.NilError(t, err)

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.BoolVarP(&conf.Debug, "debug", "D", false, "")
	flags.BoolVarP(&conf.AutoRestart, "restart", "r", true, "")
	flags.Var(opts.NewNamedUlimitOpt("default-ulimits", &conf.Ulimits), "default-ulimit", "")
	flags.StringVar(&conf.LogConfig.Type, "log-driver", "json-file", "")
	flags.Var(opts.NewNamedMapOpts("log-opts", conf.LogConfig.Config, nil), "log-opt", "")
	assert.Check(t, flags.Set("restart", "true"))
	assert.Check(t, flags.Set("log-driver", "syslog"))
	assert.Check(t, flags.Set("log-opt", "tag=from_flag"))

	cc, err := MergeDaemonConfigurations(conf, flags, configFile)
	assert.NilError(t, err)

	assert.Check(t, cc.Debug)
	assert.Check(t, cc.AutoRestart)

	expectedLogConfig := LogConfig{
		Type:   "syslog",
		Config: map[string]string{"tag": "from_flag"},
	}

	assert.Check(t, is.DeepEqual(expectedLogConfig, cc.LogConfig))

	expectedUlimits := map[string]*container.Ulimit{
		"nofile": {
			Name: "nofile",
			Hard: 2048,
			Soft: 1024,
		},
	}

	assert.Check(t, is.DeepEqual(expectedUlimits, cc.Ulimits))
}

func TestDaemonConfigurationMergeShmSize(t *testing.T) {
	configFile := makeConfigFile(t, `{"default-shm-size": "1g"}`)

	c, err := New()
	assert.NilError(t, err)

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	shmSize := opts.MemBytes(DefaultShmSize)
	flags.Var(&shmSize, "default-shm-size", "")

	cc, err := MergeDaemonConfigurations(c, flags, configFile)
	assert.NilError(t, err)

	expectedValue := 1 * 1024 * 1024 * 1024
	assert.Check(t, is.Equal(int64(expectedValue), cc.ShmSize.Value()))
}

func TestDaemonConfigurationFeatures(t *testing.T) {
	tests := []struct {
		name, config, flags string
		expectedValue       map[string]bool
		expectedErr         string
	}{
		{
			name:          "enable from file",
			config:        `{"features": {"containerd-snapshotter": true}}`,
			expectedValue: map[string]bool{"containerd-snapshotter": true},
		},
		{
			name:          "enable from flags",
			config:        `{}`,
			flags:         "containerd-snapshotter=true",
			expectedValue: map[string]bool{"containerd-snapshotter": true},
		},
		{
			name:          "disable from file",
			config:        `{"features": {"containerd-snapshotter": false}}`,
			expectedValue: map[string]bool{"containerd-snapshotter": false},
		},
		{
			name:          "disable from flags",
			config:        `{}`,
			flags:         "containerd-snapshotter=false",
			expectedValue: map[string]bool{"containerd-snapshotter": false},
		},
		{
			name:        "conflict",
			config:      `{"features": {"containerd-snapshotter": true}}`,
			flags:       "containerd-snapshotter=true",
			expectedErr: `the following directives are specified both as a flag and in the configuration file: features: (from flag: map[containerd-snapshotter:true], from file: map[containerd-snapshotter:true])`,
		},
		{
			name:        "invalid config value",
			config:      `{"features": {"containerd-snapshotter": "not-a-boolean"}}`,
			expectedErr: `json: cannot unmarshal string into Go struct field Config.features of type bool`,
		},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			c, err := New()
			assert.NilError(t, err)

			configFile := makeConfigFile(t, tc.config)
			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			flags.Var(dopts.NewNamedSetOpts("features", c.Features), "feature", "Enable feature in the daemon")
			if tc.flags != "" {
				err = flags.Set("feature", tc.flags)
				assert.NilError(t, err)
			}
			cc, err := MergeDaemonConfigurations(c, flags, configFile)
			if tc.expectedErr != "" {
				assert.Error(t, err, tc.expectedErr)
			} else {
				assert.NilError(t, err)
				assert.Check(t, is.DeepEqual(tc.expectedValue, cc.Features))
			}
		})
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
		assert.Equal(t, tc.config.GetInitPath(), tc.expectedInitPath)
	}
}
