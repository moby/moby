package registry // import "github.com/docker/docker/api/types/registry"
import (
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

const (
	unencoded        = `{"username":"testuser","password":"testpassword","serveraddress":"example.com"}`
	encoded          = `eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzc3dvcmQiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5jb20ifQ==`
	encodedNoPadding = `eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzc3dvcmQiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5jb20ifQ`
)

var expected = AuthConfig{
	Username:      "testuser",
	Password:      "testpassword",
	ServerAddress: "example.com",
}

func TestDecodeAuthConfig(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		token, err := DecodeAuthConfig(encoded)
		assert.NilError(t, err)
		assert.Equal(t, *token, expected)
	})

	t.Run("empty", func(t *testing.T) {
		token, err := DecodeAuthConfig("")
		assert.NilError(t, err)
		assert.Equal(t, *token, AuthConfig{})
	})

	// We currently only support base64url encoding with padding, so
	// un-padded should produce an error.
	//
	// RFC4648, section 5: https://tools.ietf.org/html/rfc4648#section-5
	// RFC4648, section 3.2: https://tools.ietf.org/html/rfc4648#section-3.2
	t.Run("invalid encoding", func(t *testing.T) {
		token, err := DecodeAuthConfig(encodedNoPadding)

		assert.ErrorType(t, err, errInvalidParameter{})
		assert.ErrorContains(t, err, "invalid X-Registry-Auth header: unexpected EOF")
		assert.Equal(t, *token, AuthConfig{})
	})
}

func TestDecodeAuthConfigBody(t *testing.T) {
	token, err := DecodeAuthConfigBody(io.NopCloser(strings.NewReader(unencoded)))
	assert.NilError(t, err)
	assert.Equal(t, *token, expected)
}

func TestEncodeAuthConfig(t *testing.T) {
	token, err := EncodeAuthConfig(expected)
	assert.NilError(t, err)
	assert.Equal(t, token, encoded)
}
