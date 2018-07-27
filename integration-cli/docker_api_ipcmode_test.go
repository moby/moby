// build +linux
package main

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/go-check/check"
)

/* testIpcCheckDevExists checks whether a given mount (identified by its
 * major:minor pair from /proc/self/mountinfo) exists on the host system.
 *
 * The format of /proc/self/mountinfo is like:
 *
 * 29 23 0:24 / /dev/shm rw,nosuid,nodev shared:4 - tmpfs tmpfs rw
 *       ^^^^\
 *            - this is the minor:major we look for
 */
func testIpcCheckDevExists(mm string) (bool, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 7 {
			continue
		}
		if fields[2] == mm {
			return true, nil
		}
	}

	return false, s.Err()
}

/* TestAPIIpcModeHost checks that a container created with --ipc host
 * can use IPC of the host system.
 */
func (s *DockerSuite) TestAPIIpcModeHost(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon, NotUserNamespace)

	cfg := container.Config{
		Image: "busybox",
		Cmd:   []string{"top"},
	}
	hostCfg := container.HostConfig{
		IpcMode: container.IpcMode("host"),
	}
	ctx := context.Background()

	client := testEnv.APIClient()
	resp, err := client.ContainerCreate(ctx, &cfg, &hostCfg, nil, "")
	c.Assert(err, checker.IsNil)
	c.Assert(len(resp.Warnings), checker.Equals, 0)
	name := resp.ID

	err = client.ContainerStart(ctx, name, types.ContainerStartOptions{})
	c.Assert(err, checker.IsNil)

	// check that IPC is shared
	// 1. create a file inside container
	cli.DockerCmd(c, "exec", name, "sh", "-c", "printf covfefe > /dev/shm/."+name)
	// 2. check it's the same on the host
	bytes, err := ioutil.ReadFile("/dev/shm/." + name)
	c.Assert(err, checker.IsNil)
	c.Assert(string(bytes), checker.Matches, "^covfefe$")
	// 3. clean up
	cli.DockerCmd(c, "exec", name, "rm", "-f", "/dev/shm/."+name)
}
