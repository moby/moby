package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/registry"
	"github.com/go-check/check"
)

func init() {
	check.Suite(&DockerManifestSuite{
		ds: &DockerSuite{},
	})
}

type DockerManifestSuite struct {
	ds  *DockerSuite
	reg *registry.V2
}

func (s *DockerManifestSuite) SetUpSuite(c *check.C) {
	// make config.json if it doesn't exist, and add insecure registry to it
	os.Mkdir("/root/.docker/", 0770)
	if _, err := os.Stat("/root/.docker/config.json"); os.IsNotExist(err) {
		os.Create("/root/.docker/config.json")
	}
	f, err := os.OpenFile("/root/.docker/config.json", os.O_APPEND|os.O_WRONLY, 0600)
	c.Assert(err, checker.IsNil)
	defer f.Close()

	insecureRegistry := "{\"insecure-registries\" : [\"127.0.0.1:5000\"]}"
	_, err = f.WriteString(insecureRegistry)
	c.Assert(err, checker.IsNil)

	configLocation := "/root/.docker/config.json"
	_, err = os.Stat(configLocation)
	c.Assert(err, checker.IsNil)

}

func (s *DockerManifestSuite) TearDownSuite(c *check.C) {
	// intetionally empty
}

func (s *DockerManifestSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, registry.Hosting)

	// setup registry and populate it with two busybox images
	s.reg = setupRegistry(c, false, "", privateRegistryURL)

	image1 := fmt.Sprintf("%s/busybox", privateRegistryURL)
	image2 := fmt.Sprintf("%s/busybox2", privateRegistryURL)

	dockerCmd(c, "tag", "busybox", image1)
	dockerCmd(c, "tag", "busybox", image2)

	_, _, err := dockerCmdWithError("push", image1)
	c.Assert(err, checker.IsNil)

	_, _, err = dockerCmdWithError("push", image2)
	c.Assert(err, checker.IsNil)
}

func (s *DockerManifestSuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	s.ds.TearDownTest(c)
}

func (s *DockerManifestSuite) TestManifestInspectUntagged(c *check.C) {
	testRepo := "testrepo"
	testRepoRegistry := fmt.Sprintf("%s/%s", privateRegistryURL, testRepo)

	image1 := fmt.Sprintf("%s/busybox", testRepoRegistry)

	dockerCmd(c, "tag", "busybox", image1)
	dockerCmd(c, "push", image1)

	out, _, err := dockerCmdWithError("manifest", "inspect", image1)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), "not found")
}

func (s *DockerManifestSuite) TestManifestInspectTagged(c *check.C) {
	testRepo := "testrepo"
	testRepoRegistry := fmt.Sprintf("%s/%s", privateRegistryURL, testRepo)

	imageFound := fmt.Sprintf("%s/busybox:push", testRepoRegistry)
	imageNotfound := fmt.Sprintf("%s/busybox:nopush", testRepoRegistry)

	dockerCmd(c, "tag", "busybox", imageFound)
	dockerCmd(c, "push", imageFound)

	// Make sure the error message always contains "not found"
	out, _, err := dockerCmdWithError("manifest", "inspect", imageNotfound)
	c.Assert(err, checker.Not(checker.IsNil))
	c.Assert(out, checker.Contains, "not found")

	// Make sure tags are kept
	out, _, err = dockerCmdWithError("manifest", "inspect", imageFound)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), "not found")
}

func (s *DockerManifestSuite) TestManifestCreate(c *check.C) {
	testRepo := "testrepo/busybox"

	out, _, _ := dockerCmdWithError("manifest", "create", testRepo, "busybox", "busybox:thisdoesntexist")
	c.Assert(out, checker.Contains, "manifest unknown")

	_, _, err := dockerCmdWithError("manifest", "create", testRepo, "busybox", "debian:jessie")
	c.Assert(err, checker.IsNil)

	splitRepo := strings.Split(testRepo, "/")
	c.Assert(len(splitRepo), checker.Equals, 2)

	manifestLocation := "/root/.docker/manifests/docker.io_" + splitRepo[0] + "_" + splitRepo[1] + "-latest"
	_, err = os.Stat(manifestLocation)
	c.Assert(err, checker.IsNil, check.Commentf("Manifest not found in ", manifestLocation))

}

func (s *DockerManifestSuite) TestManifestPush(c *check.C) {
	testRepo := "testrepo"
	testRepoRegistry := fmt.Sprintf("%s/%s", privateRegistryURL, testRepo)

	image1 := fmt.Sprintf("%s/busybox", privateRegistryURL)
	image2 := fmt.Sprintf("%s/busybox2", privateRegistryURL)

	dockerCmd(c, "manifest", "create", testRepoRegistry, image1, image2)

	dockerCmd(c, "manifest", "annotate", testRepoRegistry, image1, "--os", runtime.GOOS, "--arch", runtime.GOARCH)
	dockerCmd(c, "manifest", "annotate", testRepoRegistry, image2, "--os", runtime.GOOS, "--arch", runtime.GOARCH)

	out, _, err := dockerCmdWithError("manifest", "push", testRepoRegistry)
	c.Assert(err, checker.IsNil)
	successfulPush := "Succesfully pushed manifest list " + testRepo
	c.Assert(out, checker.Contains, successfulPush)
}

func (s *DockerManifestSuite) TestManifestAnnotatePushInspect(c *check.C) {
	testRepo := "testrepo"
	testRepoRegistry := fmt.Sprintf("%s/%s", privateRegistryURL, testRepo)

	image1 := fmt.Sprintf("%s/busybox", privateRegistryURL)
	image2 := fmt.Sprintf("%s/busybox2", privateRegistryURL)

	dockerCmd(c, "manifest", "create", testRepoRegistry, image1, image2)

	// test with bad os / arch
	out, _, _ := dockerCmdWithError("manifest", "annotate", testRepoRegistry, image1, "--os", "bados", "--arch", "amd64")
	c.Assert(out, checker.Contains, "Manifest entry for image has unsupported os/arch combination")

	out, _, _ = dockerCmdWithError("manifest", "annotate", testRepoRegistry, image2, "--os", "linux", "--arch", "badarch")
	c.Assert(out, checker.Contains, "Manifest entry for image has unsupported os/arch combination")

	// now annotate correctly, but give duplicate cpu and os features
	_, _, err := dockerCmdWithError("manifest", "annotate", testRepoRegistry, image1, "--os", "linux", "--arch", "amd64", "--cpuFeatures", "sse1, sse1", "--osFeatures", "osf1, osf1")
	c.Assert(err, checker.IsNil)
	_, _, err = dockerCmdWithError("manifest", "annotate", testRepoRegistry, image2, "--os", "freebsd", "--arch", "arm", "--cpuFeatures", "sse2", "--osFeatures", "osf2")
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "manifest", "push", testRepoRegistry)

	out, _ = dockerCmd(c, "manifest", "inspect", testRepoRegistry)
	c.Assert(out, checker.Contains, "linux")
	c.Assert(out, checker.Contains, "freebsd")
	c.Assert(out, checker.Contains, "amd64")
	c.Assert(out, checker.Contains, "arm")
	c.Assert(out, checker.Contains, "sse1")
	c.Assert(out, checker.Contains, "sse2")
	c.Assert(out, checker.Contains, "osf1")
	c.Assert(out, checker.Contains, "osf2")
	c.Assert(strings.Count(out, "sse1"), checker.Equals, 1)
	c.Assert(strings.Count(out, "osf1"), checker.Equals, 1)

}
