package agent

import (
	"github.com/docker/go-events"
	"github.com/moby/swarmkit/v2/agent/exec"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/connectionbroker"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
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

	// SessionTracker, if provided, will have its SessionClosed and SessionError methods called
	// when sessions close and error.
	SessionTracker SessionTracker

	// FIPS returns whether the node is FIPS-enabled
	FIPS bool
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

// A SessionTracker gets notified when sessions close and error
type SessionTracker interface {
	// SessionClosed is called whenever a session is closed - if the function errors, the agent
	// will exit with the returned error.  Otherwise the agent can continue and rebuild a new session.
	SessionClosed() error

	// SessionError is called whenever a session errors
	SessionError(err error)

	// SessionEstablished is called whenever a session is established
	SessionEstablished()
}
