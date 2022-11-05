package process

import (
	"os"

	"golang.org/x/sys/windows"
)

// Alive returns true if process with a given pid is running.
func Alive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	var c uint32
	err = windows.GetExitCodeProcess(h, &c)
	_ = windows.CloseHandle(h)
	if err != nil {
		// From the GetExitCodeProcess function (processthreadsapi.h) API docs:
		// https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getexitcodeprocess
		//
		// The GetExitCodeProcess function returns a valid error code defined by the
		// application only after the thread terminates. Therefore, an application should
		// not use STILL_ACTIVE (259) as an error code (STILL_ACTIVE is a macro for
		// STATUS_PENDING (minwinbase.h)). If a thread returns STILL_ACTIVE (259) as
		// an error code, then applications that test for that value could interpret it
		// to mean that the thread is still running, and continue to test for the
		// completion of the thread after the thread has terminated, which could put
		// the application into an infinite loop.
		return c == uint32(windows.STATUS_PENDING)
	}
	return true
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
