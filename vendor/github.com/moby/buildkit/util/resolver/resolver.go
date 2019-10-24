package resolver

import (
	"math/rand"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/util/tracing"
)

type RegistryConf struct {
	Mirrors   []string
	PlainHTTP bool
}

type ResolveOptionsFunc func(string) docker.ResolverOptions

func NewResolveOptionsFunc(m map[string]RegistryConf) ResolveOptionsFunc {
	return func(ref string) docker.ResolverOptions {
		def := docker.ResolverOptions{
			Client: tracing.DefaultClient,
		}

		parsed, err := reference.ParseNormalizedNamed(ref)
		if err != nil {
			return def
		}
		host := reference.Domain(parsed)

		c, ok := m[host]
		if !ok {
			return def
		}

		if len(c.Mirrors) > 0 {
			def.Host = func(string) (string, error) {
				return c.Mirrors[rand.Intn(len(c.Mirrors))], nil
			}
		}

		def.PlainHTTP = c.PlainHTTP

		return def
	}
}
