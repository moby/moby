package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

// testPullImageWithAliases pulls a specific image tag and verifies that any aliases (i.e., other
// tags for the same image) are not also pulled down.
//
// Ref: docker/docker#8141
func testPullImageWithAliases(c *testing.T) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	var repos []string
	for _, tag := range []string{"recent", "fresh"} {
		repos = append(repos, fmt.Sprintf("%v:%v", repoName, tag))
	}

	// Tag and push the same image multiple times.
	for _, repo := range repos {
		dockerCmd(c, "tag", "busybox", repo)
		dockerCmd(c, "push", repo)
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	dockerCmd(c, args...)

	// Pull a single tag and verify it doesn't bring down all aliases.
	dockerCmd(c, "pull", repos[0])
	dockerCmd(c, "inspect", repos[0])
	for _, repo := range repos[1:] {
		_, _, err := dockerCmdWithError("inspect", repo)
		assert.ErrorContains(c, err, "", "Image %v shouldn't have been pulled down", repo)
	}
}

func (s *DockerRegistrySuite) TestPullImageWithAliases(c *testing.T) {
	testPullImageWithAliases(c)
}

func (s *DockerSchema1RegistrySuite) TestPullImageWithAliases(c *testing.T) {
	testPullImageWithAliases(c)
}

// testConcurrentPullWholeRepo pulls the same repo concurrently.
func testConcurrentPullWholeRepo(c *testing.T) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	var repos []string
	for _, tag := range []string{"recent", "fresh", "todays"} {
		repo := fmt.Sprintf("%v:%v", repoName, tag)
		buildImageSuccessfully(c, repo, build.WithDockerfile(fmt.Sprintf(`
		    FROM busybox
		    ENTRYPOINT ["/bin/echo"]
		    ENV FOO foo
		    ENV BAR bar
		    CMD echo %s
		`, repo)))
		dockerCmd(c, "push", repo)
		repos = append(repos, repo)
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	dockerCmd(c, args...)

	// Run multiple re-pulls concurrently
	numPulls := 3
	results := make(chan error, numPulls)

	for i := 0; i != numPulls; i++ {
		go func() {
			result := icmd.RunCommand(dockerBinary, "pull", "-a", repoName)
			results <- result.Error
		}()
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for i := 0; i != numPulls; i++ {
		err := <-results
		assert.NilError(c, err, "concurrent pull failed with error: %v", err)
	}

	// Ensure all tags were pulled successfully
	for _, repo := range repos {
		dockerCmd(c, "inspect", repo)
		out, _ := dockerCmd(c, "run", "--rm", repo)
		assert.Equal(c, strings.TrimSpace(out), "/bin/sh -c echo "+repo)
	}
}

func (s *DockerRegistrySuite) TestConcurrentPullWholeRepo(c *testing.T) {
	testConcurrentPullWholeRepo(c)
}

func (s *DockerSchema1RegistrySuite) TestConcurrentPullWholeRepo(c *testing.T) {
	testConcurrentPullWholeRepo(c)
}

// testConcurrentFailingPull tries a concurrent pull that doesn't succeed.
func testConcurrentFailingPull(c *testing.T) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	// Run multiple pulls concurrently
	numPulls := 3
	results := make(chan error, numPulls)

	for i := 0; i != numPulls; i++ {
		go func() {
			result := icmd.RunCommand(dockerBinary, "pull", repoName+":asdfasdf")
			results <- result.Error
		}()
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for i := 0; i != numPulls; i++ {
		err := <-results
		assert.ErrorContains(c, err, "", "expected pull to fail")
	}
}

func (s *DockerRegistrySuite) TestConcurrentFailingPull(c *testing.T) {
	testConcurrentFailingPull(c)
}

func (s *DockerSchema1RegistrySuite) TestConcurrentFailingPull(c *testing.T) {
	testConcurrentFailingPull(c)
}

// testConcurrentPullMultipleTags pulls multiple tags from the same repo
// concurrently.
func testConcurrentPullMultipleTags(c *testing.T) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	var repos []string
	for _, tag := range []string{"recent", "fresh", "todays"} {
		repo := fmt.Sprintf("%v:%v", repoName, tag)
		buildImageSuccessfully(c, repo, build.WithDockerfile(fmt.Sprintf(`
		    FROM busybox
		    ENTRYPOINT ["/bin/echo"]
		    ENV FOO foo
		    ENV BAR bar
		    CMD echo %s
		`, repo)))
		dockerCmd(c, "push", repo)
		repos = append(repos, repo)
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	dockerCmd(c, args...)

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
		assert.NilError(c, err, "concurrent pull failed with error: %v", err)
	}

	// Ensure all tags were pulled successfully
	for _, repo := range repos {
		dockerCmd(c, "inspect", repo)
		out, _ := dockerCmd(c, "run", "--rm", repo)
		assert.Equal(c, strings.TrimSpace(out), "/bin/sh -c echo "+repo)
	}
}

