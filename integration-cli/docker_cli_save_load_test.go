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
	"testing"
)

// save a repo using gz compression and try to load it using stdout
func TestSaveXzAndLoadRepoStdout(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to create a container: %v %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	repoName := "foobar-save-load-test-xz-gz"

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("output should've been a container id: %v %v", cleanedContainerID, err)
	}

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID, repoName)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		t.Fatalf("failed to commit container: %v %v", out, err)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	before, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("the repo should exist before saving it: %v %v", before, err)
	}

	repoTarball, _, err := runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("xz", "-c"),
		exec.Command("gzip", "-c"))
	if err != nil {
		t.Fatalf("failed to save repo: %v %v", out, err)
	}
	deleteImages(repoName)

	loadCmd := exec.Command(dockerBinary, "load")
	loadCmd.Stdin = strings.NewReader(repoTarball)
	out, _, err = runCommandWithOutput(loadCmd)
	if err == nil {
		t.Fatalf("expected error, but succeeded with no error and output: %v", out)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	after, _, err := runCommandWithOutput(inspectCmd)
	if err == nil {
		t.Fatalf("the repo should not exist: %v", after)
	}

	deleteImages(repoName)

	logDone("load - save a repo with xz compression & load it using stdout")
}

// save a repo using xz+gz compression and try to load it using stdout
func TestSaveXzGzAndLoadRepoStdout(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to create a container: %v %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	repoName := "foobar-save-load-test-xz-gz"

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("output should've been a container id: %v %v", cleanedContainerID, err)
	}

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID, repoName)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		t.Fatalf("failed to commit container: %v %v", out, err)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	before, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("the repo should exist before saving it: %v %v", before, err)
	}

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("xz", "-c"),
		exec.Command("gzip", "-c"))
	if err != nil {
		t.Fatalf("failed to save repo: %v %v", out, err)
	}

	deleteImages(repoName)

	loadCmd := exec.Command(dockerBinary, "load")
	loadCmd.Stdin = strings.NewReader(out)
	out, _, err = runCommandWithOutput(loadCmd)
	if err == nil {
		t.Fatalf("expected error, but succeeded with no error and output: %v", out)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	after, _, err := runCommandWithOutput(inspectCmd)
	if err == nil {
		t.Fatalf("the repo should not exist: %v", after)
	}

	deleteContainer(cleanedContainerID)
	deleteImages(repoName)

	logDone("load - save a repo with xz+gz compression & load it using stdout")
}

func TestSaveSingleTag(t *testing.T) {
	repoName := "foobar-save-single-tag-test"

	tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", fmt.Sprintf("%v:latest", repoName))
	defer deleteImages(repoName)
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		t.Fatalf("failed to tag repo: %s, %v", out, err)
	}

	idCmd := exec.Command(dockerBinary, "images", "-q", "--no-trunc", repoName)
	out, _, err := runCommandWithOutput(idCmd)
	if err != nil {
		t.Fatalf("failed to get repo ID: %s, %v", out, err)
	}
	cleanedImageID := stripTrailingCharacters(out)

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", fmt.Sprintf("%v:latest", repoName)),
		exec.Command("tar", "t"),
		exec.Command("grep", "-E", fmt.Sprintf("(^repositories$|%v)", cleanedImageID)))
	if err != nil {
		t.Fatalf("failed to save repo with image ID and 'repositories' file: %s, %v", out, err)
	}

	logDone("save - save a specific image:tag")
}

