package registry

import (
	"runtime"

	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/requestdecorator"
)

func HTTPRequestFactory(metaHeaders map[string][]string) *requestdecorator.RequestFactory {
	// FIXME: this replicates the 'info' job.
	httpVersion := make([]requestdecorator.UAVersionInfo, 0, 4)
	httpVersion = append(httpVersion, requestdecorator.NewUAVersionInfo("docker", dockerversion.VERSION))
	httpVersion = append(httpVersion, requestdecorator.NewUAVersionInfo("go", runtime.Version()))
	httpVersion = append(httpVersion, requestdecorator.NewUAVersionInfo("git-commit", dockerversion.GITCOMMIT))
	if kernelVersion, err := kernel.GetKernelVersion(); err == nil {
		httpVersion = append(httpVersion, requestdecorator.NewUAVersionInfo("kernel", kernelVersion.String()))
	}
	httpVersion = append(httpVersion, requestdecorator.NewUAVersionInfo("os", runtime.GOOS))
	httpVersion = append(httpVersion, requestdecorator.NewUAVersionInfo("arch", runtime.GOARCH))
	uad := &requestdecorator.UserAgentDecorator{
		Versions: httpVersion,
	}
	mhd := &requestdecorator.MetaHeadersDecorator{
		Headers: metaHeaders,
	}
	factory := requestdecorator.NewRequestFactory(uad, mhd)
	return factory
}
