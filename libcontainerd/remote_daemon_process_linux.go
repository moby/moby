package libcontainerd

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var fdNames = map[int]string{
	unix.Stdin:  "stdin",
	unix.Stdout: "stdout",
	unix.Stderr: "stderr",
}

func (p *process) pipeName(index int) string {
	return filepath.Join(p.root, p.id+"-"+fdNames[index])
}

func (p *process) IOPaths() (string, string, string) {
	var (
		stdin  = p.pipeName(unix.Stdin)
		stdout = p.pipeName(unix.Stdout)
		stderr = p.pipeName(unix.Stderr)
	)
	// TODO: debug why we're having zombies when I don't unset those
	if p.io.Stdin == nil {
		stdin = ""
	}
	if p.io.Stderr == nil {
		stderr = ""
	}
	return stdin, stdout, stderr
}

func (p *process) Cleanup() error {
	var retErr error

	// Ensure everything was closed
	p.CloseIO()

	for _, i := range [3]string{
		p.pipeName(unix.Stdin),
		p.pipeName(unix.Stdout),
		p.pipeName(unix.Stderr),
	} {
		err := os.Remove(i)
		if err != nil {
			if retErr == nil {
				retErr = errors.Wrapf(err, "failed to remove %s", i)
			} else {
				retErr = errors.Wrapf(retErr, "failed to remove %s", i)
			}
		}
	}

	return retErr
}
