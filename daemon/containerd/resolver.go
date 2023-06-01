package containerd

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/version"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/useragent"
	"github.com/docker/docker/registry"
	"github.com/sirupsen/logrus"
)

func (i *ImageService) newResolverFromAuthConfig(ctx context.Context, authConfig *registrytypes.AuthConfig) (remotes.Resolver, docker.StatusTracker) {
	tracker := docker.NewInMemoryTracker()
	hostsFn := i.registryHosts.RegistryHosts()

	hosts := hostsWrapper(hostsFn, authConfig, i.registryService)
	headers := http.Header{}
	headers.Set("User-Agent", dockerversion.DockerUserAgent(ctx, useragent.VersionInfo{Name: "containerd-client", Version: version.Version}, useragent.VersionInfo{Name: "storage-driver", Version: i.snapshotter}))

	return docker.NewResolver(docker.ResolverOptions{
		Hosts:   hosts,
		Tracker: tracker,
		Headers: headers,
	}), tracker
}

func hostsWrapper(hostsFn docker.RegistryHosts, optAuthConfig *registrytypes.AuthConfig, regService RegistryConfigProvider) docker.RegistryHosts {
	var authorizer docker.Authorizer
	if optAuthConfig != nil {
		authorizer = docker.NewDockerAuthorizer(authorizationCredsFromAuthConfig(*optAuthConfig))
	}

	return func(n string) ([]docker.RegistryHost, error) {
		hosts, err := hostsFn(n)
		if err != nil {
			return nil, err
		}

		for i := range hosts {
			if hosts[i].Authorizer == nil {
				hosts[i].Authorizer = authorizer
				isInsecure := regService.IsInsecureRegistry(hosts[i].Host)
				if hosts[i].Client.Transport != nil && isInsecure {
					hosts[i].Client.Transport = httpFallback{super: hosts[i].Client.Transport}
				}
			}
		}
		return hosts, nil
	}
}

func authorizationCredsFromAuthConfig(authConfig registrytypes.AuthConfig) docker.AuthorizerOpt {
	cfgHost := registry.ConvertToHostname(authConfig.ServerAddress)
	if cfgHost == "" || cfgHost == registry.IndexHostname {
		cfgHost = registry.DefaultRegistryHost
	}

	return docker.WithAuthCreds(func(host string) (string, string, error) {
		if cfgHost != host {
			logrus.WithFields(logrus.Fields{
				"host":    host,
				"cfgHost": cfgHost,
			}).Warn("Host doesn't match")
			return "", "", nil
		}
		if authConfig.IdentityToken != "" {
			return "", authConfig.IdentityToken, nil
		}
		return authConfig.Username, authConfig.Password, nil
	})
}

type httpFallback struct {
	super http.RoundTripper
}

func (f httpFallback) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := f.super.RoundTrip(r)
	var tlsErr tls.RecordHeaderError
	if errors.As(err, &tlsErr) && string(tlsErr.RecordHeader[:]) == "HTTP/" {
		// server gave HTTP response to HTTPS client
		plainHttpUrl := *r.URL
		plainHttpUrl.Scheme = "http"

		plainHttpRequest := *r
		plainHttpRequest.URL = &plainHttpUrl

		return http.DefaultTransport.RoundTrip(&plainHttpRequest)
	}

	return resp, err
}
