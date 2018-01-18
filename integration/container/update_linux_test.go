package container

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/util/request"
	"github.com/docker/docker/pkg/stdcopy"
)

func TestUpdateCPUQUota(t *testing.T) {
	t.Parallel()

	client := request.NewAPIClient(t)
	ctx := context.Background()

	c, err := client.ContainerCreate(ctx, &container.Config{
		Image: "busybox",
		Cmd:   []string{"top"},
	}, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
			panic(fmt.Sprintf("failed to clean up after test: %v", err))
		}
	}()

	if err := client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		desc   string
		update int64
	}{
		{desc: "some random value", update: 15000},
		{desc: "a higher value", update: 20000},
		{desc: "a lower value", update: 10000},
		{desc: "unset value", update: -1},
	} {
		if _, err := client.ContainerUpdate(ctx, c.ID, container.UpdateConfig{
			Resources: container.Resources{
				CPUQuota: test.update,
			},
		}); err != nil {
			t.Fatal(err)
		}

		inspect, err := client.ContainerInspect(ctx, c.ID)
		if err != nil {
			t.Fatal(err)
		}

		if inspect.HostConfig.CPUQuota != test.update {
			t.Fatalf("quota not updated in the API, expected %d, got: %d", test.update, inspect.HostConfig.CPUQuota)
		}

		execCreate, err := client.ContainerExecCreate(ctx, c.ID, types.ExecConfig{
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
