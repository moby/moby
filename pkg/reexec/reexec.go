// reexec is a simple pkg that will allow you to sepecify a binary path
// and manually set the arv[0] to what you need.
package reexec

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Return return current binaries absolute path
func SelfPath() string {
	path, err := exec.LookPath(os.Args[0])
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		if execErr, ok := err.(*exec.Error); ok && os.IsNotExist(execErr.Err) {
			return ""
		}
		panic(err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		panic(err)
	}
	return path
}

func Command(argv0 string, args ...string) *exec.Cmd {
	return &exec.Cmd{
		Path: SelfPath(),
		Args: append([]string{argv0}, args...),
	}
}
