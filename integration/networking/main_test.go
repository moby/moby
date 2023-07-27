package networking

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/testutil/environment"
	"gotest.tools/v3/assert"
)

var testEnv *environment.Execution

func TestMain(m *testing.M) {
	var err error
	testEnv, err = environment.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = environment.EnsureFrozenImagesLinux(testEnv)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	testEnv.Print()
	os.Exit(m.Run())
}

func sanitizeCtrName(name string) string {
	r := strings.NewReplacer("/", "-", "=", "-")
	return r.Replace(name)
}

func getContainerLogs(t *testing.T, ctx context.Context, c *client.Client, cid string) string {
	t.Helper()
	r, err := c.ContainerLogs(ctx, cid, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	assert.NilError(t, err)

	buf, err := io.ReadAll(r)
	assert.NilError(t, err)

	return string(buf)
}
