package dockerversion

import (
	"runtime"

	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/useragent"
	"golang.org/x/net/context"
)

// uaStringKey is used as key type for user agent string in net/context struct
type uaStringKey int

// DockerUserAgent is the User-Agent the Docker client uses to identify itself.
// In accordance with RFC 7231 (5.5.3) is of the form:
//    [upstream UA] [docker client's UA]
func DockerUserAgent(upstreamUA string) string {
	if len(upstreamUA) > 0 {
		upstreamUA = upstreamUA + " upstream-ua/end; "
	}

	httpVersion := make([]useragent.VersionInfo, 0, 6)
	httpVersion = append(httpVersion, useragent.VersionInfo{Name: "docker", Version: Version})
	httpVersion = append(httpVersion, useragent.VersionInfo{Name: "go", Version: runtime.Version()})
	httpVersion = append(httpVersion, useragent.VersionInfo{Name: "git-commit", Version: GitCommit})
	if kernelVersion, err := kernel.GetKernelVersion(); err == nil {
		httpVersion = append(httpVersion, useragent.VersionInfo{Name: "kernel", Version: kernelVersion.String()})
	}
	httpVersion = append(httpVersion, useragent.VersionInfo{Name: "os", Version: runtime.GOOS})
	httpVersion = append(httpVersion, useragent.VersionInfo{Name: "arch", Version: runtime.GOARCH})

	return upstreamUA + useragent.AppendVersions("", httpVersion...)
}

// WithUserAgent adds the user-agent string ua to the context ctx
func WithUserAgent(ctx context.Context, ua string) context.Context {
	var k uaStringKey
	return context.WithValue(ctx, k, ua)
}

// GetUserAgentFromContext returns the previously saved user-agent context stored in ctx, if one exists
func GetUserAgentFromContext(ctx context.Context) string {
	var upstreamUA string
	if ctx != nil {
		var k uaStringKey
		var ki interface{} = ctx.Value(k)
		if ki != nil {
			upstreamUA = ctx.Value(k).(string)
		}
	}
	return upstreamUA
}
