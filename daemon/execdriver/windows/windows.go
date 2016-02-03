// +build windows

package windows

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/engine-api/types/container"
	"golang.org/x/sys/windows/registry"
)

// TP4RetryHack is a hack to retry CreateComputeSystem if it fails with
// known return codes from Windows due to bugs in TP4.
var TP4RetryHack bool

// This is a daemon development variable only and should not be
// used for running production containers on Windows.
var dummyMode bool

// This allows the daemon to terminate containers rather than shutdown
// This allows the daemon to force kill (HCS terminate) rather than shutdown
var forceKill bool

// DefaultIsolation allows users to specify a default isolation technology for
// when running a container on Windows. For example docker daemon -D
// --exec-opt isolation=hyperv will cause Windows to always run containers
// as Hyper-V containers unless otherwise specified.
var DefaultIsolation container.Isolation = "process"

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
	return fmt.Sprintf("\n Name: %s\n Build: %s \n Default Isolation: %s", DriverName, Version, DefaultIsolation)
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
			if !container.Isolation(val).IsValid() {
				return nil, fmt.Errorf("Unrecognised exec driver option 'isolation':'%s'", val)
			}
			if container.Isolation(val).IsHyperV() {
				DefaultIsolation = "hyperv"
			}
			logrus.Infof("Windows default isolation: '%s'", val)
		default:
			return nil, fmt.Errorf("Unrecognised exec driver option %s\n", key)
		}
	}

	// TODO Windows TP5 timeframe. Remove this next block of code once TP4
	// is no longer supported. Also remove the workaround in run.go.
	//
	// Hack for TP4 - determine the version of Windows from the registry.
	// This overcomes an issue on TP4 which causes CreateComputeSystem to
	// intermittently fail. It's predominantly here to make Windows to Windows
	// CI more reliable.
	TP4RetryHack = false
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return &Driver{}, err
	}
	defer k.Close()

	s, _, err := k.GetStringValue("BuildLab")
	if err != nil {
		return &Driver{}, err
	}
	parts := strings.Split(s, ".")
	if len(parts) < 1 {
		return &Driver{}, err
	}
	var val int
	if val, err = strconv.Atoi(parts[0]); err != nil {
		return &Driver{}, err
	}
	if val < 14250 {
		TP4RetryHack = true
	}
	// End of Windows TP4 hack

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
