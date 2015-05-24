// +build experimental

package client

import (
	"bytes"
	"testing"

	_ "github.com/docker/libnetwork/netutils"
)

func TestClientNetworkServiceInvalidCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "service", "invalid")
	if err == nil {
		t.Fatalf("Passing invalid commands must fail")
	}
}

func TestClientNetworkServiceCreate(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "service", "create", mockServiceName, mockNwName)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestClientNetworkServiceRm(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "service", "rm", mockServiceName, mockNwName)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestClientNetworkServiceLs(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "service", "ls", mockNwName)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestClientNetworkServiceInfo(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "service", "info", mockServiceName, mockNwName)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestClientNetworkServiceInfoById(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "service", "info", mockServiceID, mockNwID)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestClientNetworkServiceJoin(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "service", "join", mockContainerID, mockServiceName, mockNwName)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestClientNetworkServiceLeave(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "service", "leave", mockContainerID, mockServiceName, mockNwName)
	if err != nil {
		t.Fatal(err.Error())
	}
}

// Docker Flag processing in flag.go uses os.Exit() frequently, even for --help
// TODO : Handle the --help test-case in the IT when CLI is available
/*
func TestClientNetworkServiceCreateHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nil, 0, nil
	}
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "create", "--help")
	if err != nil {
		t.Fatalf(err.Error())
	}
}
*/

// Docker flag processing in flag.go uses os.Exit(1) for incorrect parameter case.
// TODO : Handle the missing argument case in the IT when CLI is available
/*
func TestClientNetworkServiceCreateMissingArgument(t *testing.T) {
	var out, errOut bytes.Buffer
	cFunc := func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		return nil, 0, nil
	}
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "create")
	if err != nil {
		t.Fatal(err.Error())
	}
}
*/
