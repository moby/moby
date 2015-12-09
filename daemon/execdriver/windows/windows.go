// +build windows

package windows

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/runconfig"
)

// This is a daemon development variable only and should not be
// used for running production containers on Windows.
var dummyMode bool

// This allows the daemon to terminate containers rather than shutdown
// This allows the daemon to force kill (HCS terminate) rather than shutdown
var forceKill bool

// defaultIsolation allows users to specify a default isolation mode for
// when running a container on Windows. For example docker daemon -D
// --exec-opt isolation=hyperv will cause Windows to always run containers
// as Hyper-V containers unless otherwise specified.
var defaultIsolation runconfig.IsolationLevel = "process"

// Define name and version for windows
var (
	DriverName = "Windows 1854"
	Version    = dockerversion.Version + " " + dockerversion.GitCommit
)

type activeContainer struct {
	command *execdriver.Command
}

// Driver contains all information for windows driver,
// it implements execdriver.Driver
type Driver struct {
	root             string
	activeContainers map[string]*activeContainer
	sync.Mutex
}

// Name implements the exec driver Driver interface.
func (d *Driver) Name() string {
	return fmt.Sprintf("\n Name: %s\n Build: %s \n Default Isolation: %s", DriverName, Version, defaultIsolation)
}

// NewDriver returns a new windows driver, called from NewDriver of execdriver.
func NewDriver(root string, options []string) (*Driver, error) {

	for _, option := range options {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return nil, err
		}
		key = strings.ToLower(key)
		switch key {

		case "dummy":
			switch val {
			case "1":
				dummyMode = true
				logrus.Warn("Using dummy mode in Windows exec driver. This is for development use only!")
			}

		case "forcekill":
			switch val {
			case "1":
				forceKill = true
				logrus.Warn("Using force kill mode in Windows exec driver. This is for testing purposes only.")
			}

		case "isolation":
			if !runconfig.IsolationLevel(val).IsValid() {
				return nil, fmt.Errorf("Unrecognised exec driver option 'isolation':'%s'", val)
			}
			if runconfig.IsolationLevel(val).IsHyperV() {
				defaultIsolation = "hyperv"
			}
			logrus.Infof("Windows default isolation level: '%s'", val)
		default:
			return nil, fmt.Errorf("Unrecognised exec driver option %s\n", key)
		}
	}

	return &Driver{
		root:             root,
		activeContainers: make(map[string]*activeContainer),
	}, nil
}

// setupEnvironmentVariables convert a string array of environment variables
// into a map as required by the HCS. Source array is in format [v1=k1] [v2=k2] etc.
func setupEnvironmentVariables(a []string) map[string]string {
	r := make(map[string]string)
	for _, s := range a {
		arr := strings.Split(s, "=")
		if len(arr) == 2 {
			r[arr[0]] = arr[1]
		}
	}
	return r
}
