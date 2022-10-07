//go:build go1.10
// +build go1.10

package unshare // import "github.com/docker/docker/internal/unshare"

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func init() {
	// The startup thread of a process is special in a few different ways.
	// Most pertinent to the discussion at hand, any per-thread kernel state
	// reflected in the /proc/[pid]/ directory for a process is taken from
	// the state of the startup thread. Same goes for /proc/self/; it shows
	// the state of the current process' startup thread, no matter which
	// thread the files are being opened from. For most programs this is a
	// distinction without a difference as the kernel state, such as the
	// mount namespace and current working directory, is shared among (and
	// kept synchronized across) all threads of a process. But things start
	// to break down once threads start unsharing and modifying parts of
	// their kernel state.
	//
	// The Go runtime schedules goroutines to execute on the startup thread,
	// same as any other. How this could be problematic is best illustrated
	// with a concrete example. Consider what happens if a call to
	// Go(unix.CLONE_NEWNS, ...) spawned a goroutine which gets scheduled
	// onto the startup thread. The thread's mount namespace will be
	// unshared and modified. The contents of the /proc/[pid]/mountinfo file
	// will then describe the mount tree of the unshared namespace, not the
	// namespace of any other thread. It will remain this way until the
	// process exits. (The startup thread is special in another way: exiting
	// it puts the process into a "non-waitable zombie" state. To avoid this
	// fate, the Go runtime parks the thread instead of exiting if a
	// goroutine returns while locked to the startup thread. More
	// information can be found in the Go runtime sources:
	// `go doc -u -src runtime.mexit`.) The github.com/moby/sys/mountinfo
	// package reads from /proc/self/mountinfo, so will read the mount tree
	// for the wrong namespace if the startup thread has had its mount
	// namespace unshared! The /proc/thread-self/ directory, introduced in
	// Linux 3.17, is one potential solution to this problem, but every
	// package which opens files in /proc/self/ would need to be updated,
	// and fallbacks to /proc/self/task/[tid]/ would be required to support
	// older kernels. Overlooking any reference to /proc/self/ would
	// manifest as stochastically-reproducible bugs, so this is far from an
	// ideal solution.
	//
	// Reading from /proc/self/ would not be a problem if we could prevent
	// the per-thread state of the startup thread from being modified
	// nondeterministically in the first place. We can accomplish this
	// simply by locking the main() function to the startup thread! Doing so
	// excludes any other goroutine from being scheduled on the thread.
	runtime.LockOSThread()
}

// reversibleSetnsFlags maps the unshare(2) flags whose effects can be fully
// reversed using setns(2). The values are the basenames of the corresponding
// /proc/self/task/[tid]/ns/ magic symlinks to use to save and restore the
// state.
var reversibleSetnsFlags = map[int]string{
	unix.CLONE_NEWCGROUP: "cgroup",
	unix.CLONE_NEWNET:    "net",
	unix.CLONE_NEWUTS:    "uts",
	unix.CLONE_NEWPID:    "pid",
	unix.CLONE_NEWTIME:   "time",

	// The following CLONE_NEW* flags are not included because they imply
	// another, irreversible flag when used with unshare(2).
	//  - unix.CLONE_NEWIPC:  implies CLONE_SYSVMEM
	//  - unix.CLONE_NEWNS:   implies CLONE_FS
	//  - unix.CLONE_NEWUSER: implies CLONE_FS since Linux 3.9
}

// Go calls the given functions in a new goroutine, locked to an OS thread,
// which has had the parts of its execution state disassociated from the rest of
// the current process using [unshare(2)]. It blocks until the new goroutine has
// started and setupfn has returned. fn is only called if setupfn returns nil. A
// nil setupfn or fn is equivalent to passing a no-op function.
//
// The disassociated execution state and any changes made to it are only visible
// to the goroutine which the functions are called in. Any other goroutines,
// including ones started from the function, will see the same execution state
// as the rest of the process.
//
// The acceptable flags are documented in the [unshare(2)] Linux man-page.
// The corresponding CLONE_* constants are defined in package [unix].
//
// # Warning
//
// This function may terminate the thread which the new goroutine executed on
// after fn returns, which could cause subprocesses started with the
// [syscall.SysProcAttr] Pdeathsig field set to be signaled before process
// termination. Any subprocess started before this function is called may be
// affected, in addition to any subprocesses started inside setupfn or fn.
// There are more details at https://go.dev/issue/27505.
//
// [unshare(2)]: https://man7.org/linux/man-pages/man2/unshare.2.html
func Go(flags int, setupfn func() error, fn func()) error {
	started := make(chan error)

	maskedFlags := flags
	for f := range reversibleSetnsFlags {
		maskedFlags &^= f
	}
	isReversible := maskedFlags == 0

	go func() {
		// Prepare to manipulate per-thread kernel state.
		runtime.LockOSThread()

		// Not all changes to the execution state can be reverted.
		// If an irreversible change to the execution state is made, our
		// only recourse is to have the tampered thread terminated by
		// returning from this function while the goroutine remains
		// wired to the thread. The Go runtime will terminate the thread
		// and replace it with a fresh one as needed.

		if isReversible {
			defer func() {
				if isReversible {
					// All execution state has been restored without error.
					// The thread is once again fungible.
					runtime.UnlockOSThread()
				}
			}()
			tid := unix.Gettid()
			for f, ns := range reversibleSetnsFlags {
				if flags&f != f {
					continue
				}
				// The /proc/thread-self directory was added in Linux 3.17.
				// We are not using it to maximize compatibility.
				pth := fmt.Sprintf("/proc/self/task/%d/ns/%s", tid, ns)
				fd, err := unix.Open(pth, unix.O_RDONLY|unix.O_CLOEXEC, 0)
				if err != nil {
					started <- &os.PathError{Op: "open", Path: pth, Err: err}
					return
				}
				defer func() {
					if isReversible {
						if err := unix.Setns(fd, 0); err != nil {
							isReversible = false
						}
					}
					_ = unix.Close(fd)
				}()
			}
		}

		// Threads are implemented under Linux as processes which share
		// a virtual memory space. Therefore in a multithreaded process
		// unshare(2) disassociates parts of the calling thread's
		// context from the thread it was clone(2)'d from.
		if err := unix.Unshare(flags); err != nil {
			started <- os.NewSyscallError("unshare", err)
			return
		}

		if setupfn != nil {
			if err := setupfn(); err != nil {
				started <- err
				return
			}
		}
		close(started)

		if fn != nil {
			fn()
		}
	}()

	return <-started
}
