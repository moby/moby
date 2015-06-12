package client

import (
	"bytes"
	"testing"

	_ "github.com/docker/libnetwork/netutils"
)

func TestClientServiceInvalidCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "service", "invalid")
	if err == nil {
		t.Fatalf("Passing invalid commands must fail")
	}
}

func TestClientServiceCreate(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "service", "publish", "-net="+mockNwName, mockServiceName)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientServiceRm(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "service", "unpublish", "-net="+mockNwName, mockServiceName)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientServiceLs(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "service", "ls")
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientServiceInfo(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "service", "info", "-net="+mockNwName, mockServiceName)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientServiceInfoById(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "service", "info", "-net="+mockNwName, mockServiceID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientServiceJoin(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "service", "attach", "-net="+mockNwName, mockContainerID, mockServiceName)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientServiceLeave(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "service", "detach", "-net="+mockNwName, mockContainerID, mockServiceName)
	if err != nil {
		t.Fatal(err)
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
