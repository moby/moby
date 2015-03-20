package main

import (
	"net"
	"os/exec"
	"strings"
	"testing"
)

func TestCliProxyDisableProxyUnixSock(t *testing.T) {
	testRequires(t, SameHostDaemon) // test is valid when DOCKER_HOST=unix://..

	cmd := exec.Command(dockerBinary, "info")
	cmd.Env = appendBaseEnv([]string{"HTTP_PROXY=http://127.0.0.1:9999"})

	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}

	logDone("cli proxy - HTTP_PROXY is not used when connecting to unix sock")
}

// Can't use localhost here since go has a special case to not use proxy if connecting to localhost
// See http://golang.org/pkg/net/http/#ProxyFromEnvironment
func TestCliProxyProxyTCPSock(t *testing.T) {
	testRequires(t, SameHostDaemon)
	// get the IP to use to connect since we can't use localhost
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	var ip string
	for _, addr := range addrs {
		sAddr := addr.String()
		if !strings.Contains(sAddr, "127.0.0.1") {
			addrArr := strings.Split(sAddr, "/")
			ip = addrArr[0]
			break
		}
	}

	if ip == "" {
		t.Fatal("could not find ip to connect to")
	}

	d := NewDaemon(t)
	if err := d.Start("-H", "tcp://"+ip+":2375"); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "info")
	cmd.Env = []string{"DOCKER_HOST=tcp://" + ip + ":2375", "HTTP_PROXY=127.0.0.1:9999"}
	if out, _, err := runCommandWithOutput(cmd); err == nil {
		t.Fatal(err, out)
	}

	// Test with no_proxy
	cmd.Env = append(cmd.Env, "NO_PROXY="+ip)
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "info")); err != nil {
		t.Fatal(err, out)
	}

	logDone("cli proxy - HTTP_PROXY is used for TCP sock")
}
