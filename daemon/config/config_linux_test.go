package config // import "github.com/docker/docker/daemon/config"

import (
	"testing"

	"github.com/docker/docker/opts"
	units "github.com/docker/go-units"
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

	expectedUlimits := map[string]*units.Ulimit{
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

	expectedUlimits := map[string]*units.Ulimit{
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
