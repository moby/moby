package registry

import (
	"github.com/dotcloud/docker/config"
	"io/ioutil"
	"os"
	"testing"
)

var (
	cfgFile string
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

func setupTempConfigFile() (*config.ConfigFile, error) {
	cfgFile, err := ioutil.TempDir("", "docker-test-auth")
	if err != nil {
		return nil, err
	}
	configFile, err := config.LoadConfig(cfgFile)
	if err != nil {
		return configFile, err
	}

	for _, index := range []string{"testIndex", IndexServerAddress()} {
		PutAuth(configFile, index, &AuthConfig{
			Username: "docker-user",
			Password: "docker-pass",
			Email:    "docker@docker.io",
		})
	}

	return configFile, nil
}

func TestSameAuthDataPostSave(t *testing.T) {
	configFile, err := setupTempConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cfgFile)

	err = config.SaveConfig(configFile)
	if err != nil {
		t.Fatal(err)
	}

	authConfig, err := GetAuth(configFile, "testIndex")
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
	defer os.RemoveAll(cfgFile)

	for _, index := range []string{"", IndexServerAddress()} {
		resolved := ResolveAuthConfig(configFile, index)
		auth, _ := GetAuth(configFile, IndexServerAddress())
		if resolved.ServerAddress != auth.ServerAddress {
			t.Errorf("failed to get Auth for %s - got %s and %s", index, resolved.ServerAddress, auth.ServerAddress)
		}
	}
}

func TestResolveAuthConfigFullURL(t *testing.T) {
	configFile, err := setupTempConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cfgFile)

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
	PutAuth(configFile, "https://registry.example.com/v1/", &registryAuth)
	PutAuth(configFile, "http://localhost:8000/v1/", &localAuth)
	PutAuth(configFile, "registry.com", &registryAuth)

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
		for _, index := range registries {
			resolved := ResolveAuthConfig(configFile, index)
			configured, err := GetAuth(configFile, configKey)
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Email != configured.Email {
				t.Errorf("%s -> %q != %s -> %q\n", index, resolved.Email, configKey, configured.Email)
			}
		}
	}
}
