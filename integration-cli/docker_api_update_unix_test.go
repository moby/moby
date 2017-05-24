// +build !windows

package main

import (
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
	"golang.org/x/net/context"
)

func (s *DockerSuite) TestAPIUpdateContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)
	testRequires(c, swapMemorySupport)

	name := "apiUpdateContainer"
	updateConfig := container.UpdateConfig{
		Resources: container.Resources{
			Memory:     314572800,
			MemorySwap: 524288000,
		},
	}
	dockerCmd(c, "run", "-d", "--name", name, "-m", "200M", "busybox", "top")
	cli, err := client.NewEnvClient()
	c.Assert(err, check.IsNil)
	defer cli.Close()

	_, err = cli.ContainerUpdate(context.Background(), name, updateConfig)

	c.Assert(err, check.IsNil)

	c.Assert(inspectField(c, name, "HostConfig.Memory"), checker.Equals, "314572800")
	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	out, _ := dockerCmd(c, "exec", name, "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "314572800")

	c.Assert(inspectField(c, name, "HostConfig.MemorySwap"), checker.Equals, "524288000")
	file = "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"
	out, _ = dockerCmd(c, "exec", name, "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "524288000")
}
