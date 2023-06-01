package dockerversion // import "github.com/docker/docker/dockerversion"

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/useragent"
)

// UAStringKey is used as key type for user-agent string in net/context struct
type UAStringKey struct{}

// DockerUserAgent is the User-Agent the Docker client uses to identify itself.
// In accordance with RFC 7231 (5.5.3) is of the form:
//
//	[docker client's UA] UpstreamClient([upstream client's UA])
func DockerUserAgent(ctx context.Context) string {
	daemonUA := getDaemonUserAgent()
	if upstreamUA := getUserAgentFromContext(ctx); len(upstreamUA) > 0 {
		return insertUpstreamUserAgent(upstreamUA, daemonUA)
	}
	return daemonUA
}

var (
	daemonUAOnce sync.Once
	daemonUA     string
)

// getDaemonUserAgent returns the user-agent to use for requests made by
// the daemon.
//
// It includes;
//
// - the docker version
// - go version
// - git-commit
// - kernel version
// - os
// - architecture
func getDaemonUserAgent() string {
	daemonUAOnce.Do(func() {
		httpVersion := make([]useragent.VersionInfo, 0, 6)
		httpVersion = append(httpVersion, useragent.VersionInfo{Name: "docker", Version: Version})
		httpVersion = append(httpVersion, useragent.VersionInfo{Name: "go", Version: runtime.Version()})
		httpVersion = append(httpVersion, useragent.VersionInfo{Name: "git-commit", Version: GitCommit})
		if kernelVersion, err := kernel.GetKernelVersion(); err == nil {
			httpVersion = append(httpVersion, useragent.VersionInfo{Name: "kernel", Version: kernelVersion.String()})
		}
		httpVersion = append(httpVersion, useragent.VersionInfo{Name: "os", Version: runtime.GOOS})
		httpVersion = append(httpVersion, useragent.VersionInfo{Name: "arch", Version: runtime.GOARCH})
		daemonUA = useragent.AppendVersions("", httpVersion...)
	})
	return daemonUA
}

// getUserAgentFromContext returns the previously saved user-agent context stored in ctx, if one exists
func getUserAgentFromContext(ctx context.Context) string {
	var upstreamUA string
	if ctx != nil {
		var ki interface{} = ctx.Value(UAStringKey{})
		if ki != nil {
			upstreamUA = ctx.Value(UAStringKey{}).(string)
		}
	}
	return upstreamUA
}

const charsToEscape = `();\`

// escapeStr returns s with every rune in charsToEscape escaped by a backslash
func escapeStr(s string) string {
	var ret string
	for _, currRune := range s {
		appended := false
		for _, escapableRune := range charsToEscape {
			if currRune == escapableRune {
				ret += `\` + string(currRune)
				appended = true
				break
			}
		}
		if !appended {
			ret += string(currRune)
		}
	}
	return ret
}

// insertUpstreamUserAgent adds the upstream client useragent to create a user-agent
// string of the form:
//
//	$dockerUA UpstreamClient($upstreamUA)
func insertUpstreamUserAgent(upstreamUA string, dockerUA string) string {
	return fmt.Sprintf("%s UpstreamClient(%s)", dockerUA, escapeStr(upstreamUA))
}
