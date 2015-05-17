package client

import (
	"bytes"
	"io"
	"testing"

	_ "github.com/docker/libnetwork/netutils"
)

// nopCloser is used to provide a dummy CallFunc for Cmd()
type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func TestClientDummyCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nopCloser{bytes.NewBufferString("")}, 200, nil
	}
	cli := NewNetworkCli(&out, &errOut, cFunc)

	err := cli.Cmd("docker", "dummy")
	if err == nil {
		t.Fatalf("Incorrect Command must fail")
	}
}

func TestClientNetworkInvalidCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nopCloser{bytes.NewBufferString("")}, 200, nil
	}
	cli := NewNetworkCli(&out, &errOut, cFunc)

	err := cli.Cmd("docker", "network", "invalid")
	if err == nil {
		t.Fatalf("Passing invalid commands must fail")
	}
}

func TestClientNetworkCreate(t *testing.T) {
	var out, errOut bytes.Buffer
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nopCloser{bytes.NewBufferString("")}, 200, nil
	}
	cli := NewNetworkCli(&out, &errOut, cFunc)

	err := cli.Cmd("docker", "network", "create", "test")
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestClientNetworkCreateWithDriver(t *testing.T) {
	var out, errOut bytes.Buffer
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nopCloser{bytes.NewBufferString("")}, 200, nil
	}
	cli := NewNetworkCli(&out, &errOut, cFunc)

	err := cli.Cmd("docker", "network", "create", "-f=dummy", "test")
	if err == nil {
		t.Fatalf("Passing incorrect flags to the create command must fail")
	}

	err = cli.Cmd("docker", "network", "create", "-d=dummy", "test")
	if err != nil {
		t.Fatalf(err.Error())
	}
}

func TestClientNetworkRm(t *testing.T) {
	var out, errOut bytes.Buffer
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nopCloser{bytes.NewBufferString("")}, 200, nil
	}
	cli := NewNetworkCli(&out, &errOut, cFunc)

	err := cli.Cmd("docker", "network", "rm", "test")
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestClientNetworkLs(t *testing.T) {
	var out, errOut bytes.Buffer
	networks := "db,web,test"
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nopCloser{bytes.NewBufferString(networks)}, 200, nil
	}
	cli := NewNetworkCli(&out, &errOut, cFunc)

	err := cli.Cmd("docker", "network", "ls")
	if err != nil {
		t.Fatal(err.Error())
	}
	if out.String() != networks {
		t.Fatal("Network List command fail to return the intended list")
	}
}

func TestClientNetworkInfo(t *testing.T) {
	var out, errOut bytes.Buffer
	info := "dummy info"
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nopCloser{bytes.NewBufferString(info)}, 200, nil
	}
	cli := NewNetworkCli(&out, &errOut, cFunc)

	err := cli.Cmd("docker", "network", "info", "test")
	if err != nil {
		t.Fatal(err.Error())
	}
	if out.String() != info {
		t.Fatal("Network List command fail to return the intended list")
	}
}

// Docker Flag processing in flag.go uses os.Exit() frequently, even for --help
// TODO : Handle the --help test-case in the IT when CLI is available
/*
func TestClientNetworkCreateHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nil, 0, nil
	}
	cli := NewNetworkCli(&out, &errOut, cFunc)

	err := cli.Cmd("docker", "network", "create", "--help")
	if err != nil {
		t.Fatalf(err.Error())
	}
}
*/

// Docker flag processing in flag.go uses os.Exit(1) for incorrect parameter case.
// TODO : Handle the missing argument case in the IT when CLI is available
/*
func TestClientNetworkCreateMissingArgument(t *testing.T) {
	var out, errOut bytes.Buffer
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nil, 0, nil
	}
	cli := NewNetworkCli(&out, &errOut, cFunc)

	err := cli.Cmd("docker", "network", "create")
	if err != nil {
		t.Fatal(err.Error())
	}
}
*/
