package libcontainerd // import "github.com/docker/docker/libcontainerd"

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/hcsshim"
	opengcs "github.com/Microsoft/opengcs/client"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

const InitProcessName = "init"

type process struct {
	id         string
	pid        int
	hcsProcess hcsshim.Process
}

type container struct {
	sync.Mutex

	// The ociSpec is required, as client.Create() needs a spec, but can
	// be called from the RestartManager context which does not otherwise
	// have access to the Spec
	ociSpec *specs.Spec

	isWindows           bool
	manualStopRequested bool
	hcsContainer        hcsshim.Container

	id            string
	status        Status
	exitedAt      time.Time
	exitCode      uint32
	waitCh        chan struct{}
	init          *process
	execs         map[string]*process
	updatePending bool
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

// defaultOwner is a tag passed to HCS to allow it to differentiate between
// container creator management stacks. We hard code "docker" in the case
// of docker.
const defaultOwner = "docker"

func (c *client) Version(ctx context.Context) (containerd.Version, error) {
	return containerd.Version{}, errors.New("not implemented on Windows")
}

// Create is the entrypoint to create a container from a spec.
// Table below shows the fields required for HCS JSON calling parameters,
// where if not populated, is omitted.
// +-----------------+--------------------------------------------+---------------------------------------------------+
// |                 | Isolation=Process                          | Isolation=Hyper-V                                 |
// +-----------------+--------------------------------------------+---------------------------------------------------+
// | VolumePath      | \\?\\Volume{GUIDa}                         |                                                   |
// | LayerFolderPath | %root%\windowsfilter\containerID           | %root%\windowsfilter\containerID (servicing only) |
// | Layers[]        | ID=GUIDb;Path=%root%\windowsfilter\layerID | ID=GUIDb;Path=%root%\windowsfilter\layerID        |
// | HvRuntime       |                                            | ImagePath=%root%\BaseLayerID\UtilityVM            |
// +-----------------+--------------------------------------------+---------------------------------------------------+
//
// Isolation=Process example:
//
// {
//	"SystemType": "Container",
//	"Name": "5e0055c814a6005b8e57ac59f9a522066e0af12b48b3c26a9416e23907698776",
//	"Owner": "docker",
//	"VolumePath": "\\\\\\\\?\\\\Volume{66d1ef4c-7a00-11e6-8948-00155ddbef9d}",
//	"IgnoreFlushesDuringBoot": true,
//	"LayerFolderPath": "C:\\\\control\\\\windowsfilter\\\\5e0055c814a6005b8e57ac59f9a522066e0af12b48b3c26a9416e23907698776",
//	"Layers": [{
//		"ID": "18955d65-d45a-557b-bf1c-49d6dfefc526",
//		"Path": "C:\\\\control\\\\windowsfilter\\\\65bf96e5760a09edf1790cb229e2dfb2dbd0fcdc0bf7451bae099106bfbfea0c"
//	}],
//	"HostName": "5e0055c814a6",
//	"MappedDirectories": [],
//	"HvPartition": false,
//	"EndpointList": ["eef2649d-bb17-4d53-9937-295a8efe6f2c"],
//	"Servicing": false
//}
//
// Isolation=Hyper-V example:
//
//{
//	"SystemType": "Container",
//	"Name": "475c2c58933b72687a88a441e7e0ca4bd72d76413c5f9d5031fee83b98f6045d",
//	"Owner": "docker",
//	"IgnoreFlushesDuringBoot": true,
//	"Layers": [{
//		"ID": "18955d65-d45a-557b-bf1c-49d6dfefc526",
//		"Path": "C:\\\\control\\\\windowsfilter\\\\65bf96e5760a09edf1790cb229e2dfb2dbd0fcdc0bf7451bae099106bfbfea0c"
//	}],
//	"HostName": "475c2c58933b",
//	"MappedDirectories": [],
//	"HvPartition": true,
//	"EndpointList": ["e1bb1e61-d56f-405e-b75d-fd520cefa0cb"],
//	"DNSSearchList": "a.com,b.com,c.com",
//	"HvRuntime": {
//		"ImagePath": "C:\\\\control\\\\windowsfilter\\\\65bf96e5760a09edf1790cb229e2dfb2dbd0fcdc0bf7451bae099106bfbfea0c\\\\UtilityVM"
//	},
//	"Servicing": false
//}
func (c *client) Create(_ context.Context, id string, spec *specs.Spec, runtimeOptions interface{}) error {
	if ctr := c.getContainer(id); ctr != nil {
		return errors.WithStack(newConflictError("id already in use"))
	}

	// spec.Linux must be nil for Windows containers, but spec.Windows
	// will be filled in regardless of container platform.  This is a
	// temporary workaround due to LCOW requiring layer folder paths,
	// which are stored under spec.Windows.
	//
	// TODO: @darrenstahlmsft fix this once the OCI spec is updated to
	// support layer folder paths for LCOW
	if spec.Linux == nil {
		return c.createWindows(id, spec, runtimeOptions)
	}
	return c.createLinux(id, spec, runtimeOptions)
}

func (c *client) createWindows(id string, spec *specs.Spec, runtimeOptions interface{}) error {
	logger := c.logger.WithField("container", id)
	configuration := &hcsshim.ContainerConfig{
		SystemType: "Container",
		Name:       id,
		Owner:      defaultOwner,
		IgnoreFlushesDuringBoot: spec.Windows.IgnoreFlushesDuringBoot,
		HostName:                spec.Hostname,
		HvPartition:             false,
		Servicing:               spec.Windows.Servicing,
	}

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
		return fmt.Errorf("OCI spec is invalid - at least two LayerFolders must be supplied to the runtime")
	}

	// Strip off the top-most layer as that's passed in separately to HCS
	configuration.LayerFolderPath = spec.Windows.LayerFolders[len(spec.Windows.LayerFolders)-1]
	layerFolders := spec.Windows.LayerFolders[:len(spec.Windows.LayerFolders)-1]

	if configuration.HvPartition {
		// We don't currently support setting the utility VM image explicitly.
		// TODO @swernli/jhowardmsft circa RS5, this may be re-locatable.
		if spec.Windows.HyperV.UtilityVMPath != "" {
			return errors.New("runtime does not support an explicit utility VM path for Hyper-V containers")
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
				return err
			}
		}
		if uvmImagePath == "" {
			return errors.New("utility VM image could not be found")
		}
		configuration.HvRuntime = &hcsshim.HvRuntime{ImagePath: uvmImagePath}

		if spec.Root.Path != "" {
			return errors.New("OCI spec is invalid - Root.Path must be omitted for a Hyper-V container")
		}
	} else {
		const volumeGUIDRegex = `^\\\\\?\\(Volume)\{{0,1}[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}(\}){0,1}\}\\$`
		if _, err := regexp.MatchString(volumeGUIDRegex, spec.Root.Path); err != nil {
			return fmt.Errorf(`OCI spec is invalid - Root.Path '%s' must be a volume GUID path in the format '\\?\Volume{GUID}\'`, spec.Root.Path)
		}
		// HCS API requires the trailing backslash to be removed
		configuration.VolumePath = spec.Root.Path[:len(spec.Root.Path)-1]
	}

	if spec.Root.Readonly {
		return errors.New(`OCI spec is invalid - Root.Readonly must not be set on Windows`)
	}

	for _, layerPath := range layerFolders {
		_, filename := filepath.Split(layerPath)
		g, err := hcsshim.NameToGuid(filename)
		if err != nil {
			return err
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
			return fmt.Errorf("OCI spec is invalid - Mount.Type '%s' must not be set", mount.Type)
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
	if len(mps) > 0 && system.GetOSVersion().Build < 16299 { // RS3
		return errors.New("named pipe mounts are not supported on this version of Windows")
	}
	configuration.MappedPipes = mps

	hcsContainer, err := hcsshim.CreateContainer(id, configuration)
	if err != nil {
		return err
	}

	// Construct a container object for calling start on it.
	ctr := &container{
		id:           id,
		execs:        make(map[string]*process),
		isWindows:    true,
		ociSpec:      spec,
		hcsContainer: hcsContainer,
		status:       StatusCreated,
		waitCh:       make(chan struct{}),
	}

	// Start the container. If this is a servicing container, this call
	// will block until the container is done with the servicing
	// execution.
	logger.Debug("starting container")
	if err = hcsContainer.Start(); err != nil {
		c.logger.WithError(err).Error("failed to start container")
		ctr.debugGCS()
		if err := c.terminateContainer(ctr); err != nil {
			c.logger.WithError(err).Error("failed to cleanup after a failed Start")
		} else {
			c.logger.Debug("cleaned up after failed Start by calling Terminate")
		}
		return err
	}
	ctr.debugGCS()

	c.Lock()
	c.containers[id] = ctr
	c.Unlock()

	logger.Debug("createWindows() completed successfully")
	return nil

}

func (c *client) createLinux(id string, spec *specs.Spec, runtimeOptions interface{}) error {
	logrus.Debugf("libcontainerd: createLinux(): containerId %s ", id)
	logger := c.logger.WithField("container", id)

	if runtimeOptions == nil {
		return fmt.Errorf("lcow option must be supplied to the runtime")
	}
	lcowConfig, ok := runtimeOptions.(*opengcs.Config)
	if !ok {
		return fmt.Errorf("lcow option must be supplied to the runtime")
	}

	configuration := &hcsshim.ContainerConfig{
		HvPartition:   true,
		Name:          id,
		SystemType:    "container",
		ContainerType: "linux",
		Owner:         defaultOwner,
		TerminateOnLastHandleClosed: true,
	}

	if lcowConfig.ActualMode == opengcs.ModeActualVhdx {
		configuration.HvRuntime = &hcsshim.HvRuntime{
			ImagePath:          lcowConfig.Vhdx,
			BootSource:         "Vhd",
			WritableBootSource: false,
		}
	} else {
		configuration.HvRuntime = &hcsshim.HvRuntime{
			ImagePath:           lcowConfig.KirdPath,
			LinuxKernelFile:     lcowConfig.KernelFile,
			LinuxInitrdFile:     lcowConfig.InitrdFile,
			LinuxBootParameters: lcowConfig.BootParameters,
		}
	}

	if spec.Windows == nil {
		return fmt.Errorf("spec.Windows must not be nil for LCOW containers")
	}

	// We must have least one layer in the spec
	if spec.Windows.LayerFolders == nil || len(spec.Windows.LayerFolders) == 0 {
		return fmt.Errorf("OCI spec is invalid - at least one LayerFolders must be supplied to the runtime")
	}

	// Strip off the top-most layer as that's passed in separately to HCS
	configuration.LayerFolderPath = spec.Windows.LayerFolders[len(spec.Windows.LayerFolders)-1]
	layerFolders := spec.Windows.LayerFolders[:len(spec.Windows.LayerFolders)-1]

	for _, layerPath := range layerFolders {
		_, filename := filepath.Split(layerPath)
		g, err := hcsshim.NameToGuid(filename)
		if err != nil {
			return err
		}
		configuration.Layers = append(configuration.Layers, hcsshim.Layer{
			ID:   g.ToString(),
			Path: filepath.Join(layerPath, "layer.vhd"),
		})
	}

	if spec.Windows.Network != nil {
		configuration.EndpointList = spec.Windows.Network.EndpointList
		configuration.AllowUnqualifiedDNSQuery = spec.Windows.Network.AllowUnqualifiedDNSQuery
		if spec.Windows.Network.DNSSearchList != nil {
			configuration.DNSSearchList = strings.Join(spec.Windows.Network.DNSSearchList, ",")
		}
		configuration.NetworkSharedContainerName = spec.Windows.Network.NetworkSharedContainerName
	}

	// Add the mounts (volumes, bind mounts etc) to the structure. We have to do
	// some translation for both the mapped directories passed into HCS and in
	// the spec.
	//
	// For HCS, we only pass in the mounts from the spec which are type "bind".
	// Further, the "ContainerPath" field (which is a little mis-leadingly
	// named when it applies to the utility VM rather than the container in the
	// utility VM) is moved to under /tmp/gcs/<ID>/binds, where this is passed
	// by the caller through a 'uvmpath' option.
	//
	// We do similar translation for the mounts in the spec by stripping out
	// the uvmpath option, and translating the Source path to the location in the
	// utility VM calculated above.
	//
	// From inside the utility VM, you would see a 9p mount such as in the following
	// where a host folder has been mapped to /target. The line with /tmp/gcs/<ID>/binds
	// specifically:
	//
	//	/ # mount
	//	rootfs on / type rootfs (rw,size=463736k,nr_inodes=115934)
	//	proc on /proc type proc (rw,relatime)
	//	sysfs on /sys type sysfs (rw,relatime)
	//	udev on /dev type devtmpfs (rw,relatime,size=498100k,nr_inodes=124525,mode=755)
	//	tmpfs on /run type tmpfs (rw,relatime)
	//	cgroup on /sys/fs/cgroup type cgroup (rw,relatime,cpuset,cpu,cpuacct,blkio,memory,devices,freezer,net_cls,perf_event,net_prio,hugetlb,pids,rdma)
	//	mqueue on /dev/mqueue type mqueue (rw,relatime)
	//	devpts on /dev/pts type devpts (rw,relatime,mode=600,ptmxmode=000)
	//	/binds/b3ea9126d67702173647ece2744f7c11181c0150e9890fc9a431849838033edc/target on /binds/b3ea9126d67702173647ece2744f7c11181c0150e9890fc9a431849838033edc/target type 9p (rw,sync,dirsync,relatime,trans=fd,rfdno=6,wfdno=6)
	//	/dev/pmem0 on /tmp/gcs/b3ea9126d67702173647ece2744f7c11181c0150e9890fc9a431849838033edc/layer0 type ext4 (ro,relatime,block_validity,delalloc,norecovery,barrier,dax,user_xattr,acl)
	//	/dev/sda on /tmp/gcs/b3ea9126d67702173647ece2744f7c11181c0150e9890fc9a431849838033edc/scratch type ext4 (rw,relatime,block_validity,delalloc,barrier,user_xattr,acl)
	//	overlay on /tmp/gcs/b3ea9126d67702173647ece2744f7c11181c0150e9890fc9a431849838033edc/rootfs type overlay (rw,relatime,lowerdir=/tmp/base/:/tmp/gcs/b3ea9126d67702173647ece2744f7c11181c0150e9890fc9a431849838033edc/layer0,upperdir=/tmp/gcs/b3ea9126d67702173647ece2744f7c11181c0150e9890fc9a431849838033edc/scratch/upper,workdir=/tmp/gcs/b3ea9126d67702173647ece2744f7c11181c0150e9890fc9a431849838033edc/scratch/work)
	//
	//  /tmp/gcs/b3ea9126d67702173647ece2744f7c11181c0150e9890fc9a431849838033edc # ls -l
	//	total 16
	//	drwx------    3 0        0               60 Sep  7 18:54 binds
	//	-rw-r--r--    1 0        0             3345 Sep  7 18:54 config.json
	//	drwxr-xr-x   10 0        0             4096 Sep  6 17:26 layer0
	//	drwxr-xr-x    1 0        0             4096 Sep  7 18:54 rootfs
	//	drwxr-xr-x    5 0        0             4096 Sep  7 18:54 scratch
	//
	//	/tmp/gcs/b3ea9126d67702173647ece2744f7c11181c0150e9890fc9a431849838033edc # ls -l binds
	//	total 0
	//	drwxrwxrwt    2 0        0             4096 Sep  7 16:51 target

	mds := []hcsshim.MappedDir{}
	specMounts := []specs.Mount{}
	for _, mount := range spec.Mounts {
		specMount := mount
		if mount.Type == "bind" {
			// Strip out the uvmpath from the options
			updatedOptions := []string{}
			uvmPath := ""
			readonly := false
			for _, opt := range mount.Options {
				dropOption := false
				elements := strings.SplitN(opt, "=", 2)
				switch elements[0] {
				case "uvmpath":
					uvmPath = elements[1]
					dropOption = true
				case "rw":
				case "ro":
					readonly = true
				case "rbind":
				default:
					return fmt.Errorf("unsupported option %q", opt)
				}
				if !dropOption {
					updatedOptions = append(updatedOptions, opt)
				}
			}
			mount.Options = updatedOptions
			if uvmPath == "" {
				return fmt.Errorf("no uvmpath for bind mount %+v", mount)
			}
			md := hcsshim.MappedDir{
				HostPath:          mount.Source,
				ContainerPath:     path.Join(uvmPath, mount.Destination),
				CreateInUtilityVM: true,
				ReadOnly:          readonly,
			}
			mds = append(mds, md)
			specMount.Source = path.Join(uvmPath, mount.Destination)
		}
		specMounts = append(specMounts, specMount)
	}
	configuration.MappedDirectories = mds

	hcsContainer, err := hcsshim.CreateContainer(id, configuration)
	if err != nil {
		return err
	}

	spec.Mounts = specMounts

	// Construct a container object for calling start on it.
	ctr := &container{
		id:           id,
		execs:        make(map[string]*process),
		isWindows:    false,
		ociSpec:      spec,
		hcsContainer: hcsContainer,
		status:       StatusCreated,
		waitCh:       make(chan struct{}),
	}

	// Start the container. If this is a servicing container, this call
	// will block until the container is done with the servicing
	// execution.
	logger.Debug("starting container")
	if err = hcsContainer.Start(); err != nil {
		c.logger.WithError(err).Error("failed to start container")
		ctr.debugGCS()
		if err := c.terminateContainer(ctr); err != nil {
			c.logger.WithError(err).Error("failed to cleanup after a failed Start")
		} else {
			c.logger.Debug("cleaned up after failed Start by calling Terminate")
		}
		return err
	}
	ctr.debugGCS()

	c.Lock()
	c.containers[id] = ctr
	c.Unlock()

	c.eventQ.append(id, func() {
		ei := EventInfo{
			ContainerID: id,
		}
		c.logger.WithFields(logrus.Fields{
			"container": ctr.id,
			"event":     EventCreate,
		}).Info("sending event")
		err := c.backend.ProcessEvent(id, EventCreate, ei)
		if err != nil {
			c.logger.WithError(err).WithFields(logrus.Fields{
				"container": id,
				"event":     EventCreate,
			}).Error("failed to process event")
		}
	})

	logger.Debug("createLinux() completed successfully")
	return nil
}

func (c *client) Start(_ context.Context, id, _ string, withStdin bool, attachStdio StdioCallback) (int, error) {
	ctr := c.getContainer(id)
	switch {
	case ctr == nil:
		return -1, errors.WithStack(newNotFoundError("no such container"))
	case ctr.init != nil:
		return -1, errors.WithStack(newConflictError("container already started"))
	}

	logger := c.logger.WithField("container", id)

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
		createStdErrPipe = !ctr.ociSpec.Process.Terminal && !ctr.ociSpec.Windows.Servicing
	}

	createProcessParms := &hcsshim.ProcessConfig{
		EmulateConsole:   emulateConsole,
		WorkingDirectory: ctr.ociSpec.Process.Cwd,
		CreateStdInPipe:  !ctr.ociSpec.Windows.Servicing,
		CreateStdOutPipe: !ctr.ociSpec.Windows.Servicing,
		CreateStdErrPipe: createStdErrPipe,
	}

	if ctr.ociSpec.Process != nil && ctr.ociSpec.Process.ConsoleSize != nil {
		createProcessParms.ConsoleSize[0] = uint(ctr.ociSpec.Process.ConsoleSize.Height)
		createProcessParms.ConsoleSize[1] = uint(ctr.ociSpec.Process.ConsoleSize.Width)
	}

	// Configure the environment for the process
	createProcessParms.Environment = setupEnvironmentVariables(ctr.ociSpec.Process.Env)
	if ctr.isWindows {
		createProcessParms.CommandLine = strings.Join(ctr.ociSpec.Process.Args, " ")
	} else {
		createProcessParms.CommandArgs = ctr.ociSpec.Process.Args
	}
	createProcessParms.User = ctr.ociSpec.Process.User.Username

	// LCOW requires the raw OCI spec passed through HCS and onwards to
	// GCS for the utility VM.
	if !ctr.isWindows {
		ociBuf, err := json.Marshal(ctr.ociSpec)
		if err != nil {
			return -1, err
		}
		ociRaw := json.RawMessage(ociBuf)
		createProcessParms.OCISpecification = &ociRaw
	}

	ctr.Lock()
	defer ctr.Unlock()

	// Start the command running in the container.
	newProcess, err := ctr.hcsContainer.CreateProcess(createProcessParms)
	if err != nil {
		logger.WithError(err).Error("CreateProcess() failed")
		return -1, err
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
	p := &process{
		hcsProcess: newProcess,
		id:         InitProcessName,
		pid:        newProcess.Pid(),
	}
	logger.WithField("pid", p.pid).Debug("init process started")

	// If this is a servicing container, wait on the process synchronously here and
	// if it succeeds, wait for it cleanly shutdown and merge into the parent container.
	if ctr.ociSpec.Windows.Servicing {
		// reapProcess takes the lock
		ctr.Unlock()
		defer ctr.Lock()
		exitCode := c.reapProcess(ctr, p)

		if exitCode != 0 {
			return -1, errors.Errorf("libcontainerd: servicing container %s returned non-zero exit code %d", ctr.id, exitCode)
		}

		return p.pid, nil
	}

	dio, err := newIOFromProcess(newProcess, ctr.ociSpec.Process.Terminal)
	if err != nil {
		logger.WithError(err).Error("failed to get stdio pipes")
		return -1, err
	}
	_, err = attachStdio(dio)
	if err != nil {
		logger.WithError(err).Error("failed to attache stdio")
		return -1, err
	}
	ctr.status = StatusRunning
	ctr.init = p

	// Spin up a go routine waiting for exit to handle cleanup
	go c.reapProcess(ctr, p)

	// Generate the associated event
	c.eventQ.append(id, func() {
		ei := EventInfo{
			ContainerID: id,
			ProcessID:   InitProcessName,
			Pid:         uint32(p.pid),
		}
		c.logger.WithFields(logrus.Fields{
			"container":  ctr.id,
			"event":      EventStart,
			"event-info": ei,
		}).Info("sending event")
		err := c.backend.ProcessEvent(ei.ContainerID, EventStart, ei)
		if err != nil {
			c.logger.WithError(err).WithFields(logrus.Fields{
				"container":  id,
				"event":      EventStart,
				"event-info": ei,
			}).Error("failed to process event")
		}
	})
	logger.Debug("start() completed")
	return p.pid, nil
}

func newIOFromProcess(newProcess hcsshim.Process, terminal bool) (*cio.DirectIO, error) {
	stdin, stdout, stderr, err := newProcess.Stdio()
	if err != nil {
		return nil, err
	}

	dio := cio.NewDirectIO(createStdInCloser(stdin, newProcess), nil, nil, terminal)

	// Convert io.ReadClosers to io.Readers
	if stdout != nil {
		dio.Stdout = ioutil.NopCloser(&autoClosingReader{ReadCloser: stdout})
	}
	if stderr != nil {
		dio.Stderr = ioutil.NopCloser(&autoClosingReader{ReadCloser: stderr})
	}
	return dio, nil
}

// Exec adds a process in an running container
func (c *client) Exec(ctx context.Context, containerID, processID string, spec *specs.Process, withStdin bool, attachStdio StdioCallback) (int, error) {
	ctr := c.getContainer(containerID)
	switch {
	case ctr == nil:
		return -1, errors.WithStack(newNotFoundError("no such container"))
	case ctr.hcsContainer == nil:
		return -1, errors.WithStack(newInvalidParameterError("container is not running"))
	case ctr.execs != nil && ctr.execs[processID] != nil:
		return -1, errors.WithStack(newConflictError("id already in use"))
	}
	logger := c.logger.WithFields(logrus.Fields{
		"container": containerID,
		"exec":      processID,
	})

	// Note we always tell HCS to
	// create stdout as it's required regardless of '-i' or '-t' options, so that
	// docker can always grab the output through logs. We also tell HCS to always
	// create stdin, even if it's not used - it will be closed shortly. Stderr
	// is only created if it we're not -t.
	createProcessParms := hcsshim.ProcessConfig{
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
		createProcessParms.WorkingDirectory = ctr.ociSpec.Process.Cwd
	}

	// Configure the environment for the process
	createProcessParms.Environment = setupEnvironmentVariables(spec.Env)
	if ctr.isWindows {
		createProcessParms.CommandLine = strings.Join(spec.Args, " ")
	} else {
		createProcessParms.CommandArgs = spec.Args
	}
	createProcessParms.User = spec.User.Username

	logger.Debugf("exec commandLine: %s", createProcessParms.CommandLine)

	// Start the command running in the container.
	newProcess, err := ctr.hcsContainer.CreateProcess(&createProcessParms)
	if err != nil {
		logger.WithError(err).Errorf("exec's CreateProcess() failed")
		return -1, err
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
		return -1, err
	}
	// Tell the engine to attach streams back to the client
	_, err = attachStdio(dio)
	if err != nil {
		return -1, err
	}

	p := &process{
		id:         processID,
		pid:        pid,
		hcsProcess: newProcess,
	}

	// Add the process to the container's list of processes
	ctr.Lock()
	ctr.execs[processID] = p
	ctr.Unlock()

	// Spin up a go routine waiting for exit to handle cleanup
	go c.reapProcess(ctr, p)

	c.eventQ.append(ctr.id, func() {
		ei := EventInfo{
			ContainerID: ctr.id,
			ProcessID:   p.id,
			Pid:         uint32(p.pid),
		}
		c.logger.WithFields(logrus.Fields{
			"container":  ctr.id,
			"event":      EventExecAdded,
			"event-info": ei,
		}).Info("sending event")
		err := c.backend.ProcessEvent(ctr.id, EventExecAdded, ei)
		if err != nil {
			c.logger.WithError(err).WithFields(logrus.Fields{
				"container":  ctr.id,
				"event":      EventExecAdded,
				"event-info": ei,
			}).Error("failed to process event")
		}
		err = c.backend.ProcessEvent(ctr.id, EventExecStarted, ei)
		if err != nil {
			c.logger.WithError(err).WithFields(logrus.Fields{
				"container":  ctr.id,
				"event":      EventExecStarted,
				"event-info": ei,
			}).Error("failed to process event")
		}
	})

	return pid, nil
}

// Signal handles `docker stop` on Windows. While Linux has support for
// the full range of signals, signals aren't really implemented on Windows.
// We fake supporting regular stop and -9 to force kill.
func (c *client) SignalProcess(_ context.Context, containerID, processID string, signal int) error {
	ctr, p, err := c.getProcess(containerID, processID)
	if err != nil {
		return err
	}

	ctr.manualStopRequested = true

	logger := c.logger.WithFields(logrus.Fields{
		"container": containerID,
		"process":   processID,
		"pid":       p.pid,
		"signal":    signal,
	})
	logger.Debug("Signal()")

	if processID == InitProcessName {
		if syscall.Signal(signal) == syscall.SIGKILL {
			// Terminate the compute system
			if err := ctr.hcsContainer.Terminate(); err != nil {
				if !hcsshim.IsPending(err) {
					logger.WithError(err).Error("failed to terminate hccshim container")
				}
			}
		} else {
			// Shut down the container
			if err := ctr.hcsContainer.Shutdown(); err != nil {
				if !hcsshim.IsPending(err) && !hcsshim.IsAlreadyStopped(err) {
					// ignore errors
					logger.WithError(err).Error("failed to shutdown hccshim container")
				}
			}
		}
	} else {
		return p.hcsProcess.Kill()
	}

	return nil
}

// Resize handles a CLI event to resize an interactive docker run or docker
// exec window.
func (c *client) ResizeTerminal(_ context.Context, containerID, processID string, width, height int) error {
	_, p, err := c.getProcess(containerID, processID)
	if err != nil {
		return err
	}

	c.logger.WithFields(logrus.Fields{
		"container": containerID,
		"process":   processID,
		"height":    height,
		"width":     width,
		"pid":       p.pid,
	}).Debug("resizing")
	return p.hcsProcess.ResizeConsole(uint16(width), uint16(height))
}

func (c *client) CloseStdin(_ context.Context, containerID, processID string) error {
	_, p, err := c.getProcess(containerID, processID)
	if err != nil {
		return err
	}

	return p.hcsProcess.CloseStdin()
}

// Pause handles pause requests for containers
func (c *client) Pause(_ context.Context, containerID string) error {
	ctr, _, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return err
	}

	if ctr.ociSpec.Windows.HyperV == nil {
		return errors.New("cannot pause Windows Server Containers")
	}

	ctr.Lock()
	defer ctr.Unlock()

	if err = ctr.hcsContainer.Pause(); err != nil {
		return err
	}

	ctr.status = StatusPaused

	c.eventQ.append(containerID, func() {
		err := c.backend.ProcessEvent(containerID, EventPaused, EventInfo{
			ContainerID: containerID,
			ProcessID:   InitProcessName,
		})
		c.logger.WithFields(logrus.Fields{
			"container": ctr.id,
			"event":     EventPaused,
		}).Info("sending event")
		if err != nil {
			c.logger.WithError(err).WithFields(logrus.Fields{
				"container": containerID,
				"event":     EventPaused,
			}).Error("failed to process event")
		}
	})

	return nil
}

