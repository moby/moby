package client

import (
	"fmt"
	"io"
	"net"
)

// A CallDetails type contains all of the information necessarry to
// to communicate CLI user intent to a remote Docker system.
type CallDetails struct {
	Method       string
	Path         string
	Data         interface{}
	PassAuthInfo bool
}

// A CliRemote type represents a plugable component that can both create
// network connections via the Dial method and act upon them via the Call
// method.
//
// By implementing the hooks, a remote plugin can successfully route and
// manage the outgoing command.
type CliRemote interface {
	Call(cli *DockerCli, callDetails *CallDetails) (io.ReadCloser, int, error)
	Dial(cli *DockerCli) (net.Conn, error)
}

// A CliRemote needs a portable initialization method to allow for easier
// registration and management.
type CliRemoteInit func() (CliRemote, error)

var (
	// All registered remotes
	remotes map[string]CliRemoteInit
)

func init() {
	remotes = make(map[string]CliRemoteInit)
}

// Finds a CliRemote with a given name. If there is no regisitered remote
// with a matching name then an error is returned.
func findRemote(name string) (CliRemoteInit, error) {
	if initFunc, exists := remotes[name]; exists {
		return initFunc, nil
	}

	return nil, fmt.Errorf("No such remote: %s", name)
}

// Registers a CliRemote in order to make it available for activation and
// use.
func RegisterRemote(name string, initFunc CliRemoteInit) error {
	if _, exists := remotes[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}

	remotes[name] = initFunc
	return nil
}

// Creates a new CliRemote instance of the registered remote with a name
// matching the user supplied name argument. If no remote is found with
// a matching name, an error is returned.
func NewCliRemote(name string) (CliRemote, error) {
	if initFunc, err := findRemote(name); err == nil {
		return initFunc()
	} else {
		return nil, err
	}
}
