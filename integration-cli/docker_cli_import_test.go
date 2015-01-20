package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestImportDisplay(t *testing.T) {
	server, err := fileServer(map[string]string{
		"/cirros.tar.gz": "/cirros.tar.gz",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	fileURL := fmt.Sprintf("%s/cirros.tar.gz", server.URL)
	importCmd := exec.Command(dockerBinary, "import", fileURL, "cirros")
	out, _, err := runCommandWithOutput(importCmd)
	if err != nil {
		t.Errorf("import failed with errors: %v, output: %q", err, out)
	}

	if n := strings.Count(out, "\n"); n != 2 {
		t.Fatalf("display is messed up: %d '\\n' instead of 2", n)
	}

	deleteImages("cirros")

	logDone("import - cirros was imported and display is fine")
}