// Resume handles resume requests for containers
func (c *client) Resume(_ context.Context, containerID string) error {
	ctr, _, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return err
	}

	if ctr.ociSpec.Windows.HyperV == nil {
		return errors.New("cannot resume Windows Server Containers")
	}

	ctr.Lock()
	defer ctr.Unlock()

	if err = ctr.hcsContainer.Resume(); err != nil {
		return err
	}

	ctr.status = StatusRunning

	c.eventQ.append(containerID, func() {
		err := c.backend.ProcessEvent(containerID, EventResumed, EventInfo{
			ContainerID: containerID,
			ProcessID:   InitProcessName,
		})
		c.logger.WithFields(logrus.Fields{
			"container": ctr.id,
			"event":     EventResumed,
		}).Info("sending event")
		if err != nil {
			c.logger.WithError(err).WithFields(logrus.Fields{
				"container": containerID,
				"event":     EventResumed,
			}).Error("failed to process event")
		}
	})

	return nil
}

// Stats handles stats requests for containers
func (c *client) Stats(_ context.Context, containerID string) (*Stats, error) {
	ctr, _, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return nil, err
	}

	readAt := time.Now()
	s, err := ctr.hcsContainer.Statistics()
	if err != nil {
		return nil, err
	}
	return &Stats{
		Read:     readAt,
		HCSStats: &s,
	}, nil
}

