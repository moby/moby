package libcontainerd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

type client struct {
	clientCommon

	// Platform specific properties below here (none presently on Windows)
}

// Win32 error codes that are used for various workarounds
// These really should be ALL_CAPS to match golangs syscall library and standard
// Win32 error conventions, but golint insists on CamelCase.
const (
	CoEClassstring     = syscall.Errno(0x800401F3) // Invalid class string
	ErrorNoNetwork     = syscall.Errno(1222)       // The network is not present or not started
	ErrorBadPathname   = syscall.Errno(161)        // The specified path is invalid
	ErrorInvalidObject = syscall.Errno(0x800710D8) // The object identifier does not represent a valid object
)

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
	Nat         natSettings
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

// TODO Windows: @darrenstahlmsft Add ProcessorCount
type containerInit struct {
	SystemType              string      // HCS requires this to be hard-coded to "Container"
	Name                    string      // Name of the container. We use the docker ID.
	Owner                   string      // The management platform that created this container
	IsDummy                 bool        // Used for development purposes.
	VolumePath              string      // Windows volume path for scratch space
	Devices                 []device    // Devices used by the container
	IgnoreFlushesDuringBoot bool        // Optimization hint for container startup in Windows
	LayerFolderPath         string      // Where the layer folders are located
	Layers                  []layer     // List of storage layers
	ProcessorWeight         uint64      `json:",omitempty"` // CPU Shares 0..10000 on Windows; where 0 will be omitted and HCS will default.
	ProcessorMaximum        int64       `json:",omitempty"` // CPU maximum usage percent 1..100
	StorageIOPSMaximum      uint64      `json:",omitempty"` // Maximum Storage IOPS
	StorageBandwidthMaximum uint64      `json:",omitempty"` // Maximum Storage Bandwidth in bytes per second
	StorageSandboxSize      uint64      `json:",omitempty"` // Size in bytes that the container system drive should be expanded to if smaller
	MemoryMaximumInMB       int64       `json:",omitempty"` // Maximum memory available to the container in Megabytes
	HostName                string      // Hostname
	MappedDirectories       []mappedDir // List of mapped directories (volumes/mounts)
	SandboxPath             string      // Location of unmounted sandbox (used for Hyper-V containers)
	HvPartition             bool        // True if it a Hyper-V Container
	EndpointList            []string    // List of networking endpoints to be attached to container
}

// defaultOwner is a tag passed to HCS to allow it to differentiate between
// container creator management stacks. We hard code "docker" in the case
// of docker.
const defaultOwner = "docker"

// Create is the entrypoint to create a container from a spec, and if successfully
// created, start it too.
func (clnt *client) Create(containerID string, spec Spec, options ...CreateOption) error {
	logrus.Debugln("LCD client.Create() with spec", spec)

	cu := &containerInit{
		SystemType: "Container",
		Name:       containerID,
		Owner:      defaultOwner,

		VolumePath:              spec.Root.Path,
		IgnoreFlushesDuringBoot: spec.Windows.FirstStart,
		LayerFolderPath:         spec.Windows.LayerFolder,
		HostName:                spec.Hostname,
	}

	if spec.Windows.Networking != nil {
		cu.EndpointList = spec.Windows.Networking.EndpointList
	}

	if spec.Windows.Resources != nil {
		if spec.Windows.Resources.CPU != nil {
			if spec.Windows.Resources.CPU.Shares != nil {
				cu.ProcessorWeight = *spec.Windows.Resources.CPU.Shares
			}
			if spec.Windows.Resources.CPU.Percent != nil {
				cu.ProcessorMaximum = *spec.Windows.Resources.CPU.Percent * 100 // ProcessorMaximum is a value between 1 and 10000
			}
		}
		if spec.Windows.Resources.Memory != nil {
			if spec.Windows.Resources.Memory.Limit != nil {
				cu.MemoryMaximumInMB = *spec.Windows.Resources.Memory.Limit / 1024 / 1024
			}
		}
		if spec.Windows.Resources.Storage != nil {
			if spec.Windows.Resources.Storage.Bps != nil {
				cu.StorageBandwidthMaximum = *spec.Windows.Resources.Storage.Bps
			}
			if spec.Windows.Resources.Storage.Iops != nil {
				cu.StorageIOPSMaximum = *spec.Windows.Resources.Storage.Iops
			}
			if spec.Windows.Resources.Storage.SandboxSize != nil {
				cu.StorageSandboxSize = *spec.Windows.Resources.Storage.SandboxSize
			}
		}
	}

	cu.HvPartition = (spec.Windows.HvRuntime != nil)

	// TODO Windows @jhowardmsft. FIXME post TP5.
	//	if spec.Windows.HvRuntime != nil {
	//		if spec.WIndows.HVRuntime.ImagePath != "" {
	//			cu.TBD = spec.Windows.HvRuntime.ImagePath
	//		}
	//	}

	if cu.HvPartition {
		cu.SandboxPath = filepath.Dir(spec.Windows.LayerFolder)
	} else {
		cu.VolumePath = spec.Root.Path
		cu.LayerFolderPath = spec.Windows.LayerFolder
	}

	for _, layerPath := range spec.Windows.LayerPaths {
		_, filename := filepath.Split(layerPath)
		g, err := hcsshim.NameToGuid(filename)
		if err != nil {
			return err
		}
		cu.Layers = append(cu.Layers, layer{
			ID:   g.ToString(),
			Path: layerPath,
		})
	}

	// Add the mounts (volumes, bind mounts etc) to the structure
	mds := make([]mappedDir, len(spec.Mounts))
	for i, mount := range spec.Mounts {
		mds[i] = mappedDir{
			HostPath:      mount.Source,
			ContainerPath: mount.Destination,
			ReadOnly:      mount.Readonly}
	}
	cu.MappedDirectories = mds

	configurationb, err := json.Marshal(cu)
	if err != nil {
		return err
	}

	// Create the compute system
	configuration := string(configurationb)
	if err := hcsshim.CreateComputeSystem(containerID, configuration); err != nil {
		return err
	}

	// Construct a container object for calling start on it.
	container := &container{
		containerCommon: containerCommon{
			process: process{
				processCommon: processCommon{
					containerID:  containerID,
					client:       clnt,
					friendlyName: InitFriendlyName,
				},
				commandLine: strings.Join(spec.Process.Args, " "),
			},
			processes: make(map[string]*process),
		},
		ociSpec: spec,
	}

	container.options = options
	for _, option := range options {
		if err := option.Apply(container); err != nil {
			logrus.Error(err)
		}
	}

	// Call start, and if it fails, delete the container from our
	// internal structure, and also keep HCS in sync by deleting the
	// container there.
	logrus.Debugf("Create() id=%s, Calling start()", containerID)
	if err := container.start(); err != nil {
		clnt.deleteContainer(containerID)
		return err
	}

	logrus.Debugf("Create() id=%s completed successfully", containerID)
	return nil

}

