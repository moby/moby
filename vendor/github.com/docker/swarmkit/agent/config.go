package agent

import (
	"github.com/boltdb/bolt"
	"github.com/docker/go-events"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/connectionbroker"
	"github.com/pkg/errors"
	"google.golang.org/grpc/credentials"
)

// NodeChanges encapsulates changes that should be made to the node as per session messages
// from the dispatcher
type NodeChanges struct {
	Node     *api.Node
	RootCert []byte
}

// Config provides values for an Agent.
type Config struct {
	// Hostname the name of host for agent instance.
	Hostname string

	// ConnBroker provides a connection broker for retrieving gRPC
	// connections to managers.
	ConnBroker *connectionbroker.Broker

	// Executor specifies the executor to use for the agent.
	Executor exec.Executor

	// DB used for task storage. Must be open for the lifetime of the agent.
	DB *bolt.DB

	// NotifyNodeChange channel receives new node changes from session messages.
	NotifyNodeChange chan<- *NodeChanges

	// NotifyTLSChange channel sends new TLS information changes, which can cause a session to restart
	NotifyTLSChange <-chan events.Event

	// Credentials is credentials for grpc connection to manager.
	Credentials credentials.TransportCredentials

	// NodeTLSInfo contains the starting node TLS info to bootstrap into the agent
	NodeTLSInfo *api.NodeTLSInfo
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

	if c.NodeTLSInfo == nil {
		return errors.New("agent: Node TLS info is required")
	}

	return nil
}
