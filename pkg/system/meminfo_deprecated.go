package system

import "github.com/docker/docker/pkg/meminfo"

// MemInfo contains memory statistics of the host system.
//
// Deprecated: use [meminfo.Memory].
type MemInfo = meminfo.Memory

// ReadMemInfo retrieves memory statistics of the host system and returns a
// MemInfo type.
//
// Deprecated: use [meminfo.Read].
func ReadMemInfo() (*meminfo.Memory, error) {
	return meminfo.Read()
}