// AddProcess is the handler for adding a process to an already running
// container. It's called through docker exec.
func (clnt *client) AddProcess(containerID, processFriendlyName string, procToAdd Process) error {

	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	container, err := clnt.getContainer(containerID)
	if err != nil {
		return err
	}

	createProcessParms := hcsshim.CreateProcessParams{
		EmulateConsole: procToAdd.Terminal,
		ConsoleSize:    procToAdd.InitialConsoleSize,
	}

	// Take working directory from the process to add if it is defined,
	// otherwise take from the first process.
	if procToAdd.Cwd != "" {
		createProcessParms.WorkingDirectory = procToAdd.Cwd
	} else {
		createProcessParms.WorkingDirectory = container.ociSpec.Process.Cwd
	}

	// Configure the environment for the process
	createProcessParms.Environment = setupEnvironmentVariables(procToAdd.Env)
	createProcessParms.CommandLine = strings.Join(procToAdd.Args, " ")

	logrus.Debugf("commandLine: %s", createProcessParms.CommandLine)

	// Start the command running in the container. Note we always tell HCS to
	// create stdout as it's required regardless of '-i' or '-t' options, so that
	// docker can always grab the output through logs. We also tell HCS to always
	// create stdin, even if it's not used - it will be closed shortly. Stderr
	// is only created if it we're not -t.
	var stdout, stderr io.ReadCloser
	var pid uint32
	iopipe := &IOPipe{Terminal: procToAdd.Terminal}
	pid, iopipe.Stdin, stdout, stderr, err = hcsshim.CreateProcessInComputeSystem(
		containerID,
		true,
		true,
		!procToAdd.Terminal,
		createProcessParms)
	if err != nil {
		logrus.Errorf("AddProcess %s CreateProcessInComputeSystem() failed %s", containerID, err)
		return err
	}

	// Convert io.ReadClosers to io.Readers
	if stdout != nil {
		iopipe.Stdout = openReaderFromPipe(stdout)
	}
	if stderr != nil {
		iopipe.Stderr = openReaderFromPipe(stderr)
	}

	// Add the process to the containers list of processes
	container.processes[processFriendlyName] =
		&process{
			processCommon: processCommon{
				containerID:  containerID,
				friendlyName: processFriendlyName,
				client:       clnt,
				systemPid:    pid,
			},
			commandLine: createProcessParms.CommandLine,
		}

	// Make sure the lock is not held while calling back into the daemon
	clnt.unlock(containerID)

	// Tell the engine to attach streams back to the client
	if err := clnt.backend.AttachStreams(processFriendlyName, *iopipe); err != nil {
		return err
	}

	// Lock again so that the defer unlock doesn't fail. (I really don't like this code)
	clnt.lock(containerID)

	// Spin up a go routine waiting for exit to handle cleanup
	go container.waitExit(pid, processFriendlyName, false)

	return nil
}

