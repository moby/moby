package process

import "fmt"

// Alive returns true if process with a given pid is running.
//
// It only considers positive PIDs; 0 (all processes in the current process
// group), -1 (all processes with a PID larger than 1), and negative (-n,
// all processes in process group "n") values for pid are never considered
// to be alive.
func Alive(pid int) bool {
	if pid < 1 {
		return false
	}
	return alive(pid)
}

// Kill force-stops a process. It only allows positive PIDs; 0 (all processes
// in the current process group), -1 (all processes with a PID larger than 1),
// and negative (-n, all processes in process group "n") values for pid producs
// an error. Refer to [KILL(2)] for details.
//
// [KILL(2)]: https://man7.org/linux/man-pages/man2/kill.2.html
func Kill(pid int) error {
	if pid < 1 {
		return fmt.Errorf("invalid PID (%d): only positive PIDs are allowed", pid)
	}
	return kill(pid)
}
