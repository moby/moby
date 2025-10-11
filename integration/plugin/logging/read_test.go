package logging

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/stdcopy"
	testContainer "github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

// TestReadPluginNoRead tests that reads are supported even if the plugin isn't capable.
func TestReadPluginNoRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no unix domain sockets on Windows")
	}
	t.Parallel()

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)

	apiclient, err := d.NewClient()
	assert.Assert(t, err)
	createPlugin(ctx, t, apiclient, "test", "discard", asLogDriver)

	err = apiclient.PluginEnable(ctx, "test", client.PluginEnableOptions{Timeout: 30})
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
			ctx := testutil.StartSpan(ctx, t)
			d.Start(t, append([]string{"--iptables=false", "--ip6tables=false"}, test.dOpts...)...)
			defer d.Stop(t)
			c, err := apiclient.ContainerCreate(ctx,
				cfg,
				&container.HostConfig{LogConfig: container.LogConfig{Type: "test"}},
				nil,
				nil,
				"",
			)
			assert.Assert(t, err)
			defer apiclient.ContainerRemove(ctx, c.ID, client.ContainerRemoveOptions{Force: true})

			err = apiclient.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
			assert.Assert(t, err)

			poll.WaitOn(t, testContainer.IsStopped(ctx, apiclient, c.ID))
			logs, err := apiclient.ContainerLogs(ctx, c.ID, client.ContainerLogsOptions{ShowStdout: true})
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
