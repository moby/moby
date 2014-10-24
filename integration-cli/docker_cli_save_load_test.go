package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// save a repo and try to load it using stdout
func TestSaveAndLoadRepoStdout(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to create a container: %v %v", out, err))

	cleanedContainerID := stripTrailingCharacters(out)

	repoName := "foobar-save-load-test"

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("output should've been a container id: %v %v", cleanedContainerID, err))

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID, repoName)
	out, _, err = runCommandWithOutput(commitCmd)
	errorOut(err, t, fmt.Sprintf("failed to commit container: %v %v", out, err))

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	before, _, err := runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("the repo should exist before saving it: %v %v", before, err))

	saveCmdTemplate := `%v save %v > /tmp/foobar-save-load-test.tar`
	saveCmdFinal := fmt.Sprintf(saveCmdTemplate, dockerBinary, repoName)
	saveCmd := exec.Command("bash", "-c", saveCmdFinal)
	out, _, err = runCommandWithOutput(saveCmd)
	errorOut(err, t, fmt.Sprintf("failed to save repo: %v %v", out, err))

	deleteImages(repoName)

	loadCmdFinal := `cat /tmp/foobar-save-load-test.tar | docker load`
	loadCmd := exec.Command("bash", "-c", loadCmdFinal)
	out, _, err = runCommandWithOutput(loadCmd)
	errorOut(err, t, fmt.Sprintf("failed to load repo: %v %v", out, err))

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	after, _, err := runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("the repo should exist after loading it: %v %v", after, err))

	if before != after {
		t.Fatalf("inspect is not the same after a save / load")
	}

	deleteContainer(cleanedContainerID)
	deleteImages(repoName)

	os.Remove("/tmp/foobar-save-load-test.tar")

	logDone("save - save a repo using stdout")
	logDone("load - load a repo using stdout")
}

func TestSaveSingleTag(t *testing.T) {
	repoName := "foobar-save-single-tag-test"

	tagCmdFinal := fmt.Sprintf("%v tag busybox:latest %v:latest", dockerBinary, repoName)
	tagCmd := exec.Command("bash", "-c", tagCmdFinal)
	out, _, err := runCommandWithOutput(tagCmd)
	errorOut(err, t, fmt.Sprintf("failed to tag repo: %v %v", out, err))

	idCmdFinal := fmt.Sprintf("%v images -q --no-trunc %v", dockerBinary, repoName)
	idCmd := exec.Command("bash", "-c", idCmdFinal)
	out, _, err = runCommandWithOutput(idCmd)
	errorOut(err, t, fmt.Sprintf("failed to get repo ID: %v %v", out, err))

	cleanedImageID := stripTrailingCharacters(out)

	saveCmdFinal := fmt.Sprintf("%v save %v:latest | tar t | grep -E '(^repositories$|%v)'", dockerBinary, repoName, cleanedImageID)
	saveCmd := exec.Command("bash", "-c", saveCmdFinal)
	out, _, err = runCommandWithOutput(saveCmd)
	errorOut(err, t, fmt.Sprintf("failed to save repo with image ID and 'repositories' file: %v %v", out, err))

	deleteImages(repoName)

	logDone("save - save a specific image:tag")
}

func TestSaveImageId(t *testing.T) {
	repoName := "foobar-save-image-id-test"

	tagCmdFinal := fmt.Sprintf("%v tag scratch:latest %v:latest", dockerBinary, repoName)
	tagCmd := exec.Command("bash", "-c", tagCmdFinal)
	out, _, err := runCommandWithOutput(tagCmd)
	errorOut(err, t, fmt.Sprintf("failed to tag repo: %v %v", out, err))

	idLongCmdFinal := fmt.Sprintf("%v images -q --no-trunc %v", dockerBinary, repoName)
	idLongCmd := exec.Command("bash", "-c", idLongCmdFinal)
	out, _, err = runCommandWithOutput(idLongCmd)
	errorOut(err, t, fmt.Sprintf("failed to get repo ID: %v %v", out, err))

	cleanedLongImageID := stripTrailingCharacters(out)

	idShortCmdFinal := fmt.Sprintf("%v images -q %v", dockerBinary, repoName)
	idShortCmd := exec.Command("bash", "-c", idShortCmdFinal)
	out, _, err = runCommandWithOutput(idShortCmd)
	errorOut(err, t, fmt.Sprintf("failed to get repo short ID: %v %v", out, err))

	cleanedShortImageID := stripTrailingCharacters(out)

	saveCmdFinal := fmt.Sprintf("%v save %v | tar t | grep %v", dockerBinary, cleanedShortImageID, cleanedLongImageID)
	saveCmd := exec.Command("bash", "-c", saveCmdFinal)
	out, _, err = runCommandWithOutput(saveCmd)
	errorOut(err, t, fmt.Sprintf("failed to save repo with image ID: %v %v", out, err))

	deleteImages(repoName)

	logDone("save - save a image by ID")
}

