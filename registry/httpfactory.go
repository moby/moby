package registry

import (
	"runtime"

	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/utils"
)

func HTTPRequestFactory(metaHeaders map[string][]string) *utils.HTTPRequestFactory {
	// FIXME: this replicates the 'info' job.
	httpVersion := make([]utils.VersionInfo, 0, 4)
	httpVersion = append(httpVersion, &simpleVersionInfo{"docker", dockerversion.VERSION})
	httpVersion = append(httpVersion, &simpleVersionInfo{"go", runtime.Version()})
	httpVersion = append(httpVersion, &simpleVersionInfo{"git-commit", dockerversion.GITCOMMIT})
	if kernelVersion, err := kernel.GetKernelVersion(); err == nil {
		httpVersion = append(httpVersion, &simpleVersionInfo{"kernel", kernelVersion.String()})
	}
	httpVersion = append(httpVersion, &simpleVersionInfo{"os", runtime.GOOS})
	httpVersion = append(httpVersion, &simpleVersionInfo{"arch", runtime.GOARCH})
	ud := utils.NewHTTPUserAgentDecorator(httpVersion...)
	md := &utils.HTTPMetaHeadersDecorator{
		Headers: metaHeaders,
	}
	factory := utils.NewHTTPRequestFactory(ud, md)
	return factory
}

// simpleVersionInfo is a simple implementation of
// the interface VersionInfo, which is used
// to provide version information for some product,
// component, etc. It stores the product name and the version
// in string and returns them on calls to Name() and Version().
type simpleVersionInfo struct {
	name    string
	version string
}

func (v *simpleVersionInfo) Name() string {
	return v.name
}

func (v *simpleVersionInfo) Version() string {
	return v.version
}
