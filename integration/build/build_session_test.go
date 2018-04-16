package build

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	dclient "github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/request"
	"github.com/docker/docker/internal/test/daemon"
	"github.com/docker/docker/internal/test/fakecontext"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"golang.org/x/sync/errgroup"
)

func TestBuildWithSession(t *testing.T) {
	d := daemon.New(t, daemon.WithExperimental)
	d.StartWithBusybox(t)
	defer d.Stop(t)

	client, err := d.NewClient()
	assert.NilError(t, err)

	dockerfile := `
		FROM busybox
		COPY file /
		RUN cat /file
	`

	fctx := fakecontext.New(t, "",
		fakecontext.WithFile("file", "some content"),
	)
	defer fctx.Close()

	out := testBuildWithSession(t, client, d.Sock(), fctx.Dir, dockerfile)
	assert.Check(t, is.Contains(out, "some content"))

	fctx.Add("second", "contentcontent")

	dockerfile += `
	COPY second /
	RUN cat /second
	`

	out = testBuildWithSession(t, client, d.Sock(), fctx.Dir, dockerfile)
	assert.Check(t, is.Equal(strings.Count(out, "Using cache"), 2))
	assert.Check(t, is.Contains(out, "contentcontent"))

	du, err := client.DiskUsage(context.TODO())
	assert.Check(t, err)
	assert.Check(t, du.BuilderSize > 10)

	out = testBuildWithSession(t, client, d.Sock(), fctx.Dir, dockerfile)
	assert.Check(t, is.Equal(strings.Count(out, "Using cache"), 4))

	du2, err := client.DiskUsage(context.TODO())
	assert.Check(t, err)
	assert.Check(t, is.Equal(du.BuilderSize, du2.BuilderSize))

	// rebuild with regular tar, confirm cache still applies
	fctx.Add("Dockerfile", dockerfile)
	// FIXME(vdemeester) use sock here
	res, body, err := request.DoOnHost(d.Sock(),
		"/build",
		request.Method(http.MethodPost),
		request.RawContent(fctx.AsTarReader(t)),
		request.ContentType("application/x-tar"))
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(http.StatusOK, res.StatusCode))

	outBytes, err := request.ReadBody(body)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(outBytes), "Successfully built"))
	assert.Check(t, is.Equal(strings.Count(string(outBytes), "Using cache"), 4))

	_, err = client.BuildCachePrune(context.TODO())
	assert.Check(t, err)

	du, err = client.DiskUsage(context.TODO())
	assert.Check(t, err)
	assert.Check(t, is.Equal(du.BuilderSize, int64(0)))
}

func testBuildWithSession(t *testing.T, client dclient.APIClient, daemonSock string, dir, dockerfile string) (outStr string) {
	sess, err := session.NewSession("foo1", "foo")
	assert.Check(t, err)

	fsProvider := filesync.NewFSSyncProvider([]filesync.SyncedDir{
		{Dir: dir},
	})
	sess.Allow(fsProvider)

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		return sess.Run(ctx, client.DialSession)
	})

	g.Go(func() error {
		// FIXME use sock here
		res, body, err := request.DoOnHost(
			daemonSock,
			"/build?remote=client-session&session="+sess.ID(),
			request.Method(http.MethodPost),
			func(req *http.Request) error {
				req.Body = ioutil.NopCloser(strings.NewReader(dockerfile))
				return nil
			},
		)
		if err != nil {
			return err
		}
		assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusOK))
		out, err := request.ReadBody(body)
		assert.NilError(t, err)
		assert.Check(t, is.Contains(string(out), "Successfully built"))
		sess.Close()
		outStr = string(out)
		return nil
	})

	err = g.Wait()
	assert.Check(t, err)
	return
}
