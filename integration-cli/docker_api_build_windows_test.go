//go:build windows

package main

import (
	"net/http"
	"testing"

	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func (s *DockerAPISuite) TestBuildWithRecycleBin(c *testing.T) {
	testRequires(c, DaemonIsWindows)

	dockerfile := "" +
		"FROM " + testEnv.PlatformDefaults.BaseImage + "\n" +
		"RUN md $REcycLE.biN && md missing\n" +
		"RUN dir $Recycle.Bin && exit 1 || exit 0\n" +
		"RUN dir missing\n"

	buildCtx := fakecontext.New(c, "", fakecontext.WithDockerfile(dockerfile))

	res, body, err := request.Post(testutil.GetContext(c),
		"/build",
		request.RawContent(buildCtx.AsTarReader(c)),
		request.ContentType("application/x-tar"))

	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)

	out, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Check(c, is.Contains(string(out), "Successfully built"))
}
