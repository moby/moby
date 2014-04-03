package docker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/dotcloud/docker/registry"
	"os"
	"strings"
	"testing"
)

// FIXME: these tests have an external dependency on a staging index hosted
// on the docker.io infrastructure. That dependency should be removed.
// - Unit tests should have no side-effect dependencies.
// - Integration tests should have side-effects limited to the host environment being tested.

func TestLogin(t *testing.T) {
	t.Skip("FIXME: please remove dependency on external services")
	os.Setenv("DOCKER_INDEX_URL", "https://indexstaging-docker.dotcloud.com")
	defer os.Setenv("DOCKER_INDEX_URL", "")
	authConfig := &registry.AuthConfig{
		Username:      "unittester",
		Password:      "surlautrerivejetattendrai",
		Email:         "noise+unittester@docker.com",
		ServerAddress: "https://indexstaging-docker.dotcloud.com/v1/",
	}
	status, err := registry.Login(authConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != "Login Succeeded" {
		t.Fatalf("Expected status \"Login Succeeded\", found \"%s\" instead", status)
	}
}

func TestCreateAccount(t *testing.T) {
	t.Skip("FIXME: please remove dependency on external services")
	tokenBuffer := make([]byte, 16)
	_, err := rand.Read(tokenBuffer)
	if err != nil {
		t.Fatal(err)
	}
	token := hex.EncodeToString(tokenBuffer)[:12]
	username := "ut" + token
	authConfig := &registry.AuthConfig{
		Username:      username,
		Password:      "test42",
		Email:         fmt.Sprintf("docker-ut+%s@example.com", token),
		ServerAddress: "https://indexstaging-docker.dotcloud.com/v1/",
	}
	status, err := registry.Login(authConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	expectedStatus := fmt.Sprintf(
		"Account created. Please see the documentation of the registry %s for instructions how to activate it.",
		authConfig.ServerAddress,
	)
	if status != expectedStatus {
		t.Fatalf("Expected status: \"%s\", found \"%s\" instead.", expectedStatus, status)
	}

	status, err = registry.Login(authConfig, nil)
	if err == nil {
		t.Fatalf("Expected error but found nil instead")
	}

	expectedError := "Login: Account is not Active"

	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("Expected message \"%s\" but found \"%s\" instead", expectedError, err)
	}
}
