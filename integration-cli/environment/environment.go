package environment

import (
	"os"

	"os/exec"

	"github.com/docker/docker/internal/test/environment"
)

var (
	// DefaultClientBinary is the name of the docker binary
	DefaultClientBinary = os.Getenv("TEST_CLIENT_BINARY")
)

func init() {
	if DefaultClientBinary == "" {
		DefaultClientBinary = "docker"
	}
}

// Execution contains information about the current test execution and daemon
// under test
type Execution struct {
	environment.Execution
	dockerBinary string
}

// DockerBinary returns the docker binary for this testing environment
func (e *Execution) DockerBinary() string {
	return e.dockerBinary
}

// New returns details about the testing environment
func New() (*Execution, error) {
	env, err := environment.New()
	if err != nil {
		return nil, err
	}

	dockerBinary, err := exec.LookPath(DefaultClientBinary)
	if err != nil {
		return nil, err
	}

	return &Execution{
		Execution:    *env,
		dockerBinary: dockerBinary,
	}, nil
}

// DockerBasePath is the base path of the docker folder (by default it is -/var/run/docker)
// TODO: remove
// Deprecated: use Execution.DaemonInfo.DockerRootDir
func (e *Execution) DockerBasePath() string {
	return e.DaemonInfo.DockerRootDir
}

// ExperimentalDaemon tell whether the main daemon has
// experimental features enabled or not
// Deprecated: use DaemonInfo.ExperimentalBuild
func (e *Execution) ExperimentalDaemon() bool {
	return e.DaemonInfo.ExperimentalBuild
}

// DaemonPlatform is held globally so that tests can make intelligent
// decisions on how to configure themselves according to the platform
// of the daemon. This is initialized in docker_utils by sending
// a version call to the daemon and examining the response header.
// Deprecated: use Execution.OSType
func (e *Execution) DaemonPlatform() string {
	return e.OSType
}

// MinimalBaseImage is the image used for minimal builds (it depends on the platform)
// Deprecated: use Execution.PlatformDefaults.BaseImage
func (e *Execution) MinimalBaseImage() string {
	return e.PlatformDefaults.BaseImage
}
