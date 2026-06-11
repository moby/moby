package container

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/blkiodev"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/request"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestUpdateMemory(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")
	skip.If(t, !testEnv.DaemonInfo.MemoryLimit)
	skip.If(t, !testEnv.DaemonInfo.SwapLimit)

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.HostConfig.Resources = containertypes.Resources{
			Memory: 200 * 1024 * 1024,
		}
	})

	const (
		setMemory     int64 = 314572800
		setMemorySwap int64 = 524288000
	)

	_, err := apiClient.ContainerUpdate(ctx, cID, client.ContainerUpdateOptions{
		Resources: &containertypes.Resources{
			Memory:     setMemory,
			MemorySwap: setMemorySwap,
		},
	})
	assert.NilError(t, err)

	inspect, err := apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(setMemory, inspect.Container.HostConfig.Memory))
	assert.Check(t, is.Equal(setMemorySwap, inspect.Container.HostConfig.MemorySwap))

	memoryFile := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	if testEnv.DaemonInfo.CgroupVersion == "2" {
		memoryFile = "/sys/fs/cgroup/memory.max"
	}
	res, err := container.Exec(ctx, apiClient, cID,
		[]string{"cat", memoryFile})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Equal(strconv.FormatInt(setMemory, 10), strings.TrimSpace(res.Stdout())))

	// see ConvertMemorySwapToCgroupV2Value() for the convention:
	// https://github.com/opencontainers/runc/commit/c86be8a2c118ca7bad7bbe9eaf106c659a83940d
	if testEnv.DaemonInfo.CgroupVersion == "2" {
		res, err = container.Exec(ctx, apiClient, cID,
			[]string{"cat", "/sys/fs/cgroup/memory.swap.max"})
		assert.NilError(t, err)
		assert.Assert(t, is.Len(res.Stderr(), 0))
		assert.Equal(t, 0, res.ExitCode)
		assert.Check(t, is.Equal(strconv.FormatInt(setMemorySwap-setMemory, 10), strings.TrimSpace(res.Stdout())))
	} else {
		res, err = container.Exec(ctx, apiClient, cID,
			[]string{"cat", "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"})
		assert.NilError(t, err)
		assert.Assert(t, is.Len(res.Stderr(), 0))
		assert.Equal(t, 0, res.ExitCode)
		assert.Check(t, is.Equal(strconv.FormatInt(setMemorySwap, 10), strings.TrimSpace(res.Stdout())))
	}
}

