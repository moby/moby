package auth

import (
	"testing"
)

func TestEncodeAuth(t *testing.T) {
	newAuthConfig := AuthConfig{Username: "ken", Password: "test", Email: "test@example.com"}
	authStr := EncodeAuth(newAuthConfig)
	decAuthConfig, err := DecodeAuth(authStr)
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
