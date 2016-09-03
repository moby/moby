package daemon

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/go-check/check"
	"github.com/spf13/pflag"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestDaemonConfigurationMerge(c *check.C) {
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		c.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"debug": true}`))
	f.Close()

	cf := &Config{
		CommonConfig: CommonConfig{
			AutoRestart: true,
			LogConfig: LogConfig{
				Type:   "syslog",
				Config: map[string]string{"tag": "test"},
			},
		},
	}

	cc, err := MergeDaemonConfigurations(cf, nil, configFile)
	if err != nil {
		c.Fatal(err)
	}
	if !cc.Debug {
		c.Fatalf("expected %v, got %v\n", true, cc.Debug)
	}
	if !cc.AutoRestart {
		c.Fatalf("expected %v, got %v\n", true, cc.AutoRestart)
	}
	if cc.LogConfig.Type != "syslog" {
		c.Fatalf("expected syslog config, got %q\n", cc.LogConfig)
	}
}

func (s *DockerSuite) TestDaemonConfigurationNotFound(c *check.C) {
	_, err := MergeDaemonConfigurations(&Config{}, nil, "/tmp/foo-bar-baz-docker")
	if err == nil || !os.IsNotExist(err) {
		c.Fatalf("expected does not exist error, got %v", err)
	}
}

func (s *DockerSuite) TestDaemonBrokenConfiguration(c *check.C) {
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		c.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"Debug": tru`))
	f.Close()

	_, err = MergeDaemonConfigurations(&Config{}, nil, configFile)
	if err == nil {
		c.Fatalf("expected error, got %v", err)
	}
}

func (s *DockerSuite) TestParseClusterAdvertiseSettings(c *check.C) {
	_, err := parseClusterAdvertiseSettings("something", "")
	if err != errDiscoveryDisabled {
		c.Fatalf("expected discovery disabled error, got %v\n", err)
	}

	_, err = parseClusterAdvertiseSettings("", "something")
	if err == nil {
		c.Fatalf("expected discovery store error, got %v\n", err)
	}

	_, err = parseClusterAdvertiseSettings("etcd", "127.0.0.1:8080")
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestFindConfigurationConflicts(c *check.C) {
	config := map[string]interface{}{"authorization-plugins": "foobar"}
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	flags.String("authorization-plugins", "", "")
	assert.NilError(c, flags.Set("authorization-plugins", "asdf"))

	assert.Error(c,
		findConfigurationConflicts(config, flags),
		"authorization-plugins: (from flag: asdf, from file: foobar)")
}

func (s *DockerSuite) TestFindConfigurationConflictsWithNamedOptions(c *check.C) {
	config := map[string]interface{}{"hosts": []string{"qwer"}}
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	var hosts []string
	flags.VarP(opts.NewNamedListOptsRef("hosts", &hosts, opts.ValidateHost), "host", "H", "Daemon socket(s) to connect to")
	assert.NilError(c, flags.Set("host", "tcp://127.0.0.1:4444"))
	assert.NilError(c, flags.Set("host", "unix:///var/run/docker.sock"))

	assert.Error(c, findConfigurationConflicts(config, flags), "hosts")
}

func (s *DockerSuite) TestDaemonConfigurationMergeConflicts(c *check.C) {
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		c.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"debug": true}`))
	f.Close()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Bool("debug", false, "")
	flags.Set("debug", "false")

	_, err = MergeDaemonConfigurations(&Config{}, flags, configFile)
	if err == nil {
		c.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "debug") {
		c.Fatalf("expected debug conflict, got %v", err)
	}
}

func (s *DockerSuite) TestDaemonConfigurationMergeConflictsWithInnerStructs(c *check.C) {
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		c.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{"tlscacert": "/etc/certificates/ca.pem"}`))
	f.Close()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("tlscacert", "", "")
	flags.Set("tlscacert", "~/.docker/ca.pem")

	_, err = MergeDaemonConfigurations(&Config{}, flags, configFile)
	if err == nil {
		c.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tlscacert") {
		c.Fatalf("expected tlscacert conflict, got %v", err)
	}
}

func (s *DockerSuite) TestFindConfigurationConflictsWithUnknownKeys(c *check.C) {
	config := map[string]interface{}{"tls-verify": "true"}
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	flags.Bool("tlsverify", false, "")
	err := findConfigurationConflicts(config, flags)
	if err == nil {
		c.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "the following directives don't match any configuration option: tls-verify") {
		c.Fatalf("expected tls-verify conflict, got %v", err)
	}
}

func (s *DockerSuite) TestFindConfigurationConflictsWithMergedValues(c *check.C) {
	var hosts []string
	config := map[string]interface{}{"hosts": "tcp://127.0.0.1:2345"}
	flags := pflag.NewFlagSet("base", pflag.ContinueOnError)
	flags.VarP(opts.NewNamedListOptsRef("hosts", &hosts, nil), "host", "H", "")

	err := findConfigurationConflicts(config, flags)
	if err != nil {
		c.Fatal(err)
	}

	flags.Set("host", "unix:///var/run/docker.sock")
	err = findConfigurationConflicts(config, flags)
	if err == nil {
		c.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "hosts: (from flag: [unix:///var/run/docker.sock], from file: tcp://127.0.0.1:2345)") {
		c.Fatalf("expected hosts conflict, got %v", err)
	}
}

func (s *DockerSuite) TestValidateConfiguration(c *check.C) {
	c1 := &Config{
		CommonConfig: CommonConfig{
			Labels: []string{"one"},
		},
	}

	err := ValidateConfiguration(c1)
	if err == nil {
		c.Fatal("expected error, got nil")
	}

	c2 := &Config{
		CommonConfig: CommonConfig{
			Labels: []string{"one=two"},
		},
	}

	err = ValidateConfiguration(c2)
	if err != nil {
		c.Fatalf("expected no error, got error %v", err)
	}

	c3 := &Config{
		CommonConfig: CommonConfig{
			DNS: []string{"1.1.1.1"},
		},
	}

	err = ValidateConfiguration(c3)
	if err != nil {
		c.Fatalf("expected no error, got error %v", err)
	}

	c4 := &Config{
		CommonConfig: CommonConfig{
			DNS: []string{"1.1.1.1o"},
		},
	}

	err = ValidateConfiguration(c4)
	if err == nil {
		c.Fatal("expected error, got nil")
	}

	c5 := &Config{
		CommonConfig: CommonConfig{
			DNSSearch: []string{"a.b.c"},
		},
	}

	err = ValidateConfiguration(c5)
	if err != nil {
		c.Fatalf("expected no error, got error %v", err)
	}

	c6 := &Config{
		CommonConfig: CommonConfig{
			DNSSearch: []string{"123456"},
		},
	}

	err = ValidateConfiguration(c6)
	if err == nil {
		c.Fatal("expected error, got nil")
	}
}
