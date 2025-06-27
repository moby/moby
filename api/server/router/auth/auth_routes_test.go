package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"gotest.tools/v3/assert"
)

// mockAuthBackend is a mock implementation of the AuthBackend interface
type mockAuthBackend struct{}

func (m *mockAuthBackend) Auth(ctx context.Context, authConfig *types.AuthConfig, userAgent string) (string, string, error) {
	return "OK", "", nil
}

func TestDebugToken(t *testing.T) {
	backend := &mockAuthBackend{}
	router := NewRouter(backend)
	server := httptest.NewServer(router.Handler())
	defer server.Close()

	// Create a sample JWT payload
	payload := `{"sub":"user123","exp":1735689600,"access":[{"type":"repository","name":"library/alpine","actions":["pull"]}]}`

	// Base64-encode the payload (simplified JWT creation for test)
	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	encodedSignature := base64.RawURLEncoding.EncodeToString([]byte("signature"))

	// Create the token
	tokenStr := strings.Join([]string{encodedHeader, encodedPayload, encodedSignature}, ".")

	// Test valid token
	form := url.Values{}
	form.Add("token", tokenStr)

	resp, err := http.Post(server.URL+"/auth/token/debug", "application/x-www-form-urlencoded", bytes.NewBufferString(form.Encode()))
	assert.NilError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var debugResp types.AuthTokenDebugResponse
	err = json.NewDecoder(resp.Body).Decode(&debugResp)
	assert.NilError(t, err)
	resp.Body.Close()

	// Verify the claims
	assert.Equal(t, "user123", debugResp.Claims["sub"])

	// Verify access rights are included
	accessList, ok := debugResp.Claims["access"].([]interface{})
	assert.Assert(t, ok, "access field should be an array")
	assert.Equal(t, 1, len(accessList))

	// Test missing token
	form = url.Values{}
	resp, err = http.Post(server.URL+"/auth/token/debug", "application/x-www-form-urlencoded", bytes.NewBufferString(form.Encode()))
	assert.NilError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// Test invalid token format
	form = url.Values{}
	form.Add("token", "invalid-token")
	resp, err = http.Post(server.URL+"/auth/token/debug", "application/x-www-form-urlencoded", bytes.NewBufferString(form.Encode()))
	assert.NilError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}
