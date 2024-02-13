package main

import (
	"archive/tar"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

type DockerCLIPushSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIPushSuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerCLIPushSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerRegistrySuite) TestPushBusyboxImage(c *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/busybox"
	// tag the image to upload it to the private registry
	cli.DockerCmd(c, "tag", "busybox", imgRepo)
	// push the image to the registry
	cli.DockerCmd(c, "push", imgRepo)
}

// pushing an image without a prefix should throw an error
func (s *DockerCLIPushSuite) TestPushUnprefixedRepo(c *testing.T) {
	out, _, err := dockerCmdWithError("push", "busybox")
	assert.ErrorContains(c, err, "", "pushing an unprefixed repo didn't result in a non-zero exit status: %s", out)
}

func (s *DockerRegistrySuite) TestPushUntagged(c *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/busybox"

	out, _, err := dockerCmdWithError("push", imgRepo)
	assert.ErrorContains(c, err, "", "pushing the image to the private registry should have failed: output %q", out)
	const expected = "An image does not exist locally with the tag"
	assert.Assert(c, strings.Contains(out, expected), "pushing the image failed")
}

func (s *DockerRegistrySuite) TestPushBadTag(c *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/busybox:latest"

	out, _, err := dockerCmdWithError("push", imgRepo)
	assert.ErrorContains(c, err, "", "pushing the image to the private registry should have failed: output %q", out)
	const expected = "does not exist"
	assert.Assert(c, strings.Contains(out, expected), "pushing the image failed")
}

func (s *DockerRegistrySuite) TestPushMultipleTags(c *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/busybox"
	const repoTag1 = imgRepo + ":t1"
	const repoTag2 = imgRepo + ":t2"
	// tag the image and upload it to the private registry
	cli.DockerCmd(c, "tag", "busybox", repoTag1)
	cli.DockerCmd(c, "tag", "busybox", repoTag2)

	args := []string{"push"}
	if versions.GreaterThanOrEqualTo(DockerCLIVersion(c), "20.10.0") {
		// 20.10 CLI removed implicit push all tags and requires the "--all" flag
		args = append(args, "--all-tags")
	}
	args = append(args, imgRepo)

	cli.DockerCmd(c, args...)

	imageAlreadyExists := ": Image already exists"

	// Ensure layer list is equivalent for repoTag1 and repoTag2
	out1 := cli.DockerCmd(c, "push", repoTag1).Combined()
	var out1Lines []string
	for _, outputLine := range strings.Split(out1, "\n") {
		if strings.Contains(outputLine, imageAlreadyExists) {
			out1Lines = append(out1Lines, outputLine)
		}
	}

	out2 := cli.DockerCmd(c, "push", repoTag2).Combined()
	var out2Lines []string
	for _, outputLine := range strings.Split(out2, "\n") {
		if strings.Contains(outputLine, imageAlreadyExists) {
			out2Lines = append(out2Lines, outputLine)
		}
	}
	assert.DeepEqual(c, out1Lines, out2Lines)
}

func (s *DockerRegistrySuite) TestPushEmptyLayer(c *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/emptylayer"

	emptyTarball, err := os.CreateTemp("", "empty_tarball")
	assert.NilError(c, err, "Unable to create test file")

	tw := tar.NewWriter(emptyTarball)
	err = tw.Close()
	assert.NilError(c, err, "Error creating empty tarball")

	freader, err := os.Open(emptyTarball.Name())
	assert.NilError(c, err, "Could not open test tarball")
	defer freader.Close()

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "import", "-", imgRepo},
		Stdin:   freader,
	}).Assert(c, icmd.Success)

	// Now verify we can push it
	out, _, err := dockerCmdWithError("push", imgRepo)
	assert.NilError(c, err, "pushing the image to the private registry has failed: %s", out)
}

