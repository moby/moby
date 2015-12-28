// +build !windows

package main

import (
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestApiUpdateContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)
	testRequires(c, swapMemorySupport)

	name := "apiUpdateContainer"
	hostConfig := map[string]interface{}{
		"Memory":     314572800,
		"MemorySwap": 524288000,
	}
	dockerCmd(c, "run", "-d", "--name", name, "-m", "200M", "busybox", "top")
	_, _, err := sockRequest("POST", "/containers/"+name+"/update", hostConfig)
	c.Assert(err, check.IsNil)

	memory, err := inspectField(name, "HostConfig.Memory")
	c.Assert(err, check.IsNil)
	if memory != "314572800" {
		c.Fatalf("Got the wrong memory value, we got %d, expected 314572800(300M).", memory)
	}
	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	out, _ := dockerCmd(c, "exec", name, "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "314572800")

	memorySwap, err := inspectField(name, "HostConfig.MemorySwap")
	c.Assert(err, check.IsNil)
	if memorySwap != "524288000" {
		c.Fatalf("Got the wrong memorySwap value, we got %d, expected 524288000(500M).", memorySwap)
	}
	file = "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"
	out, _ = dockerCmd(c, "exec", name, "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "524288000")
}
