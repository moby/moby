package main

import (
	"testing"

	"github.com/moby/moby/v2/integration-cli/daemon"
)

func (s *DockerSwarmSuite) getDaemon(t *testing.T, nodeID string) *daemon.Daemon {
	s.daemonsLock.Lock()
	defer s.daemonsLock.Unlock()
	for _, d := range s.daemons {
		if d.NodeID() == nodeID {
			return d
		}
	}
	t.Fatalf("could not find node with id: %s", nodeID)
	return nil
}

// nodeCmd executes a command on a given node via the normal docker socket
func (s *DockerSwarmSuite) nodeCmd(t *testing.T, id string, args ...string) (string, error) {
	return s.getDaemon(t, id).Cmd(args...)
}
