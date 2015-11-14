// +build windows

package windows

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/microsoft/hcsshim"
)

// defaultContainerNAT is the default name of the container NAT device that is
// preconfigured on the server.
const defaultContainerNAT = "ContainerNAT"

type layer struct {
	ID   string
	Path string
}

type defConfig struct {
	DefFile string
}

type portBinding struct {
	Protocol     string
	InternalPort int
	ExternalPort int
}

type natSettings struct {
	Name         string
	PortBindings []portBinding
}

type networkConnection struct {
	NetworkName string
	// TODO Windows: Add Ip4Address string to this structure when hooked up in
	// docker CLI. This is present in the HCS JSON handler.
	EnableNat bool
	Nat       natSettings
}
type networkSettings struct {
	MacAddress string
}

type device struct {
	DeviceType string
	Connection interface{}
	Settings   interface{}
}

type mappedDir struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

type containerInit struct {
	SystemType              string      // HCS requires this to be hard-coded to "Container"
	Name                    string      // Name of the container. We use the docker ID.
	Owner                   string      // The management platform that created this container
	IsDummy                 bool        // Used for development purposes.
	VolumePath              string      // Windows volume path for scratch space
	Devices                 []device    // Devices used by the container
	IgnoreFlushesDuringBoot bool        // Optimisation hint for container startup in Windows
	LayerFolderPath         string      // Where the layer folders are located
	Layers                  []layer     // List of storage layers
	ProcessorWeight         int64       `json:",omitempty"` // CPU Shares 0..10000 on Windows; where 0 will be ommited and HCS will default.
	HostName                string      // Hostname
	MappedDirectories       []mappedDir // List of mapped directories (volumes/mounts)
	SandboxPath             string      // Location of unmounted sandbox (used for Hyper-V containers, not Windows Server containers)
	HvPartition             bool        // True if it a Hyper-V Container
}

// defaultOwner is a tag passed to HCS to allow it to differentiate between
// container creator management stacks. We hard code "docker" in the case
// of docker.
const defaultOwner = "docker"

