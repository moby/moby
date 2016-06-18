// +build !windows

package plugins

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLocalSocket(t *testing.T) {
	// TODO Windows: Enable a similar version for Windows named pipes
	tmpdir, unregister := Setup(t)
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

		r := newLocalRegistry()
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
			t.Fatalf("Expected plugin `echo`, got %s\n", p.Name)
		}

		addr := fmt.Sprintf("unix://%s", c)
		if p.Addr != addr {
			t.Fatalf("Expected plugin addr `%s`, got %s\n", addr, p.Addr)
		}
		if p.TLSConfig.InsecureSkipVerify != true {
			t.Fatalf("Expected TLS verification to be skipped")
		}
		l.Close()
	}
}
