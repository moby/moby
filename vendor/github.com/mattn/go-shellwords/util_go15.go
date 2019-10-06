// +build !go1.6

package shellwords

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func shellRun(line, dir string) (string, error) {
	var b []byte
	var err error
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command(os.Getenv("COMSPEC"), "/c", line)
	} else {
		cmd = exec.Command(os.Getenv("SHELL"), "-c", line)
	}
	if dir != "" {
		cmd.Dir = dir
	}
	b, err = cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
