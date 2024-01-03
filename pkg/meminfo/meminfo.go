// Package meminfo provides utilites to retrieve memory statistics of
// the host system.
package meminfo

// Read retrieves memory statistics of the host system and returns a
// Memory type. It is only supported on Linux and Windows, and returns an
// error on other platforms.
func Read() (*Memory, error) {
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
