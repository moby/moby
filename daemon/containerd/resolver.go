package containerd

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/version"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/useragent"
	"github.com/docker/docker/registry"
)

func (i *ImageService) newResolverFromAuthConfig(ctx context.Context, authConfig *registrytypes.AuthConfig, ref reference.Named) (remotes.Resolver, docker.StatusTracker) {
	tracker := docker.NewInMemoryTracker()

	hosts := hostsWrapper(i.registryHosts, authConfig, ref, i.registryService)
	headers := http.Header{}
	headers.Set("User-Agent", dockerversion.DockerUserAgent(ctx, useragent.VersionInfo{Name: "containerd-client", Version: version.Version}, useragent.VersionInfo{Name: "storage-driver", Version: i.snapshotter}))

	return docker.NewResolver(docker.ResolverOptions{
		Hosts:   hosts,
		Tracker: tracker,
		Headers: headers,
	}), tracker
}

func hostsWrapper(hostsFn docker.RegistryHosts, optAuthConfig *registrytypes.AuthConfig, ref reference.Named, regService registryResolver) docker.RegistryHosts {
	var authorizer docker.Authorizer
	if optAuthConfig != nil {
		authorizer = authorizerFromAuthConfig(*optAuthConfig, ref)
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

func authorizerFromAuthConfig(authConfig registrytypes.AuthConfig, ref reference.Named) docker.Authorizer {
	cfgHost := registry.ConvertToHostname(authConfig.ServerAddress)
	if cfgHost == "" {
		cfgHost = reference.Domain(ref)
	}
	if cfgHost == registry.IndexHostname || cfgHost == registry.IndexName {
		cfgHost = registry.DefaultRegistryHost
	}

	if authConfig.RegistryToken != "" {
		return &bearerAuthorizer{
			host:   cfgHost,
			bearer: authConfig.RegistryToken,
		}
	}

	return docker.NewDockerAuthorizer(docker.WithAuthCreds(func(host string) (string, string, error) {
		if cfgHost != host {
			log.G(context.TODO()).WithFields(log.Fields{
				"host":    host,
				"cfgHost": cfgHost,
			}).Warn("Host doesn't match")
			return "", "", nil
		}
		if authConfig.IdentityToken != "" {
			return "", authConfig.IdentityToken, nil
		}
		return authConfig.Username, authConfig.Password, nil
	}))
}

type bearerAuthorizer struct {
	host   string
	bearer string
}

func (a *bearerAuthorizer) Authorize(ctx context.Context, req *http.Request) error {
	if req.Host != a.host {
		log.G(ctx).WithFields(log.Fields{
			"host":    req.Host,
			"cfgHost": a.host,
		}).Warn("Host doesn't match for bearer token")
		return nil
	}

	req.Header.Set("Authorization", "Bearer "+a.bearer)

	return nil
}

func (a *bearerAuthorizer) AddResponses(context.Context, []*http.Response) error {
	// Return not implemented to prevent retry of the request when bearer did not succeed
	return cerrdefs.ErrNotImplemented
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
