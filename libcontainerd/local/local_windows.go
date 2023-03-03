package local // import "github.com/docker/docker/libcontainerd/local"

// This package contains the legacy in-proc calls in HCS using the v1 schema
// for Windows runtime purposes.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	containerderrdefs "github.com/containerd/containerd/errdefs"

	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libcontainerd/queue"
	libcontainerdtypes "github.com/docker/docker/libcontainerd/types"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

type process struct {
	// mu guards the mutable fields of this struct.
	//
	// Always lock mu before ctr's mutex to prevent deadlocks.
	mu         sync.Mutex
	id         string                 // Invariants: immutable
	ctr        *container             // Invariants: immutable, ctr != nil
	hcsProcess hcsshim.Process        // Is set to nil on process exit
	exited     *containerd.ExitStatus // Valid iff waitCh is closed
	waitCh     chan struct{}
}

type task struct {
	process
}

type container struct {
	mu sync.Mutex

	// The ociSpec is required, as client.Create() needs a spec, but can
	// be called from the RestartManager context which does not otherwise
	// have access to the Spec
	//
	// A container value with ociSpec == nil represents a container which
	// has been loaded with (*client).LoadContainer, and is ineligible to
	// be Start()ed.
	ociSpec *specs.Spec

	hcsContainer hcsshim.Container // Is set to nil on container delete
	isPaused     bool

	client           *client
	id               string
	terminateInvoked bool

	// task is a reference to the current task for the container. As a
	// corollary, when task == nil the container has no current task: the
	// container was never Start()ed or the task was Delete()d.
	task *task
}

// defaultOwner is a tag passed to HCS to allow it to differentiate between
// container creator management stacks. We hard code "docker" in the case
// of docker.
const defaultOwner = "docker"

type client struct {
	stateDir string
	backend  libcontainerdtypes.Backend
	logger   *logrus.Entry
	eventQ   queue.Queue
}

// NewClient creates a new local executor for windows
func NewClient(ctx context.Context, cli *containerd.Client, stateDir, ns string, b libcontainerdtypes.Backend) (libcontainerdtypes.Client, error) {
	c := &client{
		stateDir: stateDir,
		backend:  b,
		logger:   logrus.WithField("module", "libcontainerd").WithField("namespace", ns),
	}

	return c, nil
}

func (c *client) Version(ctx context.Context) (containerd.Version, error) {
	return containerd.Version{}, errors.New("not implemented on Windows")
}

// NewContainer is the entrypoint to create a container from a spec.
// Table below shows the fields required for HCS JSON calling parameters,
// where if not populated, is omitted.
// +-----------------+--------------------------------------------+---------------------------------------------------+
// |                 | Isolation=Process                          | Isolation=Hyper-V                                 |
// +-----------------+--------------------------------------------+---------------------------------------------------+
// | VolumePath      | \\?\\Volume{GUIDa}                         |                                                   |
// | LayerFolderPath | %root%\windowsfilter\containerID           |                                                   |
// | Layers[]        | ID=GUIDb;Path=%root%\windowsfilter\layerID | ID=GUIDb;Path=%root%\windowsfilter\layerID        |
// | HvRuntime       |                                            | ImagePath=%root%\BaseLayerID\UtilityVM            |
// +-----------------+--------------------------------------------+---------------------------------------------------+
//
// Isolation=Process example:
//
//	{
//		"SystemType": "Container",
//		"Name": "5e0055c814a6005b8e57ac59f9a522066e0af12b48b3c26a9416e23907698776",
//		"Owner": "docker",
//		"VolumePath": "\\\\\\\\?\\\\Volume{66d1ef4c-7a00-11e6-8948-00155ddbef9d}",
//		"IgnoreFlushesDuringBoot": true,
//		"LayerFolderPath": "C:\\\\control\\\\windowsfilter\\\\5e0055c814a6005b8e57ac59f9a522066e0af12b48b3c26a9416e23907698776",
//		"Layers": [{
//			"ID": "18955d65-d45a-557b-bf1c-49d6dfefc526",
//			"Path": "C:\\\\control\\\\windowsfilter\\\\65bf96e5760a09edf1790cb229e2dfb2dbd0fcdc0bf7451bae099106bfbfea0c"
//		}],
//		"HostName": "5e0055c814a6",
//		"MappedDirectories": [],
//		"HvPartition": false,
//		"EndpointList": ["eef2649d-bb17-4d53-9937-295a8efe6f2c"],
//	}
//
// Isolation=Hyper-V example:
//
//	{
//		"SystemType": "Container",
//		"Name": "475c2c58933b72687a88a441e7e0ca4bd72d76413c5f9d5031fee83b98f6045d",
//		"Owner": "docker",
//		"IgnoreFlushesDuringBoot": true,
//		"Layers": [{
//			"ID": "18955d65-d45a-557b-bf1c-49d6dfefc526",
//			"Path": "C:\\\\control\\\\windowsfilter\\\\65bf96e5760a09edf1790cb229e2dfb2dbd0fcdc0bf7451bae099106bfbfea0c"
//		}],
//		"HostName": "475c2c58933b",
//		"MappedDirectories": [],
//		"HvPartition": true,
//		"EndpointList": ["e1bb1e61-d56f-405e-b75d-fd520cefa0cb"],
//		"DNSSearchList": "a.com,b.com,c.com",
//		"HvRuntime": {
//			"ImagePath": "C:\\\\control\\\\windowsfilter\\\\65bf96e5760a09edf1790cb229e2dfb2dbd0fcdc0bf7451bae099106bfbfea0c\\\\UtilityVM"
//		},
//	}
func (c *client) NewContainer(_ context.Context, id string, spec *specs.Spec, shim string, runtimeOptions interface{}, opts ...containerd.NewContainerOpts) (libcontainerdtypes.Container, error) {
	var err error
	if spec.Linux != nil {
		return nil, errors.New("linux containers are not supported on this platform")
	}
	ctr, err := c.createWindows(id, spec, runtimeOptions)

	if err == nil {
		c.eventQ.Append(id, func() {
			ei := libcontainerdtypes.EventInfo{
				ContainerID: id,
			}
			c.logger.WithFields(logrus.Fields{
				"container": id,
				"event":     libcontainerdtypes.EventCreate,
			}).Info("sending event")
			err := c.backend.ProcessEvent(id, libcontainerdtypes.EventCreate, ei)
			if err != nil {
				c.logger.WithError(err).WithFields(logrus.Fields{
					"container": id,
					"event":     libcontainerdtypes.EventCreate,
				}).Error("failed to process event")
			}
		})
	}
	return ctr, err
}

