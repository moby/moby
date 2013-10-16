package auth

import (
	"crypto/rand"
	"encoding/hex"
	"io/ioutil"
	"os"
	"strings"
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

func TestLogin(t *testing.T) {
	os.Setenv("DOCKER_INDEX_URL", "https://indexstaging-docker.dotcloud.com")
	defer os.Setenv("DOCKER_INDEX_URL", "")
	authConfig := &AuthConfig{Username: "unittester", Password: "surlautrerivejetattendrai", Email: "noise+unittester@dotcloud.com"}
	status, err := Login(authConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != "Login Succeeded" {
		t.Fatalf("Expected status \"Login Succeeded\", found \"%s\" instead", status)
	}
}

func TestCreateAccount(t *testing.T) {
	os.Setenv("DOCKER_INDEX_URL", "https://indexstaging-docker.dotcloud.com")
	defer os.Setenv("DOCKER_INDEX_URL", "")
	tokenBuffer := make([]byte, 16)
	_, err := rand.Read(tokenBuffer)
	if err != nil {
		t.Fatal(err)
	}
	token := hex.EncodeToString(tokenBuffer)[:12]
	username := "ut" + token
	authConfig := &AuthConfig{Username: username, Password: "test42", Email: "docker-ut+" + token + "@example.com"}
	status, err := Login(authConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	expectedStatus := "Account created. Please use the confirmation link we sent" +
		" to your e-mail to activate it."
	if status != expectedStatus {
		t.Fatalf("Expected status: \"%s\", found \"%s\" instead.", expectedStatus, status)
	}

	status, err = Login(authConfig, nil)
	if err == nil {
		t.Fatalf("Expected error but found nil instead")
	}

	expectedError := "Login: Account is not Active"

	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("Expected message \"%s\" but found \"%s\" instead", expectedError, err)
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
