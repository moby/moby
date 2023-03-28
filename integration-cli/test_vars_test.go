package main

// sleepCommandForDaemonPlatform is a helper function that determines what
// the command is for a sleeping container based on the daemon platform.
// The Windows busybox image does not have a `top` command.
func sleepCommandForDaemonPlatform() []string {
	if testEnv.OSType == "windows" {
		return []string{"sleep", "600"}
	}
	return []string{"top"}
}
