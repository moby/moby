package logging

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestContinueAfterPluginCrash(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "test requires daemon on the same host")
	t.Parallel()

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "--iptables=false", "--ip6tables=false", "--init")
	defer d.Stop(t)

	apiclient := d.NewClientT(t)
	createPlugin(ctx, t, apiclient, "test", "close_on_start", asLogDriver)

	ctxT, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	assert.Assert(t, apiclient.PluginEnable(ctxT, "test", client.PluginEnableOptions{Timeout: 30}))
	cancel()
	defer apiclient.PluginRemove(ctx, "test", client.PluginRemoveOptions{Force: true})

	ctxT, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	id := container.Run(ctxT, t, apiclient,
		container.WithAutoRemove,
		container.WithLogDriver("test"),
		container.WithCmd(
			"/bin/sh", "-c", "while true; do sleep 1; echo hello; done",
		),
	)
	cancel()
	defer apiclient.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})

	// Attach to the container to make sure it's written a few times to stdout
	attach, err := apiclient.ContainerAttach(ctx, id, containertypes.AttachOptions{Stream: true, Stdout: true})
	assert.NilError(t, err)

	chErr := make(chan error, 1)
	go func() {
		defer close(chErr)
		rdr := bufio.NewReader(attach.Reader)
		for i := 0; i < 5; i++ {
			_, _, err := rdr.ReadLine()
			if err != nil {
				chErr <- err
				return
			}
		}
	}()

	select {
	case err := <-chErr:
		assert.NilError(t, err)
	case <-time.After(60 * time.Second):
		t.Fatal("timeout waiting for container i/o")
	}

	// check daemon logs for "broken pipe"
	// TODO(@cpuguy83): This is horribly hacky but is the only way to really test this case right now.
	// It would be nice if there was a way to know that a broken pipe has occurred without looking through the logs.
	log, err := os.Open(d.LogFileName())
	assert.NilError(t, err)
	scanner := bufio.NewScanner(log)
	for scanner.Scan() {
		assert.Assert(t, !strings.Contains(scanner.Text(), "broken pipe"))
	}
}
