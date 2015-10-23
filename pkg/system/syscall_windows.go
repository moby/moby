package system

// UnmountWithSyscall is a platform-specific helper function to call
// the unmount syscall. Not supported on Windows
func UnmountWithSyscall(dest string) {
}