func (c *client) createWindows(id string, spec *specs.Spec, runtimeOptions interface{}) (*container, error) {
	logger := c.logger.WithField("container", id)
	configuration := &hcsshim.ContainerConfig{
		SystemType:              "Container",
		Name:                    id,
		Owner:                   defaultOwner,
		IgnoreFlushesDuringBoot: spec.Windows.IgnoreFlushesDuringBoot,
		HostName:                spec.Hostname,
		HvPartition:             false,
	}

	c.extractResourcesFromSpec(spec, configuration)

	if spec.Windows.Resources != nil {
		if spec.Windows.Resources.Storage != nil {
			if spec.Windows.Resources.Storage.Bps != nil {
				configuration.StorageBandwidthMaximum = *spec.Windows.Resources.Storage.Bps
			}
			if spec.Windows.Resources.Storage.Iops != nil {
				configuration.StorageIOPSMaximum = *spec.Windows.Resources.Storage.Iops
			}
		}
	}

	if spec.Windows.HyperV != nil {
		configuration.HvPartition = true
	}

	if spec.Windows.Network != nil {
		configuration.EndpointList = spec.Windows.Network.EndpointList
		configuration.AllowUnqualifiedDNSQuery = spec.Windows.Network.AllowUnqualifiedDNSQuery
		if spec.Windows.Network.DNSSearchList != nil {
			configuration.DNSSearchList = strings.Join(spec.Windows.Network.DNSSearchList, ",")
		}
		configuration.NetworkSharedContainerName = spec.Windows.Network.NetworkSharedContainerName
	}

	if cs, ok := spec.Windows.CredentialSpec.(string); ok {
		configuration.Credentials = cs
	}

	// We must have least two layers in the spec, the bottom one being a
	// base image, the top one being the RW layer.
	if spec.Windows.LayerFolders == nil || len(spec.Windows.LayerFolders) < 2 {
		return nil, fmt.Errorf("OCI spec is invalid - at least two LayerFolders must be supplied to the runtime")
	}

	// Strip off the top-most layer as that's passed in separately to HCS
	configuration.LayerFolderPath = spec.Windows.LayerFolders[len(spec.Windows.LayerFolders)-1]
	layerFolders := spec.Windows.LayerFolders[:len(spec.Windows.LayerFolders)-1]

	if configuration.HvPartition {
		// We don't currently support setting the utility VM image explicitly.
		// TODO circa RS5, this may be re-locatable.
		if spec.Windows.HyperV.UtilityVMPath != "" {
			return nil, errors.New("runtime does not support an explicit utility VM path for Hyper-V containers")
		}

		// Find the upper-most utility VM image.
		var uvmImagePath string
		for _, path := range layerFolders {
			fullPath := filepath.Join(path, "UtilityVM")
			_, err := os.Stat(fullPath)
			if err == nil {
				uvmImagePath = fullPath
				break
			}
			if !os.IsNotExist(err) {
				return nil, err
			}
		}
		if uvmImagePath == "" {
			return nil, errors.New("utility VM image could not be found")
		}
		configuration.HvRuntime = &hcsshim.HvRuntime{ImagePath: uvmImagePath}

		if spec.Root.Path != "" {
			return nil, errors.New("OCI spec is invalid - Root.Path must be omitted for a Hyper-V container")
		}
	} else {
		const volumeGUIDRegex = `^\\\\\?\\(Volume)\{{0,1}[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}(\}){0,1}\}\\$`
		if _, err := regexp.MatchString(volumeGUIDRegex, spec.Root.Path); err != nil {
			return nil, fmt.Errorf(`OCI spec is invalid - Root.Path '%s' must be a volume GUID path in the format '\\?\Volume{GUID}\'`, spec.Root.Path)
		}
		// HCS API requires the trailing backslash to be removed
		configuration.VolumePath = spec.Root.Path[:len(spec.Root.Path)-1]
	}

	if spec.Root.Readonly {
		return nil, errors.New(`OCI spec is invalid - Root.Readonly must not be set on Windows`)
	}

	for _, layerPath := range layerFolders {
		_, filename := filepath.Split(layerPath)
		g, err := hcsshim.NameToGuid(filename)
		if err != nil {
			return nil, err
		}
		configuration.Layers = append(configuration.Layers, hcsshim.Layer{
			ID:   g.ToString(),
			Path: layerPath,
		})
	}

	// Add the mounts (volumes, bind mounts etc) to the structure
	var mds []hcsshim.MappedDir
	var mps []hcsshim.MappedPipe
	for _, mount := range spec.Mounts {
		const pipePrefix = `\\.\pipe\`
		if mount.Type != "" {
			return nil, fmt.Errorf("OCI spec is invalid - Mount.Type '%s' must not be set", mount.Type)
		}
		if strings.HasPrefix(mount.Destination, pipePrefix) {
			mp := hcsshim.MappedPipe{
				HostPath:          mount.Source,
				ContainerPipeName: mount.Destination[len(pipePrefix):],
			}
			mps = append(mps, mp)
		} else {
			md := hcsshim.MappedDir{
				HostPath:      mount.Source,
				ContainerPath: mount.Destination,
				ReadOnly:      false,
			}
			for _, o := range mount.Options {
				if strings.ToLower(o) == "ro" {
					md.ReadOnly = true
				}
			}
			mds = append(mds, md)
		}
	}
	configuration.MappedDirectories = mds
	configuration.MappedPipes = mps

	if len(spec.Windows.Devices) > 0 {
		// Add any device assignments
		if configuration.HvPartition {
			return nil, errors.New("device assignment is not supported for HyperV containers")
		}
		for _, d := range spec.Windows.Devices {
			// Per https://github.com/microsoft/hcsshim/blob/v0.9.2/internal/uvm/virtual_device.go#L17-L18,
			// these represent an Interface Class GUID.
			if d.IDType != "class" && d.IDType != "vpci-class-guid" {
				return nil, errors.Errorf("device assignment of type '%s' is not supported", d.IDType)
			}
			configuration.AssignedDevices = append(configuration.AssignedDevices, hcsshim.AssignedDevice{InterfaceClassGUID: d.ID})
		}
	}

	hcsContainer, err := hcsshim.CreateContainer(id, configuration)
	if err != nil {
		return nil, err
	}

	// Construct a container object for calling start on it.
	ctr := &container{
		client:       c,
		id:           id,
		ociSpec:      spec,
		hcsContainer: hcsContainer,
	}

	logger.Debug("starting container")
	if err := ctr.hcsContainer.Start(); err != nil {
		logger.WithError(err).Error("failed to start container")
		ctr.mu.Lock()
		if err := ctr.terminateContainer(); err != nil {
			logger.WithError(err).Error("failed to cleanup after a failed Start")
		} else {
			logger.Debug("cleaned up after failed Start by calling Terminate")
		}
		ctr.mu.Unlock()
		return nil, err
	}

	logger.Debug("createWindows() completed successfully")
	return ctr, nil

}

func (c *client) extractResourcesFromSpec(spec *specs.Spec, configuration *hcsshim.ContainerConfig) {
	if spec.Windows.Resources != nil {
		if spec.Windows.Resources.CPU != nil {
			if spec.Windows.Resources.CPU.Count != nil {
				// This check is being done here rather than in adaptContainerSettings
				// because we don't want to update the HostConfig in case this container
				// is moved to a host with more CPUs than this one.
				cpuCount := *spec.Windows.Resources.CPU.Count
				hostCPUCount := uint64(sysinfo.NumCPU())
				if cpuCount > hostCPUCount {
					c.logger.Warnf("Changing requested CPUCount of %d to current number of processors, %d", cpuCount, hostCPUCount)
					cpuCount = hostCPUCount
				}
				configuration.ProcessorCount = uint32(cpuCount)
			}
			if spec.Windows.Resources.CPU.Shares != nil {
				configuration.ProcessorWeight = uint64(*spec.Windows.Resources.CPU.Shares)
			}
			if spec.Windows.Resources.CPU.Maximum != nil {
				configuration.ProcessorMaximum = int64(*spec.Windows.Resources.CPU.Maximum)
			}
		}
		if spec.Windows.Resources.Memory != nil {
			if spec.Windows.Resources.Memory.Limit != nil {
				configuration.MemoryMaximumInMB = int64(*spec.Windows.Resources.Memory.Limit) / 1024 / 1024
			}
		}
	}
}

func (ctr *container) NewTask(_ context.Context, _ string, withStdin bool, attachStdio libcontainerdtypes.StdioCallback) (libcontainerdtypes.Task, error) {
	ctr.mu.Lock()
	defer ctr.mu.Unlock()

	switch {
	case ctr.ociSpec == nil:
		return nil, errors.WithStack(errdefs.NotImplemented(errors.New("a restored container cannot be started")))
	case ctr.task != nil:
		return nil, errors.WithStack(errdefs.NotModified(containerderrdefs.ErrAlreadyExists))
	}

	logger := ctr.client.logger.WithField("container", ctr.id)

	// Note we always tell HCS to create stdout as it's required
	// regardless of '-i' or '-t' options, so that docker can always grab
	// the output through logs. We also tell HCS to always create stdin,
	// even if it's not used - it will be closed shortly. Stderr is only
	// created if it we're not -t.
	var (
		emulateConsole   bool
		createStdErrPipe bool
	)
	if ctr.ociSpec.Process != nil {
		emulateConsole = ctr.ociSpec.Process.Terminal
		createStdErrPipe = !ctr.ociSpec.Process.Terminal
	}

	createProcessParms := &hcsshim.ProcessConfig{
		EmulateConsole:   emulateConsole,
		WorkingDirectory: ctr.ociSpec.Process.Cwd,
		CreateStdInPipe:  true,
		CreateStdOutPipe: true,
		CreateStdErrPipe: createStdErrPipe,
	}

	if ctr.ociSpec.Process != nil && ctr.ociSpec.Process.ConsoleSize != nil {
		createProcessParms.ConsoleSize[0] = uint(ctr.ociSpec.Process.ConsoleSize.Height)
		createProcessParms.ConsoleSize[1] = uint(ctr.ociSpec.Process.ConsoleSize.Width)
	}

	// Configure the environment for the process
	createProcessParms.Environment = setupEnvironmentVariables(ctr.ociSpec.Process.Env)

	// Configure the CommandLine/CommandArgs
	setCommandLineAndArgs(ctr.ociSpec.Process, createProcessParms)
	logger.Debugf("start commandLine: %s", createProcessParms.CommandLine)

	createProcessParms.User = ctr.ociSpec.Process.User.Username

	// Start the command running in the container.
	newProcess, err := ctr.hcsContainer.CreateProcess(createProcessParms)
	if err != nil {
		logger.WithError(err).Error("CreateProcess() failed")
		return nil, err
	}

	defer func() {
		if err != nil {
			if err := newProcess.Kill(); err != nil {
				logger.WithError(err).Error("failed to kill process")
			}
			go func() {
				if err := newProcess.Wait(); err != nil {
					logger.WithError(err).Error("failed to wait for process")
				}
				if err := newProcess.Close(); err != nil {
					logger.WithError(err).Error("failed to clean process resources")
				}
			}()
		}
	}()
	t := &task{process: process{
		id:         ctr.id,
		ctr:        ctr,
		hcsProcess: newProcess,
		waitCh:     make(chan struct{}),
	}}
	pid := t.Pid()
	logger.WithField("pid", pid).Debug("init process started")

	// Spin up a goroutine to notify the backend and clean up resources when
	// the task exits. Defer until after the start event is sent so that the
	// exit event is not sent out-of-order.
	defer func() { go t.reap() }()

	// Don't shadow err here due to our deferred clean-up.
	var dio *cio.DirectIO
	dio, err = newIOFromProcess(newProcess, ctr.ociSpec.Process.Terminal)
	if err != nil {
		logger.WithError(err).Error("failed to get stdio pipes")
		return nil, err
	}
	_, err = attachStdio(dio)
	if err != nil {
		logger.WithError(err).Error("failed to attach stdio")
		return nil, err
	}

	// All fallible operations have succeeded so it is now safe to set the
	// container's current task.
	ctr.task = t

	// Generate the associated event
	ctr.client.eventQ.Append(ctr.id, func() {
		ei := libcontainerdtypes.EventInfo{
			ContainerID: ctr.id,
			ProcessID:   t.id,
			Pid:         pid,
		}
		ctr.client.logger.WithFields(logrus.Fields{
			"container":  ctr.id,
			"event":      libcontainerdtypes.EventStart,
			"event-info": ei,
		}).Info("sending event")
		err := ctr.client.backend.ProcessEvent(ei.ContainerID, libcontainerdtypes.EventStart, ei)
		if err != nil {
			ctr.client.logger.WithError(err).WithFields(logrus.Fields{
				"container":  ei.ContainerID,
				"event":      libcontainerdtypes.EventStart,
				"event-info": ei,
			}).Error("failed to process event")
		}
	})
	logger.Debug("start() completed")
	return t, nil
}

func (*task) Start(context.Context) error {
	// No-op on Windows.
	return nil
}

func (ctr *container) Task(context.Context) (libcontainerdtypes.Task, error) {
	ctr.mu.Lock()
	defer ctr.mu.Unlock()
	if ctr.task == nil {
		return nil, errdefs.NotFound(containerderrdefs.ErrNotFound)
	}
	return ctr.task, nil
}

// setCommandLineAndArgs configures the HCS ProcessConfig based on an OCI process spec
func setCommandLineAndArgs(process *specs.Process, createProcessParms *hcsshim.ProcessConfig) {
	if process.CommandLine != "" {
		createProcessParms.CommandLine = process.CommandLine
	} else {
		createProcessParms.CommandLine = system.EscapeArgs(process.Args)
	}
}

func newIOFromProcess(newProcess hcsshim.Process, terminal bool) (*cio.DirectIO, error) {
	stdin, stdout, stderr, err := newProcess.Stdio()
	if err != nil {
		return nil, err
	}

	dio := cio.NewDirectIO(createStdInCloser(stdin, newProcess), nil, nil, terminal)

	// Convert io.ReadClosers to io.Readers
	if stdout != nil {
		dio.Stdout = io.NopCloser(&autoClosingReader{ReadCloser: stdout})
	}
	if stderr != nil {
		dio.Stderr = io.NopCloser(&autoClosingReader{ReadCloser: stderr})
	}
	return dio, nil
}

// Exec launches a process in a running container.
//
// The processID argument is entirely informational. As there is no mechanism
// (exposed through the libcontainerd interfaces) to enumerate or reference an
// exec'd process by ID, uniqueness is not currently enforced.
func (t *task) Exec(ctx context.Context, processID string, spec *specs.Process, withStdin bool, attachStdio libcontainerdtypes.StdioCallback) (libcontainerdtypes.Process, error) {
	hcsContainer, err := t.getHCSContainer()
	if err != nil {
		return nil, err
	}
	logger := t.ctr.client.logger.WithFields(logrus.Fields{
		"container": t.ctr.id,
		"exec":      processID,
	})

	// Note we always tell HCS to
	// create stdout as it's required regardless of '-i' or '-t' options, so that
	// docker can always grab the output through logs. We also tell HCS to always
	// create stdin, even if it's not used - it will be closed shortly. Stderr
	// is only created if it we're not -t.
	createProcessParms := &hcsshim.ProcessConfig{
		CreateStdInPipe:  true,
		CreateStdOutPipe: true,
		CreateStdErrPipe: !spec.Terminal,
	}
	if spec.Terminal {
		createProcessParms.EmulateConsole = true
		if spec.ConsoleSize != nil {
			createProcessParms.ConsoleSize[0] = uint(spec.ConsoleSize.Height)
			createProcessParms.ConsoleSize[1] = uint(spec.ConsoleSize.Width)
		}
	}

	// Take working directory from the process to add if it is defined,
	// otherwise take from the first process.
	if spec.Cwd != "" {
		createProcessParms.WorkingDirectory = spec.Cwd
	} else {
		createProcessParms.WorkingDirectory = t.ctr.ociSpec.Process.Cwd
	}

	// Configure the environment for the process
	createProcessParms.Environment = setupEnvironmentVariables(spec.Env)

	// Configure the CommandLine/CommandArgs
	setCommandLineAndArgs(spec, createProcessParms)
	logger.Debugf("exec commandLine: %s", createProcessParms.CommandLine)

	createProcessParms.User = spec.User.Username

	// Start the command running in the container.
	newProcess, err := hcsContainer.CreateProcess(createProcessParms)
	if err != nil {
		logger.WithError(err).Errorf("exec's CreateProcess() failed")
		return nil, err
	}
	pid := newProcess.Pid()
	defer func() {
		if err != nil {
			if err := newProcess.Kill(); err != nil {
				logger.WithError(err).Error("failed to kill process")
			}
			go func() {
				if err := newProcess.Wait(); err != nil {
					logger.WithError(err).Error("failed to wait for process")
				}
				if err := newProcess.Close(); err != nil {
					logger.WithError(err).Error("failed to clean process resources")
				}
			}()
		}
	}()

	dio, err := newIOFromProcess(newProcess, spec.Terminal)
	if err != nil {
		logger.WithError(err).Error("failed to get stdio pipes")
		return nil, err
	}
	// Tell the engine to attach streams back to the client
	_, err = attachStdio(dio)
	if err != nil {
		return nil, err
	}

	p := &process{
		id:         processID,
		ctr:        t.ctr,
		hcsProcess: newProcess,
		waitCh:     make(chan struct{}),
	}

	// Spin up a goroutine to notify the backend and clean up resources when
	// the process exits. Defer until after the start event is sent so that
	// the exit event is not sent out-of-order.
	defer func() { go p.reap() }()

	t.ctr.client.eventQ.Append(t.ctr.id, func() {
		ei := libcontainerdtypes.EventInfo{
			ContainerID: t.ctr.id,
			ProcessID:   p.id,
			Pid:         uint32(pid),
		}
		t.ctr.client.logger.WithFields(logrus.Fields{
			"container":  t.ctr.id,
			"event":      libcontainerdtypes.EventExecAdded,
			"event-info": ei,
		}).Info("sending event")
		err := t.ctr.client.backend.ProcessEvent(t.ctr.id, libcontainerdtypes.EventExecAdded, ei)
		if err != nil {
			t.ctr.client.logger.WithError(err).WithFields(logrus.Fields{
				"container":  t.ctr.id,
				"event":      libcontainerdtypes.EventExecAdded,
				"event-info": ei,
			}).Error("failed to process event")
		}
		err = t.ctr.client.backend.ProcessEvent(t.ctr.id, libcontainerdtypes.EventExecStarted, ei)
		if err != nil {
			t.ctr.client.logger.WithError(err).WithFields(logrus.Fields{
				"container":  t.ctr.id,
				"event":      libcontainerdtypes.EventExecStarted,
				"event-info": ei,
			}).Error("failed to process event")
		}
	})

	return p, nil
}

func (p *process) Pid() uint32 {
	p.mu.Lock()
	hcsProcess := p.hcsProcess
	p.mu.Unlock()
	if hcsProcess == nil {
		return 0
	}
	return uint32(hcsProcess.Pid())
}

func (p *process) Kill(_ context.Context, signal syscall.Signal) error {
	p.mu.Lock()
	hcsProcess := p.hcsProcess
	p.mu.Unlock()
	if hcsProcess == nil {
		return errors.WithStack(errdefs.NotFound(errors.New("process not found")))
	}
	return hcsProcess.Kill()
}

// Kill handles `docker stop` on Windows. While Linux has support for
// the full range of signals, signals aren't really implemented on Windows.
// We fake supporting regular stop and -9 to force kill.
func (t *task) Kill(_ context.Context, signal syscall.Signal) error {
	hcsContainer, err := t.getHCSContainer()
	if err != nil {
		return err
	}

	logger := t.ctr.client.logger.WithFields(logrus.Fields{
		"container": t.ctr.id,
		"process":   t.id,
		"pid":       t.Pid(),
		"signal":    signal,
	})
	logger.Debug("Signal()")

	var op string
	if signal == syscall.SIGKILL {
		// Terminate the compute system
		t.ctr.mu.Lock()
		t.ctr.terminateInvoked = true
		t.ctr.mu.Unlock()
		op, err = "terminate", hcsContainer.Terminate()
	} else {
		// Shut down the container
		op, err = "shutdown", hcsContainer.Shutdown()
	}
	if err != nil {
		if !hcsshim.IsPending(err) && !hcsshim.IsAlreadyStopped(err) {
			// ignore errors
			logger.WithError(err).Errorf("failed to %s hccshim container", op)
		}
	}

	return nil
}

// Resize handles a CLI event to resize an interactive docker run or docker
// exec window.
func (p *process) Resize(_ context.Context, width, height uint32) error {
	p.mu.Lock()
	hcsProcess := p.hcsProcess
	p.mu.Unlock()
	if hcsProcess == nil {
		return errors.WithStack(errdefs.NotFound(errors.New("process not found")))
	}

	p.ctr.client.logger.WithFields(logrus.Fields{
		"container": p.ctr.id,
		"process":   p.id,
		"height":    height,
		"width":     width,
		"pid":       hcsProcess.Pid(),
	}).Debug("resizing")
	return hcsProcess.ResizeConsole(uint16(width), uint16(height))
}

func (p *process) CloseStdin(context.Context) error {
	p.mu.Lock()
	hcsProcess := p.hcsProcess
	p.mu.Unlock()
	if hcsProcess == nil {
		return errors.WithStack(errdefs.NotFound(errors.New("process not found")))
	}

	return hcsProcess.CloseStdin()
}

// Pause handles pause requests for containers
func (t *task) Pause(_ context.Context) error {
	if t.ctr.ociSpec.Windows.HyperV == nil {
		return containerderrdefs.ErrNotImplemented
	}

	t.ctr.mu.Lock()
	defer t.ctr.mu.Unlock()

	if err := t.assertIsCurrentTask(); err != nil {
		return err
	}
	if t.ctr.hcsContainer == nil {
		return errdefs.NotFound(errors.WithStack(fmt.Errorf("container %q not found", t.ctr.id)))
	}
	if err := t.ctr.hcsContainer.Pause(); err != nil {
		return err
	}

	t.ctr.isPaused = true

	t.ctr.client.eventQ.Append(t.ctr.id, func() {
		err := t.ctr.client.backend.ProcessEvent(t.ctr.id, libcontainerdtypes.EventPaused, libcontainerdtypes.EventInfo{
			ContainerID: t.ctr.id,
			ProcessID:   t.id,
		})
		t.ctr.client.logger.WithFields(logrus.Fields{
			"container": t.ctr.id,
			"event":     libcontainerdtypes.EventPaused,
		}).Info("sending event")
		if err != nil {
			t.ctr.client.logger.WithError(err).WithFields(logrus.Fields{
				"container": t.ctr.id,
				"event":     libcontainerdtypes.EventPaused,
			}).Error("failed to process event")
		}
	})

	return nil
}

// Resume handles resume requests for containers
func (t *task) Resume(ctx context.Context) error {
	if t.ctr.ociSpec.Windows.HyperV == nil {
		return errors.New("cannot resume Windows Server Containers")
	}

	t.ctr.mu.Lock()
	defer t.ctr.mu.Unlock()

	if err := t.assertIsCurrentTask(); err != nil {
		return err
	}
	if t.ctr.hcsContainer == nil {
		return errdefs.NotFound(errors.WithStack(fmt.Errorf("container %q not found", t.ctr.id)))
	}
	if err := t.ctr.hcsContainer.Resume(); err != nil {
		return err
	}

	t.ctr.isPaused = false

	t.ctr.client.eventQ.Append(t.ctr.id, func() {
		err := t.ctr.client.backend.ProcessEvent(t.ctr.id, libcontainerdtypes.EventResumed, libcontainerdtypes.EventInfo{
			ContainerID: t.ctr.id,
			ProcessID:   t.id,
		})
		t.ctr.client.logger.WithFields(logrus.Fields{
			"container": t.ctr.id,
			"event":     libcontainerdtypes.EventResumed,
		}).Info("sending event")
		if err != nil {
			t.ctr.client.logger.WithError(err).WithFields(logrus.Fields{
				"container": t.ctr.id,
				"event":     libcontainerdtypes.EventResumed,
			}).Error("failed to process event")
		}
	})

	return nil
}

// Stats handles stats requests for containers
func (t *task) Stats(_ context.Context) (*libcontainerdtypes.Stats, error) {
	hc, err := t.getHCSContainer()
	if err != nil {
		return nil, err
	}

	readAt := time.Now()
	s, err := hc.Statistics()
	if err != nil {
		return nil, err
	}
	return &libcontainerdtypes.Stats{
		Read:     readAt,
		HCSStats: &s,
	}, nil
}

// LoadContainer is the handler for restoring a container
func (c *client) LoadContainer(ctx context.Context, id string) (libcontainerdtypes.Container, error) {
	c.logger.WithField("container", id).Debug("LoadContainer()")

	// TODO Windows: On RS1, a re-attach isn't possible.
	// However, there is a scenario in which there is an issue.
	// Consider a background container. The daemon dies unexpectedly.
	// HCS will still have the compute service alive and running.
	// For consistence, we call in to shoot it regardless if HCS knows about it
	// We explicitly just log a warning if the terminate fails.
	// Then we tell the backend the container exited.
	hc, err := hcsshim.OpenContainer(id)
	if err != nil {
		return nil, errdefs.NotFound(errors.New("container not found"))
	}
	const terminateTimeout = time.Minute * 2
	err = hc.Terminate()

	if hcsshim.IsPending(err) {
		err = hc.WaitTimeout(terminateTimeout)
	} else if hcsshim.IsAlreadyStopped(err) {
		err = nil
	}

	if err != nil {
		c.logger.WithField("container", id).WithError(err).Debug("terminate failed on restore")
		return nil, err
	}
	return &container{
		client:       c,
		hcsContainer: hc,
		id:           id,
	}, nil
}

// AttachTask is only called by the daemon when restoring containers. As
// re-attach isn't possible (see LoadContainer), a NotFound error is
// unconditionally returned to allow restore to make progress.
func (*container) AttachTask(context.Context, libcontainerdtypes.StdioCallback) (libcontainerdtypes.Task, error) {
	return nil, errdefs.NotFound(containerderrdefs.ErrNotImplemented)
}

// Pids returns a list of process IDs running in a container. It is not
// implemented on Windows.
func (t *task) Pids(context.Context) ([]containerd.ProcessInfo, error) {
	return nil, errors.New("not implemented on Windows")
}

// Summary returns a summary of the processes running in a container.
// This is present in Windows to support docker top. In linux, the
// engine shells out to ps to get process information. On Windows, as
// the containers could be Hyper-V containers, they would not be
// visible on the container host. However, libcontainerd does have
// that information.
func (t *task) Summary(_ context.Context) ([]libcontainerdtypes.Summary, error) {
	hc, err := t.getHCSContainer()
	if err != nil {
		return nil, err
	}

	p, err := hc.ProcessList()
	if err != nil {
		return nil, err
	}

	pl := make([]libcontainerdtypes.Summary, len(p))
	for i := range p {
		pl[i] = libcontainerdtypes.Summary{
			ImageName:                    p[i].ImageName,
			CreatedAt:                    p[i].CreateTimestamp,
			KernelTime_100Ns:             p[i].KernelTime100ns,
			MemoryCommitBytes:            p[i].MemoryCommitBytes,
			MemoryWorkingSetPrivateBytes: p[i].MemoryWorkingSetPrivateBytes,
			MemoryWorkingSetSharedBytes:  p[i].MemoryWorkingSetSharedBytes,
			ProcessID:                    p[i].ProcessId,
			UserTime_100Ns:               p[i].UserTime100ns,
			ExecID:                       "",
		}
	}
	return pl, nil
}

func (p *process) Delete(ctx context.Context) (*containerd.ExitStatus, error) {
	select {
	case <-ctx.Done():
		return nil, errors.WithStack(ctx.Err())
	case <-p.waitCh:
	default:
		return nil, errdefs.Conflict(errors.New("process is running"))
	}
	return p.exited, nil
}

func (t *task) Delete(ctx context.Context) (*containerd.ExitStatus, error) {
	select {
	case <-ctx.Done():
		return nil, errors.WithStack(ctx.Err())
	case <-t.waitCh:
	default:
		return nil, errdefs.Conflict(errors.New("container is not stopped"))
	}

	t.ctr.mu.Lock()
	defer t.ctr.mu.Unlock()
	if err := t.assertIsCurrentTask(); err != nil {
		return nil, err
	}
	t.ctr.task = nil
	return t.exited, nil
}

func (t *task) ForceDelete(ctx context.Context) error {
	select {
	case <-t.waitCh: // Task is already stopped.
		_, err := t.Delete(ctx)
		return err
	default:
	}

	if err := t.Kill(ctx, syscall.SIGKILL); err != nil {
		return errors.Wrap(err, "could not force-kill task")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.waitCh:
		_, err := t.Delete(ctx)
		return err
	}
}

func (t *task) Status(ctx context.Context) (containerd.Status, error) {
	select {
	case <-t.waitCh:
		return containerd.Status{
			Status:     containerd.Stopped,
			ExitStatus: t.exited.ExitCode(),
			ExitTime:   t.exited.ExitTime(),
		}, nil
	default:
	}

	t.ctr.mu.Lock()
	defer t.ctr.mu.Unlock()
	s := containerd.Running
	if t.ctr.isPaused {
		s = containerd.Paused
	}
	return containerd.Status{Status: s}, nil
}

func (*task) UpdateResources(ctx context.Context, resources *libcontainerdtypes.Resources) error {
	// Updating resource isn't supported on Windows
	// but we should return nil for enabling updating container
	return nil
}

func (*task) CreateCheckpoint(ctx context.Context, checkpointDir string, exit bool) error {
	return errors.New("Windows: Containers do not support checkpoints")
}

// assertIsCurrentTask returns a non-nil error if the task has been deleted.
func (t *task) assertIsCurrentTask() error {
	if t.ctr.task != t {
		return errors.WithStack(errdefs.NotFound(fmt.Errorf("task %q not found", t.id)))
	}
	return nil
}

// getHCSContainer returns a reference to the hcsshim Container for the task's
// container if neither the task nor container have been deleted.
//
// t.ctr.mu must not be locked by the calling goroutine when calling this
// function.
func (t *task) getHCSContainer() (hcsshim.Container, error) {
	t.ctr.mu.Lock()
	defer t.ctr.mu.Unlock()
	if err := t.assertIsCurrentTask(); err != nil {
		return nil, err
	}
	hc := t.ctr.hcsContainer
	if hc == nil {
		return nil, errors.WithStack(errdefs.NotFound(fmt.Errorf("container %q not found", t.ctr.id)))
	}
	return hc, nil
}

// ctr mutex must be held when calling this function.
func (ctr *container) shutdownContainer() error {
	var err error
	const waitTimeout = time.Minute * 5

	if !ctr.terminateInvoked {
		err = ctr.hcsContainer.Shutdown()
	}

	if hcsshim.IsPending(err) || ctr.terminateInvoked {
		err = ctr.hcsContainer.WaitTimeout(waitTimeout)
	} else if hcsshim.IsAlreadyStopped(err) {
		err = nil
	}

	if err != nil {
		ctr.client.logger.WithError(err).WithField("container", ctr.id).
			Debug("failed to shutdown container, terminating it")
		terminateErr := ctr.terminateContainer()
		if terminateErr != nil {
			ctr.client.logger.WithError(terminateErr).WithField("container", ctr.id).
				Error("failed to shutdown container, and subsequent terminate also failed")
			return fmt.Errorf("%s: subsequent terminate failed %s", err, terminateErr)
		}
		return err
	}

	return nil
}

// ctr mutex must be held when calling this function.
func (ctr *container) terminateContainer() error {
	const terminateTimeout = time.Minute * 5
	ctr.terminateInvoked = true
	err := ctr.hcsContainer.Terminate()

	if hcsshim.IsPending(err) {
		err = ctr.hcsContainer.WaitTimeout(terminateTimeout)
	} else if hcsshim.IsAlreadyStopped(err) {
		err = nil
	}

	if err != nil {
		ctr.client.logger.WithError(err).WithField("container", ctr.id).
			Debug("failed to terminate container")
		return err
	}

	return nil
}

func (p *process) reap() {
	logger := p.ctr.client.logger.WithFields(logrus.Fields{
		"container": p.ctr.id,
		"process":   p.id,
	})

	var eventErr error

	// Block indefinitely for the process to exit.
	if err := p.hcsProcess.Wait(); err != nil {
		if herr, ok := err.(*hcsshim.ProcessError); ok && herr.Err != windows.ERROR_BROKEN_PIPE {
			logger.WithError(err).Warnf("Wait() failed (container may have been killed)")
		}
		// Fall through here, do not return. This ensures we tell the
		// docker engine that the process/container has exited to avoid
		// a container being dropped on the floor.
	}
	exitedAt := time.Now()

	exitCode, err := p.hcsProcess.ExitCode()
	if err != nil {
		if herr, ok := err.(*hcsshim.ProcessError); ok && herr.Err != windows.ERROR_BROKEN_PIPE {
			logger.WithError(err).Warnf("unable to get exit code for process")
		}
		// Since we got an error retrieving the exit code, make sure that the
		// code we return doesn't incorrectly indicate success.
		exitCode = -1

		// Fall through here, do not return. This ensures we tell the
		// docker engine that the process/container has exited to avoid
		// a container being dropped on the floor.
	}

	p.mu.Lock()
	hcsProcess := p.hcsProcess
	p.hcsProcess = nil
	p.mu.Unlock()

	if err := hcsProcess.Close(); err != nil {
		logger.WithError(err).Warnf("failed to cleanup hcs process resources")
		exitCode = -1
		eventErr = fmt.Errorf("hcsProcess.Close() failed %s", err)
	}

	// Explicit locking is not required as reads from exited are
	// synchronized using waitCh.
	p.exited = containerd.NewExitStatus(uint32(exitCode), exitedAt, nil)
	close(p.waitCh)

	p.ctr.client.eventQ.Append(p.ctr.id, func() {
		ei := libcontainerdtypes.EventInfo{
			ContainerID: p.ctr.id,
			ProcessID:   p.id,
			Pid:         uint32(hcsProcess.Pid()),
			ExitCode:    uint32(exitCode),
			ExitedAt:    exitedAt,
			Error:       eventErr,
		}
		p.ctr.client.logger.WithFields(logrus.Fields{
			"container":  p.ctr.id,
			"event":      libcontainerdtypes.EventExit,
			"event-info": ei,
		}).Info("sending event")
		err := p.ctr.client.backend.ProcessEvent(p.ctr.id, libcontainerdtypes.EventExit, ei)
		if err != nil {
			p.ctr.client.logger.WithError(err).WithFields(logrus.Fields{
				"container":  p.ctr.id,
				"event":      libcontainerdtypes.EventExit,
				"event-info": ei,
			}).Error("failed to process event")
		}
	})
}

func (ctr *container) Delete(context.Context) error {
	ctr.mu.Lock()
	defer ctr.mu.Unlock()

	if ctr.hcsContainer == nil {
		return errors.WithStack(errdefs.NotFound(fmt.Errorf("container %q not found", ctr.id)))
	}

	// Check that there is no task currently running.
	if ctr.task != nil {
		select {
		case <-ctr.task.waitCh:
		default:
			return errors.WithStack(errdefs.Conflict(errors.New("container is not stopped")))
		}
	}

	var (
		logger = ctr.client.logger.WithFields(logrus.Fields{
			"container": ctr.id,
		})
		thisErr error
	)

	if err := ctr.shutdownContainer(); err != nil {
		logger.WithError(err).Warn("failed to shutdown container")
		thisErr = errors.Wrap(err, "failed to shutdown container")
	} else {
		logger.Debug("completed container shutdown")
	}

	if err := ctr.hcsContainer.Close(); err != nil {
		logger.WithError(err).Error("failed to clean hcs container resources")
		thisErr = errors.Wrap(err, "failed to terminate container")
	}

	ctr.hcsContainer = nil
	return thisErr
}
