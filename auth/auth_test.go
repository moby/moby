package auth

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestEncodeAuth(t *testing.T) {
	newAuthConfig := &AuthConfig{Username: "ken", Password: "test", Email: "test@example.com"}
	authStr := encodeAuth(newAuthConfig)
	decAuthConfig, err := decodeAuth(authStr)
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
	authConfig := NewAuthConfig("unittester", "surlautrerivejetattendrai", "noise+unittester@dotcloud.com", "/tmp")
	status, err := Login(authConfig, false)
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
	authConfig := NewAuthConfig(username, "test42", "docker-ut+"+token+"@example.com", "/tmp")
	status, err := Login(authConfig, false)
	if err != nil {
		t.Fatal(err)
	}
	expectedStatus := "Account created. Please use the confirmation link we sent" +
		" to your e-mail to activate it."
	if status != expectedStatus {
		t.Fatalf("Expected status: \"%s\", found \"%s\" instead.", expectedStatus, status)
	}

	status, err = Login(authConfig, false)
	if err == nil {
		t.Fatalf("Expected error but found nil instead")
	}

	expectedError := "Login: Account is not Active"

	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("Expected message \"%s\" but found \"%s\" instead", expectedError, err)
	}
}
