package dockerversion

import (
	"context"
	"runtime"
	"strings"
	"sync"

	"github.com/moby/moby/v2/pkg/parsers/kernel"
	"github.com/moby/moby/v2/pkg/useragent"
)

// uaStringKey is used as key type for user-agent string in net/context struct
type uaStringKey struct{}

// WithUpstreamUserAgent returns a new context carrying the upstream client's
// User-Agent string.
func WithUpstreamUserAgent(ctx context.Context, ua string) context.Context {
	if ua == "" {
		return ctx
	}
	return context.WithValue(ctx, uaStringKey{}, ua)
}

// DockerUserAgent is the User-Agent used by the daemon.
//
// It consists of the daemon's User-Agent, optional version metadata, and
// an optional upstream client comment:
//
//	[daemon user agent] [extra] [UpstreamClient(<upstream-user-agent>)]
//
// "UpstreamClient" is a Docker-defined convention. The upstream value is
// sanitized before inclusion. See [RFC9110], section 10.1.5.
//
// [RFC9110]: https://www.rfc-editor.org/rfc/rfc9110#section-10.1.5
func DockerUserAgent(ctx context.Context, extraVersions ...useragent.VersionInfo) string {
	ua := useragent.AppendVersions(getDaemonUserAgent(), extraVersions...)
	if upstreamUA := getUpstreamUserAgent(ctx); upstreamUA != "" {
		ua += " " + upstreamUA
	}
	return ua
}

var (
	daemonUAOnce sync.Once
	daemonUA     string
)

// getDaemonUserAgent returns the user-agent to use for requests made by
// the daemon.
//
// It includes:
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

// getUpstreamUserAgent returns the previously saved user-agent context stored
// in ctx, if one exists, and formats it as:
//
//	UpstreamClient(<upstream user agent string>)
//
// It returns an empty string if no user-agent is present in the context.
func getUpstreamUserAgent(ctx context.Context) string {
	upstreamUA, ok := ctx.Value(uaStringKey{}).(string)
	if !ok || upstreamUA == "" {
		return ""
	}

	return "UpstreamClient(" + escapeStr(upstreamUA) + ")"
}

// escapeStr escapes and sanitizes s for use in a User-Agent comment ([RFC9110]).
//
// [RFC9110]: https://www.rfc-editor.org/rfc/rfc9110#section-10.1.5
func escapeStr(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := range len(s) {
		switch c := s[i]; c {
		// TODO(thaJeztah): remove redundant escaping semicolons; see https://github.com/moby/moby/pull/52356#discussion_r3234266285
		case '(', ')', ';', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\t':
			b.WriteByte(c)
		default:
			if c >= 0x20 && c != 0x7f {
				b.WriteByte(c)
			}
		}
	}

	return b.String()
}
