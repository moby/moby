package config // import "github.com/docker/docker/daemon/config"

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/opts"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/imdario/mergo"
	"github.com/spf13/pflag"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/skip"
)

func TestDaemonConfigurationNotFound(t *testing.T) {
	_, err := MergeDaemonConfigurations(&Config{}, nil, "/tmp/foo-bar-baz-docker")
	assert.Check(t, os.IsNotExist(err), "got: %[1]T: %[1]v", err)
}

func TestDaemonBrokenConfiguration(t *testing.T) {
	f, err := os.CreateTemp("", "docker-config-")
	assert.NilError(t, err)

	configFile := f.Name()
	f.Write([]byte(`{"Debug": tru`))
	f.Close()

	_, err = MergeDaemonConfigurations(&Config{}, nil, configFile)
	assert.ErrorContains(t, err, `invalid character ' ' in literal true`)
}

// TestDaemonConfigurationWithBOM ensures that the UTF-8 byte order mark is ignored when reading the configuration file.
func TestDaemonConfigurationWithBOM(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "daemon.json")

	f, err := os.Create(configFile)
	assert.NilError(t, err)

	f.Write([]byte("\xef\xbb\xbf{\"debug\": true}"))
	f.Close()

	_, err = MergeDaemonConfigurations(&Config{}, nil, configFile)
	assert.NilError(t, err)
}

func TestFindConfigurationConflicts(t *testing.T) {
	config := map[string]interface{}{"authorization-plugins": "foobar"}
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	flags.String("authorization-plugins", "", "")
	assert.Check(t, flags.Set("authorization-plugins", "asdf"))
	assert.Check(t, is.ErrorContains(findConfigurationConflicts(config, flags), "authorization-plugins: (from flag: asdf, from file: foobar)"))
}

func TestFindConfigurationConflictsWithNamedOptions(t *testing.T) {
	config := map[string]interface{}{"hosts": []string{"qwer"}}
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	var hosts []string
	flags.VarP(opts.NewNamedListOptsRef("hosts", &hosts, opts.ValidateHost), "host", "H", "Daemon socket(s) to connect to")
	assert.Check(t, flags.Set("host", "tcp://127.0.0.1:4444"))
	assert.Check(t, flags.Set("host", "unix:///var/run/docker.sock"))
	assert.Check(t, is.ErrorContains(findConfigurationConflicts(config, flags), "hosts"))
}

func TestDaemonConfigurationMergeConflicts(t *testing.T) {
	f, err := os.CreateTemp("", "docker-config-")
	assert.NilError(t, err)

	configFile := f.Name()
	f.Write([]byte(`{"debug": true}`))
	f.Close()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Bool("debug", false, "")
	assert.Check(t, flags.Set("debug", "false"))

	_, err = MergeDaemonConfigurations(&Config{}, flags, configFile)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "debug") {
		t.Fatalf("expected debug conflict, got %v", err)
	}
}

func TestDaemonConfigurationMergeConcurrent(t *testing.T) {
	f, err := os.CreateTemp("", "docker-config-")
	assert.NilError(t, err)

	configFile := f.Name()
	f.Write([]byte(`{"max-concurrent-downloads": 1}`))
	f.Close()

	_, err = MergeDaemonConfigurations(&Config{}, nil, configFile)
	assert.NilError(t, err)
}

func TestDaemonConfigurationMergeConcurrentError(t *testing.T) {
	f, err := os.CreateTemp("", "docker-config-")
	assert.NilError(t, err)

	configFile := f.Name()
	f.Write([]byte(`{"max-concurrent-downloads": -1}`))
	f.Close()

	_, err = MergeDaemonConfigurations(&Config{}, nil, configFile)
	assert.ErrorContains(t, err, `invalid max concurrent downloads: -1`)
}

