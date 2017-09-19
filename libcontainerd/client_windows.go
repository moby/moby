package libcontainerd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/context"

	"github.com/Microsoft/hcsshim"
	opengcs "github.com/Microsoft/opengcs/client"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
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

// defaultOwner is a tag passed to HCS to allow it to differentiate between
// container creator management stacks. We hard code "docker" in the case
// of docker.
const defaultOwner = "docker"

// Create is the entrypoint to create a container from a spec, and if successfully
// created, start it too. Table below shows the fields required for HCS JSON calling parameters,
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
func (clnt *client) Create(containerID string, checkpoint string, checkpointDir string, spec specs.Spec, attachStdio StdioCallback, options ...CreateOption) error {
	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	if b, err := json.Marshal(spec); err == nil {
		logrus.Debugln("libcontainerd: client.Create() with spec", string(b))
	}

	// spec.Linux must be nil for Windows containers, but spec.Windows will be filled in regardless of container platform.
	// This is a temporary workaround due to LCOW requiring layer folder paths, which are stored under spec.Windows.
	// TODO: @darrenstahlmsft fix this once the OCI spec is updated to support layer folder paths for LCOW
	if spec.Linux == nil {
		return clnt.createWindows(containerID, checkpoint, checkpointDir, spec, attachStdio, options...)
	}
	return clnt.createLinux(containerID, checkpoint, checkpointDir, spec, attachStdio, options...)
}

func (clnt *client) createWindows(containerID string, checkpoint string, checkpointDir string, spec specs.Spec, attachStdio StdioCallback, options ...CreateOption) error {
	configuration := &hcsshim.ContainerConfig{
		SystemType: "Container",
		Name:       containerID,
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
					logrus.Warnf("Changing requested CPUCount of %d to current number of processors, %d", cpuCount, hostCPUCount)
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

	// We must have least two layers in the spec, the bottom one being a base image,
	// the top one being the RW layer.
	if spec.Windows.LayerFolders == nil || len(spec.Windows.LayerFolders) < 2 {
		return fmt.Errorf("OCI spec is invalid - at least two LayerFolders must be supplied to the runtime")
	}

	// Strip off the top-most layer as that's passed in separately to HCS
	configuration.LayerFolderPath = spec.Windows.LayerFolders[len(spec.Windows.LayerFolders)-1]
	layerFolders := spec.Windows.LayerFolders[:len(spec.Windows.LayerFolders)-1]

	if configuration.HvPartition {
		// We don't currently support setting the utility VM image explicitly.
		// TODO @swernli/jhowardmsft circa RS3/4, this may be re-locatable.
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
	if len(mps) > 0 && system.GetOSVersion().Build < 16210 { // replace with Win10 RS3 build number at RTM
		return errors.New("named pipe mounts are not supported on this version of Windows")
	}
	configuration.MappedPipes = mps

	hcsContainer, err := hcsshim.CreateContainer(containerID, configuration)
	if err != nil {
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
			},
			processes: make(map[string]*process),
		},
		isWindows:    true,
		ociSpec:      spec,
		hcsContainer: hcsContainer,
	}

	container.options = options
	for _, option := range options {
		if err := option.Apply(container); err != nil {
			logrus.Errorf("libcontainerd: %v", err)
		}
	}

	// Call start, and if it fails, delete the container from our
	// internal structure, start will keep HCS in sync by deleting the
	// container there.
	logrus.Debugf("libcontainerd: createWindows() id=%s, Calling start()", containerID)
	if err := container.start(attachStdio); err != nil {
		clnt.deleteContainer(containerID)
		return err
	}

	logrus.Debugf("libcontainerd: createWindows() id=%s completed successfully", containerID)
	return nil

}

