package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/go-check/check"
)

// save a repo using gz compression and try to load it using stdout
func (s *DockerSuite) TestSaveXzAndLoadRepoStdout(c *check.C) {
	name := "test-save-xz-and-load-repo-stdout"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to create a container: %v %v", out, err)
	}

	repoName := "foobar-save-load-test-xz-gz"

	commitCmd := exec.Command(dockerBinary, "commit", name, repoName)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		c.Fatalf("failed to commit container: %v %v", out, err)
	}

	inspectCmd := exec.Command(dockerBinary, "inspect", repoName)
	before, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("the repo should exist before saving it: %v %v", before, err)
	}

	repoTarball, _, err := runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("xz", "-c"),
		exec.Command("gzip", "-c"))
	if err != nil {
		c.Fatalf("failed to save repo: %v %v", out, err)
	}
	deleteImages(repoName)

	loadCmd := exec.Command(dockerBinary, "load")
	loadCmd.Stdin = strings.NewReader(repoTarball)
	out, _, err = runCommandWithOutput(loadCmd)
	if err == nil {
		c.Fatalf("expected error, but succeeded with no error and output: %v", out)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	after, _, err := runCommandWithOutput(inspectCmd)
	if err == nil {
		c.Fatalf("the repo should not exist: %v", after)
	}

	deleteImages(repoName)

}

// save a repo using xz+gz compression and try to load it using stdout
func (s *DockerSuite) TestSaveXzGzAndLoadRepoStdout(c *check.C) {
	name := "test-save-xz-gz-and-load-repo-stdout"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to create a container: %v %v", out, err)
	}

	repoName := "foobar-save-load-test-xz-gz"

	commitCmd := exec.Command(dockerBinary, "commit", name, repoName)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		c.Fatalf("failed to commit container: %v %v", out, err)
	}

	inspectCmd := exec.Command(dockerBinary, "inspect", repoName)
	before, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("the repo should exist before saving it: %v %v", before, err)
	}

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("xz", "-c"),
		exec.Command("gzip", "-c"))
	if err != nil {
		c.Fatalf("failed to save repo: %v %v", out, err)
	}

	deleteImages(repoName)

	loadCmd := exec.Command(dockerBinary, "load")
	loadCmd.Stdin = strings.NewReader(out)
	out, _, err = runCommandWithOutput(loadCmd)
	if err == nil {
		c.Fatalf("expected error, but succeeded with no error and output: %v", out)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	after, _, err := runCommandWithOutput(inspectCmd)
	if err == nil {
		c.Fatalf("the repo should not exist: %v", after)
	}
}

func (s *DockerSuite) TestSaveSingleTag(c *check.C) {
	repoName := "foobar-save-single-tag-test"

	tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", fmt.Sprintf("%v:latest", repoName))
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatalf("failed to tag repo: %s, %v", out, err)
	}

	idCmd := exec.Command(dockerBinary, "images", "-q", "--no-trunc", repoName)
	out, _, err := runCommandWithOutput(idCmd)
	if err != nil {
		c.Fatalf("failed to get repo ID: %s, %v", out, err)
	}
	cleanedImageID := strings.TrimSpace(out)

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", fmt.Sprintf("%v:latest", repoName)),
		exec.Command("tar", "t"),
		exec.Command("grep", "-E", fmt.Sprintf("(^repositories$|%v)", cleanedImageID)))
	if err != nil {
		c.Fatalf("failed to save repo with image ID and 'repositories' file: %s, %v", out, err)
	}

}

func (s *DockerSuite) TestSaveImageId(c *check.C) {
	repoName := "foobar-save-image-id-test"

	tagCmd := exec.Command(dockerBinary, "tag", "emptyfs:latest", fmt.Sprintf("%v:latest", repoName))
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatalf("failed to tag repo: %s, %v", out, err)
	}

	idLongCmd := exec.Command(dockerBinary, "images", "-q", "--no-trunc", repoName)
	out, _, err := runCommandWithOutput(idLongCmd)
	if err != nil {
		c.Fatalf("failed to get repo ID: %s, %v", out, err)
	}

	cleanedLongImageID := strings.TrimSpace(out)

	idShortCmd := exec.Command(dockerBinary, "images", "-q", repoName)
	out, _, err = runCommandWithOutput(idShortCmd)
	if err != nil {
		c.Fatalf("failed to get repo short ID: %s, %v", out, err)
	}

	cleanedShortImageID := strings.TrimSpace(out)

	saveCmd := exec.Command(dockerBinary, "save", cleanedShortImageID)
	tarCmd := exec.Command("tar", "t")
	tarCmd.Stdin, err = saveCmd.StdoutPipe()
	if err != nil {
		c.Fatalf("cannot set stdout pipe for tar: %v", err)
	}
	grepCmd := exec.Command("grep", cleanedLongImageID)
	grepCmd.Stdin, err = tarCmd.StdoutPipe()
	if err != nil {
		c.Fatalf("cannot set stdout pipe for grep: %v", err)
	}

	if err = tarCmd.Start(); err != nil {
		c.Fatalf("tar failed with error: %v", err)
	}
	if err = saveCmd.Start(); err != nil {
		c.Fatalf("docker save failed with error: %v", err)
	}
	defer saveCmd.Wait()
	defer tarCmd.Wait()

	out, _, err = runCommandWithOutput(grepCmd)

	if err != nil {
		c.Fatalf("failed to save repo with image ID: %s, %v", out, err)
	}

}

