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

func TestUpdateMemory(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, !testEnv.DaemonInfo.MemoryLimit)
	skip.If(t, !testEnv.DaemonInfo.SwapLimit)

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, func(c *container.TestContainerConfig) {
		c.HostConfig.Resources = containertypes.Resources{
			Memory: 200 * 1024 * 1024,
		}
	})

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	const (
		setMemory     int64 = 314572800
		setMemorySwap int64 = 524288000
	)

	_, err := client.ContainerUpdate(ctx, cID, containertypes.UpdateConfig{
		Resources: containertypes.Resources{
			Memory:     setMemory,
			MemorySwap: setMemorySwap,
		},
	})
	assert.NilError(t, err)

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(setMemory, inspect.HostConfig.Memory))
	assert.Check(t, is.Equal(setMemorySwap, inspect.HostConfig.MemorySwap))

	res, err := container.Exec(ctx, client, cID,
		[]string{"cat", "/sys/fs/cgroup/memory/memory.limit_in_bytes"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Equal(strconv.FormatInt(setMemory, 10), strings.TrimSpace(res.Stdout())))

	res, err = container.Exec(ctx, client, cID,
		[]string{"cat", "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Equal(strconv.FormatInt(setMemorySwap, 10), strings.TrimSpace(res.Stdout())))
}

func TestUpdateCPUQuota(t *testing.T) {
	t.Parallel()

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client)

	for _, test := range []struct {
		desc   string
		update int64
	}{
		{desc: "some random value", update: 15000},
		{desc: "a higher value", update: 20000},
		{desc: "a lower value", update: 10000},
		{desc: "unset value", update: -1},
	} {
		if _, err := client.ContainerUpdate(ctx, cID, containertypes.UpdateConfig{
			Resources: containertypes.Resources{
				CPUQuota: test.update,
			},
		}); err != nil {
			t.Fatal(err)
		}

		inspect, err := client.ContainerInspect(ctx, cID)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(test.update, inspect.HostConfig.CPUQuota))

		res, err := container.Exec(ctx, client, cID,
			[]string{"/bin/cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"})
		assert.NilError(t, err)
		assert.Assert(t, is.Len(res.Stderr(), 0))
		assert.Equal(t, 0, res.ExitCode)

		assert.Check(t, is.Equal(strconv.FormatInt(test.update, 10), strings.TrimSpace(res.Stdout())))
	}
}
