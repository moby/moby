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
	username0, password0, err0 := decodeAuth("dXNlcm5hbWU6cGFzc3dvcmQ=")
	if err0 != nil {
		t.Fatal(err0)
	}
	if username0 != "username" || password0 != "password" {
		t.Fatal("AuthString decoding failed for base64 encoded auth string (validity).")
	}
	username1, password1, err1 := decodeAuth("username:password")
	if err1 != nil {
		t.Fatal(err1)
	}
	if username1 != "username" || password1 != "password" {
		t.Fatal("AuthString decoding failed for clear text auth string (validity).")
	}
	username2, password2, err2 := decodeAuth("cGFzc3dvcmQ6dXNlcm5hbWU=")
	if err2 != nil {
		t.Fatal(err2)
	}
	if username2 == "username" && password2 == "password" {
		t.Fatal("AuthString decoding failed for base64 encoded auth string (invalidity).")
	}
	username3, password3, err3 := decodeAuth("password:username")
	if err3 != nil {
		t.Fatal(err3)
	}
	if username3 == "username" && password3 == "password" {
		t.Fatal("AuthString decoding failed for clear text auth string (invalidity).")
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

	indexConfig := configFile.Configs[IndexServerAddress()]

	officialIndex := &IndexInfo{
		Official: true,
	}
	privateIndex := &IndexInfo{
		Official: false,
	}

	resolved := configFile.ResolveAuthConfig(officialIndex)
	assertEqual(t, resolved, indexConfig, "Expected ResolveAuthConfig to return IndexServerAddress()")

	resolved = configFile.ResolveAuthConfig(privateIndex)
	assertNotEqual(t, resolved, indexConfig, "Expected ResolveAuthConfig to not return IndexServerAddress()")
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
	officialAuth := AuthConfig{
		Username: "baz-user",
		Password: "baz-pass",
		Email:    "baz@example.com",
	}
	configFile.Configs[IndexServerAddress()] = officialAuth

	expectedAuths := map[string]AuthConfig{
		"registry.example.com": registryAuth,
		"localhost:8000":       localAuth,
		"registry.com":         localAuth,
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
		"registry.com": {
			"https://registry.com/v1/",
			"http://registry.com/v1/",
			"registry.com",
			"registry.com/v1/",
		},
	}

	for configKey, registries := range validRegistries {
		configured, ok := expectedAuths[configKey]
		if !ok || configured.Email == "" {
			t.Fail()
		}
		index := &IndexInfo{
			Name: configKey,
		}
		for _, registry := range registries {
			configFile.Configs[registry] = configured
			resolved := configFile.ResolveAuthConfig(index)
			if resolved.Email != configured.Email {
				t.Errorf("%s -> %q != %q\n", registry, resolved.Email, configured.Email)
			}
			delete(configFile.Configs, registry)
			resolved = configFile.ResolveAuthConfig(index)
			if resolved.Email == configured.Email {
				t.Errorf("%s -> %q == %q\n", registry, resolved.Email, configured.Email)
			}
		}
	}
}
