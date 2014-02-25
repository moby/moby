package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestImagesEnsureImageIsListed(t *testing.T) {
	imagesCmd := exec.Command(dockerBinary, "images")
	out, _, err := runCommandWithOutput(imagesCmd)
	errorOut(err, t, fmt.Sprintf("listing images failed with errors: %v", err))

	if !strings.Contains(out, "busybox") {
		t.Fatal("images should've listed busybox")
	}

	logDone("images - busybox should be listed")
}
