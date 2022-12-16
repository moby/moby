package sysinfo

// ReadMemInfo retrieves memory statistics of the host system and returns a
// Memory type. It is only supported on Linux and Windows, and returns an
// error on other platforms.
func ReadMemInfo() (*Memory, error) {
	return readMemInfo()
}

// Memory contains memory statistics of the host system.
type Memory struct {
	// Total usable RAM (i.e. physical RAM minus a few reserved bits and the
	// kernel binary code).
	MemTotal int64

	// Amount of free memory.
	MemFree int64

	// Total amount of swap space available.
	SwapTotal int64

	// Amount of swap space that is currently unused.
	SwapFree int64
}
