package utils

// IsProcessAlive returns true if process with a given pid is running.
func IsProcessAlive(pid int) bool {
	// TODO Windows containerd. Not sure this is needed
	//	p, err := os.FindProcess(pid)
	//	if err == nil {
	//		return true
	//	}
	return false
}

// KillProcess force-stops a process.
func KillProcess(pid int) {
	// TODO Windows containerd. Not sure this is needed
	//	p, err := os.FindProcess(pid)
	//	if err == nil {
	//		p.Kill()
	//	}
}
