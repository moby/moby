package plugins

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-check/check"
)

func Setup(c *check.C) (string, func()) {
	tmpdir, err := ioutil.TempDir("", "docker-test")
	if err != nil {
		c.Fatal(err)
	}
	backup := socketsPath
	socketsPath = tmpdir
	specsPaths = []string{tmpdir}

	return tmpdir, func() {
		socketsPath = backup
		os.RemoveAll(tmpdir)
	}
}

func (s *DockerSuite) TestFileSpecPlugin(c *check.C) {
	tmpdir, unregister := Setup(c)
	defer unregister()

	cases := []struct {
		path string
		name string
		addr string
		fail bool
	}{
		// TODO Windows: Factor out the unix:// variants.
		{filepath.Join(tmpdir, "echo.spec"), "echo", "unix://var/lib/docker/plugins/echo.sock", false},
		{filepath.Join(tmpdir, "echo", "echo.spec"), "echo", "unix://var/lib/docker/plugins/echo.sock", false},
		{filepath.Join(tmpdir, "foo.spec"), "foo", "tcp://localhost:8080", false},
		{filepath.Join(tmpdir, "foo", "foo.spec"), "foo", "tcp://localhost:8080", false},
		{filepath.Join(tmpdir, "bar.spec"), "bar", "localhost:8080", true}, // unknown transport
	}

	for _, ca := range cases {
		if err := os.MkdirAll(filepath.Dir(ca.path), 0755); err != nil {
			c.Fatal(err)
		}
		if err := ioutil.WriteFile(ca.path, []byte(ca.addr), 0644); err != nil {
			c.Fatal(err)
		}

		r := newLocalRegistry()
		p, err := r.Plugin(ca.name)
		if ca.fail && err == nil {
			continue
		}

		if err != nil {
			c.Fatal(err)
		}

		if p.name != ca.name {
			c.Fatalf("Expected plugin `%s`, got %s\n", ca.name, p.name)
		}

		if p.Addr != ca.addr {
			c.Fatalf("Expected plugin addr `%s`, got %s\n", ca.addr, p.Addr)
		}

		if p.TLSConfig.InsecureSkipVerify != true {
			c.Fatalf("Expected TLS verification to be skipped")
		}
	}
}

func (s *DockerSuite) TestFileJSONSpecPlugin(c *check.C) {
	tmpdir, unregister := Setup(c)
	defer unregister()

	p := filepath.Join(tmpdir, "example.json")
	spec := `{
  "Name": "plugin-example",
  "Addr": "https://example.com/docker/plugin",
  "TLSConfig": {
    "CAFile": "/usr/shared/docker/certs/example-ca.pem",
    "CertFile": "/usr/shared/docker/certs/example-cert.pem",
    "KeyFile": "/usr/shared/docker/certs/example-key.pem"
	}
}`

	if err := ioutil.WriteFile(p, []byte(spec), 0644); err != nil {
		c.Fatal(err)
	}

	r := newLocalRegistry()
	plugin, err := r.Plugin("example")
	if err != nil {
		c.Fatal(err)
	}

	if expected, actual := "example", plugin.name; expected != actual {
		c.Fatalf("Expected plugin %q, got %s\n", expected, actual)
	}

	if plugin.Addr != "https://example.com/docker/plugin" {
		c.Fatalf("Expected plugin addr `https://example.com/docker/plugin`, got %s\n", plugin.Addr)
	}

	if plugin.TLSConfig.CAFile != "/usr/shared/docker/certs/example-ca.pem" {
		c.Fatalf("Expected plugin CA `/usr/shared/docker/certs/example-ca.pem`, got %s\n", plugin.TLSConfig.CAFile)
	}

	if plugin.TLSConfig.CertFile != "/usr/shared/docker/certs/example-cert.pem" {
		c.Fatalf("Expected plugin Certificate `/usr/shared/docker/certs/example-cert.pem`, got %s\n", plugin.TLSConfig.CertFile)
	}

	if plugin.TLSConfig.KeyFile != "/usr/shared/docker/certs/example-key.pem" {
		c.Fatalf("Expected plugin Key `/usr/shared/docker/certs/example-key.pem`, got %s\n", plugin.TLSConfig.KeyFile)
	}
}

func (s *DockerSuite) TestFileJSONSpecPluginWithoutTLSConfig(c *check.C) {
	tmpdir, unregister := Setup(c)
	defer unregister()

	p := filepath.Join(tmpdir, "example.json")
	spec := `{
  "Name": "plugin-example",
  "Addr": "https://example.com/docker/plugin"
}`

	if err := ioutil.WriteFile(p, []byte(spec), 0644); err != nil {
		c.Fatal(err)
	}

	r := newLocalRegistry()
	plugin, err := r.Plugin("example")
	if err != nil {
		c.Fatal(err)
	}

	if expected, actual := "example", plugin.name; expected != actual {
		c.Fatalf("Expected plugin %q, got %s\n", expected, actual)
	}

	if plugin.Addr != "https://example.com/docker/plugin" {
		c.Fatalf("Expected plugin addr `https://example.com/docker/plugin`, got %s\n", plugin.Addr)
	}

	if plugin.TLSConfig != nil {
		c.Fatalf("Expected plugin TLSConfig nil, got %v\n", plugin.TLSConfig)
	}
}
