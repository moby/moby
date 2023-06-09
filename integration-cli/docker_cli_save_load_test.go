package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/integration-cli/cli/build"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

type DockerCLISaveLoadSuite struct {
	ds *DockerSuite
}

func (s *DockerCLISaveLoadSuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLISaveLoadSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

// save a repo using gz compression and try to load it using stdout
func (s *DockerCLISaveLoadSuite) TestSaveXzAndLoadRepoStdout(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	name := "test-save-xz-and-load-repo-stdout"
	dockerCmd(c, "run", "--name", name, "busybox", "true")

	repoName := "foobar-save-load-test-xz-gz"
	out, _ := dockerCmd(c, "commit", name, repoName)

	dockerCmd(c, "inspect", repoName)

	repoTarball, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("xz", "-c"),
		exec.Command("gzip", "-c"))
	assert.NilError(c, err, "failed to save repo: %v %v", out, err)
	deleteImages(repoName)

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "load"},
		Stdin:   strings.NewReader(repoTarball),
	}).Assert(c, icmd.Expected{
		ExitCode: 1,
	})

	after, _, err := dockerCmdWithError("inspect", repoName)
	assert.ErrorContains(c, err, "", "the repo should not exist: %v", after)
}

// save a repo using xz+gz compression and try to load it using stdout
func (s *DockerCLISaveLoadSuite) TestSaveXzGzAndLoadRepoStdout(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	name := "test-save-xz-gz-and-load-repo-stdout"
	dockerCmd(c, "run", "--name", name, "busybox", "true")

	repoName := "foobar-save-load-test-xz-gz"
	dockerCmd(c, "commit", name, repoName)

	dockerCmd(c, "inspect", repoName)

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("xz", "-c"),
		exec.Command("gzip", "-c"))
	assert.NilError(c, err, "failed to save repo: %v %v", out, err)

	deleteImages(repoName)

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "load"},
		Stdin:   strings.NewReader(out),
	}).Assert(c, icmd.Expected{
		ExitCode: 1,
	})

	after, _, err := dockerCmdWithError("inspect", repoName)
	assert.ErrorContains(c, err, "", "the repo should not exist: %v", after)
}

func (s *DockerCLISaveLoadSuite) TestSaveSingleTag(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	repoName := "foobar-save-single-tag-test"
	dockerCmd(c, "tag", "busybox:latest", fmt.Sprintf("%v:latest", repoName))

	out, _ := dockerCmd(c, "images", "-q", "--no-trunc", repoName)
	cleanedImageID := strings.TrimSpace(out)

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", fmt.Sprintf("%v:latest", repoName)),
		exec.Command("tar", "t"),
		exec.Command("grep", "-E", fmt.Sprintf("(^repositories$|%v)", cleanedImageID)))
	assert.NilError(c, err, "failed to save repo with image ID and 'repositories' file: %s, %v", out, err)
}

func (s *DockerCLISaveLoadSuite) TestSaveImageId(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	repoName := "foobar-save-image-id-test"
	dockerCmd(c, "tag", "emptyfs:latest", fmt.Sprintf("%v:latest", repoName))

	out, _ := dockerCmd(c, "images", "-q", "--no-trunc", repoName)
	cleanedLongImageID := strings.TrimPrefix(strings.TrimSpace(out), "sha256:")

	out, _ = dockerCmd(c, "images", "-q", repoName)
	cleanedShortImageID := strings.TrimSpace(out)

	// Make sure IDs are not empty
	assert.Assert(c, cleanedLongImageID != "", "Id should not be empty.")
	assert.Assert(c, cleanedShortImageID != "", "Id should not be empty.")

	saveCmd := exec.Command(dockerBinary, "save", cleanedShortImageID)
	tarCmd := exec.Command("tar", "t")

	var err error
	tarCmd.Stdin, err = saveCmd.StdoutPipe()
	assert.Assert(c, err == nil, "cannot set stdout pipe for tar: %v", err)
	grepCmd := exec.Command("grep", cleanedLongImageID)
	grepCmd.Stdin, err = tarCmd.StdoutPipe()
	assert.Assert(c, err == nil, "cannot set stdout pipe for grep: %v", err)

	assert.Assert(c, tarCmd.Start() == nil, "tar failed with error: %v", err)
	assert.Assert(c, saveCmd.Start() == nil, "docker save failed with error: %v", err)
	defer func() {
		saveCmd.Wait()
		tarCmd.Wait()
		dockerCmd(c, "rmi", repoName)
	}()

	out, _, err = runCommandWithOutput(grepCmd)

	assert.Assert(c, err == nil, "failed to save repo with image ID: %s, %v", out, err)
}