// TestConcurrentPush pushes multiple tags to the same repo
// concurrently.
func (s *DockerRegistrySuite) TestConcurrentPush(c *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/busybox"

	var repos []string
	for _, tag := range []string{"push1", "push2", "push3"} {
		repo := fmt.Sprintf("%v:%v", imgRepo, tag)
		buildImageSuccessfully(c, repo, build.WithDockerfile(fmt.Sprintf(`
	FROM busybox
	ENTRYPOINT ["/bin/echo"]
	ENV FOO foo
	ENV BAR bar
	CMD echo %s
`, repo)))
		repos = append(repos, repo)
	}

	// Push tags, in parallel
	results := make(chan error, len(repos))

	for _, repo := range repos {
		go func(repo string) {
			result := icmd.RunCommand(dockerBinary, "push", repo)
			results <- result.Error
		}(repo)
	}

	for range repos {
		err := <-results
		assert.NilError(c, err, "concurrent push failed with error: %v", err)
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	cli.DockerCmd(c, args...)

	// Re-pull and run individual tags, to make sure pushes succeeded
	for _, repo := range repos {
		cli.DockerCmd(c, "pull", repo)
		cli.DockerCmd(c, "inspect", repo)
		out := cli.DockerCmd(c, "run", "--rm", repo).Combined()
		assert.Equal(c, strings.TrimSpace(out), "/bin/sh -c echo "+repo)
	}
}

func (s *DockerRegistrySuite) TestCrossRepositoryLayerPush(c *testing.T) {
	const sourceRepoName = privateRegistryURL + "/crossrepopush/busybox"

	// tag the image to upload it to the private registry
	cli.DockerCmd(c, "tag", "busybox", sourceRepoName)
	// push the image to the registry
	out1, _, err := dockerCmdWithError("push", sourceRepoName)
	assert.NilError(c, err, "pushing the image to the private registry has failed: %s", out1)
	// ensure that none of the layers were mounted from another repository during push
	assert.Assert(c, !strings.Contains(out1, "Mounted from"))

	digest1 := reference.DigestRegexp.FindString(out1)
	assert.Assert(c, len(digest1) > 0, "no digest found for pushed manifest")

	const destRepoName = privateRegistryURL + "/crossrepopush/img"

	// retag the image to upload the same layers to another repo in the same registry
	cli.DockerCmd(c, "tag", "busybox", destRepoName)
	// push the image to the registry
	out2, _, err := dockerCmdWithError("push", destRepoName)
	assert.NilError(c, err, "pushing the image to the private registry has failed: %s", out2)

	// ensure that layers were mounted from the first repo during push
	assert.Assert(c, strings.Contains(out2, "Mounted from crossrepopush/busybox"))

	digest2 := reference.DigestRegexp.FindString(out2)
	assert.Assert(c, len(digest2) > 0, "no digest found for pushed manifest")
	assert.Equal(c, digest1, digest2)

	// ensure that pushing again produces the same digest
	out3, _, err := dockerCmdWithError("push", destRepoName)
	assert.NilError(c, err, "pushing the image to the private registry has failed: %s", out3)

	digest3 := reference.DigestRegexp.FindString(out3)
	assert.Assert(c, len(digest3) > 0, "no digest found for pushed manifest")
	assert.Equal(c, digest3, digest2)

	// ensure that we can pull and run the cross-repo-pushed repository
	cli.DockerCmd(c, "rmi", destRepoName)
	cli.DockerCmd(c, "pull", destRepoName)
	out4 := cli.DockerCmd(c, "run", destRepoName, "echo", "-n", "hello world").Combined()
	assert.Equal(c, out4, "hello world")
}

func (s *DockerRegistryAuthHtpasswdSuite) TestPushNoCredentialsNoRetry(c *testing.T) {
	const imgRepo = privateRegistryURL + "/busybox"
	cli.DockerCmd(c, "tag", "busybox", imgRepo)
	out, _, err := dockerCmdWithError("push", imgRepo)
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, !strings.Contains(out, "Retrying"))
	assert.Assert(c, strings.Contains(out, "no basic auth credentials"))
}

// This may be flaky but it's needed not to regress on unauthorized push, see #21054
func (s *DockerCLIPushSuite) TestPushToCentralRegistryUnauthorized(c *testing.T) {
	testRequires(c, Network)

	const imgRepo = "test/busybox"
	cli.DockerCmd(c, "tag", "busybox", imgRepo)
	out, _, err := dockerCmdWithError("push", imgRepo)
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, !strings.Contains(out, "Retrying"))
}

