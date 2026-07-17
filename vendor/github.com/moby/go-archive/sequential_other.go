//go:build !windows

package archive

// windows_O_FILE_FLAG_SEQUENTIAL_SCAN is not supported on go < 1.26.
const windows_O_FILE_FLAG_SEQUENTIAL_SCAN = 0
