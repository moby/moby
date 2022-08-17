package registry // import "github.com/docker/docker/registry"

import (
	"testing"

	"github.com/docker/docker/api/types/registry"
	"gotest.tools/v3/assert"
)

func buildAuthConfigs() map[string]registry.AuthConfig {
	authConfigs := map[string]registry.AuthConfig{}

	for _, reg := range []string{"testIndex", IndexServer} {
		authConfigs[reg] = registry.AuthConfig{
			Username: "docker-user",
			Password: "docker-pass",
		}
	}

	return authConfigs
}

func TestResolveAuthConfigIndexServer(t *testing.T) {
	authConfigs := buildAuthConfigs()
	indexConfig := authConfigs[IndexServer]

	officialIndex := &registry.IndexInfo{
		Official: true,
	}
	privateIndex := &registry.IndexInfo{
		Official: false,
	}

	resolved := ResolveAuthConfig(authConfigs, officialIndex)
	assert.Equal(t, resolved, indexConfig, "Expected ResolveAuthConfig to return IndexServer")

	resolved = ResolveAuthConfig(authConfigs, privateIndex)
	assert.Check(t, resolved != indexConfig, "Expected ResolveAuthConfig to not return IndexServer")
}

func TestResolveAuthConfigFullURL(t *testing.T) {
	authConfigs := buildAuthConfigs()

	registryAuth := registry.AuthConfig{
		Username: "foo-user",
		Password: "foo-pass",
	}
	localAuth := registry.AuthConfig{
		Username: "bar-user",
		Password: "bar-pass",
	}
	officialAuth := registry.AuthConfig{
		Username: "baz-user",
		Password: "baz-pass",
	}
	authConfigs[IndexServer] = officialAuth

	expectedAuths := map[string]registry.AuthConfig{
		"registry.example.com": registryAuth,
		"localhost:8000":       localAuth,
		"example.com":          localAuth,
	}

	validRegistries := map[string][]string{
		"registry.example.com": {
			"https://registry.example.com/v1/",
			"http://registry.example.com/v1/",
			"registry.example.com",
			"registry.example.com/v1/",
		},
		"localhost:8000": {
			"https://localhost:8000/v1/",
			"http://localhost:8000/v1/",
			"localhost:8000",
			"localhost:8000/v1/",
		},
		"example.com": {
			"https://example.com/v1/",
			"http://example.com/v1/",
			"example.com",
			"example.com/v1/",
		},
	}

	for configKey, registries := range validRegistries {
		configured, ok := expectedAuths[configKey]
		if !ok {
			t.Fail()
		}
		index := &registry.IndexInfo{
			Name: configKey,
		}
		for _, reg := range registries {
			authConfigs[reg] = configured
			resolved := ResolveAuthConfig(authConfigs, index)
			if resolved.Username != configured.Username || resolved.Password != configured.Password {
				t.Errorf("%s -> %v != %v\n", reg, resolved, configured)
			}
			delete(authConfigs, reg)
			resolved = ResolveAuthConfig(authConfigs, index)
			if resolved.Username == configured.Username || resolved.Password == configured.Password {
				t.Errorf("%s -> %v == %v\n", reg, resolved, configured)
			}
		}
	}
}