// Restore is the handler for restoring a container
func (c *client) Restore(ctx context.Context, id string, attachStdio StdioCallback) (bool, int, error) {
	c.logger.WithField("container", id).Debug("restore()")

	// TODO Windows: On RS1, a re-attach isn't possible.
	// However, there is a scenario in which there is an issue.
	// Consider a background container. The daemon dies unexpectedly.
	// HCS will still have the compute service alive and running.
	// For consistence, we call in to shoot it regardless if HCS knows about it
	// We explicitly just log a warning if the terminate fails.
	// Then we tell the backend the container exited.
	if hc, err := hcsshim.OpenContainer(id); err == nil {
		const terminateTimeout = time.Minute * 2
		err := hc.Terminate()

		if hcsshim.IsPending(err) {
			err = hc.WaitTimeout(terminateTimeout)
		} else if hcsshim.IsAlreadyStopped(err) {
			err = nil
		}

		if err != nil {
			c.logger.WithField("container", id).WithError(err).Debug("terminate failed on restore")
			return false, -1, err
		}
	}
	return false, -1, nil
}

// GetPidsForContainer returns a list of process IDs running in a container.
// Not used on Windows.
func (c *client) ListPids(_ context.Context, _ string) ([]uint32, error) {
	return nil, errors.New("not implemented on Windows")
}