func TestSaveImageId(t *testing.T) {
	repoName := "foobar-save-image-id-test"

	tagCmd := exec.Command(dockerBinary, "tag", "emptyfs:latest", fmt.Sprintf("%v:latest", repoName))
	defer deleteImages(repoName)
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		t.Fatalf("failed to tag repo: %s, %v", out, err)
	}

	idLongCmd := exec.Command(dockerBinary, "images", "-q", "--no-trunc", repoName)
	out, _, err := runCommandWithOutput(idLongCmd)
	if err != nil {
		t.Fatalf("failed to get repo ID: %s, %v", out, err)
	}

	cleanedLongImageID := stripTrailingCharacters(out)

	idShortCmd := exec.Command(dockerBinary, "images", "-q", repoName)
	out, _, err = runCommandWithOutput(idShortCmd)
	if err != nil {
		t.Fatalf("failed to get repo short ID: %s, %v", out, err)
	}

	cleanedShortImageID := stripTrailingCharacters(out)

	saveCmd := exec.Command(dockerBinary, "save", cleanedShortImageID)
	tarCmd := exec.Command("tar", "t")
	tarCmd.Stdin, err = saveCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("cannot set stdout pipe for tar: %v", err)
	}
	grepCmd := exec.Command("grep", cleanedLongImageID)
	grepCmd.Stdin, err = tarCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("cannot set stdout pipe for grep: %v", err)
	}

	if err = tarCmd.Start(); err != nil {
		t.Fatalf("tar failed with error: %v", err)
	}
	if err = saveCmd.Start(); err != nil {
		t.Fatalf("docker save failed with error: %v", err)
	}
	defer saveCmd.Wait()
	defer tarCmd.Wait()

	out, _, err = runCommandWithOutput(grepCmd)

	if err != nil {
		t.Fatalf("failed to save repo with image ID: %s, %v", out, err)
	}

	logDone("save - save a image by ID")
}

// save a repo and try to load it using flags
func TestSaveAndLoadRepoFlags(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to create a container: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteContainer(cleanedContainerID)

	repoName := "foobar-save-load-test"

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	if out, _, err = runCommandWithOutput(inspectCmd); err != nil {
		t.Fatalf("output should've been a container id: %s, %v", out, err)
	}

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID, repoName)
	deleteImages(repoName)
	if out, _, err = runCommandWithOutput(commitCmd); err != nil {
		t.Fatalf("failed to commit container: %s, %v", out, err)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	before, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("the repo should exist before saving it: %s, %v", before, err)

	}

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command(dockerBinary, "load"))
	if err != nil {
		t.Fatalf("failed to save and load repo: %s, %v", out, err)
	}

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	after, _, err := runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("the repo should exist after loading it: %s, %v", after, err)
	}

	if before != after {
		t.Fatalf("inspect is not the same after a save / load")
	}

	logDone("save - save a repo using -o && load a repo using -i")
}

func TestSaveMultipleNames(t *testing.T) {
	repoName := "foobar-save-multi-name-test"

	// Make one image
	tagCmd := exec.Command(dockerBinary, "tag", "emptyfs:latest", fmt.Sprintf("%v-one:latest", repoName))
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		t.Fatalf("failed to tag repo: %s, %v", out, err)
	}
	defer deleteImages(repoName + "-one")

	// Make two images
	tagCmd = exec.Command(dockerBinary, "tag", "emptyfs:latest", fmt.Sprintf("%v-two:latest", repoName))
	out, _, err := runCommandWithOutput(tagCmd)
	if err != nil {
		t.Fatalf("failed to tag repo: %s, %v", out, err)
	}
	defer deleteImages(repoName + "-two")

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", fmt.Sprintf("%v-one", repoName), fmt.Sprintf("%v-two:latest", repoName)),
		exec.Command("tar", "xO", "repositories"),
		exec.Command("grep", "-q", "-E", "(-one|-two)"),
	)
	if err != nil {
		t.Fatalf("failed to save multiple repos: %s, %v", out, err)
	}

	logDone("save - save by multiple names")
}