func TestDaemonConfigurationMergeConflictsWithInnerStructs(t *testing.T) {
	f, err := os.CreateTemp("", "docker-config-")
	assert.NilError(t, err)

	configFile := f.Name()
	f.Write([]byte(`{"tlscacert": "/etc/certificates/ca.pem"}`))
	f.Close()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("tlscacert", "", "")
	assert.Check(t, flags.Set("tlscacert", "~/.docker/ca.pem"))

	_, err = MergeDaemonConfigurations(&Config{}, flags, configFile)
	assert.ErrorContains(t, err, `the following directives are specified both as a flag and in the configuration file: tlscacert`)
}

// Test for #40711
func TestDaemonConfigurationMergeDefaultAddressPools(t *testing.T) {
	emptyConfigFile := fs.NewFile(t, "config", fs.WithContent(`{}`))
	defer emptyConfigFile.Remove()
	configFile := fs.NewFile(t, "config", fs.WithContent(`{"default-address-pools":[{"base": "10.123.0.0/16", "size": 24 }]}`))
	defer configFile.Remove()

	expected := []*ipamutils.NetworkToSplit{{Base: "10.123.0.0/16", Size: 24}}

	t.Run("empty config file", func(t *testing.T) {
		var conf = Config{}
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.Var(&conf.NetworkConfig.DefaultAddressPools, "default-address-pool", "")
		assert.Check(t, flags.Set("default-address-pool", "base=10.123.0.0/16,size=24"))

		config, err := MergeDaemonConfigurations(&conf, flags, emptyConfigFile.Path())
		assert.NilError(t, err)
		assert.DeepEqual(t, config.DefaultAddressPools.Value(), expected)
	})

	t.Run("config file", func(t *testing.T) {
		var conf = Config{}
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.Var(&conf.NetworkConfig.DefaultAddressPools, "default-address-pool", "")

		config, err := MergeDaemonConfigurations(&conf, flags, configFile.Path())
		assert.NilError(t, err)
		assert.DeepEqual(t, config.DefaultAddressPools.Value(), expected)
	})

	t.Run("with conflicting options", func(t *testing.T) {
		var conf = Config{}
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.Var(&conf.NetworkConfig.DefaultAddressPools, "default-address-pool", "")
		assert.Check(t, flags.Set("default-address-pool", "base=10.123.0.0/16,size=24"))

		_, err := MergeDaemonConfigurations(&conf, flags, configFile.Path())
		assert.ErrorContains(t, err, "the following directives are specified both as a flag and in the configuration file")
		assert.ErrorContains(t, err, "default-address-pools")
	})
}

func TestFindConfigurationConflictsWithUnknownKeys(t *testing.T) {
	config := map[string]interface{}{"tls-verify": "true"}
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	flags.Bool("tlsverify", false, "")
	err := findConfigurationConflicts(config, flags)
	assert.ErrorContains(t, err, "the following directives don't match any configuration option: tls-verify")
}

func TestFindConfigurationConflictsWithMergedValues(t *testing.T) {
	var hosts []string
	config := map[string]interface{}{"hosts": "tcp://127.0.0.1:2345"}
	flags := pflag.NewFlagSet("base", pflag.ContinueOnError)
	flags.VarP(opts.NewNamedListOptsRef("hosts", &hosts, nil), "host", "H", "")

	err := findConfigurationConflicts(config, flags)
	assert.NilError(t, err)

	assert.Check(t, flags.Set("host", "unix:///var/run/docker.sock"))
	err = findConfigurationConflicts(config, flags)
	assert.ErrorContains(t, err, "hosts: (from flag: [unix:///var/run/docker.sock], from file: tcp://127.0.0.1:2345)")
}