// save a repo and try to load it using flags
func (s *DockerSuite) TestSaveAndLoadRepoFlags(c *check.C) {
	name := "test-save-and-load-repo-flags"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to create a container: %s, %v", out, err)
	}
	repoName := "foobar-save-load-test"

	commitCmd := exec.Command(dockerBinary, "commit", name, repoName)
	deleteImages(repoName)
	if out, _, err = runCommandWithOutput(commitCmd); err != nil {
		c.Fatalf("failed to commit container: %s, %v", out, err)
	}

	inspectCmd := exec.Command(dockerBinary, "inspect", repoName)
	before, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("the repo should exist before saving it: %s, %v", before, err)

	}

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command(dockerBinary, "load"))
	if err != nil {
		c.Fatalf("failed to save and load repo: %s, %v", out, err)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	after, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("the repo should exist after loading it: %s, %v", after, err)
	}

	if before != after {
		c.Fatalf("inspect is not the same after a save / load")
	}
}

func (s *DockerSuite) TestSaveMultipleNames(c *check.C) {
	repoName := "foobar-save-multi-name-test"

	// Make one image
	tagCmd := exec.Command(dockerBinary, "tag", "emptyfs:latest", fmt.Sprintf("%v-one:latest", repoName))
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatalf("failed to tag repo: %s, %v", out, err)
	}

	// Make two images
	tagCmd = exec.Command(dockerBinary, "tag", "emptyfs:latest", fmt.Sprintf("%v-two:latest", repoName))
	out, _, err := runCommandWithOutput(tagCmd)
	if err != nil {
		c.Fatalf("failed to tag repo: %s, %v", out, err)
	}

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", fmt.Sprintf("%v-one", repoName), fmt.Sprintf("%v-two:latest", repoName)),
		exec.Command("tar", "xO", "repositories"),
		exec.Command("grep", "-q", "-E", "(-one|-two)"),
	)
	if err != nil {
		c.Fatalf("failed to save multiple repos: %s, %v", out, err)
	}

}

func (s *DockerSuite) TestSaveRepoWithMultipleImages(c *check.C) {

	makeImage := func(from string, tag string) string {
		runCmd := exec.Command(dockerBinary, "run", "-d", from, "true")
		var (
			out string
			err error
		)
		if out, _, err = runCommandWithOutput(runCmd); err != nil {
			c.Fatalf("failed to create a container: %v %v", out, err)
		}
		cleanedContainerID := strings.TrimSpace(out)

		commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID, tag)
		if out, _, err = runCommandWithOutput(commitCmd); err != nil {
			c.Fatalf("failed to commit container: %v %v", out, err)
		}
		imageID := strings.TrimSpace(out)
		return imageID
	}

	repoName := "foobar-save-multi-images-test"
	tagFoo := repoName + ":foo"
	tagBar := repoName + ":bar"

	idFoo := makeImage("busybox:latest", tagFoo)
	idBar := makeImage("busybox:latest", tagBar)

	deleteImages(repoName)

	// create the archive
	out, _, err := runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("tar", "t"),
		exec.Command("grep", "VERSION"),
		exec.Command("cut", "-d", "/", "-f1"))
	if err != nil {
		c.Fatalf("failed to save multiple images: %s, %v", out, err)
	}
	actual := strings.Split(strings.TrimSpace(out), "\n")

	// make the list of expected layers
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "history", "-q", "--no-trunc", "busybox:latest"))
	if err != nil {
		c.Fatalf("failed to get history: %s, %v", out, err)
	}

	expected := append(strings.Split(strings.TrimSpace(out), "\n"), idFoo, idBar)

	sort.Strings(actual)
	sort.Strings(expected)
	if !reflect.DeepEqual(expected, actual) {
		c.Fatalf("archive does not contains the right layers: got %v, expected %v", actual, expected)
	}

}

// Issue #6722 #5892 ensure directories are included in changes
func (s *DockerSuite) TestSaveDirectoryPermissions(c *check.C) {
	layerEntries := []string{"opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}
	layerEntriesAUFS := []string{"./", ".wh..wh.aufs", ".wh..wh.orph/", ".wh..wh.plnk/", "opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}

	name := "save-directory-permissions"
	tmpDir, err := ioutil.TempDir("", "save-layers-with-directories")
	if err != nil {
		c.Errorf("failed to create temporary directory: %s", err)
	}
	extractionDirectory := filepath.Join(tmpDir, "image-extraction-dir")
	os.Mkdir(extractionDirectory, 0777)

	defer os.RemoveAll(tmpDir)
	_, err = buildImage(name,
		`FROM busybox
	RUN adduser -D user && mkdir -p /opt/a/b && chown -R user:user /opt/a
	RUN touch /opt/a/b/c && chown user:user /opt/a/b/c`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	if out, _, err := runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", name),
		exec.Command("tar", "-xf", "-", "-C", extractionDirectory),
	); err != nil {
		c.Errorf("failed to save and extract image: %s", out)
	}

	dirs, err := ioutil.ReadDir(extractionDirectory)
	if err != nil {
		c.Errorf("failed to get a listing of the layer directories: %s", err)
	}

	found := false
	for _, entry := range dirs {
		var entriesSansDev []string
		if entry.IsDir() {
			layerPath := filepath.Join(extractionDirectory, entry.Name(), "layer.tar")

			f, err := os.Open(layerPath)
			if err != nil {
				c.Fatalf("failed to open %s: %s", layerPath, err)
			}

			entries, err := ListTar(f)
			for _, e := range entries {
				if !strings.Contains(e, "dev/") {
					entriesSansDev = append(entriesSansDev, e)
				}
			}
			if err != nil {
				c.Fatalf("encountered error while listing tar entries: %s", err)
			}

			if reflect.DeepEqual(entriesSansDev, layerEntries) || reflect.DeepEqual(entriesSansDev, layerEntriesAUFS) {
				found = true
				break
			}
		}
	}

	if !found {
		c.Fatalf("failed to find the layer with the right content listing")
	}

}
