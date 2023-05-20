//go:build !windows

package plugins // import "github.com/docker/docker/pkg/plugins"

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gotest.tools/v3/assert"
)

func TestLocalSocket(t *testing.T) {
	// TODO Windows: Enable a similar version for Windows named pipes
	tmpdir, unregister, r := Setup(t)
	defer unregister()

	cases := []string{
		filepath.Join(tmpdir, "echo.sock"),
		filepath.Join(tmpdir, "echo", "echo.sock"),
	}

	for _, c := range cases {
		if err := os.MkdirAll(filepath.Dir(c), 0755); err != nil {
			t.Fatal(err)
		}

		l, err := net.Listen("unix", c)
		if err != nil {
			t.Fatal(err)
		}

		p, err := r.Plugin("echo")
		if err != nil {
			t.Fatal(err)
		}

		pp, err := r.Plugin("echo")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p, pp) {
			t.Fatalf("Expected %v, was %v\n", p, pp)
		}

		if p.name != "echo" {
			t.Fatalf("Expected plugin `echo`, got %s\n", p.name)
		}

		addr := fmt.Sprintf("unix://%s", c)
		if p.Addr != addr {
			t.Fatalf("Expected plugin addr `%s`, got %s\n", addr, p.Addr)
		}
		if !p.TLSConfig.InsecureSkipVerify {
			t.Fatalf("Expected TLS verification to be skipped")
		}
		l.Close()
	}
}

func TestScan(t *testing.T) {
	tmpdir, unregister, r := Setup(t)
	defer unregister()

	pluginNames, err := r.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if pluginNames != nil {
		t.Fatal("Plugin names should be empty.")
	}

	path := filepath.Join(tmpdir, "echo.spec")
	addr := "unix://var/lib/docker/plugins/echo.sock"
	name := "echo"

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, []byte(addr), 0644)
	if err != nil {
		t.Fatal(err)
	}

	p, err := r.Plugin(name)
	assert.NilError(t, err)

	pluginNamesNotEmpty, err := r.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(pluginNamesNotEmpty) != 1 {
		t.Fatalf("expected 1 plugin entry: %v", pluginNamesNotEmpty)
	}
	if p.Name() != pluginNamesNotEmpty[0] {
		t.Fatalf("Unable to scan plugin with name %s", p.name)
	}
}

func TestScanNotPlugins(t *testing.T) {
	tmpdir, unregister, localRegistry := Setup(t)
	defer unregister()

	// not that `Setup()` above sets the sockets path and spec path dirs, which
	// `Scan()` uses to find plugins to the returned `tmpdir`

	notPlugin := filepath.Join(tmpdir, "not-a-plugin")
	if err := os.MkdirAll(notPlugin, 0700); err != nil {
		t.Fatal(err)
	}

	// this is named differently than the dir it's in, so the scanner should ignore it
	l, err := net.Listen("unix", filepath.Join(notPlugin, "foo.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// same let's test a spec path
	f, err := os.Create(filepath.Join(notPlugin, "foo.spec"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	names, err := localRegistry.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("expected no plugins, got %v", names)
	}

	// Just as a sanity check, let's make an entry that the scanner should read
	f, err = os.Create(filepath.Join(notPlugin, "not-a-plugin.spec"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	names, err = localRegistry.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 {
		t.Fatalf("expected 1 entry in result: %v", names)
	}
	if names[0] != "not-a-plugin" {
		t.Fatalf("expected plugin named `not-a-plugin`, got: %s", names[0])
	}
}
