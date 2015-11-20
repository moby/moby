// +build !windows,!test_no_exec

package main

import (
	"bytes"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"github.com/kr/pty"
)

// regression test for #12546
func (s *DockerSuite) TestExecInteractiveStdinClose(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-itd", "busybox", "/bin/cat")
	contID := strings.TrimSpace(out)

	cmd := exec.Command(dockerBinary, "exec", "-i", contID, "echo", "-n", "hello")
	p, err := pty.Start(cmd)
	c.Assert(err, checker.IsNil)

	b := bytes.NewBuffer(nil)
	go io.Copy(b, p)

	ch := make(chan error)
	go func() { ch <- cmd.Wait() }()

	select {
	case err := <-ch:
		c.Assert(err, checker.IsNil)
		output := b.String()
		c.Assert(strings.TrimSpace(output), checker.Equals, "hello")
	case <-time.After(5 * time.Second):
		c.Fatal("timed out running docker exec")
	}
}

func (s *DockerSuite) TestExecTTY(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name=test", "busybox", "sh", "-c", "echo hello > /foo && top")

	cmd := exec.Command(dockerBinary, "exec", "-it", "test", "sh")
	p, err := pty.Start(cmd)
	c.Assert(err, checker.IsNil)
	defer p.Close()

	_, err = p.Write([]byte("cat /foo && exit\n"))
	c.Assert(err, checker.IsNil)

	chErr := make(chan error)
	go func() {
		chErr <- cmd.Wait()
	}()
	select {
	case err := <-chErr:
		c.Assert(err, checker.IsNil)
	case <-time.After(3 * time.Second):
		c.Fatal("timeout waiting for exec to exit")
	}

	buf := make([]byte, 256)
	read, err := p.Read(buf)
	c.Assert(err, checker.IsNil)
	c.Assert(bytes.Contains(buf, []byte("hello")), checker.Equals, true, check.Commentf(string(buf[:read])))
}
