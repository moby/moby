package containerd

import (
	"crypto/tls"
	"errors"
	"net/http"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/registry"
	"github.com/sirupsen/logrus"
)

func (i *ImageService) newResolverFromAuthConfig(authConfig *registrytypes.AuthConfig) (remotes.Resolver, docker.StatusTracker) {
	tracker := docker.NewInMemoryTracker()
	hostsFn := i.registryHosts.RegistryHosts()

	hosts := hostsWrapper(hostsFn, authConfig, i.registryService)

	return docker.NewResolver(docker.ResolverOptions{
		Hosts:   hosts,
		Tracker: tracker,
	}), tracker
}

func hostsWrapper(hostsFn docker.RegistryHosts, authConfig *registrytypes.AuthConfig, regService RegistryConfigProvider) docker.RegistryHosts {
	return func(n string) ([]docker.RegistryHost, error) {
		hosts, err := hostsFn(n)
		if err != nil {
			return nil, err
		}

		for i := range hosts {
			if hosts[i].Authorizer == nil {
				var opts []docker.AuthorizerOpt
				if authConfig != nil {
					opts = append(opts, authorizationCredsFromAuthConfig(*authConfig))
				}
				hosts[i].Authorizer = docker.NewDockerAuthorizer(opts...)

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
	if cfgHost == registry.IndexHostname {
		cfgHost = registry.DefaultRegistryHost
	}

	return docker.WithAuthCreds(func(host string) (string, string, error) {
		if cfgHost != host {
			logrus.WithField("host", host).WithField("cfgHost", cfgHost).Warn("Host doesn't match")
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