// save a repo and try to load it using flags
func (s *DockerCLISaveLoadSuite) TestSaveAndLoadRepoFlags(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	name := "test-save-and-load-repo-flags"
	dockerCmd(c, "run", "--name", name, "busybox", "true")

	repoName := "foobar-save-load-test"

	deleteImages(repoName)
	dockerCmd(c, "commit", name, repoName)

	before, _ := dockerCmd(c, "inspect", repoName)

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command(dockerBinary, "load"))
	assert.NilError(c, err, "failed to save and load repo: %s, %v", out, err)

	after, _ := dockerCmd(c, "inspect", repoName)
	assert.Equal(c, before, after, "inspect is not the same after a save / load")
}

func (s *DockerCLISaveLoadSuite) TestSaveWithNoExistImage(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	imgName := "foobar-non-existing-image"

	out, _, err := dockerCmdWithError("save", "-o", "test-img.tar", imgName)
	assert.ErrorContains(c, err, "", "save image should fail for non-existing image")
	assert.Assert(c, strings.Contains(out, fmt.Sprintf("No such image: %s", imgName)))
}

func (s *DockerCLISaveLoadSuite) TestSaveMultipleNames(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	repoName := "foobar-save-multi-name-test"

	// Make one image
	dockerCmd(c, "tag", "emptyfs:latest", fmt.Sprintf("%v-one:latest", repoName))

	// Make two images
	dockerCmd(c, "tag", "emptyfs:latest", fmt.Sprintf("%v-two:latest", repoName))

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", fmt.Sprintf("%v-one", repoName), fmt.Sprintf("%v-two:latest", repoName)),
		exec.Command("tar", "xO", "repositories"),
		exec.Command("grep", "-q", "-E", "(-one|-two)"),
	)
	assert.NilError(c, err, "failed to save multiple repos: %s, %v", out, err)
}

// Test loading a weird image where one of the layers is of zero size.
// The layer.tar file is actually zero bytes, no padding or anything else.
// See issue: 18170
func (s *DockerCLISaveLoadSuite) TestLoadZeroSizeLayer(c *testing.T) {
	// this will definitely not work if using remote daemon
	// very weird test
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)

	dockerCmd(c, "load", "-i", "testdata/emptyLayer.tar")
}

func (s *DockerCLISaveLoadSuite) TestSaveLoadParents(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	makeImage := func(from string, addfile string) string {
		var (
			out string
		)
		out, _ = dockerCmd(c, "run", "-d", from, "touch", addfile)
		cleanedContainerID := strings.TrimSpace(out)

		out, _ = dockerCmd(c, "commit", cleanedContainerID)
		imageID := strings.TrimSpace(out)

		dockerCmd(c, "rm", "-f", cleanedContainerID)
		return imageID
	}

	idFoo := makeImage("busybox", "foo")
	idBar := makeImage(idFoo, "bar")

	tmpDir, err := os.MkdirTemp("", "save-load-parents")
	assert.NilError(c, err)
	defer os.RemoveAll(tmpDir)

	c.Log("tmpdir", tmpDir)

	outfile := filepath.Join(tmpDir, "out.tar")

	dockerCmd(c, "save", "-o", outfile, idBar, idFoo)
	dockerCmd(c, "rmi", idBar)
	dockerCmd(c, "load", "-i", outfile)

	inspectOut := inspectField(c, idBar, "Parent")
	assert.Equal(c, inspectOut, idFoo)

	inspectOut = inspectField(c, idFoo, "Parent")
	assert.Equal(c, inspectOut, "")
}

func (s *DockerCLISaveLoadSuite) TestSaveLoadNoTag(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	name := "saveloadnotag"

	buildImageSuccessfully(c, name, build.WithDockerfile("FROM busybox\nENV foo=bar"))
	id := inspectField(c, name, "Id")

	// Test to make sure that save w/o name just shows imageID during load
	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", id),
		exec.Command(dockerBinary, "load"))
	assert.NilError(c, err, "failed to save and load repo: %s, %v", out, err)

	// Should not show 'name' but should show the image ID during the load
	assert.Assert(c, !strings.Contains(out, "Loaded image: "))
	assert.Assert(c, strings.Contains(out, "Loaded image ID:"))
	assert.Assert(c, strings.Contains(out, id))
	// Test to make sure that save by name shows that name during load
	out, err = RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", name),
		exec.Command(dockerBinary, "load"))
	assert.NilError(c, err, "failed to save and load repo: %s, %v", out, err)

	assert.Assert(c, strings.Contains(out, "Loaded image: "+name+":latest"))
	assert.Assert(c, !strings.Contains(out, "Loaded image ID:"))
}
