package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestImportDisplay(t *testing.T) {
	importCmd := exec.Command(dockerBinary, "import", "https://github.com/ewindisch/docker-cirros/raw/master/cirros-0.3.0-x86_64-lxc.tar.gz")
	out, _, err := runCommandWithOutput(importCmd)
	errorOut(err, t, fmt.Sprintf("import failed with errors: %v", err))

	if n := strings.Count(out, "\n"); n != 2 {
		t.Fatalf("display is messed up: %d '\\n' instead of 2", n)
	}

	logDone("import - cirros was imported and display is fine")
}
