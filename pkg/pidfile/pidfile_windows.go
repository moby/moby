package pidfile

import "syscall"

const (
	processQueryLimitedInformation = 0x1000

	stillActive = 259
)

func processExists(pid int) bool {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	var c uint32
	err = syscall.GetExitCodeProcess(h, &c)
	syscall.Close(h)
	if err != nil {
		return c == stillActive
	}
	return true
}
