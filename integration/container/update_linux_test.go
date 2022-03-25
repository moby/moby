package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestUpdateMemory(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")
	skip.If(t, !testEnv.DaemonInfo.MemoryLimit)
	skip.If(t, !testEnv.DaemonInfo.SwapLimit)

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client, func(c *container.TestContainerConfig) {
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

	memoryFile := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	if testEnv.DaemonInfo.CgroupVersion == "2" {
		memoryFile = "/sys/fs/cgroup/memory.max"
	}
	res, err := container.Exec(ctx, client, cID,
		[]string{"cat", memoryFile})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Equal(strconv.FormatInt(setMemory, 10), strings.TrimSpace(res.Stdout())))

	// see ConvertMemorySwapToCgroupV2Value() for the convention:
	// https://github.com/opencontainers/runc/commit/c86be8a2c118ca7bad7bbe9eaf106c659a83940d
	if testEnv.DaemonInfo.CgroupVersion == "2" {
		res, err = container.Exec(ctx, client, cID,
			[]string{"cat", "/sys/fs/cgroup/memory.swap.max"})
		assert.NilError(t, err)
		assert.Assert(t, is.Len(res.Stderr(), 0))
		assert.Equal(t, 0, res.ExitCode)
		assert.Check(t, is.Equal(strconv.FormatInt(setMemorySwap-setMemory, 10), strings.TrimSpace(res.Stdout())))
	} else {
		res, err = container.Exec(ctx, client, cID,
			[]string{"cat", "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"})
		assert.NilError(t, err)
		assert.Assert(t, is.Len(res.Stderr(), 0))
		assert.Equal(t, 0, res.ExitCode)
		assert.Check(t, is.Equal(strconv.FormatInt(setMemorySwap, 10), strings.TrimSpace(res.Stdout())))
	}
}

func TestUpdateCPUQuota(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client)

	for _, test := range []struct {
		desc   string
		update int64
	}{
		{desc: "some random value", update: 15000},
		{desc: "a higher value", update: 20000},
		{desc: "a lower value", update: 10000},
		{desc: "unset value", update: -1},
	} {
		if testEnv.DaemonInfo.CgroupVersion == "2" {
			// On v2, specifying CPUQuota without CPUPeriod is currently broken:
			// https://github.com/opencontainers/runc/issues/2456
			// As a workaround we set them together.
			_, err := client.ContainerUpdate(ctx, cID, containertypes.UpdateConfig{
				Resources: containertypes.Resources{
					CPUQuota:  test.update,
					CPUPeriod: 100000,
				},
			})
			assert.NilError(t, err)
		} else {
			_, err := client.ContainerUpdate(ctx, cID, containertypes.UpdateConfig{
				Resources: containertypes.Resources{
					CPUQuota: test.update,
				},
			})
			assert.NilError(t, err)
		}

		inspect, err := client.ContainerInspect(ctx, cID)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(test.update, inspect.HostConfig.CPUQuota))

		if testEnv.DaemonInfo.CgroupVersion == "2" {
			res, err := container.Exec(ctx, client, cID,
				[]string{"/bin/cat", "/sys/fs/cgroup/cpu.max"})
			assert.NilError(t, err)
			assert.Assert(t, is.Len(res.Stderr(), 0))
			assert.Equal(t, 0, res.ExitCode)

			quotaPeriodPair := strings.Fields(res.Stdout())
			quota := quotaPeriodPair[0]
			if test.update == -1 {
				assert.Check(t, is.Equal("max", quota))
			} else {
				assert.Check(t, is.Equal(strconv.FormatInt(test.update, 10), quota))
			}
		} else {
			res, err := container.Exec(ctx, client, cID,
				[]string{"/bin/cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"})
			assert.NilError(t, err)
			assert.Assert(t, is.Len(res.Stderr(), 0))
			assert.Equal(t, 0, res.ExitCode)

			assert.Check(t, is.Equal(strconv.FormatInt(test.update, 10), strings.TrimSpace(res.Stdout())))
		}
	}
}

func TestUpdatePidsLimit(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")
	skip.If(t, !testEnv.DaemonInfo.PidsLimit)

	defer setupTest(t)()
	apiClient := testEnv.APIClient()
	oldAPIclient := request.NewAPIClient(t, client.WithVersion("1.24"))
	ctx := context.Background()

	intPtr := func(i int64) *int64 {
		return &i
	}

	for _, test := range []struct {
		desc     string
		oldAPI   bool
		initial  *int64
		update   *int64
		expect   int64
		expectCg string
	}{
		{desc: "update from none", update: intPtr(32), expect: 32, expectCg: "32"},
		{desc: "no change", initial: intPtr(32), expect: 32, expectCg: "32"},
		{desc: "update lower", initial: intPtr(32), update: intPtr(16), expect: 16, expectCg: "16"},
		{desc: "update on old api ignores value", oldAPI: true, initial: intPtr(32), update: intPtr(16), expect: 32, expectCg: "32"},
		{desc: "unset limit with zero", initial: intPtr(32), update: intPtr(0), expect: 0, expectCg: "max"},
		{desc: "unset limit with minus one", initial: intPtr(32), update: intPtr(-1), expect: 0, expectCg: "max"},
		{desc: "unset limit with minus two", initial: intPtr(32), update: intPtr(-2), expect: 0, expectCg: "max"},
	} {
		c := apiClient
		if test.oldAPI {
			c = oldAPIclient
		}

		t.Run(test.desc, func(t *testing.T) {
			opts := []func(*container.TestContainerConfig){container.WithPidsLimit(test.initial)}
			// Using "network=host" to speed up creation (13.96s vs 6.54s)
			// This hack does not work with user namespaces
			if !testEnv.IsUserNamespace() {
				opts = append(opts, container.WithNetworkMode("host"))
			}
			cID := container.Run(ctx, t, apiClient, opts...)

			_, err := c.ContainerUpdate(ctx, cID, containertypes.UpdateConfig{
				Resources: containertypes.Resources{
					PidsLimit: test.update,
				},
			})
			assert.NilError(t, err)

			inspect, err := c.ContainerInspect(ctx, cID)
			assert.NilError(t, err)
			assert.Assert(t, inspect.HostConfig.Resources.PidsLimit != nil)
			assert.Equal(t, *inspect.HostConfig.Resources.PidsLimit, test.expect)

			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			pidsFile := "/sys/fs/cgroup/pids/pids.max"
			if testEnv.DaemonInfo.CgroupVersion == "2" {
				pidsFile = "/sys/fs/cgroup/pids.max"
			}
			res, err := container.Exec(ctx, c, cID, []string{"cat", pidsFile})
			assert.NilError(t, err)
			assert.Assert(t, is.Len(res.Stderr(), 0))

			out := strings.TrimSpace(res.Stdout())
			assert.Equal(t, out, test.expectCg)
		})
	}
}
