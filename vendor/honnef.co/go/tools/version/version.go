package version

import (
	"fmt"
	"os"
	"path/filepath"
)

const Version = "devel"

func Print() {
	if Version == "devel" {
		fmt.Printf("%s (no version)\n", filepath.Base(os.Args[0]))
	} else {
		fmt.Printf("%s %s\n", filepath.Base(os.Args[0]), Version)
	}
}