func (s *DockerRegistrySuite) TestConcurrentPullMultipleTags(c *testing.T) {
	testConcurrentPullMultipleTags(c)
}

func (s *DockerSchema1RegistrySuite) TestConcurrentPullMultipleTags(c *testing.T) {
	testConcurrentPullMultipleTags(c)
}

// testPullIDStability verifies that pushing an image and pulling it back
// preserves the image ID.
func testPullIDStability(c *testing.T) {
	derivedImage := privateRegistryURL + "/dockercli/id-stability"
	baseImage := "busybox"

	buildImageSuccessfully(c, derivedImage, build.WithDockerfile(fmt.Sprintf(`
	    FROM %s
	    ENV derived true
	    ENV asdf true
	    RUN dd if=/dev/zero of=/file bs=1024 count=1024
	    CMD echo %s
	`, baseImage, derivedImage)))

	originalID := getIDByName(c, derivedImage)
	dockerCmd(c, "push", derivedImage)

	// Pull
	out, _ := dockerCmd(c, "pull", derivedImage)
	if strings.Contains(out, "Pull complete") {
		c.Fatalf("repull redownloaded a layer: %s", out)
	}

	derivedIDAfterPull := getIDByName(c, derivedImage)

	if derivedIDAfterPull != originalID {
		c.Fatal("image's ID unexpectedly changed after a repush/repull")
	}

	// Make sure the image runs correctly
	out, _ = dockerCmd(c, "run", "--rm", derivedImage)
	if strings.TrimSpace(out) != derivedImage {
		c.Fatalf("expected %s; got %s", derivedImage, out)
	}

	// Confirm that repushing and repulling does not change the computed ID
	dockerCmd(c, "push", derivedImage)
	dockerCmd(c, "rmi", derivedImage)
	dockerCmd(c, "pull", derivedImage)

	derivedIDAfterPull = getIDByName(c, derivedImage)

	if derivedIDAfterPull != originalID {
		c.Fatal("image's ID unexpectedly changed after a repush/repull")
	}

	// Make sure the image still runs
	out, _ = dockerCmd(c, "run", "--rm", derivedImage)
	if strings.TrimSpace(out) != derivedImage {
		c.Fatalf("expected %s; got %s", derivedImage, out)
	}
}

func (s *DockerRegistrySuite) TestPullIDStability(c *testing.T) {
	testPullIDStability(c)
}

func (s *DockerSchema1RegistrySuite) TestPullIDStability(c *testing.T) {
	testPullIDStability(c)
}

// #21213
func testPullNoLayers(c *testing.T) {
	repoName := fmt.Sprintf("%v/dockercli/scratch", privateRegistryURL)

	buildImageSuccessfully(c, repoName, build.WithDockerfile(`
	FROM scratch
	ENV foo bar`))
	dockerCmd(c, "push", repoName)
	dockerCmd(c, "rmi", repoName)
	dockerCmd(c, "pull", repoName)
}

func (s *DockerRegistrySuite) TestPullNoLayers(c *testing.T) {
	testPullNoLayers(c)
}

func (s *DockerSchema1RegistrySuite) TestPullNoLayers(c *testing.T) {
	testPullNoLayers(c)
}