func TestUpdateCPUQuota(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)

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
			_, err := apiClient.ContainerUpdate(ctx, cID, client.ContainerUpdateOptions{
				Resources: &containertypes.Resources{
					CPUQuota:  test.update,
					CPUPeriod: 100000,
				},
			})
			assert.NilError(t, err)
		} else {
			_, err := apiClient.ContainerUpdate(ctx, cID, client.ContainerUpdateOptions{
				Resources: &containertypes.Resources{
					CPUQuota: test.update,
				},
			})
			assert.NilError(t, err)
		}

		inspect, err := apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(test.update, inspect.Container.HostConfig.CPUQuota))

		if testEnv.DaemonInfo.CgroupVersion == "2" {
			res, err := container.Exec(ctx, apiClient, cID,
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
			res, err := container.Exec(ctx, apiClient, cID,
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

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()
	oldAPIClient := request.NewAPIClient(t, client.WithAPIVersion("1.24"))

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
			c = oldAPIClient
		}

		t.Run(test.desc, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			// Using "network=host" to speed up creation (13.96s vs 6.54s)
			cID := container.Run(ctx, t, apiClient, container.WithPidsLimit(test.initial), container.WithNetworkMode("host"))

			_, err := c.ContainerUpdate(ctx, cID, client.ContainerUpdateOptions{
				Resources: &containertypes.Resources{
					PidsLimit: test.update,
				},
			})
			assert.NilError(t, err)

			inspect, err := c.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
			assert.NilError(t, err)
			assert.Assert(t, inspect.Container.HostConfig.Resources.PidsLimit != nil)
			assert.Equal(t, *inspect.Container.HostConfig.Resources.PidsLimit, test.expect)

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

// blkioTestDevice finds a usable block device on the host by examining the
// device backing the root filesystem.  The returned path (e.g. "/dev/sda")
// and the decimal major:minor string are suitable for docker update flags and
// cgroup file lookups respectively.
func blkioTestDevice(t *testing.T) (devPath, majMin string) {
	t.Helper()

	var st unix.Stat_t
	assert.NilError(t, unix.Stat("/", &st), "stat /")

	major := unix.Major(st.Dev)
	minor := unix.Minor(st.Dev)

	// Resolve the device name via sysfs so we get a real /dev/… path.
	uevent, err := os.ReadFile(fmt.Sprintf("/sys/dev/block/%d:%d/uevent", major, minor))
	if err != nil {
		t.Skipf("cannot resolve block device for / from sysfs: %v", err)
	}
	var name string
	for _, line := range strings.Split(string(uevent), "\n") {
		if after, ok := strings.CutPrefix(line, "DEVNAME="); ok {
			name = strings.TrimSpace(after)
			break
		}
	}
	if name == "" {
		t.Skip("DEVNAME not found in sysfs uevent for / block device")
	}
	return "/dev/" + name, fmt.Sprintf("%d:%d", major, minor)
}

// parseIOMax returns the value of field (rbps/wbps/riops/wiops) for the given
// major:minor device from a cgroup v2 io.max file content.
func parseIOMax(content, majMin, field string) string {
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, majMin+" ") {
			continue
		}
		for _, part := range strings.Fields(line[len(majMin)+1:]) {
			if k, v, ok := strings.Cut(part, "="); ok && k == field {
				return v
			}
		}
	}
	return ""
}

// TestUpdateBlkioThrottleDevices verifies that docker update correctly applies
// per-device blkio throttle limits to the container's cgroup.
//
// On cgroup v2 the limits are reflected in /sys/fs/cgroup/io.max inside the
// container.  On cgroup v1 only the API-level values are verified (the blkio
// throttle files are not bind-mounted into the container namespace).
func TestUpdateBlkioThrottleDevices(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")

	devPath, majMin := blkioTestDevice(t)

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)

	const (
		readBps   uint64 = 5 * 1024 * 1024 // 5 MiB/s
		writeBps  uint64 = 2 * 1024 * 1024 // 2 MiB/s
		readIops  uint64 = 100
		writeIops uint64 = 50
	)

	_, err := apiClient.ContainerUpdate(ctx, cID, client.ContainerUpdateOptions{
		Resources: &containertypes.Resources{
			BlkioDeviceReadBps:   []*blkiodev.ThrottleDevice{{Path: devPath, Rate: readBps}},
			BlkioDeviceWriteBps:  []*blkiodev.ThrottleDevice{{Path: devPath, Rate: writeBps}},
			BlkioDeviceReadIOps:  []*blkiodev.ThrottleDevice{{Path: devPath, Rate: readIops}},
			BlkioDeviceWriteIOps: []*blkiodev.ThrottleDevice{{Path: devPath, Rate: writeIops}},
		},
	})
	assert.NilError(t, err)

	// Verify API-level values are persisted.
	inspect, err := apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(inspect.Container.HostConfig.BlkioDeviceReadBps, 1))
	assert.Check(t, is.Equal(inspect.Container.HostConfig.BlkioDeviceReadBps[0].Rate, readBps))
	assert.Assert(t, is.Len(inspect.Container.HostConfig.BlkioDeviceWriteBps, 1))
	assert.Check(t, is.Equal(inspect.Container.HostConfig.BlkioDeviceWriteBps[0].Rate, writeBps))
	assert.Assert(t, is.Len(inspect.Container.HostConfig.BlkioDeviceReadIOps, 1))
	assert.Check(t, is.Equal(inspect.Container.HostConfig.BlkioDeviceReadIOps[0].Rate, readIops))
	assert.Assert(t, is.Len(inspect.Container.HostConfig.BlkioDeviceWriteIOps, 1))
	assert.Check(t, is.Equal(inspect.Container.HostConfig.BlkioDeviceWriteIOps[0].Rate, writeIops))

	// On cgroup v2 verify the limits landed in io.max inside the container.
	if testEnv.DaemonInfo.CgroupVersion != "2" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := container.Exec(ctx, apiClient, cID, []string{"cat", "/sys/fs/cgroup/io.max"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)

	ioMax := res.Stdout()
	assert.Check(t, is.Equal(parseIOMax(ioMax, majMin, "rbps"), strconv.FormatUint(readBps, 10)),
		"io.max rbps for %s (io.max content: %q)", majMin, ioMax)
	assert.Check(t, is.Equal(parseIOMax(ioMax, majMin, "wbps"), strconv.FormatUint(writeBps, 10)),
		"io.max wbps for %s (io.max content: %q)", majMin, ioMax)
	assert.Check(t, is.Equal(parseIOMax(ioMax, majMin, "riops"), strconv.FormatUint(readIops, 10)),
		"io.max riops for %s (io.max content: %q)", majMin, ioMax)
	assert.Check(t, is.Equal(parseIOMax(ioMax, majMin, "wiops"), strconv.FormatUint(writeIops, 10)),
		"io.max wiops for %s (io.max content: %q)", majMin, ioMax)
}
