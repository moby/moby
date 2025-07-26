package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/moby/moby/v2/integration-cli/cli"
	"github.com/moby/moby/v2/integration-cli/cli/build"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/skip"
)

// testPullImageWithAliases pulls a specific image tag and verifies that any aliases (i.e., other
// tags for the same image) are not also pulled down.
//
// Ref: docker/docker#8141
func (s *DockerRegistrySuite) TestPullImageWithAliases(t *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/busybox"

	var repos []string
	for _, tag := range []string{"recent", "fresh"} {
		repos = append(repos, fmt.Sprintf("%v:%v", imgRepo, tag))
	}

	// Tag and push the same image multiple times.
	for _, repo := range repos {
		cli.DockerCmd(t, "tag", "busybox", repo)
		cli.DockerCmd(t, "push", repo)
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	cli.DockerCmd(t, args...)

	// Pull a single tag and verify it doesn't bring down all aliases.
	cli.DockerCmd(t, "pull", repos[0])
	cli.DockerCmd(t, "inspect", repos[0])
	for _, repo := range repos[1:] {
		_, _, err := dockerCmdWithError("inspect", repo)
		assert.ErrorContains(t, err, "", "Image %v shouldn't have been pulled down", repo)
	}
}

// TestConcurrentPullWholeRepo pulls the same repo concurrently.
func (s *DockerRegistrySuite) TestConcurrentPullWholeRepo(t *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/busybox"

	var repos []string
	for _, tag := range []string{"recent", "fresh", "todays"} {
		repo := fmt.Sprintf("%v:%v", imgRepo, tag)
		buildImageSuccessfully(t, repo, build.WithDockerfile(fmt.Sprintf(`
		    FROM busybox
		    ENTRYPOINT ["/bin/echo"]
		    ENV FOO foo
		    ENV BAR bar
		    CMD echo %s
		`, repo)))
		cli.DockerCmd(t, "push", repo)
		repos = append(repos, repo)
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	cli.DockerCmd(t, args...)

	// Run multiple re-pulls concurrently
	numPulls := 3
	results := make(chan error, numPulls)

	for i := 0; i != numPulls; i++ {
		go func() {
			result := icmd.RunCommand(dockerBinary, "pull", "-a", imgRepo)
			results <- result.Error
		}()
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for i := 0; i != numPulls; i++ {
		err := <-results
		assert.NilError(t, err, "concurrent pull failed with error: %v", err)
	}

	// Ensure all tags were pulled successfully
	for _, repo := range repos {
		cli.DockerCmd(t, "inspect", repo)
		out := cli.DockerCmd(t, "run", "--rm", repo).Combined()
		assert.Equal(t, strings.TrimSpace(out), "/bin/sh -c echo "+repo)
	}
}

// TestConcurrentFailingPull tries a concurrent pull that doesn't succeed.
func (s *DockerRegistrySuite) TestConcurrentFailingPull(t *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/busybox"

	// Run multiple pulls concurrently
	numPulls := 3
	results := make(chan error, numPulls)

	for i := 0; i != numPulls; i++ {
		go func() {
			result := icmd.RunCommand(dockerBinary, "pull", imgRepo+":asdfasdf")
			results <- result.Error
		}()
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for i := 0; i != numPulls; i++ {
		err := <-results
		assert.ErrorContains(t, err, "", "expected pull to fail")
	}
}

// TestConcurrentPullMultipleTags pulls multiple tags from the same repo
// concurrently.
func (s *DockerRegistrySuite) TestConcurrentPullMultipleTags(t *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/busybox"

	var repos []string
	for _, tag := range []string{"recent", "fresh", "todays"} {
		repo := fmt.Sprintf("%v:%v", imgRepo, tag)
		buildImageSuccessfully(t, repo, build.WithDockerfile(fmt.Sprintf(`
		    FROM busybox
		    ENTRYPOINT ["/bin/echo"]
		    ENV FOO foo
		    ENV BAR bar
		    CMD echo %s
		`, repo)))
		cli.DockerCmd(t, "push", repo)
		repos = append(repos, repo)
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	cli.DockerCmd(t, args...)

	// Re-pull individual tags, in parallel
	results := make(chan error, len(repos))

	for _, repo := range repos {
		go func(repo string) {
			result := icmd.RunCommand(dockerBinary, "pull", repo)
			results <- result.Error
		}(repo)
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for range repos {
		err := <-results
		assert.NilError(t, err, "concurrent pull failed with error: %v", err)
	}

	// Ensure all tags were pulled successfully
	for _, repo := range repos {
		cli.DockerCmd(t, "inspect", repo)
		out := cli.DockerCmd(t, "run", "--rm", repo).Combined()
		assert.Equal(t, strings.TrimSpace(out), "/bin/sh -c echo "+repo)
	}
}

// TestPullIDStability verifies that pushing an image and pulling it back
// preserves the image ID.
func (s *DockerRegistrySuite) TestPullIDStability(t *testing.T) {
	const derivedImage = privateRegistryURL + "/dockercli/id-stability"
	const baseImage = "busybox"

	buildImageSuccessfully(t, derivedImage, build.WithDockerfile(fmt.Sprintf(`
	    FROM %s
	    ENV derived true
	    ENV asdf true
	    RUN dd if=/dev/zero of=/file bs=1024 count=1024
	    CMD echo %s
	`, baseImage, derivedImage)))

	originalID := getIDByName(t, derivedImage)
	cli.DockerCmd(t, "push", derivedImage)

	// Pull
	out := cli.DockerCmd(t, "pull", derivedImage).Combined()
	if strings.Contains(out, "Pull complete") {
		t.Fatalf("repull redownloaded a layer: %s", out)
	}

	derivedIDAfterPull := getIDByName(t, derivedImage)

	if derivedIDAfterPull != originalID {
		t.Fatal("image's ID unexpectedly changed after a repush/repull")
	}

	// Make sure the image runs correctly
	out = cli.DockerCmd(t, "run", "--rm", derivedImage).Combined()
	if strings.TrimSpace(out) != derivedImage {
		t.Fatalf("expected %s; got %s", derivedImage, out)
	}

	// Confirm that repushing and repulling does not change the computed ID
	cli.DockerCmd(t, "push", derivedImage)
	cli.DockerCmd(t, "rmi", derivedImage)
	cli.DockerCmd(t, "pull", derivedImage)

	derivedIDAfterPull = getIDByName(t, derivedImage)
	if derivedIDAfterPull != originalID {
		t.Fatal("image's ID unexpectedly changed after a repush/repull")
	}

	// Make sure the image still runs
	out = cli.DockerCmd(t, "run", "--rm", derivedImage).Combined()
	if strings.TrimSpace(out) != derivedImage {
		t.Fatalf("expected %s; got %s", derivedImage, out)
	}
}

// #21213
func (s *DockerRegistrySuite) TestPullNoLayers(t *testing.T) {
	const imgRepo = privateRegistryURL + "/dockercli/scratch"

	buildImageSuccessfully(t, imgRepo, build.WithDockerfile(`
	FROM scratch
	ENV foo bar`))
	cli.DockerCmd(t, "push", imgRepo)
	cli.DockerCmd(t, "rmi", imgRepo)
	cli.DockerCmd(t, "pull", imgRepo)
}

func (s *DockerRegistrySuite) TestPullManifestList(c *testing.T) {
	skip.If(c, testEnv.UsingSnapshotter(), "containerd knows how to pull manifest lists")
	pushDigest, err := setupImage(c)
	assert.NilError(c, err, "error setting up image")

	// Inject a manifest list into the registry
	manifestList := &ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{
			{
				Digest:    "sha256:1a9ec845ee94c202b2d5da74a24f0ed2058318bfa9879fa541efaecba272e86b",
				Size:      3253,
				MediaType: ocispec.MediaTypeImageManifest,
				Platform: &ocispec.Platform{
					Architecture: "bogus_arch",
					OS:           "bogus_os",
				},
			},
			{
				Digest:    pushDigest,
				Size:      3253,
				MediaType: ocispec.MediaTypeImageManifest,
				Platform: &ocispec.Platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
				},
			},
		},
	}

	manifestListJSON, err := json.MarshalIndent(manifestList, "", "   ")
	assert.NilError(c, err, "error marshalling manifest list")

	manifestListDigest := digest.FromBytes(manifestListJSON)
	hexDigest := manifestListDigest.Encoded()

	registryV2Path := s.reg.Path()

	// Write manifest list to blob store
	blobDir := filepath.Join(registryV2Path, "blobs", "sha256", hexDigest[:2], hexDigest)
	err = os.MkdirAll(blobDir, 0o755)
	assert.NilError(c, err, "error creating blob dir")
	blobPath := filepath.Join(blobDir, "data")
	err = os.WriteFile(blobPath, manifestListJSON, 0o644)
	assert.NilError(c, err, "error writing manifest list")

	// Add to revision store
	revisionDir := filepath.Join(registryV2Path, "repositories", remoteRepoName, "_manifests", "revisions", "sha256", hexDigest)
	err = os.Mkdir(revisionDir, 0o755)
	assert.Assert(c, err == nil, "error creating revision dir")
	revisionPath := filepath.Join(revisionDir, "link")
	err = os.WriteFile(revisionPath, []byte(manifestListDigest.String()), 0o644)
	assert.Assert(c, err == nil, "error writing revision link")

	// Update tag
	tagPath := filepath.Join(registryV2Path, "repositories", remoteRepoName, "_manifests", "tags", "latest", "current", "link")
	err = os.WriteFile(tagPath, []byte(manifestListDigest.String()), 0o644)
	assert.NilError(c, err, "error writing tag link")

	// Verify that the image can be pulled through the manifest list.
	out := cli.DockerCmd(c, "pull", repoName).Combined()

	// The pull output includes "Digest: <digest>", so find that
	matches := digestRegex.FindStringSubmatch(out)
	assert.Equal(c, len(matches), 2, fmt.Sprintf("unable to parse digest from pull output: %s", out))
	pullDigest := matches[1]

	// Make sure the pushed and pull digests match
	assert.Equal(c, manifestListDigest.String(), pullDigest)

	// Was the image actually created?
	cli.DockerCmd(c, "inspect", repoName)

	cli.DockerCmd(c, "rmi", repoName)
}

// #23100
func (s *DockerRegistryAuthHtpasswdSuite) TestPullWithExternalAuthLoginWithScheme(c *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(c, err)
	absolute, err := filepath.Abs(filepath.Join(workingDir, "fixtures", "auth"))
	assert.NilError(c, err)

	osPath := os.Getenv("PATH")
	testPath := fmt.Sprintf("%s%c%s", osPath, filepath.ListSeparator, absolute)
	c.Setenv("PATH", testPath)

	const imgRepo = privateRegistryURL + "/dockercli/busybox:authtest"

	tmp, err := os.MkdirTemp("", "integration-cli-")
	assert.NilError(c, err)

	externalAuthConfig := `{ "credsStore": "shell-test" }`

	configPath := filepath.Join(tmp, "config.json")
	err = os.WriteFile(configPath, []byte(externalAuthConfig), 0o644)
	assert.NilError(c, err)

	cli.DockerCmd(c, "--config", tmp, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), privateRegistryURL)

	b, err := os.ReadFile(configPath)
	assert.NilError(c, err)
	assert.Assert(c, !strings.Contains(string(b), `"auth":`))
	cli.DockerCmd(c, "--config", tmp, "tag", "busybox", imgRepo)
	cli.DockerCmd(c, "--config", tmp, "push", imgRepo)

	cli.DockerCmd(c, "--config", tmp, "logout", privateRegistryURL)
	cli.DockerCmd(c, "--config", tmp, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), "https://"+privateRegistryURL)
	cli.DockerCmd(c, "--config", tmp, "pull", imgRepo)

	// likewise push should work
	repoName2 := fmt.Sprintf("%v/dockercli/busybox:nocreds", privateRegistryURL)
	cli.DockerCmd(c, "tag", imgRepo, repoName2)
	cli.DockerCmd(c, "--config", tmp, "push", repoName2)

	// logout should work w scheme also because it will be stripped
	cli.DockerCmd(c, "--config", tmp, "logout", "https://"+privateRegistryURL)
}

func (s *DockerRegistryAuthHtpasswdSuite) TestPullWithExternalAuth(c *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(c, err)
	absolute, err := filepath.Abs(filepath.Join(workingDir, "fixtures", "auth"))
	assert.NilError(c, err)

	osPath := os.Getenv("PATH")
	testPath := fmt.Sprintf("%s%c%s", osPath, filepath.ListSeparator, absolute)
	c.Setenv("PATH", testPath)

	const imgRepo = privateRegistryURL + "/dockercli/busybox:authtest"

	tmp, err := os.MkdirTemp("", "integration-cli-")
	assert.NilError(c, err)

	externalAuthConfig := `{ "credsStore": "shell-test" }`

	configPath := filepath.Join(tmp, "config.json")
	err = os.WriteFile(configPath, []byte(externalAuthConfig), 0o644)
	assert.NilError(c, err)

	cli.DockerCmd(c, "--config", tmp, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), privateRegistryURL)

	b, err := os.ReadFile(configPath)
	assert.NilError(c, err)
	assert.Assert(c, !strings.Contains(string(b), `"auth":`))
	cli.DockerCmd(c, "--config", tmp, "tag", "busybox", imgRepo)
	cli.DockerCmd(c, "--config", tmp, "push", imgRepo)

	cli.DockerCmd(c, "--config", tmp, "pull", imgRepo)
}

// TestRunImplicitPullWithNoTag should pull implicitly only the default tag (latest)
func (s *DockerRegistrySuite) TestRunImplicitPullWithNoTag(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	const imgRepo = privateRegistryURL + "/dockercli/busybox"
	const repoTag1 = imgRepo + ":latest"
	const repoTag2 = imgRepo + ":t1"
	// tag the image and upload it to the private registry
	cli.DockerCmd(c, "tag", "busybox", repoTag1)
	cli.DockerCmd(c, "tag", "busybox", repoTag2)
	cli.DockerCmd(c, "push", imgRepo)
	cli.DockerCmd(c, "rmi", repoTag1)
	cli.DockerCmd(c, "rmi", repoTag2)

	out := cli.DockerCmd(c, "run", imgRepo).Combined()
	assert.Assert(c, is.Contains(out, fmt.Sprintf("Unable to find image '%s:latest' locally", imgRepo)))
	// There should be only one line for repo, the one with repo:latest
	outImageCmd := cli.DockerCmd(c, "images", imgRepo).Stdout()
	splitOutImageCmd := strings.Split(strings.TrimSpace(outImageCmd), "\n")
	assert.Equal(c, len(splitOutImageCmd), 2)
}
