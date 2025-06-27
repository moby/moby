package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/testutil/environment"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// Create a test environment variable
var testEnv *environment.Execution

func init() {
	testEnv = &environment.Execution{}
}

// TestAuthTokenDebug verifies the functionality of the /auth/token/debug endpoint
func TestAuthTokenDebug(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := context.Background()
	client := request.NewAPIClient(t)

	// Create a sample JWT token
	payload := `{"sub":"integration-test-user","exp":1735689600,"access":[{"type":"repository","name":"library/alpine","actions":["pull"]}]}`

	// Base64-encode the payload (simplified JWT creation for test)
	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	encodedSignature := base64.RawURLEncoding.EncodeToString([]byte("signature"))

	// Create the token
	tokenStr := strings.Join([]string{encodedHeader, encodedPayload, encodedSignature}, ".")

	// Create the request
	req, err := http.NewRequest(http.MethodPost, "/auth/token/debug", strings.NewReader(url.Values{"token": {tokenStr}}.Encode()))
	assert.NilError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, err := client.Do(req)
	assert.NilError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse the response
	var debugResp types.AuthTokenDebugResponse
	err = json.NewDecoder(resp.Body).Decode(&debugResp)
	assert.NilError(t, err)
	resp.Body.Close()

	// Verify the claims
	assert.Equal(t, "integration-test-user", debugResp.Claims["sub"])

	// Verify access rights are included
	accessList, ok := debugResp.Claims["access"].([]interface{})
	assert.Assert(t, ok, "access field should be an array")
	assert.Equal(t, 1, len(accessList))

	// Test with invalid token
	req, err = http.NewRequest(http.MethodPost, "/auth/token/debug", strings.NewReader(url.Values{"token": {"invalid-token"}}.Encode()))
	assert.NilError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = client.Do(req)
	assert.NilError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// Test with missing token
	req, err = http.NewRequest(http.MethodPost, "/auth/token/debug", strings.NewReader(""))
	assert.NilError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = client.Do(req)
	assert.NilError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}
