package agent

import (
	"github.com/boltdb/bolt"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/remotes"
	"github.com/pkg/errors"
	"google.golang.org/grpc/credentials"
)

// Config provides values for an Agent.
type Config struct {
	// Hostname the name of host for agent instance.
	Hostname string

	// Managers provides the manager backend used by the agent. It will be
	// updated with managers weights as observed by the agent.
	Managers remotes.Remotes

	// Executor specifies the executor to use for the agent.
	Executor exec.Executor

	// DB used for task storage. Must be open for the lifetime of the agent.
	DB *bolt.DB

	// NotifyNodeChange channel receives new node changes from session messages.
	NotifyNodeChange chan<- *api.Node

	// Credentials is credentials for grpc connection to manager.
	Credentials credentials.TransportCredentials
}

func (c *Config) validate() error {
	if c.Credentials == nil {
		return errors.New("agent: Credentials is required")
	}

	if c.Executor == nil {
		return errors.New("agent: executor required")
	}

	if c.DB == nil {
		return errors.New("agent: database required")
	}

	return nil
}
