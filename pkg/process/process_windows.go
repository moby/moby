package process

import "os"

// Alive returns true if process with a given pid is running.
func Alive(pid int) bool {
	_, err := os.FindProcess(pid)

	return err == nil
}

// Kill force-stops a process.
func Kill(pid int) error {
	p, err := os.FindProcess(pid)
	if err == nil {
		err = p.Kill()
		if err != nil && err != os.ErrProcessDone {
			return err
		}
	}
	return nil
}

// Zombie is not supported on Windows.
//
// TODO(thaJeztah): remove once we remove the stubs from pkg/system.
func Zombie(_ int) (bool, error) {
	return false, nil
}