func TestSaveRepoWithMultipleImages(t *testing.T) {

	makeImage := func(from string, tag string) string {
		runCmd := exec.Command(dockerBinary, "run", "-d", from, "true")
		var (
			out string
			err error
		)
		if out, _, err = runCommandWithOutput(runCmd); err != nil {
			t.Fatalf("failed to create a container: %v %v", out, err)
		}
		cleanedContainerID := stripTrailingCharacters(out)
		defer deleteContainer(cleanedContainerID)

		commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID, tag)
		if out, _, err = runCommandWithOutput(commitCmd); err != nil {
			t.Fatalf("failed to commit container: %v %v", out, err)
		}
		imageID := stripTrailingCharacters(out)
		return imageID
	}

	repoName := "foobar-save-multi-images-test"
	tagFoo := repoName + ":foo"
	tagBar := repoName + ":bar"

	idFoo := makeImage("busybox:latest", tagFoo)
	defer deleteImages(idFoo)
	idBar := makeImage("busybox:latest", tagBar)
	defer deleteImages(idBar)

	deleteImages(repoName)

	// create the archive
	out, _, err := runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("tar", "t"),
		exec.Command("grep", "VERSION"),
		exec.Command("cut", "-d", "/", "-f1"))
	if err != nil {
		t.Fatalf("failed to save multiple images: %s, %v", out, err)
	}
	actual := strings.Split(stripTrailingCharacters(out), "\n")

	// make the list of expected layers
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "history", "-q", "--no-trunc", "busybox:latest"))
	if err != nil {
		t.Fatalf("failed to get history: %s, %v", out, err)
	}

	expected := append(strings.Split(stripTrailingCharacters(out), "\n"), idFoo, idBar)

	sort.Strings(actual)
	sort.Strings(expected)
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("achive does not contains the right layers: got %v, expected %v", actual, expected)
	}

	logDone("save - save repository with multiple images")
}

// Issue #6722 #5892 ensure directories are included in changes
func TestSaveDirectoryPermissions(t *testing.T) {
	layerEntries := []string{"opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}
	layerEntriesAUFS := []string{"./", ".wh..wh.aufs", ".wh..wh.orph/", ".wh..wh.plnk/", "opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}

	name := "save-directory-permissions"
	tmpDir, err := ioutil.TempDir("", "save-layers-with-directories")
	if err != nil {
		t.Errorf("failed to create temporary directory: %s", err)
	}
	extractionDirectory := filepath.Join(tmpDir, "image-extraction-dir")
	os.Mkdir(extractionDirectory, 0777)

	defer os.RemoveAll(tmpDir)
	defer deleteImages(name)
	_, err = buildImage(name,
		`FROM busybox
	RUN adduser -D user && mkdir -p /opt/a/b && chown -R user:user /opt/a
	RUN touch /opt/a/b/c && chown user:user /opt/a/b/c`,
		true)
	if err != nil {
		t.Fatal(err)
	}

	if out, _, err := runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", name),
		exec.Command("tar", "-xf", "-", "-C", extractionDirectory),
	); err != nil {
		t.Errorf("failed to save and extract image: %s", out)
	}

	dirs, err := ioutil.ReadDir(extractionDirectory)
	if err != nil {
		t.Errorf("failed to get a listing of the layer directories: %s", err)
	}

	found := false
	for _, entry := range dirs {
		var entriesSansDev []string
		if entry.IsDir() {
			layerPath := filepath.Join(extractionDirectory, entry.Name(), "layer.tar")

			f, err := os.Open(layerPath)
			if err != nil {
				t.Fatalf("failed to open %s: %s", layerPath, err)
			}

			entries, err := ListTar(f)
			for _, e := range entries {
				if !strings.Contains(e, "dev/") {
					entriesSansDev = append(entriesSansDev, e)
				}
			}
			if err != nil {
				t.Fatalf("encountered error while listing tar entries: %s", err)
			}

			if reflect.DeepEqual(entriesSansDev, layerEntries) || reflect.DeepEqual(entriesSansDev, layerEntriesAUFS) {
				found = true
				break
			}
		}
	}

	if !found {
		t.Fatalf("failed to find the layer with the right content listing")
	}

	logDone("save - ensure directories exist in exported layers")
}