func TestValidateConfigurationErrors(t *testing.T) {
	testCases := []struct {
		name        string
		field       string
		config      *Config
		expectedErr string
	}{
		{
			name: "single label without value",
			config: &Config{
				CommonConfig: CommonConfig{
					Labels: []string{"one"},
				},
			},
			expectedErr: "bad attribute format: one",
		},
		{
			name: "multiple label without value",
			config: &Config{
				CommonConfig: CommonConfig{
					Labels: []string{"foo=bar", "one"},
				},
			},
			expectedErr: "bad attribute format: one",
		},
		{
			name: "single DNS, invalid IP-address",
			config: &Config{
				CommonConfig: CommonConfig{
					DNSConfig: DNSConfig{
						DNS: []string{"1.1.1.1o"},
					},
				},
			},
			expectedErr: "1.1.1.1o is not an ip address",
		},
		{
			name: "multiple DNS, invalid IP-address",
			config: &Config{
				CommonConfig: CommonConfig{
					DNSConfig: DNSConfig{
						DNS: []string{"2.2.2.2", "1.1.1.1o"},
					},
				},
			},
			expectedErr: "1.1.1.1o is not an ip address",
		},
		{
			name: "single DNSSearch",
			config: &Config{
				CommonConfig: CommonConfig{
					DNSConfig: DNSConfig{
						DNSSearch: []string{"123456"},
					},
				},
			},
			expectedErr: "123456 is not a valid domain",
		},
		{
			name: "multiple DNSSearch",
			config: &Config{
				CommonConfig: CommonConfig{
					DNSConfig: DNSConfig{
						DNSSearch: []string{"a.b.c", "123456"},
					},
				},
			},
			expectedErr: "123456 is not a valid domain",
		},
		{
			name: "negative MTU",
			config: &Config{
				CommonConfig: CommonConfig{
					Mtu: -10,
				},
			},
			expectedErr: "invalid default MTU: -10",
		},
		{
			name: "negative max-concurrent-downloads",
			config: &Config{
				CommonConfig: CommonConfig{
					MaxConcurrentDownloads: -10,
				},
			},
			expectedErr: "invalid max concurrent downloads: -10",
		},
		{
			name: "negative max-concurrent-uploads",
			config: &Config{
				CommonConfig: CommonConfig{
					MaxConcurrentUploads: -10,
				},
			},
			expectedErr: "invalid max concurrent uploads: -10",
		},
		{
			name: "negative max-download-attempts",
			config: &Config{
				CommonConfig: CommonConfig{
					MaxDownloadAttempts: -10,
				},
			},
			expectedErr: "invalid max download attempts: -10",
		},
		// TODO(thaJeztah) temporarily excluding this test as it assumes defaults are set before validating and applying updated configs
		/*
			{
				name:  "zero max-download-attempts",
				field: "MaxDownloadAttempts",
				config: &Config{
					CommonConfig: CommonConfig{
						MaxDownloadAttempts: 0,
					},
				},
				expectedErr: "invalid max download attempts: 0",
			},
		*/
		{
			name: "generic resource without =",
			config: &Config{
				CommonConfig: CommonConfig{
					NodeGenericResources: []string{"foo"},
				},
			},
			expectedErr: "could not parse GenericResource: incorrect term foo, missing '=' or malformed expression",
		},
		{
			name: "generic resource mixed named and discrete",
			config: &Config{
				CommonConfig: CommonConfig{
					NodeGenericResources: []string{"foo=bar", "foo=1"},
				},
			},
			expectedErr: "could not parse GenericResource: mixed discrete and named resources in expression 'foo=[bar 1]'",
		},
		{
			name: "with invalid hosts",
			config: &Config{
				CommonConfig: CommonConfig{
					Hosts: []string{"127.0.0.1:2375/path"},
				},
			},
			expectedErr: "invalid bind address (127.0.0.1:2375/path): should not contain a path element",
		},
		{
			name: "with invalid log-level",
			config: &Config{
				CommonConfig: CommonConfig{
					LogLevel: "foobar",
				},
			},
			expectedErr: "invalid logging level: foobar",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := New()
			assert.NilError(t, err)
			if tc.field != "" {
				assert.Check(t, mergo.Merge(cfg, tc.config, mergo.WithOverride, withForceOverwrite(tc.field)))
			} else {
				assert.Check(t, mergo.Merge(cfg, tc.config, mergo.WithOverride))
			}
			err = Validate(cfg)
			assert.Error(t, err, tc.expectedErr)
		})
	}
}

