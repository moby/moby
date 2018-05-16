// Package types is used for API stability in the types and response to the
// consumers of the API stats endpoint.
package types // import "github.com/docker/docker/api/types"

import (
	"github.com/docker/docker/api/types/system"
)

// ThrottlingData stores CPU throttling stats of one running container.
// Not used on Windows.
type ThrottlingData = system.ThrottlingData

// CPUUsage stores All CPU stats aggregated since container inception.
type CPUUsage = system.CPUUsage

// CPUStats aggregates and wraps all CPU related info of container
type CPUStats = system.CPUStats

// MemoryStats aggregates all memory stats since container inception on Linux.
// Windows returns stats for commit and private working set only.
type MemoryStats = system.MemoryStats

// BlkioStatEntry is one small entity to store a piece of Blkio stats
// Not used on Windows.
type BlkioStatEntry = system.BlkioStatEntry

// BlkioStats stores All IO service stats for data read and write.
// This is a Linux specific structure as the differences between expressing
// block I/O on Windows and Linux are sufficiently significant to make
// little sense attempting to morph into a combined structure.
type BlkioStats = system.BlkioStats

// StorageStats is the disk I/O stats for read/write on Windows.
type StorageStats = system.StorageStats

// NetworkStats aggregates the network stats of one container
type NetworkStats = system.NetworkStats

// PidsStats contains the stats of a container's pids
type PidsStats = system.PidsStats

// Stats is Ultimate struct aggregating all types of stats of one container
type Stats = system.Stats

// StatsJSON is newly used Networks
type StatsJSON = system.StatsJSON
