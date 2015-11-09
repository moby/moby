// +build linux freebsd

package daemon

import "github.com/docker/docker/daemon/execdriver"

// setFromExitStatus is a platform specific helper function to set the state
// based on the ExitStatus structure.
func (s *State) setFromExitStatus(exitStatus *execdriver.ExitStatus) {
	s.ExitCode = exitStatus.ExitCode
	s.OOMKilled = exitStatus.OOMKilled
}
