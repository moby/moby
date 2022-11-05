//go:build linux || freebsd || darwin || windows
// +build linux freebsd darwin windows

package system

import "github.com/docker/docker/pkg/process"

var (
	// IsProcessAlive returns true if process with a given pid is running.
	//
	// Deprecated: use [process.Alive].
	IsProcessAlive = process.Alive

	// IsProcessZombie return true if process has a state with "Z"
	//
	// Deprecated: use [process.Zombie].
	//
	// TODO(thaJeztah): remove the Windows implementation in process once we remove this stub.
	IsProcessZombie = process.Zombie
)

// KillProcess force-stops a process.
//
// Deprecated: use [process.Kill].
func KillProcess(pid int) {
	_ = process.Kill(pid)
}
