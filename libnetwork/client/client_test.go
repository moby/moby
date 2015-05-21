package client

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	_ "github.com/docker/libnetwork/netutils"
)

// nopCloser is used to provide a dummy CallFunc for Cmd()
type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func TestMain(m *testing.M) {
	setupMockHTTPCallback()
	os.Exit(m.Run())
}

var callbackFunc func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error)
var mockNwJSON, mockNwListJSON []byte
var mockNwName = "test"
var mockNwID = "23456789"

func setupMockHTTPCallback() {
	var list []networkResource
	nw := networkResource{Name: mockNwName, ID: mockNwID}
	mockNwJSON, _ = json.Marshal(nw)
	list = append(list, nw)
	mockNwListJSON, _ = json.Marshal(list)
	callbackFunc = func(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
		var rsp string
		switch method {
		case "GET":
			if strings.Contains(path, "networks?name=") {
				rsp = string(mockNwListJSON)
			} else if strings.HasSuffix(path, "networks") {
				rsp = string(mockNwListJSON)
			} else if strings.HasSuffix(path, "networks/"+mockNwID) {
				rsp = string(mockNwJSON)
			}
		case "POST":
			rsp = mockNwID
		case "PUT":
		case "DELETE":
			rsp = ""
		}
		return nopCloser{bytes.NewBufferString(rsp)}, 200, nil
	}
}

func TestClientDummyCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "dummy")
	if err == nil {
		t.Fatalf("Incorrect Command must fail")
	}
}

func TestClientNetworkInvalidCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "invalid")
	if err == nil {
		t.Fatalf("Passing invalid commands must fail")
	}
}

func TestClientNetworkCreate(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "create", mockNwName)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestClientNetworkCreateWithDriver(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "create", "-f=dummy", mockNwName)
	if err == nil {
		t.Fatalf("Passing incorrect flags to the create command must fail")
	}

	err = cli.Cmd("docker", "network", "create", "-d=dummy", mockNwName)
	if err != nil {
		t.Fatalf(err.Error())
	}
}

func TestClientNetworkRm(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "rm", mockNwName)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestClientNetworkLs(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "ls")
	if err != nil {
		t.Fatal(err.Error())
	}
	if out.String() != string(mockNwListJSON) {
		t.Fatal("Network List command fail to return the expected list")
	}
}

func TestClientNetworkInfo(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "info", mockNwName)
	if err != nil {
		t.Fatal(err.Error())
	}
	if out.String() != string(mockNwJSON) {
		t.Fatal("Network info command fail to return the expected object")
	}
}

func TestClientNetworkInfoById(t *testing.T) {
	var out, errOut bytes.Buffer
	cli := NewNetworkCli(&out, &errOut, callbackFunc)

	err := cli.Cmd("docker", "network", "info", mockNwID)
	if err != nil {
		t.Fatal(err.Error())
	}
	if out.String() != string(mockNwJSON) {
		t.Fatal("Network info command fail to return the expected object")
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
func TestClientNetworkCreateMissingArgument(t *testing.T) {
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