// Summary returns a summary of the processes running in a container.
// This is present in Windows to support docker top. In linux, the
// engine shells out to ps to get process information. On Windows, as
// the containers could be Hyper-V containers, they would not be
// visible on the container host. However, libcontainerd does have
// that information.
func (c *client) Summary(_ context.Context, containerID string) ([]Summary, error) {
	ctr, _, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return nil, err
	}

	p, err := ctr.hcsContainer.ProcessList()
	if err != nil {
		return nil, err
	}

	pl := make([]Summary, len(p))
	for i := range p {
		pl[i] = Summary(p[i])
	}
	return pl, nil
}

func (c *client) DeleteTask(ctx context.Context, containerID string) (uint32, time.Time, error) {
	ec := -1
	ctr := c.getContainer(containerID)
	if ctr == nil {
		return uint32(ec), time.Now(), errors.WithStack(newNotFoundError("no such container"))
	}

	select {
	case <-ctx.Done():
		return uint32(ec), time.Now(), errors.WithStack(ctx.Err())
	case <-ctr.waitCh:
	default:
		return uint32(ec), time.Now(), errors.New("container is not stopped")
	}

	ctr.Lock()
	defer ctr.Unlock()
	return ctr.exitCode, ctr.exitedAt, nil
}

func (c *client) Delete(_ context.Context, containerID string) error {
	c.Lock()
	defer c.Unlock()
	ctr := c.containers[containerID]
	if ctr == nil {
		return errors.WithStack(newNotFoundError("no such container"))
	}

	ctr.Lock()
	defer ctr.Unlock()

	switch ctr.status {
	case StatusCreated:
		if err := c.shutdownContainer(ctr); err != nil {
			return err
		}
		fallthrough
	case StatusStopped:
		delete(c.containers, containerID)
		return nil
	}

	return errors.WithStack(newInvalidParameterError("container is not stopped"))
}

