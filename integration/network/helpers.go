// +build !windows

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/parsers/kernel"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

// CreateMasterDummy creates a dummy network interface
func CreateMasterDummy(ctx context.Context, t *testing.T, c client.APIClient, master string) {
	// ip link add <dummy_name> type dummy
	id := container.Create(
		ctx,
		t,
		c,
		container.WithNetworkMode("host"),
		container.WithCmd("/bin/sh", "-c", "ip link add "+master+" type dummy && ip link set "+master+" up"),
		container.WithPrivileged(true),
	)
	runAndWait(ctx, t, c, id)
}

// CreateVlanInterface creates a vlan network interface
func CreateVlanInterface(ctx context.Context, t *testing.T, c client.APIClient, master, slave, id string) {
	cid := container.Create(
		ctx,
		t,
		c,
		container.WithNetworkMode("host"),
		container.WithCmd("/bin/sh", "-c", "ip link add link "+master+" name "+slave+" type vlan id "+id+" && ip link set "+slave+" up"),
		container.WithPrivileged(true),
	)
	runAndWait(ctx, t, c, cid)
}

func runAndWait(ctx context.Context, t *testing.T, c client.APIClient, id string) {
	t.Helper()

	chWait, chErr := c.ContainerWait(ctx, id, containertypes.WaitConditionNextExit)

	err := c.ContainerStart(ctx, id, types.ContainerStartOptions{})
	assert.NilError(t, err)

	select {
	case <-ctx.Done():
		assert.NilError(t, err)
	case status := <-chWait:
		assert.Assert(t, status.Error == nil)
		var logs string
		if status.StatusCode != 0 {
			stream, _ := c.ContainerLogs(ctx, id, types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			})
			b, _ := ioutil.ReadAll(stream)
			logs = string(b)
		}
		assert.Equal(t, status.StatusCode, int64(0), logs)
	case err = <-chErr:
		assert.NilError(t, err)
	}

}

// LinkExists verifies that a link exists
func LinkExists(ctx context.Context, t *testing.T, c client.APIClient, master string) {
	// verify the specified link exists, ip link show <link_name>
	id := container.Create(
		ctx,
		t,
		c,
		container.WithNetworkMode("host"),
		container.WithCmd("/bin/sh", "-c", "ip link show "+master),
	)
	runAndWait(ctx, t, c, id)
}

// IsNetworkAvailable provides a comparison to check if a docker network is available
func IsNetworkAvailable(c client.NetworkAPIClient, name string) cmp.Comparison {
	return func() cmp.Result {
		networks, err := c.NetworkList(context.Background(), types.NetworkListOptions{})
		if err != nil {
			return cmp.ResultFromError(err)
		}
		for _, network := range networks {
			if network.Name == name {
				return cmp.ResultSuccess
			}
		}
		return cmp.ResultFailure(fmt.Sprintf("could not find network %s", name))
	}
}

// IsNetworkNotAvailable provides a comparison to check if a docker network is not available
func IsNetworkNotAvailable(c client.NetworkAPIClient, name string) cmp.Comparison {
	return func() cmp.Result {
		networks, err := c.NetworkList(context.Background(), types.NetworkListOptions{})
		if err != nil {
			return cmp.ResultFromError(err)
		}
		for _, network := range networks {
			if network.Name == name {
				return cmp.ResultFailure(fmt.Sprintf("network %s is still present", name))
			}
		}
		return cmp.ResultSuccess
	}
}

// CheckKernelMajorVersionGreaterOrEqualThen returns whether the kernel version is greater or equal than the one provided
func CheckKernelMajorVersionGreaterOrEqualThen(kernelVersion int, majorVersion int) bool {
	kv, err := kernel.GetKernelVersion()
	if err != nil {
		return false
	}
	if kv.Kernel < kernelVersion || (kv.Kernel == kernelVersion && kv.Major < majorVersion) {
		return false
	}
	return true
}

// IsUserNamespace returns whether the user namespace remapping is enabled
func IsUserNamespace() bool {
	root := os.Getenv("DOCKER_REMAP_ROOT")
	return root != ""
}
