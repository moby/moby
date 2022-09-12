package containerd

import (
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/registry"
	"github.com/sirupsen/logrus"
)

func (i *ImageService) newResolverFromAuthConfig(authConfig *registrytypes.AuthConfig) (remotes.Resolver, docker.StatusTracker) {
	hostsFn := i.registryHosts.RegistryHosts()
	hosts := hostsAuthorizerWrapper(hostsFn, authConfig)

	tracker := docker.NewInMemoryTracker()

	return docker.NewResolver(docker.ResolverOptions{
		Hosts:   hosts,
		Tracker: tracker,
	}), tracker
}

func hostsAuthorizerWrapper(hostsFn docker.RegistryHosts, authConfig *registrytypes.AuthConfig) docker.RegistryHosts {
	return docker.RegistryHosts(func(n string) ([]docker.RegistryHost, error) {
		hosts, err := hostsFn(n)
		if err == nil {
			for idx, host := range hosts {
				if host.Authorizer == nil {
					var opts []docker.AuthorizerOpt
					if authConfig != nil {
						opts = append(opts, authorizationCredsFromAuthConfig(*authConfig))
					}
					host.Authorizer = docker.NewDockerAuthorizer(opts...)
					hosts[idx] = host
				}
			}
		}

		return hosts, err
	})
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