// save a repo and try to load it using flags
func TestSaveAndLoadRepoFlags(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to create a container: %v %v", out, err))

	cleanedContainerID := stripTrailingCharacters(out)

	repoName := "foobar-save-load-test"

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("output should've been a container id: %v %v", cleanedContainerID, err))

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID, repoName)
	out, _, err = runCommandWithOutput(commitCmd)
	errorOut(err, t, fmt.Sprintf("failed to commit container: %v %v", out, err))

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	before, _, err := runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("the repo should exist before saving it: %v %v", before, err))

	saveCmdTemplate := `%v save -o /tmp/foobar-save-load-test.tar %v`
	saveCmdFinal := fmt.Sprintf(saveCmdTemplate, dockerBinary, repoName)
	saveCmd := exec.Command("bash", "-c", saveCmdFinal)
	out, _, err = runCommandWithOutput(saveCmd)
	errorOut(err, t, fmt.Sprintf("failed to save repo: %v %v", out, err))

	deleteImages(repoName)

	loadCmdFinal := `docker load -i /tmp/foobar-save-load-test.tar`
	loadCmd := exec.Command("bash", "-c", loadCmdFinal)
	out, _, err = runCommandWithOutput(loadCmd)
	errorOut(err, t, fmt.Sprintf("failed to load repo: %v %v", out, err))

	inspectCmd = exec.Command(dockerBinary, "inspect", repoName)
	after, _, err := runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("the repo should exist after loading it: %v %v", after, err))

	if before != after {
		t.Fatalf("inspect is not the same after a save / load")
	}

	deleteContainer(cleanedContainerID)
	deleteImages(repoName)

	os.Remove("/tmp/foobar-save-load-test.tar")

	logDone("save - save a repo using -o")
	logDone("load - load a repo using -i")
}

func TestSaveMultipleNames(t *testing.T) {
	repoName := "foobar-save-multi-name-test"

	// Make one image
	tagCmdFinal := fmt.Sprintf("%v tag scratch:latest %v-one:latest", dockerBinary, repoName)
	tagCmd := exec.Command("bash", "-c", tagCmdFinal)
	out, _, err := runCommandWithOutput(tagCmd)
	errorOut(err, t, fmt.Sprintf("failed to tag repo: %v %v", out, err))
	// Make two images
	tagCmdFinal = fmt.Sprintf("%v tag scratch:latest %v-two:latest", dockerBinary, repoName)
	tagCmd = exec.Command("bash", "-c", tagCmdFinal)
	out, _, err = runCommandWithOutput(tagCmd)
	errorOut(err, t, fmt.Sprintf("failed to tag repo: %v %v", out, err))

	saveCmdFinal := fmt.Sprintf("%v save %v-one %v-two:latest | tar xO repositories | grep -q -E '(-one|-two)'", dockerBinary, repoName, repoName)
	saveCmd := exec.Command("bash", "-c", saveCmdFinal)
	out, _, err = runCommandWithOutput(saveCmd)
	errorOut(err, t, fmt.Sprintf("failed to save multiple repos: %v %v", out, err))

	deleteImages(repoName)

	logDone("save - save by multiple names")
}

// Issue #6722 #5892 ensure directories are included in changes
func TestSaveDirectoryPermissions(t *testing.T) {
	layerEntries := []string{"opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}
	layerEntriesAUFS := []string{"./", ".wh..wh.aufs", ".wh..wh.orph/", ".wh..wh.plnk/", "opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}

	name := "save-directory-permissions"
	tmpDir, err := ioutil.TempDir("", "save-layers-with-directories")
	extractionDirectory := filepath.Join(tmpDir, "image-extraction-dir")
	os.Mkdir(extractionDirectory, 0777)

	if err != nil {
		t.Errorf("failed to create temporary directory: %s", err)
	}
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

	saveCmdFinal := fmt.Sprintf("%s save %s | tar -xf - -C %s", dockerBinary, name, extractionDirectory)
	saveCmd := exec.Command("bash", "-c", saveCmdFinal)
	out, _, err := runCommandWithOutput(saveCmd)
	if err != nil {
		t.Errorf("failed to save and extract image: %s", out)
	}

	dirs, err := ioutil.ReadDir(extractionDirectory)
	if err != nil {
		t.Errorf("failed to get a listing of the layer directories: %s", err)
	}

	found := false
	for _, entry := range dirs {
		if entry.IsDir() {
			layerPath := filepath.Join(extractionDirectory, entry.Name(), "layer.tar")

			f, err := os.Open(layerPath)
			if err != nil {
				t.Fatalf("failed to open %s: %s", layerPath, err)
			}

			entries, err := ListTar(f)
			if err != nil {
				t.Fatalf("encountered error while listing tar entries: %s", err)
			}

			if reflect.DeepEqual(entries, layerEntries) || reflect.DeepEqual(entries, layerEntriesAUFS) {
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
