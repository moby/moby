package registry

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestEncodeAuth(t *testing.T) {
	newAuthConfig := &AuthConfig{Username: "ken", Password: "test", Email: "test@example.com"}
	authStr := encodeAuth(newAuthConfig)
	decAuthConfig := &AuthConfig{}
	var err error
	decAuthConfig.Username, decAuthConfig.Password, err = decodeAuth(authStr)
	if err != nil {
		t.Fatal(err)
	}
	if newAuthConfig.Username != decAuthConfig.Username {
		t.Fatal("Encode Username doesn't match decoded Username")
	}
	if newAuthConfig.Password != decAuthConfig.Password {
		t.Fatal("Encode Password doesn't match decoded Password")
	}
	if authStr != "a2VuOnRlc3Q=" {
		t.Fatal("AuthString encoding isn't correct.")
	}
}

func setupTempConfigFile() (*ConfigFile, error) {
	root, err := ioutil.TempDir("", "docker-test-auth")
	if err != nil {
		return nil, err
	}
	configFile := &ConfigFile{
		rootPath: root,
		Configs:  make(map[string]AuthConfig),
	}

	for _, registry := range []string{"testIndex", IndexServerAddress()} {
		configFile.Configs[registry] = AuthConfig{
			Username: "docker-user",
			Password: "docker-pass",
			Email:    "docker@docker.io",
		}
	}

	return configFile, nil
}

func TestSameAuthDataPostSave(t *testing.T) {
	configFile, err := setupTempConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(configFile.rootPath)

	err = SaveConfig(configFile)
	if err != nil {
		t.Fatal(err)
	}

	authConfig := configFile.Configs["testIndex"]
	if authConfig.Username != "docker-user" {
		t.Fail()
	}
	if authConfig.Password != "docker-pass" {
		t.Fail()
	}
	if authConfig.Email != "docker@docker.io" {
		t.Fail()
	}
	if authConfig.Auth != "" {
		t.Fail()
	}
}

func TestResolveAuthConfigIndexServer(t *testing.T) {
	configFile, err := setupTempConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(configFile.rootPath)

	for _, registry := range []string{"", IndexServerAddress()} {
		resolved := configFile.ResolveAuthConfig(registry)
		if resolved != configFile.Configs[IndexServerAddress()] {
			t.Fail()
		}
	}
}

func TestResolveAuthConfigFullURL(t *testing.T) {
	configFile, err := setupTempConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(configFile.rootPath)

	registryAuth := AuthConfig{
		Username: "foo-user",
		Password: "foo-pass",
		Email:    "foo@example.com",
	}
	localAuth := AuthConfig{
		Username: "bar-user",
		Password: "bar-pass",
		Email:    "bar@example.com",
	}
	configFile.Configs["https://registry.example.com/v1/"] = registryAuth
	configFile.Configs["http://localhost:8000/v1/"] = localAuth
	configFile.Configs["registry.com"] = registryAuth

	validRegistries := map[string][]string{
		"https://registry.example.com/v1/": {
			"https://registry.example.com/v1/",
			"http://registry.example.com/v1/",
			"registry.example.com",
			"registry.example.com/v1/",
		},
		"http://localhost:8000/v1/": {
			"https://localhost:8000/v1/",
			"http://localhost:8000/v1/",
			"localhost:8000",
			"localhost:8000/v1/",
		},
		"registry.com": {
			"https://registry.com/v1/",
			"http://registry.com/v1/",
			"registry.com",
			"registry.com/v1/",
		},
	}

	for configKey, registries := range validRegistries {
		for _, registry := range registries {
			var (
				configured AuthConfig
				ok         bool
			)
			resolved := configFile.ResolveAuthConfig(registry)
			if configured, ok = configFile.Configs[configKey]; !ok {
				t.Fail()
			}
			if resolved.Email != configured.Email {
				t.Errorf("%s -> %q != %q\n", registry, resolved.Email, configured.Email)
			}
		}
	}
}
