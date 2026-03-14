package containerd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/version"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	registrytypes "github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/v2/daemon/pkg/registry"
	"github.com/moby/moby/v2/dockerversion"
	"github.com/moby/moby/v2/pkg/useragent"
)

func (i *ImageService) newResolverFromAuthConfig(ctx context.Context, authConfig *registrytypes.AuthConfig, ref reference.Named, metaHeaders http.Header, clientAuth bool) (remotes.Resolver, docker.StatusTracker) {
	tracker := docker.NewInMemoryTracker()

	hosts := hostsWrapper(i.registryHosts, authConfig, ref, clientAuth)
	headers := http.Header{}
	if metaHeaders != nil {
		headers = metaHeaders.Clone()
	}
	headers.Set("User-Agent", dockerversion.DockerUserAgent(ctx, useragent.VersionInfo{Name: "containerd-client", Version: version.Version}, useragent.VersionInfo{Name: "storage-driver", Version: i.snapshotter}))

	return docker.NewResolver(docker.ResolverOptions{
		Hosts:   hosts,
		Tracker: tracker,
		Headers: headers,
	}), tracker
}

func hostsWrapper(hostsFn docker.RegistryHosts, optAuthConfig *registrytypes.AuthConfig, ref reference.Named, clientAuth bool) docker.RegistryHosts {
	if optAuthConfig == nil {
		return hostsFn
	}

	authorizer := authorizerFromAuthConfig(*optAuthConfig, ref, clientAuth)

	return func(n string) ([]docker.RegistryHost, error) {
		hosts, err := hostsFn(n)
		if err != nil {
			return nil, err
		}

		for i := range hosts {
			hosts[i].Authorizer = authorizer
		}
		return hosts, nil
	}
}

func authorizerFromAuthConfig(authConfig registrytypes.AuthConfig, ref reference.Named, clientAuth bool) docker.Authorizer {
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

	if clientAuth {
		return &clientChallengeAuthorizer{
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

type ErrAuthenticationChallenge struct {
	WwwAuthenticate string
}

func (e *ErrAuthenticationChallenge) Error() string {
	return fmt.Sprintf("%s - %s", e.Unwrap().Error(), e.WwwAuthenticate)
}

func (e *ErrAuthenticationChallenge) Unwrap() error {
	return docker.ErrInvalidAuthorization
}

type clientChallengeAuthorizer struct {
	host   string
	bearer string
}

func (c *clientChallengeAuthorizer) Authorize(ctx context.Context, req *http.Request) error {
	if req.Host != c.host {
		log.G(ctx).WithFields(log.Fields{
			"host":    req.Host,
			"cfgHost": c.host,
		}).Warn("Host doesn't match for bearer token")
		return nil
	}

	req.Header.Set("Authorization", "Bearer "+c.bearer)

	return nil
}

func (c *clientChallengeAuthorizer) AddResponses(ctx context.Context, responses []*http.Response) error {
	last := responses[len(responses)-1]
	if challenge := last.Header.Get("WWW-Authenticate"); challenge != "" {
		log.G(ctx).WithFields(log.Fields{
			"challenge": challenge,
		}).Debug("Authorizer received 401 - authenticate challenge")
		return &ErrAuthenticationChallenge{
			WwwAuthenticate: challenge,
		}
	}
	return nil
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
