package fileutils // import "github.com/docker/docker/pkg/fileutils"

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
)

// GetTotalUsedFds returns the number of used File Descriptors by executing
// "lsof -lnP -Ff -p PID".
//
// It uses the "-F" option to only print file-descriptors (f), and the "-l",
// "-n", and "-P" options to omit looking up user-names, host-names, and port-
// names. See [LSOF(8)].
//
// [LSOF(8)]: https://opensource.apple.com/source/lsof/lsof-49/lsof/lsof.man.auto.html
func GetTotalUsedFds() int {
	output, err := exec.Command("lsof", "-lnP", "-Ff", "-p", strconv.Itoa(os.Getpid())).CombinedOutput()
	if err != nil {
		return -1
	}

	return bytes.Count(output, []byte("\nf")) // Count number of file descriptor fields in output.
}
