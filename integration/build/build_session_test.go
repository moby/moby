package build

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/builder/buildutil"
	"github.com/docker/docker/internal/test/fakecontext"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
)

func TestBuildWithSession(t *testing.T) {
	skip.If(t, !testEnv.DaemonInfo.ExperimentalBuild)

	client := testEnv.APIClient()

	dockerfile := `
		FROM busybox
		COPY file /
		RUN cat /file
	`

	fctx := fakecontext.New(t, "",
		fakecontext.WithFile("file", "some content"),
	)
	defer fctx.Close()

	res, err := buildutil.Build(client, buildutil.BuildInput{ContextDir: fctx.Dir, Dockerfile: []byte(dockerfile)}, types.ImageBuildOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(res.Output), "some content"))

	fctx.Add("second", "contentcontent")

	dockerfile += `
	COPY second /
	RUN cat /second
	`

	res, err = buildutil.Build(client, buildutil.BuildInput{ContextDir: fctx.Dir, Dockerfile: []byte(dockerfile)}, types.ImageBuildOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(strings.Count(string(res.Output), "Using cache"), 2))
	assert.Check(t, is.Contains(string(res.Output), "contentcontent"))

	du, err := client.DiskUsage(context.TODO())
	assert.Check(t, err)
	assert.Check(t, du.BuilderSize > 10)

	res, err = buildutil.Build(client, buildutil.BuildInput{ContextDir: fctx.Dir, Dockerfile: []byte(dockerfile)}, types.ImageBuildOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(strings.Count(string(res.Output), "Using cache"), 4))

	du2, err := client.DiskUsage(context.TODO())
	assert.Check(t, err)
	assert.Check(t, is.Equal(du.BuilderSize, du2.BuilderSize))

	// rebuild with regular tar, confirm cache still applies
	fctx.Add("Dockerfile", dockerfile)

	res, err = buildutil.Build(
		client,
		buildutil.BuildInput{Context: fctx.AsTarReader(t)},
		types.ImageBuildOptions{},
	)
	assert.NilError(t, err)

	assert.Check(t, is.Contains(string(res.Output), "Successfully built"))
	assert.Check(t, is.Equal(strings.Count(string(res.Output), "Using cache"), 4))

	_, err = client.BuildCachePrune(context.TODO())
	assert.Check(t, err)

	du, err = client.DiskUsage(context.TODO())
	assert.Check(t, err)
	assert.Check(t, is.Equal(du.BuilderSize, int64(0)))
}
