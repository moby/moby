package logging

import (
	"bytes"
	"testing"

	"context"

	"time"

	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
)

// TestReadPluginNoRead tests that reads are supported even if the plugin isn't capable.
func TestReadPluginNoRead(t *testing.T) {
	t.Parallel()
	d := daemon.New(t)
	d.StartWithBusybox(t, "--iptables=false")
	defer d.Stop(t)

	client, err := d.NewClient()
	assert.Assert(t, err)
	createPlugin(t, client, "test", "discard", asLogDriver)

	ctx := context.Background()
	defer func() {
		err = client.PluginRemove(ctx, "test", types.PluginRemoveOptions{Force: true})
		assert.Check(t, err)
	}()

	err = client.PluginEnable(ctx, "test", types.PluginEnableOptions{Timeout: 30})
	assert.Check(t, err)

	c, err := client.ContainerCreate(ctx,
		&container.Config{
			Image: "busybox",
			Cmd:   []string{"/bin/echo", "hello world"},
		},
		&container.HostConfig{LogConfig: container.LogConfig{Type: "test"}},
		nil,
		"",
	)
	assert.Assert(t, err)

	err = client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
	assert.Assert(t, err)

	logs, err := client.ContainerLogs(ctx, c.ID, types.ContainerLogsOptions{ShowStdout: true})
	assert.Assert(t, err)
	defer logs.Close()

	buf := bytes.NewBuffer(nil)

	errCh := make(chan error)
	go func() {
		_, err := stdcopy.StdCopy(buf, buf, logs)
		errCh <- err
	}()

	select {
	case <-time.After(60 * time.Second):
		t.Fatal("timeout waiting for IO to complete")
	case err := <-errCh:
		assert.Assert(t, err)
	}
	assert.Assert(t, strings.TrimSpace(buf.String()) == "hello world", buf.Bytes())
}