func (c *client) Status(ctx context.Context, containerID string) (Status, error) {
	c.Lock()
	defer c.Unlock()
	ctr := c.containers[containerID]
	if ctr == nil {
		return StatusUnknown, errors.WithStack(newNotFoundError("no such container"))
	}

	ctr.Lock()
	defer ctr.Unlock()
	return ctr.status, nil
}

func (c *client) UpdateResources(ctx context.Context, containerID string, resources *Resources) error {
	// Updating resource isn't supported on Windows
	// but we should return nil for enabling updating container
	return nil
}

func (c *client) CreateCheckpoint(ctx context.Context, containerID, checkpointDir string, exit bool) error {
	return errors.New("Windows: Containers do not support checkpoints")
}

func (c *client) getContainer(id string) *container {
	c.Lock()
	ctr := c.containers[id]
	c.Unlock()

	return ctr
}

func (c *client) getProcess(containerID, processID string) (*container, *process, error) {
	ctr := c.getContainer(containerID)
	switch {
	case ctr == nil:
		return nil, nil, errors.WithStack(newNotFoundError("no such container"))
	case ctr.init == nil:
		return nil, nil, errors.WithStack(newNotFoundError("container is not running"))
	case processID == InitProcessName:
		return ctr, ctr.init, nil
	default:
		ctr.Lock()
		defer ctr.Unlock()
		if ctr.execs == nil {
			return nil, nil, errors.WithStack(newNotFoundError("no execs"))
		}
	}

	p := ctr.execs[processID]
	if p == nil {
		return nil, nil, errors.WithStack(newNotFoundError("no such exec"))
	}

	return ctr, p, nil
}