func getTestTokenService(status int, body string, retries int) *httptest.Server {
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if retries > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"errors":[{"code":"UNAVAILABLE","message":"cannot create token at this time"}]}`))
			retries--
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			w.Write([]byte(body))
		}
		mu.Unlock()
	}))
}

func (s *DockerRegistryAuthTokenSuite) TestPushTokenServiceUnauthResponse(c *testing.T) {
	ts := getTestTokenService(http.StatusUnauthorized, `{"errors": [{"Code":"UNAUTHORIZED", "message": "a message", "detail": null}]}`, 0)
	defer ts.Close()
	s.setupRegistryWithTokenService(c, ts.URL)

	const imgRepo = privateRegistryURL + "/busybox"
	cli.DockerCmd(c, "tag", "busybox", imgRepo)
	out, _, err := dockerCmdWithError("push", imgRepo)
	assert.ErrorContains(c, err, "", out)

	assert.Check(c, !strings.Contains(out, "Retrying"))

	// Auth service errors are not part of the spec and containerd doesn't parse them.
	if testEnv.UsingSnapshotter() {
		assert.Check(c, is.Contains(out, "failed to authorize: failed to fetch anonymous token"))
		assert.Check(c, is.Contains(out, "401 Unauthorized"))
	} else {
		assert.Check(c, is.Contains(out, "unauthorized: a message"))
	}
}

func (s *DockerRegistryAuthTokenSuite) TestPushMisconfiguredTokenServiceResponseUnauthorized(c *testing.T) {
	ts := getTestTokenService(http.StatusUnauthorized, `{"error": "unauthorized"}`, 0)
	defer ts.Close()
	s.setupRegistryWithTokenService(c, ts.URL)

	const imgRepo = privateRegistryURL + "/busybox"
	cli.DockerCmd(c, "tag", "busybox", imgRepo)
	out, _, err := dockerCmdWithError("push", imgRepo)
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, !strings.Contains(out, "Retrying"))

	// Auth service errors are not part of the spec and containerd doesn't parse them.
	if testEnv.UsingSnapshotter() {
		assert.Check(c, is.Contains(out, "failed to authorize: failed to fetch anonymous token"))
		assert.Check(c, is.Contains(out, "401 Unauthorized"))
	} else {
		split := strings.Split(out, "\n")
		assert.Check(c, is.Contains(split[len(split)-2], "unauthorized: authentication required"))
	}
}

func (s *DockerRegistryAuthTokenSuite) TestPushMisconfiguredTokenServiceResponseError(c *testing.T) {
	ts := getTestTokenService(http.StatusTooManyRequests, `{"errors": [{"code":"TOOMANYREQUESTS","message":"out of tokens"}]}`, 3)
	defer ts.Close()
	s.setupRegistryWithTokenService(c, ts.URL)

	const imgRepo = privateRegistryURL + "/busybox"
	cli.DockerCmd(c, "tag", "busybox", imgRepo)
	out, _, err := dockerCmdWithError("push", imgRepo)
	assert.ErrorContains(c, err, "", out)
	// TODO: isolate test so that it can be guaranteed that the 503 will trigger xfer retries
	// assert.Assert(c, strings.Contains(out, "Retrying"))
	// assert.Assert(c, !strings.Contains(out, "Retrying in 15"))

	// Auth service errors are not part of the spec and containerd doesn't parse them.
	if testEnv.UsingSnapshotter() {
		assert.Check(c, is.Contains(out, "failed to authorize: failed to fetch anonymous token"))
		assert.Check(c, is.Contains(out, "503 Service Unavailable"))
	} else {
		split := strings.Split(out, "\n")
		assert.Check(c, is.Equal(split[len(split)-2], "toomanyrequests: out of tokens"))
	}
}

func (s *DockerRegistryAuthTokenSuite) TestPushMisconfiguredTokenServiceResponseUnparsable(c *testing.T) {
	ts := getTestTokenService(http.StatusForbidden, `no way`, 0)
	defer ts.Close()
	s.setupRegistryWithTokenService(c, ts.URL)

	const imgRepo = privateRegistryURL + "/busybox"
	cli.DockerCmd(c, "tag", "busybox", imgRepo)
	out, _, err := dockerCmdWithError("push", imgRepo)
	assert.ErrorContains(c, err, "", out)
	assert.Check(c, !strings.Contains(out, "Retrying"))

	// Auth service errors are not part of the spec and containerd doesn't parse them.
	if testEnv.UsingSnapshotter() {
		assert.Check(c, is.Contains(out, "failed to authorize: failed to fetch anonymous token"))
		assert.Check(c, is.Contains(out, "403 Forbidden"))
	} else {
		split := strings.Split(out, "\n")
		assert.Check(c, is.Contains(split[len(split)-2], "error parsing HTTP 403 response body: "))
	}
}

func (s *DockerRegistryAuthTokenSuite) TestPushMisconfiguredTokenServiceResponseNoToken(c *testing.T) {
	ts := getTestTokenService(http.StatusOK, `{"something": "wrong"}`, 0)
	defer ts.Close()
	s.setupRegistryWithTokenService(c, ts.URL)

	const imgRepo = privateRegistryURL + "/busybox"
	cli.DockerCmd(c, "tag", "busybox", imgRepo)
	out, _, err := dockerCmdWithError("push", imgRepo)
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, !strings.Contains(out, "Retrying"))
	split := strings.Split(out, "\n")
	assert.Check(c, is.Contains(split[len(split)-2], "authorization server did not include a token in the response"))
}
