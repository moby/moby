// +build windows

package windows

// Note this is alpha code for the bring up of containers on Windows.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/microsoft/hcsshim"
	"github.com/natefinch/npipe"
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

type containerInit struct {
	SystemType              string
	Name                    string
	IsDummy                 bool
	VolumePath              string
	Devices                 []device
	IgnoreFlushesDuringBoot bool
	LayerFolderPath         string
	Layers                  []layer
}

// Run implements the exec driver Driver interface
func (d *Driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (execdriver.ExitStatus, error) {

	var (
		term                           execdriver.Terminal
		err                            error
		inListen, outListen, errListen *npipe.PipeListener
	)

	// Make sure the client isn't asking for options which aren't supported
	err = checkSupportedOptions(c)
	if err != nil {
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

	cu := &containerInit{
		SystemType:              "Container",
		Name:                    c.ID,
		IsDummy:                 dummyMode,
		VolumePath:              c.Rootfs,
		IgnoreFlushesDuringBoot: c.FirstStart,
		LayerFolderPath:         c.LayerFolder,
	}

	for i := 0; i < len(c.LayerPaths); i++ {
		cu.Layers = append(cu.Layers, layer{
			ID:   hcsshim.NewGUID(c.LayerPaths[i]).ToString(),
			Path: c.LayerPaths[i],
		})
	}

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

		if terminateMode {
			logrus.Debugf("Terminating container %s", c.ID)
			if err := hcsshim.TerminateComputeSystem(c.ID); err != nil {
				// IMPORTANT: Don't fail if fails to change state. It could already
				// have been stopped through kill().
				// Otherwise, the docker daemon will hang in job wait()
				logrus.Warnf("Ignoring error from TerminateComputeSystem %s", err)
			}
		} else {
			logrus.Debugf("Shutting down container %s", c.ID)
			if err := hcsshim.ShutdownComputeSystem(c.ID); err != nil {
				// IMPORTANT: Don't fail if fails to change state. It could already
				// have been stopped through kill().
				// Otherwise, the docker daemon will hang in job wait()
				logrus.Warnf("Ignoring error from ShutdownComputeSystem %s", err)
			}
		}
	}()

	// We use a different pipe name between real and dummy mode in the HCS
	var serverPipeFormat, clientPipeFormat string
	if dummyMode {
		clientPipeFormat = `\\.\pipe\docker-run-%[1]s-%[2]s`
		serverPipeFormat = clientPipeFormat
	} else {
		clientPipeFormat = `\\.\pipe\docker-run-%[2]s`
		serverPipeFormat = `\\.\Containers\%[1]s\Device\NamedPipe\docker-run-%[2]s`
	}

	createProcessParms := hcsshim.CreateProcessParams{
		EmulateConsole:   c.ProcessConfig.Tty,
		WorkingDirectory: c.WorkingDir,
		ConsoleSize:      c.ProcessConfig.ConsoleSize,
	}

	// Configure the environment for the process
	createProcessParms.Environment = setupEnvironmentVariables(c.ProcessConfig.Env)

	// Connect stdin
	if pipes.Stdin != nil {
		stdInPipe := fmt.Sprintf(serverPipeFormat, c.ID, "stdin")
		createProcessParms.StdInPipe = fmt.Sprintf(clientPipeFormat, c.ID, "stdin")

		// Listen on the named pipe
		inListen, err = npipe.Listen(stdInPipe)
		if err != nil {
			logrus.Errorf("stdin failed to listen on %s err=%s", stdInPipe, err)
			return execdriver.ExitStatus{ExitCode: -1}, err
		}
		defer inListen.Close()

		// Launch a goroutine to do the accept. We do this so that we can
		// cause an otherwise blocking goroutine to gracefully close when
		// the caller (us) closes the listener
		go stdinAccept(inListen, stdInPipe, pipes.Stdin)
	}

	// Connect stdout
	stdOutPipe := fmt.Sprintf(serverPipeFormat, c.ID, "stdout")
	createProcessParms.StdOutPipe = fmt.Sprintf(clientPipeFormat, c.ID, "stdout")

	outListen, err = npipe.Listen(stdOutPipe)
	if err != nil {
		logrus.Errorf("stdout failed to listen on %s err=%s", stdOutPipe, err)
		return execdriver.ExitStatus{ExitCode: -1}, err
	}
	defer outListen.Close()
	go stdouterrAccept(outListen, stdOutPipe, pipes.Stdout)

	// No stderr on TTY.
	if !c.ProcessConfig.Tty {
		// Connect stderr
		stdErrPipe := fmt.Sprintf(serverPipeFormat, c.ID, "stderr")
		createProcessParms.StdErrPipe = fmt.Sprintf(clientPipeFormat, c.ID, "stderr")
		errListen, err = npipe.Listen(stdErrPipe)
		if err != nil {
			logrus.Errorf("stderr failed to listen on %s err=%s", stdErrPipe, err)
			return execdriver.ExitStatus{ExitCode: -1}, err
		}
		defer errListen.Close()
		go stdouterrAccept(errListen, stdErrPipe, pipes.Stderr)
	}

	// This should get caught earlier, but just in case - validate that we
	// have something to run
	if c.ProcessConfig.Entrypoint == "" {
		err = errors.New("No entrypoint specified")
		logrus.Error(err)
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

	// Build the command line of the process
	createProcessParms.CommandLine = c.ProcessConfig.Entrypoint
	for _, arg := range c.ProcessConfig.Arguments {
		logrus.Debugln("appending ", arg)
		createProcessParms.CommandLine += " " + arg
	}
	logrus.Debugf("CommandLine: %s", createProcessParms.CommandLine)

	// Start the command running in the container.
	var pid uint32
	pid, err = hcsshim.CreateProcessInComputeSystem(c.ID, createProcessParms)

	if err != nil {
		logrus.Errorf("CreateProcessInComputeSystem() failed %s", err)
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

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

	// Invoke the start callback
	if startCallback != nil {
		startCallback(&c.ProcessConfig, int(pid))
	}

	var exitCode int32
	exitCode, err = hcsshim.WaitForProcessInComputeSystem(c.ID, pid)
	if err != nil {
		logrus.Errorf("Failed to WaitForProcessInComputeSystem %s", err)
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

	logrus.Debugf("Exiting Run() exitCode %d id=%s", exitCode, c.ID)
	return execdriver.ExitStatus{ExitCode: int(exitCode)}, nil
}
