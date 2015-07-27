package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestExecResizeApiHeightWidthNoInt(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	endpoint := "/exec/" + cleanedContainerID + "/resize?h=foo&w=bar"
	status, _, err := sockRequest("POST", endpoint, nil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	c.Assert(err, check.IsNil)
}

// Part of #14845
func (s *DockerSuite) TestExecResizeImmediatelyAfterExecStart(c *check.C) {
	testRequires(c, NativeExecDriver)

	name := "exec_resize_test"
	dockerCmd(c, "run", "-d", "-i", "-t", "--name", name, "--restart", "always", "busybox", "/bin/sh")

	// The panic happens when daemon.ContainerExecStart is called but the
	// container.Exec is not called.
	// Because the panic is not 100% reproducible, we send the requests concurrently
	// to increase the probability that the problem is triggered.
	n := 10
	ch := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			defer func() {
				ch <- struct{}{}
			}()

			data := map[string]interface{}{
				"AttachStdin": true,
				"Cmd":         []string{"/bin/sh"},
			}
			status, body, err := sockRequest("POST", fmt.Sprintf("/containers/%s/exec", name), data)
			c.Assert(err, check.IsNil)
			c.Assert(status, check.Equals, http.StatusCreated)

			out := map[string]string{}
			err = json.Unmarshal(body, &out)
			c.Assert(err, check.IsNil)

			execID := out["Id"]
			if len(execID) < 1 {
				c.Fatal("ExecCreate got invalid execID")
			}

			payload := bytes.NewBufferString(`{"Tty":true}`)
			conn, _, err := sockRequestHijack("POST", fmt.Sprintf("/exec/%s/start", execID), payload, "application/json")
			c.Assert(err, check.IsNil)
			defer conn.Close()

			_, rc, err := sockRequestRaw("POST", fmt.Sprintf("/exec/%s/resize?h=24&w=80", execID), nil, "text/plain")
			// It's probably a panic of the daemon if io.ErrUnexpectedEOF is returned.
			if err == io.ErrUnexpectedEOF {
				c.Fatal("The daemon might have crashed.")
			}

			if err == nil {
				rc.Close()
			}
		}()
	}

	for i := 0; i < n; i++ {
		<-ch
	}
}
