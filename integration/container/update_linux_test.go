package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"io/ioutil"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	poll.WaitOn(t, containerIsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	_, err := client.ContainerUpdate(ctx, cID, containertypes.UpdateConfig{
		Resources: containertypes.Resources{
			Memory:     314572800,
			MemorySwap: 524288000,
		},
	})
	require.NoError(t, err)

	inspect, err := client.ContainerInspect(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, inspect.HostConfig.Memory, int64(314572800))
	assert.Equal(t, inspect.HostConfig.MemorySwap, int64(524288000))

	body, err := getContainerSysFSValue(ctx, client, cID, "/sys/fs/cgroup/memory/memory.limit_in_bytes")
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(body), "314572800")

	body, err = getContainerSysFSValue(ctx, client, cID, "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes")
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(body), "524288000")
}

func TestUpdateCPUQUota(t *testing.T) {
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
		if err != nil {
			t.Fatal(err)
		}

		if inspect.HostConfig.CPUQuota != test.update {
			t.Fatalf("quota not updated in the API, expected %d, got: %d", test.update, inspect.HostConfig.CPUQuota)
		}

		execCreate, err := client.ContainerExecCreate(ctx, cID, types.ExecConfig{
			Cmd:          []string{"/bin/cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"},
			AttachStdout: true,
			AttachStderr: true,
		})
		if err != nil {
			t.Fatal(err)
		}

		attach, err := client.ContainerExecAttach(ctx, execCreate.ID, types.ExecStartCheck{})
		if err != nil {
			t.Fatal(err)
		}

		if err := client.ContainerExecStart(ctx, execCreate.ID, types.ExecStartCheck{}); err != nil {
			t.Fatal(err)
		}

		buf := bytes.NewBuffer(nil)
		ready := make(chan error)

		go func() {
			_, err := stdcopy.StdCopy(buf, buf, attach.Reader)
			ready <- err
		}()

		select {
		case <-time.After(60 * time.Second):
			t.Fatal("timeout waiting for exec to complete")
		case err := <-ready:
			if err != nil {
				t.Fatal(err)
			}
		}

		actual := strings.TrimSpace(buf.String())
		if actual != strconv.Itoa(int(test.update)) {
			t.Fatalf("expected cgroup value %d, got: %s", test.update, actual)
		}
	}

}

func getContainerSysFSValue(ctx context.Context, client client.APIClient, cID string, path string) (string, error) {
	var b bytes.Buffer

	ex, err := client.ContainerExecCreate(ctx, cID,
		types.ExecConfig{
			AttachStdout: true,
			Cmd:          strslice.StrSlice([]string{"cat", path}),
		},
	)
	if err != nil {
		return "", err
	}

	resp, err := client.ContainerExecAttach(ctx, ex.ID,
		types.ExecStartCheck{
			Detach: false,
			Tty:    false,
		},
	)
	if err != nil {
		return "", err
	}

	defer resp.Close()

	b.Reset()
	_, err = stdcopy.StdCopy(&b, ioutil.Discard, resp.Reader)
	return b.String(), err
}
