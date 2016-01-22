package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// testPullImageWithAliases pulls a specific image tag and verifies that any aliases (i.e., other
// tags for the same image) are not also pulled down.
//
// Ref: docker/docker#8141
func testPullImageWithAliases(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	repos := []string{}
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
		c.Assert(err, checker.NotNil, check.Commentf("Image %v shouldn't have been pulled down", repo))
	}
}

func (s *DockerRegistrySuite) TestPullImageWithAliases(c *check.C) {
	testPullImageWithAliases(c)
}

func (s *DockerSchema1RegistrySuite) TestPullImageWithAliases(c *check.C) {
	testPullImageWithAliases(c)
}

// testConcurrentPullWholeRepo pulls the same repo concurrently.
func testConcurrentPullWholeRepo(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	repos := []string{}
	for _, tag := range []string{"recent", "fresh", "todays"} {
		repo := fmt.Sprintf("%v:%v", repoName, tag)
		_, err := buildImage(repo, fmt.Sprintf(`
		    FROM busybox
		    ENTRYPOINT ["/bin/echo"]
		    ENV FOO foo
		    ENV BAR bar
		    CMD echo %s
		`, repo), true)
		c.Assert(err, checker.IsNil)
		dockerCmd(c, "push", repo)
		repos = append(repos, repo)
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	dockerCmd(c, args...)

	// Run multiple re-pulls concurrently
	results := make(chan error)
	numPulls := 3

	for i := 0; i != numPulls; i++ {
		go func() {
			_, _, err := runCommandWithOutput(exec.Command(dockerBinary, "pull", "-a", repoName))
			results <- err
		}()
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for i := 0; i != numPulls; i++ {
		err := <-results
		c.Assert(err, checker.IsNil, check.Commentf("concurrent pull failed with error: %v", err))
	}

	// Ensure all tags were pulled successfully
	for _, repo := range repos {
		dockerCmd(c, "inspect", repo)
		out, _ := dockerCmd(c, "run", "--rm", repo)
		c.Assert(strings.TrimSpace(out), checker.Equals, "/bin/sh -c echo "+repo)
	}
}

func (s *DockerRegistrySuite) testConcurrentPullWholeRepo(c *check.C) {
	testConcurrentPullWholeRepo(c)
}

func (s *DockerSchema1RegistrySuite) testConcurrentPullWholeRepo(c *check.C) {
	testConcurrentPullWholeRepo(c)
}

// testConcurrentFailingPull tries a concurrent pull that doesn't succeed.
func testConcurrentFailingPull(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	// Run multiple pulls concurrently
	results := make(chan error)
	numPulls := 3

	for i := 0; i != numPulls; i++ {
		go func() {
			_, _, err := runCommandWithOutput(exec.Command(dockerBinary, "pull", repoName+":asdfasdf"))
			results <- err
		}()
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for i := 0; i != numPulls; i++ {
		err := <-results
		c.Assert(err, checker.NotNil, check.Commentf("expected pull to fail"))
	}
}

func (s *DockerRegistrySuite) testConcurrentFailingPull(c *check.C) {
	testConcurrentFailingPull(c)
}

func (s *DockerSchema1RegistrySuite) testConcurrentFailingPull(c *check.C) {
	testConcurrentFailingPull(c)
}

// testConcurrentPullMultipleTags pulls multiple tags from the same repo
// concurrently.
func testConcurrentPullMultipleTags(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	repos := []string{}
	for _, tag := range []string{"recent", "fresh", "todays"} {
		repo := fmt.Sprintf("%v:%v", repoName, tag)
		_, err := buildImage(repo, fmt.Sprintf(`
		    FROM busybox
		    ENTRYPOINT ["/bin/echo"]
		    ENV FOO foo
		    ENV BAR bar
		    CMD echo %s
		`, repo), true)
		c.Assert(err, checker.IsNil)
		dockerCmd(c, "push", repo)
		repos = append(repos, repo)
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	dockerCmd(c, args...)

	// Re-pull individual tags, in parallel
	results := make(chan error)

	for _, repo := range repos {
		go func(repo string) {
			_, _, err := runCommandWithOutput(exec.Command(dockerBinary, "pull", repo))
			results <- err
		}(repo)
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for range repos {
		err := <-results
		c.Assert(err, checker.IsNil, check.Commentf("concurrent pull failed with error: %v", err))
	}

	// Ensure all tags were pulled successfully
	for _, repo := range repos {
		dockerCmd(c, "inspect", repo)
		out, _ := dockerCmd(c, "run", "--rm", repo)
		c.Assert(strings.TrimSpace(out), checker.Equals, "/bin/sh -c echo "+repo)
	}
}

func (s *DockerRegistrySuite) TestConcurrentPullMultipleTags(c *check.C) {
	testConcurrentPullMultipleTags(c)
}

func (s *DockerSchema1RegistrySuite) TestConcurrentPullMultipleTags(c *check.C) {
	testConcurrentPullMultipleTags(c)
}

// testPullIDStability verifies that pushing an image and pulling it back
// preserves the image ID.
func testPullIDStability(c *check.C) {
	derivedImage := privateRegistryURL + "/dockercli/id-stability"
	baseImage := "busybox"

	_, err := buildImage(derivedImage, fmt.Sprintf(`
	    FROM %s
	    ENV derived true
	    ENV asdf true
	    RUN dd if=/dev/zero of=/file bs=1024 count=1024
	    CMD echo %s
	`, baseImage, derivedImage), true)
	if err != nil {
		c.Fatal(err)
	}

	originalID, err := getIDByName(derivedImage)
	if err != nil {
		c.Fatalf("error inspecting: %v", err)
	}
	dockerCmd(c, "push", derivedImage)

	// Pull
	out, _ := dockerCmd(c, "pull", derivedImage)
	if strings.Contains(out, "Pull complete") {
		c.Fatalf("repull redownloaded a layer: %s", out)
	}

	derivedIDAfterPull, err := getIDByName(derivedImage)
	if err != nil {
		c.Fatalf("error inspecting: %v", err)
	}

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

	derivedIDAfterPull, err = getIDByName(derivedImage)
	if err != nil {
		c.Fatalf("error inspecting: %v", err)
	}

	if derivedIDAfterPull != originalID {
		c.Fatal("image's ID unexpectedly changed after a repush/repull")
	}
	if err != nil {
		c.Fatalf("error inspecting: %v", err)
	}

	// Make sure the image still runs
	out, _ = dockerCmd(c, "run", "--rm", derivedImage)
	if strings.TrimSpace(out) != derivedImage {
		c.Fatalf("expected %s; got %s", derivedImage, out)
	}
}

func (s *DockerRegistrySuite) TestPullIDStability(c *check.C) {
	testPullIDStability(c)
}

func (s *DockerSchema1RegistrySuite) TestPullIDStability(c *check.C) {
	testPullIDStability(c)
}

// TestPullFallbackOn404 tries to pull a nonexistent manifest and confirms that
// the pull falls back to the v1 protocol.
//
// Ref: docker/docker#18832
func (s *DockerRegistrySuite) TestPullFallbackOn404(c *check.C) {
	repoName := fmt.Sprintf("%v/does/not/exist", privateRegistryURL)

	out, _, _ := dockerCmdWithError("pull", repoName)

	c.Assert(out, checker.Contains, "v1 ping attempt")
}

func (s *DockerRegistrySuite) TestPullManifestList(c *check.C) {
	pushDigest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

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
	c.Assert(err, checker.IsNil, check.Commentf("error marshalling manifest list"))

	manifestListDigest := digest.FromBytes(manifestListJSON)
	hexDigest := manifestListDigest.Hex()

	registryV2Path := filepath.Join(s.reg.dir, "docker", "registry", "v2")

	// Write manifest list to blob store
	blobDir := filepath.Join(registryV2Path, "blobs", "sha256", hexDigest[:2], hexDigest)
	err = os.MkdirAll(blobDir, 0755)
	c.Assert(err, checker.IsNil, check.Commentf("error creating blob dir"))
	blobPath := filepath.Join(blobDir, "data")
	err = ioutil.WriteFile(blobPath, []byte(manifestListJSON), 0644)
	c.Assert(err, checker.IsNil, check.Commentf("error writing manifest list"))

	// Add to revision store
	revisionDir := filepath.Join(registryV2Path, "repositories", remoteRepoName, "_manifests", "revisions", "sha256", hexDigest)
	err = os.Mkdir(revisionDir, 0755)
	c.Assert(err, checker.IsNil, check.Commentf("error creating revision dir"))
	revisionPath := filepath.Join(revisionDir, "link")
	err = ioutil.WriteFile(revisionPath, []byte(manifestListDigest.String()), 0644)
	c.Assert(err, checker.IsNil, check.Commentf("error writing revision link"))

	// Update tag
	tagPath := filepath.Join(registryV2Path, "repositories", remoteRepoName, "_manifests", "tags", "latest", "current", "link")
	err = ioutil.WriteFile(tagPath, []byte(manifestListDigest.String()), 0644)
	c.Assert(err, checker.IsNil, check.Commentf("error writing tag link"))

	// Verify that the image can be pulled through the manifest list.
	out, _ := dockerCmd(c, "pull", repoName)

	// The pull output includes "Digest: <digest>", so find that
	matches := digestRegex.FindStringSubmatch(out)
	c.Assert(matches, checker.HasLen, 2, check.Commentf("unable to parse digest from pull output: %s", out))
	pullDigest := matches[1]

	// Make sure the pushed and pull digests match
	c.Assert(manifestListDigest.String(), checker.Equals, pullDigest)

	// Was the image actually created?
	dockerCmd(c, "inspect", repoName)

	dockerCmd(c, "rmi", repoName)
}

// Test pulls from blocked public registry and from private registry. This
// shall be called with various daemonArgs containing at least one
// `--block-registry` flag.
func (s *DockerRegistrySuite) doTestPullFromBlockedPublicRegistry(c *check.C, daemonArgs []string) {
	allBlocked := false
	for _, arg := range daemonArgs {
		if arg == "--block-registry=all" {
			allBlocked = true
		}
	}
	if err := s.d.StartWithBusybox(daemonArgs...); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}

	busyboxID := s.d.getAndTestImageEntry(c, 1, "busybox", "").id

	// try to pull from docker.io
	if out, err := s.d.Cmd("pull", "library/hello-world"); err == nil {
		c.Fatalf("pull from blocked public registry should have failed, output: %s", out)
	}

	// tag busybox as library/hello-world and push it to some private registry
	if out, err := s.d.Cmd("tag", "busybox", s.reg.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg.url+"/library/hello-world"); !allBlocked && err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg.url+"/library/hello-world", err, out)
	} else if allBlocked && err == nil {
		c.Fatalf("push to private registry should have failed, output: %q", out)
	}

	// remove library/hello-world image
	if out, err := s.d.Cmd("rmi", s.reg.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", s.reg.url+"/library/hello-world", err, out)
	}
	s.d.getAndTestImageEntry(c, 1, "busybox", busyboxID)

	// try to pull from private registry
	if out, err := s.d.Cmd("pull", s.reg.url+"/library/hello-world"); !allBlocked && err != nil {
		c.Fatalf("we should have been able to pull %s/library/hello-world: %v", s.reg.url, err)
	} else if allBlocked && err == nil {
		c.Fatalf("pull from private registry should have failed, output: %q", out)
	} else if !allBlocked {
		s.d.getAndTestImageEntry(c, 2, s.reg.url+"/library/hello-world", busyboxID)
	}
}

func (s *DockerRegistrySuite) TestPullFromBlockedPublicRegistry(c *check.C) {
	for _, blockedRegistry := range []string{"public", "docker.io"} {
		s.doTestPullFromBlockedPublicRegistry(c, []string{"--block-registry=" + blockedRegistry})
		s.d.Stop()
		s.d = NewDaemon(c)
	}
}

func (s *DockerRegistrySuite) TestPullWithAllRegistriesBlocked(c *check.C) {
	s.doTestPullFromBlockedPublicRegistry(c, []string{"--block-registry=all"})
}

// Test pulls from additional registry with public registry blocked. This
// shall be called with various daemonArgs containing at least one
// `--block-registry` flag.
func (s *DockerRegistriesSuite) doTestPullFromPrivateRegistriesWithPublicBlocked(c *check.C, daemonArgs []string) {
	allBlocked := false
	for _, arg := range daemonArgs {
		if arg == "--block-registry=all" {
			allBlocked = true
		}
	}
	daemonArgs = append(daemonArgs, "--add-registry="+s.reg1.url)
	if err := s.d.StartWithBusybox(daemonArgs...); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}

	bbImg := s.d.getAndTestImageEntry(c, 1, "busybox", "")

	// try to pull from blocked public registry
	if out, err := s.d.Cmd("pull", "library/hello-world"); err == nil {
		c.Fatalf("pulling from blocked public registry should have failed, output: %s", out)
	}

	// push busybox to
	//  additional registry as "misc/busybox"
	//  private registry as "library/busybox"
	// and remove all local images
	if out, err := s.d.Cmd("tag", "busybox", s.reg1.url+"/misc/busybox"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("tag", "busybox", s.reg2.url+"/library/busybox"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg1.url+"/misc/busybox"); err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg1.url+"/misc/busybox", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg2.url+"/library/busybox"); !allBlocked && err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg2.url+"/library/busybox", err, out)
	} else if allBlocked && err == nil {
		c.Fatalf("push to private registry should have failed, output: %q", out)
	}
	toRemove := []string{"busybox", "misc/busybox", s.reg2.url + "/library/busybox"}
	if out, err := s.d.Cmd("rmi", toRemove...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	s.d.getAndTestImageEntry(c, 0, "", "")

	// try to pull "library/busybox" from additional registry
	if out, err := s.d.Cmd("pull", "library/busybox"); err == nil {
		c.Fatalf("pull of library/busybox from additional registry should have failed, output: %q", out)
	}

	// now pull the "misc/busybox" from additional registry
	if _, err := s.d.Cmd("pull", "misc/busybox"); err != nil {
		c.Fatalf("we should have been able to pull misc/hello-world from %q: %v", s.reg1.url, err)
	}
	bb2Img := s.d.getAndTestImageEntry(c, 1, s.reg1.url+"/misc/busybox", "")
	if bb2Img.size != bbImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", bb2Img.name, bbImg.name, bb2Img.size, bbImg.size)
	}

	// try to pull "library/busybox" from private registry
	if out, err := s.d.Cmd("pull", s.reg2.url+"/library/busybox"); !allBlocked && err != nil {
		c.Fatalf("we should have been able to pull %s/library/busybox: %v", s.reg2.url, err)
	} else if allBlocked && err == nil {
		c.Fatalf("pull from private registry should have failed, output: %q", out)
	} else if !allBlocked {
		bb3Img := s.d.getAndTestImageEntry(c, 2, s.reg2.url+"/library/busybox", "")
		if bb3Img.size != bbImg.size {
			c.Fatalf("expected %s and %s to have the same size (%s != %s)", bb3Img.name, bbImg.name, bb3Img.size, bbImg.size)
		}
	}
}

func (s *DockerRegistriesSuite) TestPullFromPrivateRegistriesWithPublicBlocked(c *check.C) {
	for _, blockedRegistry := range []string{"public", "docker.io"} {
		s.doTestPullFromPrivateRegistriesWithPublicBlocked(c, []string{"--block-registry=" + blockedRegistry})
		s.d.Stop()
		s.d = NewDaemon(c)
	}
}

func (s *DockerRegistriesSuite) TestPullFromAdditionalRegistryWithAllBlocked(c *check.C) {
	s.doTestPullFromPrivateRegistriesWithPublicBlocked(c, []string{"--block-registry=all"})
}