func (c *client) shutdownContainer(ctr *container) error {
	const shutdownTimeout = time.Minute * 5
	err := ctr.hcsContainer.Shutdown()

	if hcsshim.IsPending(err) {
		err = ctr.hcsContainer.WaitTimeout(shutdownTimeout)
	} else if hcsshim.IsAlreadyStopped(err) {
		err = nil
	}

	if err != nil {
		c.logger.WithError(err).WithField("container", ctr.id).
			Debug("failed to shutdown container, terminating it")
		return c.terminateContainer(ctr)
	}

	return nil
}

func (c *client) terminateContainer(ctr *container) error {
	const terminateTimeout = time.Minute * 5
	err := ctr.hcsContainer.Terminate()

	if hcsshim.IsPending(err) {
		err = ctr.hcsContainer.WaitTimeout(terminateTimeout)
	} else if hcsshim.IsAlreadyStopped(err) {
		err = nil
	}

	if err != nil {
		c.logger.WithError(err).WithField("container", ctr.id).
			Debug("failed to terminate container")
		return err
	}

	return nil
}

func (c *client) reapProcess(ctr *container, p *process) int {
	logger := c.logger.WithFields(logrus.Fields{
		"container": ctr.id,
		"process":   p.id,
	})

	// Block indefinitely for the process to exit.
	if err := p.hcsProcess.Wait(); err != nil {
		if herr, ok := err.(*hcsshim.ProcessError); ok && herr.Err != windows.ERROR_BROKEN_PIPE {
			logger.WithError(err).Warnf("Wait() failed (container may have been killed)")
		}
		// Fall through here, do not return. This ensures we attempt to
		// continue the shutdown in HCS and tell the docker engine that the
		// process/container has exited to avoid a container being dropped on
		// the floor.
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

		// Fall through here, do not return. This ensures we attempt to
		// continue the shutdown in HCS and tell the docker engine that the
		// process/container has exited to avoid a container being dropped on
		// the floor.
	}

	if err := p.hcsProcess.Close(); err != nil {
		logger.WithError(err).Warnf("failed to cleanup hcs process resources")
	}

	var pendingUpdates bool
	if p.id == InitProcessName {
		// Update container status
		ctr.Lock()
		ctr.status = StatusStopped
		ctr.exitedAt = exitedAt
		ctr.exitCode = uint32(exitCode)
		close(ctr.waitCh)
		ctr.Unlock()

		// Handle any servicing
		if exitCode == 0 && ctr.isWindows && !ctr.ociSpec.Windows.Servicing {
			pendingUpdates, err = ctr.hcsContainer.HasPendingUpdates()
			logger.Infof("Pending updates: %v", pendingUpdates)
			if err != nil {
				logger.WithError(err).
					Warnf("failed to check for pending updates (container may have been killed)")
			}
		}

		if err := c.shutdownContainer(ctr); err != nil {
			logger.WithError(err).Warn("failed to shutdown container")
		} else {
			logger.Debug("completed container shutdown")
		}

		if err := ctr.hcsContainer.Close(); err != nil {
			logger.WithError(err).Error("failed to clean hcs container resources")
		}
	}

	if !(ctr.isWindows && ctr.ociSpec.Windows.Servicing) {
		c.eventQ.append(ctr.id, func() {
			ei := EventInfo{
				ContainerID:   ctr.id,
				ProcessID:     p.id,
				Pid:           uint32(p.pid),
				ExitCode:      uint32(exitCode),
				ExitedAt:      exitedAt,
				UpdatePending: pendingUpdates,
			}
			c.logger.WithFields(logrus.Fields{
				"container":  ctr.id,
				"event":      EventExit,
				"event-info": ei,
			}).Info("sending event")
			err := c.backend.ProcessEvent(ctr.id, EventExit, ei)
			if err != nil {
				c.logger.WithError(err).WithFields(logrus.Fields{
					"container":  ctr.id,
					"event":      EventExit,
					"event-info": ei,
				}).Error("failed to process event")
			}
			if p.id != InitProcessName {
				ctr.Lock()
				delete(ctr.execs, p.id)
				ctr.Unlock()
			}
		})
	}

	return exitCode
}