// Run implements the exec driver Driver interface
func (d *Driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, hooks execdriver.Hooks) (execdriver.ExitStatus, error) {

	var (
		term execdriver.Terminal
		err  error
	)

	cu := &containerInit{
		SystemType:              "Container",
		Name:                    c.ID,
		Owner:                   defaultOwner,
		IsDummy:                 dummyMode,
		VolumePath:              c.Rootfs,
		IgnoreFlushesDuringBoot: c.FirstStart,
		LayerFolderPath:         c.LayerFolder,
		ProcessorWeight:         c.Resources.CPUShares,
		HostName:                c.Hostname,
	}

	// Work out the isolation (whether it is a hypervisor partition)
	if c.Isolation.IsDefault() {
		// Not specified by caller. Take daemon default
		cu.HvPartition = defaultIsolation.IsHyperV()
	} else {
		// Take value specified by caller
		cu.HvPartition = c.Isolation.IsHyperV()
	}

	if cu.HvPartition {
		cu.SandboxPath = filepath.Dir(c.LayerFolder)
	} else {
		cu.VolumePath = c.Rootfs
		cu.LayerFolderPath = c.LayerFolder
	}

	for _, layerPath := range c.LayerPaths {
		_, filename := filepath.Split(layerPath)
		g, err := hcsshim.NameToGuid(filename)
		if err != nil {
			return execdriver.ExitStatus{ExitCode: -1}, err
		}
		cu.Layers = append(cu.Layers, layer{
			ID:   g.ToString(),
			Path: layerPath,
		})
	}

	// Add the mounts (volumes, bind mounts etc) to the structure
	mds := make([]mappedDir, len(c.Mounts))
	for i, mount := range c.Mounts {
		mds[i] = mappedDir{
			HostPath:      mount.Source,
			ContainerPath: mount.Destination,
			ReadOnly:      !mount.Writable}
	}
	cu.MappedDirectories = mds

	// TODO Windows. At some point, when there is CLI on docker run to
	// enable the IP Address of the container to be passed into docker run,
	// the IP Address needs to be wired through to HCS in the JSON. It
	// would be present in c.Network.Interface.IPAddress. See matching
	// TODO in daemon\container_windows.go, function populateCommand.

	if c.Network.Interface != nil {

		var pbs []portBinding

		// Enumerate through the port bindings specified by the user and convert
		// them into the internal structure matching the JSON blob that can be
		// understood by the HCS.
		for i, v := range c.Network.Interface.PortBindings {
			proto := strings.ToUpper(i.Proto())
			if proto != "TCP" && proto != "UDP" {
				return execdriver.ExitStatus{ExitCode: -1}, fmt.Errorf("invalid protocol %s", i.Proto())
			}

			if len(v) > 1 {
				return execdriver.ExitStatus{ExitCode: -1}, fmt.Errorf("Windows does not support more than one host port in NAT settings")
			}

			for _, v2 := range v {
				var (
					iPort, ePort int
					err          error
				)
				if len(v2.HostIP) != 0 {
					return execdriver.ExitStatus{ExitCode: -1}, fmt.Errorf("Windows does not support host IP addresses in NAT settings")
				}
				if ePort, err = strconv.Atoi(v2.HostPort); err != nil {
					return execdriver.ExitStatus{ExitCode: -1}, fmt.Errorf("invalid container port %s: %s", v2.HostPort, err)
				}
				if iPort, err = strconv.Atoi(i.Port()); err != nil {
					return execdriver.ExitStatus{ExitCode: -1}, fmt.Errorf("invalid internal port %s: %s", i.Port(), err)
				}
				if iPort < 0 || iPort > 65535 || ePort < 0 || ePort > 65535 {
					return execdriver.ExitStatus{ExitCode: -1}, fmt.Errorf("specified NAT port is not in allowed range")
				}
				pbs = append(pbs,
					portBinding{ExternalPort: ePort,
						InternalPort: iPort,
						Protocol:     proto})
			}
		}

		// TODO Windows: TP3 workaround. Allow the user to override the name of
		// the Container NAT device through an environment variable. This will
		// ultimately be a global daemon parameter on Windows, similar to -b
		// for the name of the virtual switch (aka bridge).
		cn := os.Getenv("DOCKER_CONTAINER_NAT")
		if len(cn) == 0 {
			cn = defaultContainerNAT
		}

		dev := device{
			DeviceType: "Network",
			Connection: &networkConnection{
				NetworkName: c.Network.Interface.Bridge,
				// TODO Windows: Fixme, next line. Needs HCS fix.
				EnableNat: false,
				Nat: natSettings{
					Name:         cn,
					PortBindings: pbs,
				},
			},
		}

		if c.Network.Interface.MacAddress != "" {
			windowsStyleMAC := strings.Replace(
				c.Network.Interface.MacAddress, ":", "-", -1)
			dev.Settings = networkSettings{
				MacAddress: windowsStyleMAC,
			}
		}
		cu.Devices = append(cu.Devices, dev)
	} else {
		logrus.Debugln("No network interface")
	}

	configurationb, err := json.Marshal(cu)
	if err != nil {
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

	configuration := string(configurationb)

	err = hcsshim.CreateComputeSystem(c.ID, configuration)
	if err != nil {
		logrus.Debugln("Failed to create temporary container ", err)
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

	// Start the container
	logrus.Debugln("Starting container ", c.ID)
	err = hcsshim.StartComputeSystem(c.ID)
	if err != nil {
		logrus.Errorf("Failed to start compute system: %s", err)
		return execdriver.ExitStatus{ExitCode: -1}, err
	}
	defer func() {
		// Stop the container
		if forceKill {
			logrus.Debugf("Forcibly terminating container %s", c.ID)
			if errno, err := hcsshim.TerminateComputeSystem(c.ID, hcsshim.TimeoutInfinite, "exec-run-defer"); err != nil {
				logrus.Warnf("Ignoring error from TerminateComputeSystem 0x%X %s", errno, err)
			}
		} else {
			logrus.Debugf("Shutting down container %s", c.ID)
			if errno, err := hcsshim.ShutdownComputeSystem(c.ID, hcsshim.TimeoutInfinite, "exec-run-defer"); err != nil {
				if errno != hcsshim.Win32SystemShutdownIsInProgress &&
					errno != hcsshim.Win32SpecifiedPathInvalid &&
					errno != hcsshim.Win32SystemCannotFindThePathSpecified {
					logrus.Warnf("Ignoring error from ShutdownComputeSystem 0x%X %s", errno, err)
				}
			}
		}
	}()

	createProcessParms := hcsshim.CreateProcessParams{
		EmulateConsole:   c.ProcessConfig.Tty,
		WorkingDirectory: c.WorkingDir,
		ConsoleSize:      c.ProcessConfig.ConsoleSize,
	}

	// Configure the environment for the process
	createProcessParms.Environment = setupEnvironmentVariables(c.ProcessConfig.Env)

	createProcessParms.CommandLine, err = createCommandLine(&c.ProcessConfig, c.ArgsEscaped)

	if err != nil {
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

	// Start the command running in the container.
	pid, stdin, stdout, stderr, _, err := hcsshim.CreateProcessInComputeSystem(c.ID, pipes.Stdin != nil, true, !c.ProcessConfig.Tty, createProcessParms)
	if err != nil {
		logrus.Errorf("CreateProcessInComputeSystem() failed %s", err)
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

	// Now that the process has been launched, begin copying data to and from
	// the named pipes for the std handles.
	setupPipes(stdin, stdout, stderr, pipes)

	//Save the PID as we'll need this in Kill()
	logrus.Debugf("PID %d", pid)
	c.ContainerPid = int(pid)

	if c.ProcessConfig.Tty {
		term = NewTtyConsole(c.ID, pid)
	} else {
		term = NewStdConsole()
	}
	c.ProcessConfig.Terminal = term

	// Maintain our list of active containers. We'll need this later for exec
	// and other commands.
	d.Lock()
	d.activeContainers[c.ID] = &activeContainer{
		command: c,
	}
	d.Unlock()

	if hooks.Start != nil {
		// A closed channel for OOM is returned here as it will be
		// non-blocking and return the correct result when read.
		chOOM := make(chan struct{})
		close(chOOM)
		hooks.Start(&c.ProcessConfig, int(pid), chOOM)
	}

	var (
		exitCode int32
		errno    uint32
	)
	exitCode, errno, err = hcsshim.WaitForProcessInComputeSystem(c.ID, pid, hcsshim.TimeoutInfinite)
	if err != nil {
		if errno != hcsshim.Win32PipeHasBeenEnded {
			logrus.Warnf("WaitForProcessInComputeSystem failed (container may have been killed): %s", err)
		}
		// Do NOT return err here as the container would have
		// started, otherwise docker will deadlock. It's perfectly legitimate
		// for WaitForProcessInComputeSystem to fail in situations such
		// as the container being killed on another thread.
		return execdriver.ExitStatus{ExitCode: hcsshim.WaitErrExecFailed}, nil
	}

	logrus.Debugf("Exiting Run() exitCode %d id=%s", exitCode, c.ID)
	return execdriver.ExitStatus{ExitCode: int(exitCode)}, nil
}

// SupportsHooks implements the execdriver Driver interface.
// The windows driver does not support the hook mechanism
func (d *Driver) SupportsHooks() bool {
	return false
}
