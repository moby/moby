package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/poll"
	"gotest.tools/skip"
)

func TestKernelTCPMemory(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.40"), "skip test from new feature")
	skip.If(t, !testEnv.DaemonInfo.KernelMemoryTCP)

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	const (
		kernelMemoryTCP int64 = 200 * 1024 * 1024
	)

	cID := container.Run(t, ctx, client, func(c *container.TestContainerConfig) {
		c.HostConfig.Resources = containertypes.Resources{
			KernelMemoryTCP: kernelMemoryTCP,
		}
	})

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(kernelMemoryTCP, inspect.HostConfig.KernelMemoryTCP))

	res, err := container.Exec(ctx, client, cID,
		[]string{"cat", "/sys/fs/cgroup/memory/memory.kmem.tcp.limit_in_bytes"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Equal(strconv.FormatInt(kernelMemoryTCP, 10), strings.TrimSpace(res.Stdout())))
}

func TestNISDomainname(t *testing.T) {
	// Older versions of the daemon would concatenate hostname and domainname,
	// so hostname "foobar" and domainname "baz.cyphar.com" would produce
	// `foobar.baz.cyphar.com` as hostname.
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.40"), "skip test from new feature")
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	const (
		hostname   = "foobar"
		domainname = "baz.cyphar.com"
	)

	cID := container.Run(t, ctx, client, func(c *container.TestContainerConfig) {
		c.Config.Hostname = hostname
		c.Config.Domainname = domainname
	})

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(hostname, inspect.Config.Hostname))
	assert.Check(t, is.Equal(domainname, inspect.Config.Domainname))

	// Check hostname.
	res, err := container.Exec(ctx, client, cID,
		[]string{"cat", "/proc/sys/kernel/hostname"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Equal(hostname, strings.TrimSpace(res.Stdout())))

	// Check domainname.
	res, err = container.Exec(ctx, client, cID,
		[]string{"cat", "/proc/sys/kernel/domainname"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Equal(domainname, strings.TrimSpace(res.Stdout())))
}

func TestCgroupNamespaces(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())

	if _, err := os.Stat("/proc/self/ns/cgroup"); os.IsNotExist(err) {
		t.Skip("cgroup namespaces are unsupported")
	}

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(t, ctx, client)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	path := filepath.Join(os.Getenv("DEST"), "docker.pid")
	b, err := ioutil.ReadFile(path)
	assert.NilError(t, err)
	link, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/cgroup", string(b)))
	assert.NilError(t, err)

	// Check that the container's cgroup doesn't match the docker daemon's
	res, err := container.Exec(ctx, client, cID, []string{"readlink", "/proc/1/ns/cgroup"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Assert(t, link != strings.TrimSpace(res.Stdout()))
}
