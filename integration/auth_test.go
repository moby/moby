package docker

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/dotcloud/docker/auth"
	"os"
	"strings"
	"testing"
)

// FIXME: these tests have an external dependency on a staging index hosted
// on the docker.io infrastructure. That dependency should be removed.
// - Unit tests should have no side-effect dependencies.
// - Integration tests should have side-effects limited to the host environment being tested.

func TestLogin(t *testing.T) {
	os.Setenv("DOCKER_INDEX_URL", "https://indexstaging-docker.dotcloud.com")
	defer os.Setenv("DOCKER_INDEX_URL", "")
	authConfig := &auth.AuthConfig{Username: "unittester", Password: "surlautrerivejetattendrai", Email: "noise+unittester@dotcloud.com"}
	status, err := auth.Login(authConfig, nil)
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
	authConfig := &auth.AuthConfig{Username: username, Password: "test42", Email: "docker-ut+" + token + "@example.com"}
	status, err := auth.Login(authConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	expectedStatus := "Account created. Please use the confirmation link we sent" +
		" to your e-mail to activate it."
	if status != expectedStatus {
		t.Fatalf("Expected status: \"%s\", found \"%s\" instead.", expectedStatus, status)
	}

	status, err = auth.Login(authConfig, nil)
	if err == nil {
		t.Fatalf("Expected error but found nil instead")
	}

	expectedError := "Login: Account is not Active"

	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("Expected message \"%s\" but found \"%s\" instead", expectedError, err)
	}
}
