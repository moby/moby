package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/skip"
)

func TestCopyFromContainerPathDoesNotExist(t *testing.T) {
	defer setupTest(t)()

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(t, ctx, apiclient)

	_, _, err := apiclient.CopyFromContainer(ctx, cid, "/dne")
	assert.Check(t, client.IsErrNotFound(err))
	expected := fmt.Sprintf("No such container:path: %s:%s", cid, "/dne")
	assert.Check(t, is.ErrorContains(err, expected))
}

func TestCopyFromContainerPathIsNotDir(t *testing.T) {
	defer setupTest(t)()
	skip.If(t, testEnv.OSType == "windows")

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(t, ctx, apiclient)

	_, _, err := apiclient.CopyFromContainer(ctx, cid, "/etc/passwd/")
	assert.Assert(t, is.ErrorContains(err, "not a directory"))
}

func TestCopyToContainerPathDoesNotExist(t *testing.T) {
	defer setupTest(t)()
	skip.If(t, testEnv.OSType == "windows")

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(t, ctx, apiclient)

	err := apiclient.CopyToContainer(ctx, cid, "/dne", nil, types.CopyToContainerOptions{})
	assert.Check(t, client.IsErrNotFound(err))
	expected := fmt.Sprintf("No such container:path: %s:%s", cid, "/dne")
	assert.Check(t, is.ErrorContains(err, expected))
}

func TestCopyToContainerPathIsNotDir(t *testing.T) {
	defer setupTest(t)()
	skip.If(t, testEnv.OSType == "windows")

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(t, ctx, apiclient)

	err := apiclient.CopyToContainer(ctx, cid, "/etc/passwd/", nil, types.CopyToContainerOptions{})
	assert.Assert(t, is.ErrorContains(err, "not a directory"))
}
