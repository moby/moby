//go:build go1.10
// +build go1.10

package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"

import (
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
	// with a concrete example. Consider what happens if a goroutine spawned
	// from Go() gets scheduled onto the startup thread. The thread's mount
	// namespace will be unshared and modified. The contents of the
	// /proc/[pid]/mountinfo file will then describe the mount tree of the
	// unshared namespace, not the namespace of any other thread. It will
	// remain this way until the process exits. (The startup thread is
	// special in another way: exiting it puts the process into a
	// "non-waitable zombie" state. To avoid this fate, the Go runtime parks
	// the thread instead of exiting if a goroutine returns while locked to
	// the startup thread. More information can be found in the Go runtime
	// sources: `go doc -u -src runtime.mexit`.)
	// The github.com/moby/sys/mountinfo package reads from
	// /proc/self/mountinfo, so will read the mount tree for the wrong
	// namespace if the startup thread has had its mount namespace unshared!
	// The /proc/thread-self/ magic symlink, introduced in Linux 3.17, is
	// one potential solution to this problem, but every package which opens
	// files in /proc/self/ would need to be updated, and fallbacks to
	// /proc/self/task/{{syscall.Gettid()}}/ would be required to support
	// older kernels. Overlooking any reference to /proc/self/ would
	// manifest as stochastically-reproducible bugs, so this is far from an
	// ideal solution.
	//
	// Reading from /proc/self/ would not be a problem if we can prevent the
	// per-thread state of the startup thread from being modified
	// nondeterministically in the first place. We can accomplish this
	// simply by locking the main() function to the startup thread! Doing so
	// excludes any other goroutine from being scheduled on the thread.
	runtime.LockOSThread()
}

// Go starts fn in a goroutine where the root directory, current working
// directory and umask are unshared from other goroutines and the root directory
// has been changed to path. These changes are only visible to the goroutine in
// which fn is executed. Any other goroutines, including ones started from fn,
// will see the same root directory and file system attributes as the rest of
// the process.
func Go(path string, fn func()) error {
	started := make(chan error)
	go func() {
		// Prepare to manipulate per-thread kernel state. Wire the
		// goroutine to the OS thread so execution of other goroutines
		// will not be scheduled on it. It is very important not to
		// unwire the goroutine from the thread so that the thread exits
		// with this goroutine and is not returned to the goroutine
		// thread pool.
		runtime.LockOSThread()

		// Under Linux, threads are implemented as processes which share
		// a virtual memory space. Therefore in a multithreaded process
		// unshare(2) disassociates parts of the calling thread's
		// context from the thread it was clone(2)'d from.
		if err := unix.Unshare(unix.CLONE_FS); err != nil {
			started <- err
			return
		}

		if err := chroot(path); err != nil {
			started <- err
			return
		}

		close(started)
		fn()
	}()
	return <-started
}
