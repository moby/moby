package config // import "github.com/docker/docker/daemon/config"

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/opts"
	units "github.com/docker/go-units"
	"github.com/spf13/pflag"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
)

func TestGetConflictFreeConfiguration(t *testing.T) {
	configFileData := `
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
			}
		}`

	file := fs.NewFile(t, "docker-config", fs.WithContent(configFileData))
	defer file.Remove()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	var debug bool
	flags.BoolVarP(&debug, "debug", "D", false, "")
	flags.Var(opts.NewNamedUlimitOpt("default-ulimits", nil), "default-ulimit", "")
	flags.Var(opts.NewNamedMapOpts("log-opts", nil, nil), "log-opt", "")

	cc, err := getConflictFreeConfiguration(file.Path(), flags)
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
	configFileData := `
		{
			"debug": true,
			"default-ulimits": {
				"nofile": {
					"Name": "nofile",
					"Hard": 2048,
					"Soft": 1024
				}
			}
		}`

	file := fs.NewFile(t, "docker-config", fs.WithContent(configFileData))
	defer file.Remove()

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

	cc, err := MergeDaemonConfigurations(conf, flags, file.Path())
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
	data := `{"default-shm-size": "1g"}`

	file := fs.NewFile(t, "docker-config", fs.WithContent(data))
	defer file.Remove()

	c := &Config{}

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	shmSize := opts.MemBytes(DefaultShmSize)
	flags.Var(&shmSize, "default-shm-size", "")

	cc, err := MergeDaemonConfigurations(c, flags, file.Path())
	assert.NilError(t, err)

	expectedValue := 1 * 1024 * 1024 * 1024
	assert.Check(t, is.Equal(int64(expectedValue), cc.ShmSize.Value()))
}

func TestUnixValidateConfigurationErrors(t *testing.T) {
	testCases := []struct {
		doc         string
		config      *Config
		expectedErr string
	}{
		{
			doc: `cannot override the stock runtime`,
			config: &Config{
				Runtimes: map[string]types.Runtime{
					StockRuntimeName: {},
				},
			},
			expectedErr: `runtime name 'runc' is reserved`,
		},
		{
			doc: `default runtime should be present in runtimes`,
			config: &Config{
				Runtimes: map[string]types.Runtime{
					"foo": {},
				},
				CommonConfig: CommonConfig{
					DefaultRuntime: "bar",
				},
			},
			expectedErr: `specified default runtime 'bar' does not exist`,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			err := Validate(tc.config)
			assert.ErrorContains(t, err, tc.expectedErr)
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
