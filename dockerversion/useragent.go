package dockerversion

import (
	"runtime"

	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/useragent"
)

// DockerUserAgent is the User-Agent the Docker client uses to identify itself.
// It is populated from version information of different components.
func DockerUserAgent() string {
	httpVersion := make([]useragent.VersionInfo, 0, 6)
	httpVersion = append(httpVersion, useragent.VersionInfo{Name: "docker", Version: Version})
	httpVersion = append(httpVersion, useragent.VersionInfo{Name: "go", Version: runtime.Version()})
	httpVersion = append(httpVersion, useragent.VersionInfo{Name: "git-commit", Version: GitCommit})
	if kernelVersion, err := kernel.GetKernelVersion(); err == nil {
		httpVersion = append(httpVersion, useragent.VersionInfo{Name: "kernel", Version: kernelVersion.String()})
	}
	httpVersion = append(httpVersion, useragent.VersionInfo{Name: "os", Version: runtime.GOOS})
	httpVersion = append(httpVersion, useragent.VersionInfo{Name: "arch", Version: runtime.GOARCH})

	return useragent.AppendVersions("", httpVersion...)
}
