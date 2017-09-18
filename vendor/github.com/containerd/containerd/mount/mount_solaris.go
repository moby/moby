package mount

// On Solaris we can't invoke the mount system call directly.  First,
// the mount system call takes more than 6 arguments, and go doesn't
// support invoking system calls that take more than 6 arguments.  Past
// that, the mount system call is a private interfaces.  For example,
// the arguments and data structures passed to the kernel to create an
// nfs mount are private and can change at any time.  The only public
// and stable interface for creating mounts on Solaris is the mount.8
// command, so we'll invoke that here.

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	mountCmd = "/usr/sbin/mount"
)

func doMount(arg ...string) error {
	cmd := exec.Command(mountCmd, arg...)

	/* Setup Stdin, Stdout, and Stderr */
	stderr := new(bytes.Buffer)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = stderr

	/*
	 * Run the command.  If the command fails create a new error
	 * object to return that includes stderr output.
	 */
	err := cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return errors.New(fmt.Sprintf("%v: %s", err, stderr.String()))
	}
	return nil
}

func (m *Mount) Mount(target string) error {
	var err error

	if len(m.Options) == 0 {
		err = doMount("-F", m.Type, m.Source, target)
	} else {
		err = doMount("-F", m.Type, "-o", strings.Join(m.Options, ","),
			m.Source, target)
	}
	return err
}

func Unmount(mount string, flags int) error {
	return unix.Unmount(mount, flags)
}

// UnmountAll repeatedly unmounts the given mount point until there
// are no mounts remaining (EINVAL is returned by mount), which is
// useful for undoing a stack of mounts on the same mount point.
func UnmountAll(mount string, flags int) error {
	for {
		if err := Unmount(mount, flags); err != nil {
			// EINVAL is returned if the target is not a
			// mount point, indicating that we are
			// done. It can also indicate a few other
			// things (such as invalid flags) which we
			// unfortunately end up squelching here too.
			if err == unix.EINVAL {
				return nil
			}
			return err
		}
	}
}
