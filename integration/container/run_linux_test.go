package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/poll"
	"gotest.tools/skip"
)

func TestKernelTCPMemory(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, !testEnv.DaemonInfo.KernelMemoryTCP)

	defer setupTest(t)()
	client := request.NewAPIClient(t)
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
