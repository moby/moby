package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/request"
	"github.com/docker/docker/pkg/stdcopy"
	"gotest.tools/assert"
	"gotest.tools/skip"
)

// Regression test for #35370
// Makes sure that when following we don't get an EOF error when there are no logs
func TestLogsFollowTailEmpty(t *testing.T) {
	// FIXME(vdemeester) fails on a e2e run on linux...
	skip.If(t, testEnv.IsRemoteDaemon())
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	id := container.Run(t, ctx, client, container.WithCmd("sleep", "100000"))

	logs, err := client.ContainerLogs(ctx, id, types.ContainerLogsOptions{ShowStdout: true, Tail: "2"})
	if logs != nil {
		defer logs.Close()
	}
	assert.Check(t, err)

	_, err = stdcopy.StdCopy(ioutil.Discard, ioutil.Discard, logs)
	assert.Check(t, err)
}

func TestLogsExtraLogInfo(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	id := container.Run(t, ctx, client, container.WithCmd("echo", "hello"), container.WithLogOpt("loginfo", "ContainerID"))

	logs, err := client.ContainerLogs(ctx, id, types.ContainerLogsOptions{ShowStdout: true, Details: true})
	assert.Check(t, err)
	defer logs.Close()

	buf := bytes.NewBuffer(nil)
	ch := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(buf, buf, logs)
		ch <- err
	}()

	select {
	case err := <-ch:
		assert.Check(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for log copy to finish")
	}

	expected := fmt.Sprintf("ContainerID=%s", id)
	actual := strings.Fields(buf.String())[0]
	assert.Equal(t, expected, actual)
}
