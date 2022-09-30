package logging

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
)

// TestReadPluginNoRead tests that reads are supported even if the plugin isn't capable.
func TestReadPluginNoRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no unix domain sockets on Windows")
	}
	t.Parallel()
	d := daemon.New(t)
	d.StartWithBusybox(t, "--iptables=false")
	defer d.Stop(t)

	client, err := d.NewClient()
	assert.Assert(t, err)
	createPlugin(t, client, "test", "discard", asLogDriver)

	ctx := context.Background()

	err = client.PluginEnable(ctx, "test", types.PluginEnableOptions{Timeout: 30})
	assert.Check(t, err)
	d.Stop(t)

	cfg := &container.Config{
		Image: "busybox",
		Cmd:   []string{"/bin/echo", "hello world"},
	}
	for desc, test := range map[string]struct {
		dOpts         []string
		logsSupported bool
	}{
		"default":                    {logsSupported: true},
		"disabled caching":           {[]string{"--log-opt=cache-disabled=true"}, false},
		"explicitly enabled caching": {[]string{"--log-opt=cache-disabled=false"}, true},
	} {
		t.Run(desc, func(t *testing.T) {
			d.Start(t, append([]string{"--iptables=false"}, test.dOpts...)...)
			defer d.Stop(t)
			c, err := client.ContainerCreate(ctx,
				cfg,
				&container.HostConfig{LogConfig: container.LogConfig{Type: "test"}},
				nil,
				nil,
				"",
			)
			assert.Assert(t, err)
			defer client.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})

			err = client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
			assert.Assert(t, err)

			logs, err := client.ContainerLogs(ctx, c.ID, types.ContainerLogsOptions{ShowStdout: true})
			if !test.logsSupported {
				assert.Assert(t, err != nil)
				return
			}
			assert.Assert(t, err)
			defer logs.Close()

			buf := bytes.NewBuffer(nil)

			errCh := make(chan error, 1)
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
		})
	}
}