func withForceOverwrite(fieldName string) func(config *mergo.Config) {
	return mergo.WithTransformers(overwriteTransformer{fieldName: fieldName})
}

type overwriteTransformer struct {
	fieldName string
}

func (tf overwriteTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(CommonConfig{}) {
		return func(dst, src reflect.Value) error {
			dst.FieldByName(tf.fieldName).Set(src.FieldByName(tf.fieldName))
			return nil
		}
	}
	return nil
}

func TestValidateConfiguration(t *testing.T) {
	testCases := []struct {
		name   string
		field  string
		config *Config
	}{
		{
			name:  "with label",
			field: "Labels",
			config: &Config{
				CommonConfig: CommonConfig{
					Labels: []string{"one=two"},
				},
			},
		},
		{
			name:  "with dns",
			field: "DNSConfig",
			config: &Config{
				CommonConfig: CommonConfig{
					DNSConfig: DNSConfig{
						DNS: []string{"1.1.1.1"},
					},
				},
			},
		},
		{
			name:  "with dns-search",
			field: "DNSConfig",
			config: &Config{
				CommonConfig: CommonConfig{
					DNSConfig: DNSConfig{
						DNSSearch: []string{"a.b.c"},
					},
				},
			},
		},
		{
			name:  "with mtu",
			field: "Mtu",
			config: &Config{
				CommonConfig: CommonConfig{
					Mtu: 1234,
				},
			},
		},
		{
			name:  "with max-concurrent-downloads",
			field: "MaxConcurrentDownloads",
			config: &Config{
				CommonConfig: CommonConfig{
					MaxConcurrentDownloads: 4,
				},
			},
		},
		{
			name:  "with max-concurrent-uploads",
			field: "MaxConcurrentUploads",
			config: &Config{
				CommonConfig: CommonConfig{
					MaxConcurrentUploads: 4,
				},
			},
		},
		{
			name:  "with max-download-attempts",
			field: "MaxDownloadAttempts",
			config: &Config{
				CommonConfig: CommonConfig{
					MaxDownloadAttempts: 4,
				},
			},
		},
		{
			name:  "with multiple node generic resources",
			field: "NodeGenericResources",
			config: &Config{
				CommonConfig: CommonConfig{
					NodeGenericResources: []string{"foo=bar", "foo=baz"},
				},
			},
		},
		{
			name:  "with node generic resources",
			field: "NodeGenericResources",
			config: &Config{
				CommonConfig: CommonConfig{
					NodeGenericResources: []string{"foo=1"},
				},
			},
		},
		{
			name:  "with hosts",
			field: "Hosts",
			config: &Config{
				CommonConfig: CommonConfig{
					Hosts: []string{"tcp://127.0.0.1:2375"},
				},
			},
		},
		{
			name:  "with log-level warn",
			field: "LogLevel",
			config: &Config{
				CommonConfig: CommonConfig{
					LogLevel: "warn",
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Start with a config with all defaults set, so that we only
			cfg, err := New()
			assert.NilError(t, err)
			assert.Check(t, mergo.Merge(cfg, tc.config, mergo.WithOverride))

			// Check that the override happened :)
			assert.Check(t, is.DeepEqual(cfg, tc.config, field(tc.field)))
			err = Validate(cfg)
			assert.NilError(t, err)
		})
	}
}

func field(field string) cmp.Option {
	tmp := reflect.TypeOf(Config{})
	ignoreFields := make([]string, 0, tmp.NumField())
	for i := 0; i < tmp.NumField(); i++ {
		if tmp.Field(i).Name != field {
			ignoreFields = append(ignoreFields, tmp.Field(i).Name)
		}
	}
	return cmpopts.IgnoreFields(Config{}, ignoreFields...)
}

// TestReloadSetConfigFileNotExist tests that when `--config-file` is set
// and it doesn't exist the `Reload` function returns an error.
func TestReloadSetConfigFileNotExist(t *testing.T) {
	configFile := "/tmp/blabla/not/exists/config.json"
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("config-file", "", "")
	assert.Check(t, flags.Set("config-file", configFile))

	err := Reload(configFile, flags, func(c *Config) {})
	assert.Check(t, is.ErrorContains(err, "unable to configure the Docker daemon with file"))
}

// TestReloadDefaultConfigNotExist tests that if the default configuration file
// doesn't exist the daemon still will be reloaded.
func TestReloadDefaultConfigNotExist(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	defaultConfigFile := "/tmp/blabla/not/exists/daemon.json"
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("config-file", defaultConfigFile, "")
	reloaded := false
	err := Reload(defaultConfigFile, flags, func(c *Config) {
		reloaded = true
	})
	assert.Check(t, err)
	assert.Check(t, reloaded)
}

// TestReloadBadDefaultConfig tests that when `--config-file` is not set
// and the default configuration file exists and is bad return an error
func TestReloadBadDefaultConfig(t *testing.T) {
	f, err := os.CreateTemp("", "docker-config-")
	assert.NilError(t, err)

	configFile := f.Name()
	f.Write([]byte(`{wrong: "configuration"}`))
	f.Close()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("config-file", configFile, "")
	reloaded := false
	err = Reload(configFile, flags, func(c *Config) {
		reloaded = true
	})
	assert.Check(t, is.ErrorContains(err, "unable to configure the Docker daemon with file"))
	assert.Check(t, reloaded == false)
}

func TestReloadWithConflictingLabels(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"labels":["foo=bar","foo=baz"]}`))
	defer tempFile.Remove()
	configFile := tempFile.Path()

	var lbls []string
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("config-file", configFile, "")
	flags.StringSlice("labels", lbls, "")
	reloaded := false
	err := Reload(configFile, flags, func(c *Config) {
		reloaded = true
	})
	assert.Check(t, is.ErrorContains(err, "conflict labels for foo=baz and foo=bar"))
	assert.Check(t, reloaded == false)
}

func TestReloadWithDuplicateLabels(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{"labels":["foo=the-same","foo=the-same"]}`))
	defer tempFile.Remove()
	configFile := tempFile.Path()

	var lbls []string
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("config-file", configFile, "")
	flags.StringSlice("labels", lbls, "")
	reloaded := false
	err := Reload(configFile, flags, func(c *Config) {
		reloaded = true
		assert.Check(t, is.DeepEqual(c.Labels, []string{"foo=the-same"}))
	})
	assert.Check(t, err)
	assert.Check(t, reloaded)
}

func TestMaskURLCredentials(t *testing.T) {
	tests := []struct {
		rawURL    string
		maskedURL string
	}{
		{
			rawURL:    "",
			maskedURL: "",
		}, {
			rawURL:    "invalidURL",
			maskedURL: "invalidURL",
		}, {
			rawURL:    "http://proxy.example.com:80/",
			maskedURL: "http://proxy.example.com:80/",
		}, {
			rawURL:    "http://USER:PASSWORD@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://PASSWORD:PASSWORD@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://USER:@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://:PASSWORD@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://USER@docker:password@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://USER%40docker:password@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://USER%40docker:pa%3Fsword@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://USER%40docker:pa%3Fsword@proxy.example.com:80/hello%20world",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/hello%20world",
		},
	}
	for _, test := range tests {
		maskedURL := MaskCredentials(test.rawURL)
		assert.Equal(t, maskedURL, test.maskedURL)
	}
}
