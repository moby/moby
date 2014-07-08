package main

import (
	"fmt"
	"os"
	"os/exec"
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
