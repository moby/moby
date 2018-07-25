package build

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/builder/buildutil"
	"github.com/docker/docker/client"
	"github.com/docker/docker/internal/test/fakecontext"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"golang.org/x/sync/errgroup"
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

	out := testBuildWithSession(t, client, fctx.Dir, dockerfile)
	assert.Check(t, is.Contains(out, "some content"))

	fctx.Add("second", "contentcontent")

	dockerfile += `
	COPY second /
	RUN cat /second
	`

	out = testBuildWithSession(t, client, fctx.Dir, dockerfile)
	assert.Check(t, is.Equal(strings.Count(out, "Using cache"), 2))
	assert.Check(t, is.Contains(out, "contentcontent"))

	du, err := client.DiskUsage(context.TODO())
	assert.Check(t, err)
	assert.Check(t, du.BuilderSize > 10)

	out = testBuildWithSession(t, client, fctx.Dir, dockerfile)
	assert.Check(t, is.Equal(strings.Count(out, "Using cache"), 4))

	du2, err := client.DiskUsage(context.TODO())
	assert.Check(t, err)
	assert.Check(t, is.Equal(du.BuilderSize, du2.BuilderSize))

	// rebuild with regular tar, confirm cache still applies
	fctx.Add("Dockerfile", dockerfile)

	res, err := buildutil.Build(
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

func testBuildWithSession(t *testing.T, client client.APIClient, dir, dockerfile string) (outStr string) {
	ctx := context.Background()
	sess, err := session.NewSession(ctx, "foo1", "foo")
	assert.Check(t, err)

	fsProvider := filesync.NewFSSyncProvider([]filesync.SyncedDir{
		{Dir: dir},
	})
	sess.Allow(fsProvider)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return sess.Run(ctx, client.DialSession)
	})

	g.Go(func() error {
		res, err := buildutil.Build(
			client,
			buildutil.BuildInput{Context: strings.NewReader(dockerfile)},
			types.ImageBuildOptions{
				RemoteContext: "client-session",
				SessionID:     sess.ID(),
			},
		)
		assert.NilError(t, err)
		sess.Close()
		outStr = string(res.Output)
		return nil
	})

	err = g.Wait()
	assert.Check(t, err)
	return
}