// Signal handles `docker stop` on Windows. While Linux has support for
// the full range of signals, signals aren't really implemented on Windows.
// We fake supporting regular stop and -9 to force kill.
func (clnt *client) Signal(containerID string, sig int) error {
	var (
		cont *container
		err  error
	)

	// Get the container as we need it to find the pid of the process.
	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	if cont, err = clnt.getContainer(containerID); err != nil {
		return err
	}

	logrus.Debugf("lcd: Signal() containerID=%s sig=%d pid=%d", containerID, sig, cont.systemPid)
	context := fmt.Sprintf("Signal: sig=%d pid=%d", sig, cont.systemPid)

	if syscall.Signal(sig) == syscall.SIGKILL {
		// Terminate the compute system
		if err := hcsshim.TerminateComputeSystem(containerID, hcsshim.TimeoutInfinite, context); err != nil {
			logrus.Errorf("Failed to terminate %s - %q", containerID, err)
		}

	} else {
		// Terminate Process
		if err = hcsshim.TerminateProcessInComputeSystem(containerID, cont.systemPid); err != nil {
			logrus.Warnf("Failed to terminate pid %d in %s: %q", cont.systemPid, containerID, err)
			// Ignore errors
			err = nil
		}

		// Shutdown the compute system
		if err := hcsshim.ShutdownComputeSystem(containerID, hcsshim.TimeoutInfinite, context); err != nil {
			logrus.Errorf("Failed to shutdown %s - %q", containerID, err)
		}
	}
	return nil
}

// Resize handles a CLI event to resize an interactive docker run or docker exec
// window.
func (clnt *client) Resize(containerID, processFriendlyName string, width, height int) error {
	// Get the libcontainerd container object
	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	cont, err := clnt.getContainer(containerID)
	if err != nil {
		return err
	}

	if processFriendlyName == InitFriendlyName {
		logrus.Debugln("Resizing systemPID in", containerID, cont.process.systemPid)
		return hcsshim.ResizeConsoleInComputeSystem(containerID, cont.process.systemPid, height, width)
	}

	for _, p := range cont.processes {
		if p.friendlyName == processFriendlyName {
			logrus.Debugln("Resizing exec'd process", containerID, p.systemPid)
			return hcsshim.ResizeConsoleInComputeSystem(containerID, p.systemPid, height, width)
		}
	}

	return fmt.Errorf("Resize could not find containerID %s to resize", containerID)

}

// Pause handles pause requests for containers
func (clnt *client) Pause(containerID string) error {
	return errors.New("Windows: Containers cannot be paused")
}

// Resume handles resume requests for containers
func (clnt *client) Resume(containerID string) error {
	return errors.New("Windows: Containers cannot be paused")
}

// Stats handles stats requests for containers
func (clnt *client) Stats(containerID string) (*Stats, error) {
	return nil, errors.New("Windows: Stats not implemented")
}

// Restore is the handler for restoring a container
func (clnt *client) Restore(containerID string, unusedOnWindows ...CreateOption) error {
	// TODO Windows: Implement this. For now, just tell the backend the container exited.
	logrus.Debugf("lcd Restore %s", containerID)
	return clnt.backend.StateChanged(containerID, StateInfo{
		CommonStateInfo: CommonStateInfo{
			State:    StateExit,
			ExitCode: 1 << 31,
		}})
}

// GetPidsForContainer returns a list of process IDs running in a container.
// Although implemented, this is not used in Windows.
func (clnt *client) GetPidsForContainer(containerID string) ([]int, error) {
	var pids []int
	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	cont, err := clnt.getContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Add the first process
	pids = append(pids, int(cont.containerCommon.systemPid))
	// And add all the exec'd processes
	for _, p := range cont.processes {
		pids = append(pids, int(p.processCommon.systemPid))
	}
	return pids, nil
}

// Summary returns a summary of the processes running in a container.
// This is present in Windows to support docker top. In linux, the
// engine shells out to ps to get process information. On Windows, as
// the containers could be Hyper-V containers, they would not be
// visible on the container host. However, libcontainerd does have
// that information.
func (clnt *client) Summary(containerID string) ([]Summary, error) {
	var s []Summary
	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	cont, err := clnt.getContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Add the first process
	s = append(s, Summary{
		Pid:     cont.containerCommon.systemPid,
		Command: cont.ociSpec.Process.Args[0]})
	// And add all the exec'd processes
	for _, p := range cont.processes {
		s = append(s, Summary{
			Pid:     p.processCommon.systemPid,
			Command: p.commandLine})
	}
	return s, nil

}

// UpdateResources updates resources for a running container.
func (clnt *client) UpdateResources(containerID string, resources Resources) error {
	// Updating resource isn't supported on Windows
	// but we should return nil for enabling updating container
	return nil
}
