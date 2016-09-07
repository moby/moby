// +build !windows

package plugins

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestLocalSocket(c *check.C) {
	// TODO Windows: Enable a similar version for Windows named pipes
	tmpdir, unregister := Setup(c)
	defer unregister()

	cases := []string{
		filepath.Join(tmpdir, "echo.sock"),
		filepath.Join(tmpdir, "echo", "echo.sock"),
	}

	for _, ca := range cases {
		if err := os.MkdirAll(filepath.Dir(ca), 0755); err != nil {
			c.Fatal(err)
		}

		l, err := net.Listen("unix", ca)
		if err != nil {
			c.Fatal(err)
		}

		r := newLocalRegistry()
		p, err := r.Plugin("echo")
		if err != nil {
			c.Fatal(err)
		}

		pp, err := r.Plugin("echo")
		if err != nil {
			c.Fatal(err)
		}
		if !reflect.DeepEqual(p, pp) {
			c.Fatalf("Expected %v, was %v\n", p, pp)
		}

		if p.name != "echo" {
			c.Fatalf("Expected plugin `echo`, got %s\n", p.name)
		}

		addr := fmt.Sprintf("unix://%s", ca)
		if p.Addr != addr {
			c.Fatalf("Expected plugin addr `%s`, got %s\n", addr, p.Addr)
		}
		if p.TLSConfig.InsecureSkipVerify != true {
			c.Fatalf("Expected TLS verification to be skipped")
		}
		l.Close()
	}
}