func (s *DockerRegistrySuite) TestPullManifestList(c *testing.T) {
	testRequires(c, NotArm)
	pushDigest, err := setupImage(c)
	assert.NilError(c, err, "error setting up image")

	// Inject a manifest list into the registry
	manifestList := &manifestlist.ManifestList{
		Versioned: manifest.Versioned{
			SchemaVersion: 2,
			MediaType:     manifestlist.MediaTypeManifestList,
		},
		Manifests: []manifestlist.ManifestDescriptor{
			{
				Descriptor: distribution.Descriptor{
					Digest:    "sha256:1a9ec845ee94c202b2d5da74a24f0ed2058318bfa9879fa541efaecba272e86b",
					Size:      3253,
					MediaType: schema2.MediaTypeManifest,
				},
				Platform: manifestlist.PlatformSpec{
					Architecture: "bogus_arch",
					OS:           "bogus_os",
				},
			},
			{
				Descriptor: distribution.Descriptor{
					Digest:    pushDigest,
					Size:      3253,
					MediaType: schema2.MediaTypeManifest,
				},
				Platform: manifestlist.PlatformSpec{
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
	err = os.MkdirAll(blobDir, 0755)
	assert.NilError(c, err, "error creating blob dir")
	blobPath := filepath.Join(blobDir, "data")
	err = os.WriteFile(blobPath, manifestListJSON, 0644)
	assert.NilError(c, err, "error writing manifest list")

	// Add to revision store
	revisionDir := filepath.Join(registryV2Path, "repositories", remoteRepoName, "_manifests", "revisions", "sha256", hexDigest)
	err = os.Mkdir(revisionDir, 0755)
	assert.Assert(c, err == nil, "error creating revision dir")
	revisionPath := filepath.Join(revisionDir, "link")
	err = os.WriteFile(revisionPath, []byte(manifestListDigest.String()), 0644)
	assert.Assert(c, err == nil, "error writing revision link")

	// Update tag
	tagPath := filepath.Join(registryV2Path, "repositories", remoteRepoName, "_manifests", "tags", "latest", "current", "link")
	err = os.WriteFile(tagPath, []byte(manifestListDigest.String()), 0644)
	assert.NilError(c, err, "error writing tag link")

	// Verify that the image can be pulled through the manifest list.
	out, _ := dockerCmd(c, "pull", repoName)

	// The pull output includes "Digest: <digest>", so find that
	matches := digestRegex.FindStringSubmatch(out)
	assert.Equal(c, len(matches), 2, fmt.Sprintf("unable to parse digest from pull output: %s", out))
	pullDigest := matches[1]

	// Make sure the pushed and pull digests match
	assert.Equal(c, manifestListDigest.String(), pullDigest)

	// Was the image actually created?
	dockerCmd(c, "inspect", repoName)

	dockerCmd(c, "rmi", repoName)
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

	repoName := fmt.Sprintf("%v/dockercli/busybox:authtest", privateRegistryURL)

	tmp, err := os.MkdirTemp("", "integration-cli-")
	assert.NilError(c, err)

	externalAuthConfig := `{ "credsStore": "shell-test" }`

	configPath := filepath.Join(tmp, "config.json")
	err = os.WriteFile(configPath, []byte(externalAuthConfig), 0644)
	assert.NilError(c, err)

	dockerCmd(c, "--config", tmp, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), privateRegistryURL)

	b, err := os.ReadFile(configPath)
	assert.NilError(c, err)
	assert.Assert(c, !strings.Contains(string(b), "\"auth\":"))
	dockerCmd(c, "--config", tmp, "tag", "busybox", repoName)
	dockerCmd(c, "--config", tmp, "push", repoName)

	dockerCmd(c, "--config", tmp, "logout", privateRegistryURL)
	dockerCmd(c, "--config", tmp, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), "https://"+privateRegistryURL)
	dockerCmd(c, "--config", tmp, "pull", repoName)

	// likewise push should work
	repoName2 := fmt.Sprintf("%v/dockercli/busybox:nocreds", privateRegistryURL)
	dockerCmd(c, "tag", repoName, repoName2)
	dockerCmd(c, "--config", tmp, "push", repoName2)

	// logout should work w scheme also because it will be stripped
	dockerCmd(c, "--config", tmp, "logout", "https://"+privateRegistryURL)
}

func (s *DockerRegistryAuthHtpasswdSuite) TestPullWithExternalAuth(c *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(c, err)
	absolute, err := filepath.Abs(filepath.Join(workingDir, "fixtures", "auth"))
	assert.NilError(c, err)

	osPath := os.Getenv("PATH")
	testPath := fmt.Sprintf("%s%c%s", osPath, filepath.ListSeparator, absolute)
	c.Setenv("PATH", testPath)

	repoName := fmt.Sprintf("%v/dockercli/busybox:authtest", privateRegistryURL)

	tmp, err := os.MkdirTemp("", "integration-cli-")
	assert.NilError(c, err)

	externalAuthConfig := `{ "credsStore": "shell-test" }`

	configPath := filepath.Join(tmp, "config.json")
	err = os.WriteFile(configPath, []byte(externalAuthConfig), 0644)
	assert.NilError(c, err)

	dockerCmd(c, "--config", tmp, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), privateRegistryURL)

	b, err := os.ReadFile(configPath)
	assert.NilError(c, err)
	assert.Assert(c, !strings.Contains(string(b), "\"auth\":"))
	dockerCmd(c, "--config", tmp, "tag", "busybox", repoName)
	dockerCmd(c, "--config", tmp, "push", repoName)

	dockerCmd(c, "--config", tmp, "pull", repoName)
}

// TestRunImplicitPullWithNoTag should pull implicitly only the default tag (latest)
func (s *DockerRegistrySuite) TestRunImplicitPullWithNoTag(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	repo := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	repoTag1 := fmt.Sprintf("%v:latest", repo)
	repoTag2 := fmt.Sprintf("%v:t1", repo)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoTag1)
	dockerCmd(c, "tag", "busybox", repoTag2)
	dockerCmd(c, "push", repo)
	dockerCmd(c, "rmi", repoTag1)
	dockerCmd(c, "rmi", repoTag2)

	out, _ := dockerCmd(c, "run", repo)
	assert.Assert(c, strings.Contains(out, fmt.Sprintf("Unable to find image '%s:latest' locally", repo)))
	// There should be only one line for repo, the one with repo:latest
	outImageCmd, _ := dockerCmd(c, "images", repo)
	splitOutImageCmd := strings.Split(strings.TrimSpace(outImageCmd), "\n")
	assert.Equal(c, len(splitOutImageCmd), 2)
}