func (clnt *client) createLinux(containerID string, checkpoint string, checkpointDir string, spec specs.Spec, attachStdio StdioCallback, options ...CreateOption) error {
	logrus.Debugf("libcontainerd: createLinux(): containerId %s ", containerID)

	var lcowOpt *LCOWOption
	for _, option := range options {
		if lcow, ok := option.(*LCOWOption); ok {
			lcowOpt = lcow
		}
	}
	if lcowOpt == nil || lcowOpt.Config == nil {
		return fmt.Errorf("lcow option must be supplied to the runtime")
	}

	configuration := &hcsshim.ContainerConfig{
		HvPartition:   true,
		Name:          containerID,
		SystemType:    "container",
		ContainerType: "linux",
		Owner:         defaultOwner,
		TerminateOnLastHandleClosed: true,
	}

	if lcowOpt.Config.ActualMode == opengcs.ModeActualVhdx {
		configuration.HvRuntime = &hcsshim.HvRuntime{
			ImagePath:          lcowOpt.Config.Vhdx,
			BootSource:         "Vhd",
			WritableBootSource: false,
		}
	} else {
		configuration.HvRuntime = &hcsshim.HvRuntime{
			ImagePath:           lcowOpt.Config.KirdPath,
			LinuxKernelFile:     lcowOpt.Config.KernelFile,
			LinuxInitrdFile:     lcowOpt.Config.InitrdFile,
			LinuxBootParameters: lcowOpt.Config.BootParameters,
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

	hcsContainer, err := hcsshim.CreateContainer(containerID, configuration)
	if err != nil {
		return err
	}

	spec.Mounts = specMounts

	// Construct a container object for calling start on it.
	container := &container{
		containerCommon: containerCommon{
			process: process{
				processCommon: processCommon{
					containerID:  containerID,
					client:       clnt,
					friendlyName: InitFriendlyName,
				},
			},
			processes: make(map[string]*process),
		},
		ociSpec:      spec,
		hcsContainer: hcsContainer,
	}

	container.options = options
	for _, option := range options {
		if err := option.Apply(container); err != nil {
			logrus.Errorf("libcontainerd: createLinux() %v", err)
		}
	}

	// Call start, and if it fails, delete the container from our
	// internal structure, start will keep HCS in sync by deleting the
	// container there.
	logrus.Debugf("libcontainerd: createLinux() id=%s, Calling start()", containerID)
	if err := container.start(attachStdio); err != nil {
		clnt.deleteContainer(containerID)
		return err
	}

	logrus.Debugf("libcontainerd: createLinux() id=%s completed successfully", containerID)
	return nil
}

// AddProcess is the handler for adding a process to an already running
// container. It's called through docker exec. It returns the system pid of the
// exec'd process.
func (clnt *client) AddProcess(ctx context.Context, containerID, processFriendlyName string, procToAdd Process, attachStdio StdioCallback) (int, error) {
	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	container, err := clnt.getContainer(containerID)
	if err != nil {
		return -1, err
	}

	defer container.debugGCS()

	// Note we always tell HCS to
	// create stdout as it's required regardless of '-i' or '-t' options, so that
	// docker can always grab the output through logs. We also tell HCS to always
	// create stdin, even if it's not used - it will be closed shortly. Stderr
	// is only created if it we're not -t.
	createProcessParms := hcsshim.ProcessConfig{
		CreateStdInPipe:  true,
		CreateStdOutPipe: true,
		CreateStdErrPipe: !procToAdd.Terminal,
	}
	if procToAdd.Terminal {
		createProcessParms.EmulateConsole = true
		if procToAdd.ConsoleSize != nil {
			createProcessParms.ConsoleSize[0] = uint(procToAdd.ConsoleSize.Height)
			createProcessParms.ConsoleSize[1] = uint(procToAdd.ConsoleSize.Width)
		}
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
	if container.isWindows {
		createProcessParms.CommandLine = strings.Join(procToAdd.Args, " ")
	} else {
		createProcessParms.CommandArgs = procToAdd.Args
	}
	createProcessParms.User = procToAdd.User.Username

	logrus.Debugf("libcontainerd: commandLine: %s", createProcessParms.CommandLine)

	// Start the command running in the container.
	var stdout, stderr io.ReadCloser
	var stdin io.WriteCloser
	newProcess, err := container.hcsContainer.CreateProcess(&createProcessParms)
	if err != nil {
		logrus.Errorf("libcontainerd: AddProcess(%s) CreateProcess() failed %s", containerID, err)
		return -1, err
	}

	pid := newProcess.Pid()

	stdin, stdout, stderr, err = newProcess.Stdio()
	if err != nil {
		logrus.Errorf("libcontainerd: %s getting std pipes failed %s", containerID, err)
		return -1, err
	}

	iopipe := &IOPipe{Terminal: procToAdd.Terminal}
	iopipe.Stdin = createStdInCloser(stdin, newProcess)

	// Convert io.ReadClosers to io.Readers
	if stdout != nil {
		iopipe.Stdout = ioutil.NopCloser(&autoClosingReader{ReadCloser: stdout})
	}
	if stderr != nil {
		iopipe.Stderr = ioutil.NopCloser(&autoClosingReader{ReadCloser: stderr})
	}

	proc := &process{
		processCommon: processCommon{
			containerID:  containerID,
			friendlyName: processFriendlyName,
			client:       clnt,
			systemPid:    uint32(pid),
		},
		hcsProcess: newProcess,
	}

	// Add the process to the container's list of processes
	container.processes[processFriendlyName] = proc

	// Tell the engine to attach streams back to the client
	if err := attachStdio(*iopipe); err != nil {
		return -1, err
	}

	// Spin up a go routine waiting for exit to handle cleanup
	go container.waitExit(proc, false)

	return pid, nil
}

// Signal handles `docker stop` on Windows. While Linux has support for
// the full range of signals, signals aren't really implemented on Windows.
// We fake supporting regular stop and -9 to force kill.
func (clnt *client) Signal(containerID string, sig int) error {
	var (
		cont *container
		err  error
	)

	// Get the container as we need it to get the container handle.
	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	if cont, err = clnt.getContainer(containerID); err != nil {
		return err
	}

	cont.manualStopRequested = true

	logrus.Debugf("libcontainerd: Signal() containerID=%s sig=%d pid=%d", containerID, sig, cont.systemPid)

	if syscall.Signal(sig) == syscall.SIGKILL {
		// Terminate the compute system
		if err := cont.hcsContainer.Terminate(); err != nil {
			if !hcsshim.IsPending(err) {
				logrus.Errorf("libcontainerd: failed to terminate %s - %q", containerID, err)
			}
		}
	} else {
		// Shut down the container
		if err := cont.hcsContainer.Shutdown(); err != nil {
			if !hcsshim.IsPending(err) && !hcsshim.IsAlreadyStopped(err) {
				// ignore errors
				logrus.Warnf("libcontainerd: failed to shutdown container %s: %q", containerID, err)
			}
		}
	}

	return nil
}

// While Linux has support for the full range of signals, signals aren't really implemented on Windows.
// We try to terminate the specified process whatever signal is requested.
func (clnt *client) SignalProcess(containerID string, processFriendlyName string, sig int) error {
	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	cont, err := clnt.getContainer(containerID)
	if err != nil {
		return err
	}

	for _, p := range cont.processes {
		if p.friendlyName == processFriendlyName {
			return p.hcsProcess.Kill()
		}
	}

	return fmt.Errorf("SignalProcess could not find process %s in %s", processFriendlyName, containerID)
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

	h, w := uint16(height), uint16(width)

	if processFriendlyName == InitFriendlyName {
		logrus.Debugln("libcontainerd: resizing systemPID in", containerID, cont.process.systemPid)
		return cont.process.hcsProcess.ResizeConsole(w, h)
	}

	for _, p := range cont.processes {
		if p.friendlyName == processFriendlyName {
			logrus.Debugln("libcontainerd: resizing exec'd process", containerID, p.systemPid)
			return p.hcsProcess.ResizeConsole(w, h)
		}
	}

	return fmt.Errorf("Resize could not find containerID %s to resize", containerID)

}

// Pause handles pause requests for containers
func (clnt *client) Pause(containerID string) error {
	unlockContainer := true
	// Get the libcontainerd container object
	clnt.lock(containerID)
	defer func() {
		if unlockContainer {
			clnt.unlock(containerID)
		}
	}()
	container, err := clnt.getContainer(containerID)
	if err != nil {
		return err
	}

	if container.ociSpec.Windows.HyperV == nil {
		return errors.New("cannot pause Windows Server Containers")
	}

	err = container.hcsContainer.Pause()
	if err != nil {
		return err
	}

	// Unlock container before calling back into the daemon
	unlockContainer = false
	clnt.unlock(containerID)

	return clnt.backend.StateChanged(containerID, StateInfo{
		CommonStateInfo: CommonStateInfo{
			State: StatePause,
		}})
}

// Resume handles resume requests for containers
func (clnt *client) Resume(containerID string) error {
	unlockContainer := true
	// Get the libcontainerd container object
	clnt.lock(containerID)
	defer func() {
		if unlockContainer {
			clnt.unlock(containerID)
		}
	}()
	container, err := clnt.getContainer(containerID)
	if err != nil {
		return err
	}

	// This should never happen, since Windows Server Containers cannot be paused

	if container.ociSpec.Windows.HyperV == nil {
		return errors.New("cannot resume Windows Server Containers")
	}

	err = container.hcsContainer.Resume()
	if err != nil {
		return err
	}

	// Unlock container before calling back into the daemon
	unlockContainer = false
	clnt.unlock(containerID)

	return clnt.backend.StateChanged(containerID, StateInfo{
		CommonStateInfo: CommonStateInfo{
			State: StateResume,
		}})
}

// Stats handles stats requests for containers
func (clnt *client) Stats(containerID string) (*Stats, error) {
	// Get the libcontainerd container object
	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	container, err := clnt.getContainer(containerID)
	if err != nil {
		return nil, err
	}
	s, err := container.hcsContainer.Statistics()
	if err != nil {
		return nil, err
	}
	st := Stats(s)
	return &st, nil
}

// Restore is the handler for restoring a container
func (clnt *client) Restore(containerID string, _ StdioCallback, unusedOnWindows ...CreateOption) error {
	logrus.Debugf("libcontainerd: Restore(%s)", containerID)

	// TODO Windows: On RS1, a re-attach isn't possible.
	// However, there is a scenario in which there is an issue.
	// Consider a background container. The daemon dies unexpectedly.
	// HCS will still have the compute service alive and running.
	// For consistence, we call in to shoot it regardless if HCS knows about it
	// We explicitly just log a warning if the terminate fails.
	// Then we tell the backend the container exited.
	if hc, err := hcsshim.OpenContainer(containerID); err == nil {
		const terminateTimeout = time.Minute * 2
		err := hc.Terminate()

		if hcsshim.IsPending(err) {
			err = hc.WaitTimeout(terminateTimeout)
		} else if hcsshim.IsAlreadyStopped(err) {
			err = nil
		}

		if err != nil {
			logrus.Warnf("libcontainerd: failed to terminate %s on restore - %q", containerID, err)
			return err
		}
	}
	return clnt.backend.StateChanged(containerID, StateInfo{
		CommonStateInfo: CommonStateInfo{
			State:    StateExit,
			ExitCode: 1 << 31,
		}})
}

// GetPidsForContainer returns a list of process IDs running in a container.
// Not used on Windows.
func (clnt *client) GetPidsForContainer(containerID string) ([]int, error) {
	return nil, errors.New("not implemented on Windows")
}

// Summary returns a summary of the processes running in a container.
// This is present in Windows to support docker top. In linux, the
// engine shells out to ps to get process information. On Windows, as
// the containers could be Hyper-V containers, they would not be
// visible on the container host. However, libcontainerd does have
// that information.
func (clnt *client) Summary(containerID string) ([]Summary, error) {

	// Get the libcontainerd container object
	clnt.lock(containerID)
	defer clnt.unlock(containerID)
	container, err := clnt.getContainer(containerID)
	if err != nil {
		return nil, err
	}
	p, err := container.hcsContainer.ProcessList()
	if err != nil {
		return nil, err
	}
	pl := make([]Summary, len(p))
	for i := range p {
		pl[i] = Summary(p[i])
	}
	return pl, nil
}

// UpdateResources updates resources for a running container.
func (clnt *client) UpdateResources(containerID string, resources Resources) error {
	// Updating resource isn't supported on Windows
	// but we should return nil for enabling updating container
	return nil
}

func (clnt *client) CreateCheckpoint(containerID string, checkpointID string, checkpointDir string, exit bool) error {
	return errors.New("Windows: Containers do not support checkpoints")
}

func (clnt *client) DeleteCheckpoint(containerID string, checkpointID string, checkpointDir string) error {
	return errors.New("Windows: Containers do not support checkpoints")
}

func (clnt *client) ListCheckpoints(containerID string, checkpointDir string) (*Checkpoints, error) {
	return nil, errors.New("Windows: Containers do not support checkpoints")
}

func (clnt *client) GetServerVersion(ctx context.Context) (*ServerVersion, error) {
	return &ServerVersion{}, nil
}
