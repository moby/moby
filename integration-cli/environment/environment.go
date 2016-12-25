package environment

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// Execution holds informations about the test execution environment.
type Execution struct {
	daemonPlatform      string
	localDaemon         bool
	experimentalDaemon  bool
	daemonStorageDriver string
	isolation           container.Isolation
	daemonPid           int
	daemonKernelVersion string
	// For a local daemon on Linux, these values will be used for testing
	// user namespace support as the standard graph path(s) will be
	// appended with the root remapped uid.gid prefix
	dockerBasePath       string
	volumesConfigPath    string
	containerStoragePath string
	// baseImage is the name of the base image for testing
	// Environment variable WINDOWS_BASE_IMAGE can override this
	baseImage string
}

// New creates a new Execution struct
func New() (*Execution, error) {
	localDaemon := true
	// Deterministically working out the environment in which CI is running
	// to evaluate whether the daemon is local or remote is not possible through
	// a build tag.
	//
	// For example Windows to Linux CI under Jenkins tests the 64-bit
	// Windows binary build with the daemon build tag, but calls a remote
	// Linux daemon.
	//
	// We can't just say if Windows then assume the daemon is local as at
	// some point, we will be testing the Windows CLI against a Windows daemon.
	//
	// Similarly, it will be perfectly valid to also run CLI tests from
	// a Linux CLI (built with the daemon tag) against a Windows daemon.
	if len(os.Getenv("DOCKER_REMOTE_DAEMON")) > 0 {
		localDaemon = false
	}
	info, err := getDaemonDockerInfo()
	if err != nil {
		return nil, err
	}
	daemonPlatform := info.OSType
	if daemonPlatform != "linux" && daemonPlatform != "windows" {
		return nil, fmt.Errorf("Cannot run tests against platform: %s", daemonPlatform)
	}
	baseImage := "scratch"
	volumesConfigPath := filepath.Join(info.DockerRootDir, "volumes")
	containerStoragePath := filepath.Join(info.DockerRootDir, "containers")
	// Make sure in context of daemon, not the local platform. Note we can't
	// use filepath.FromSlash or ToSlash here as they are a no-op on Unix.
	if daemonPlatform == "windows" {
		volumesConfigPath = strings.Replace(volumesConfigPath, `/`, `\`, -1)
		containerStoragePath = strings.Replace(containerStoragePath, `/`, `\`, -1)

		baseImage = "microsoft/windowsservercore"
		if len(os.Getenv("WINDOWS_BASE_IMAGE")) > 0 {
			baseImage = os.Getenv("WINDOWS_BASE_IMAGE")
			fmt.Println("INFO: Windows Base image is ", baseImage)
		}
	} else {
		volumesConfigPath = strings.Replace(volumesConfigPath, `\`, `/`, -1)
		containerStoragePath = strings.Replace(containerStoragePath, `\`, `/`, -1)
	}

	var daemonPid int
	dest := os.Getenv("DEST")
	b, err := ioutil.ReadFile(filepath.Join(dest, "docker.pid"))
	if err == nil {
		if p, err := strconv.ParseInt(string(b), 10, 32); err == nil {
			daemonPid = int(p)
		}
	}
	return &Execution{
		localDaemon:          localDaemon,
		daemonPlatform:       daemonPlatform,
		daemonStorageDriver:  info.Driver,
		daemonKernelVersion:  info.KernelVersion,
		dockerBasePath:       info.DockerRootDir,
		volumesConfigPath:    volumesConfigPath,
		containerStoragePath: containerStoragePath,
		isolation:            info.Isolation,
		daemonPid:            daemonPid,
		experimentalDaemon:   info.ExperimentalBuild,
		baseImage:            baseImage,
	}, nil
}
func getDaemonDockerInfo() (types.Info, error) {
	// FIXME(vdemeester) should be safe to use as is
	client, err := client.NewEnvClient()
	if err != nil {
		return types.Info{}, err
	}
	return client.Info(context.Background())
}

// LocalDaemon is true if the daemon under test is on the same
// host as the CLI.
func (e *Execution) LocalDaemon() bool {
	return e.localDaemon
}

// DaemonPlatform is held globally so that tests can make intelligent
// decisions on how to configure themselves according to the platform
// of the daemon. This is initialized in docker_utils by sending
// a version call to the daemon and examining the response header.
func (e *Execution) DaemonPlatform() string {
	return e.daemonPlatform
}

// DockerBasePath is the base path of the docker folder (by default it is -/var/run/docker)
func (e *Execution) DockerBasePath() string {
	return e.dockerBasePath
}

// VolumesConfigPath is the path of the volume configuration for the testing daemon
func (e *Execution) VolumesConfigPath() string {
	return e.volumesConfigPath
}

// ContainerStoragePath is the path where the container are stored for the testing daemon
func (e *Execution) ContainerStoragePath() string {
	return e.containerStoragePath
}

// DaemonStorageDriver is held globally so that tests can know the storage
// driver of the daemon. This is initialized in docker_utils by sending
// a version call to the daemon and examining the response header.
func (e *Execution) DaemonStorageDriver() string {
	return e.daemonStorageDriver
}

// Isolation is the isolation mode of the daemon under test
func (e *Execution) Isolation() container.Isolation {
	return e.isolation
}

// DaemonPID is the pid of the main test daemon
func (e *Execution) DaemonPID() int {
	return e.daemonPid
}

// ExperimentalDaemon tell whether the main daemon has
// experimental features enabled or not
func (e *Execution) ExperimentalDaemon() bool {
	return e.experimentalDaemon
}

// MinimalBaseImage is the image used for minimal builds (it depends on the platform)
func (e *Execution) MinimalBaseImage() string {
	return e.baseImage
}

// DaemonKernelVersion is the kernel version of the daemon
func (e *Execution) DaemonKernelVersion() string {
	return e.daemonKernelVersion
}

// WindowsKernelVersion is used on Windows to distinguish between different
// versions. This is necessary to enable certain tests based on whether
// the platform supports it. For example, Windows Server 2016 TP3 did
// not support volumes, but TP4 did.
func WindowsKernelVersion(kernelVersion string) int {
	winKV, _ := strconv.Atoi(strings.Split(kernelVersion, " ")[1])
	return winKV
}
