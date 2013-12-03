package docker

import (
	"fmt"
	"testing"
)

func TestCleanDockerfile(t *testing.T) {
	dockerfile := "from busybox\n  \n#comment with slash\\\nrun command \\\ncontinue command\n#comment line\n"
	cleanedFile := cleanDockerfile(dockerfile)
	if cleanedFile != "from busybox\nrun command continue command\n" {
		fmt.Println(cleanedFile)
		t.Fail()
	}
}
