// +build windows

package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"

	winio "github.com/Microsoft/go-winio"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/request"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestContainersAPICreateMountsBindNamedPipe(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsWindowsAtLeastBuild(16299)) // Named pipe support was added in RS3

	// Create a host pipe to map into the container
	hostPipeName := fmt.Sprintf(`\\.\pipe\docker-cli-test-pipe-%x`, rand.Uint64())
	pc := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;AU)", // Allow all users access to the pipe
	}
	l, err := winio.ListenPipe(hostPipeName, pc)
	if err != nil {
		c.Fatal(err)
	}
	defer l.Close()

	// Asynchronously read data that the container writes to the mapped pipe.
	var b []byte
	ch := make(chan error)
	go func() {
		conn, err := l.Accept()
		if err == nil {
			b, err = ioutil.ReadAll(conn)
			conn.Close()
		}
		ch <- err
	}()

	containerPipeName := `\\.\pipe\docker-cli-test-pipe`
	text := "hello from a pipe"
	cmd := fmt.Sprintf("echo %s > %s", text, containerPipeName)

	name := "test-bind-npipe"
	data := map[string]interface{}{
		"Image":      testEnv.PlatformDefaults.BaseImage,
		"Cmd":        []string{"cmd", "/c", cmd},
		"HostConfig": map[string]interface{}{"Mounts": []map[string]interface{}{{"Type": "npipe", "Source": hostPipeName, "Target": containerPipeName}}},
	}

	status, resp, err := request.SockRequest("POST", "/containers/create?name="+name, data, daemonHost())
	c.Assert(err, checker.IsNil, check.Commentf(string(resp)))
	c.Assert(status, checker.Equals, http.StatusCreated, check.Commentf(string(resp)))

	status, _, err = request.SockRequest("POST", "/containers/"+name+"/start", nil, daemonHost())
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	err = <-ch
	if err != nil {
		c.Fatal(err)
	}
	result := strings.TrimSpace(string(b))
	if result != text {
		c.Errorf("expected pipe to contain %s, got %s", text, result)
	}
}
