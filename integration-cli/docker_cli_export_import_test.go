package main

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// export an image and try to import it into a new one
func TestExportContainerAndImportImage(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal("failed to create a container", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("output should've been a container id: %s %s ", cleanedContainerID, err)
	}

	exportCmdTemplate := `%v export %v > /tmp/testexp.tar`
	exportCmdFinal := fmt.Sprintf(exportCmdTemplate, dockerBinary, cleanedContainerID)
	exportCmd := exec.Command("bash", "-c", exportCmdFinal)
	out, _, err = runCommandWithOutput(exportCmd)
	errorOut(err, t, fmt.Sprintf("failed to export container: %v %v", out, err))

	importCmdFinal := `cat /tmp/testexp.tar | docker import - testexp`
	importCmd := exec.Command("bash", "-c", importCmdFinal)
	out, _, err = runCommandWithOutput(importCmd)
	errorOut(err, t, fmt.Sprintf("failed to import image: %v %v", out, err))

	cleanedImageID := stripTrailingCharacters(out)

	inspectCmd = exec.Command(dockerBinary, "inspect", cleanedImageID)
	out, _, err = runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("output should've been an image id: %v %v", out, err))

	deleteContainer(cleanedContainerID)
	deleteImages("testexp")

	os.Remove("/tmp/testexp.tar")

	logDone("export - export a container")
	logDone("import - import an image")
}
