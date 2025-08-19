package registry

import (
	"encoding/base64"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	registrytypes "github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDecodeAuthConfig(t *testing.T) {
	tests := []struct {
		doc         string
		input       string
		inputBase64 string
		expected    registrytypes.AuthConfig
		expectedErr string
	}{
		{
			doc:         "empty",
			input:       ``,
			inputBase64: ``,
			expected:    registrytypes.AuthConfig{},
		},
		{
			doc:         "empty JSON",
			input:       `{}`,
			inputBase64: `e30=`,
			expected:    registrytypes.AuthConfig{},
		},
		{
			doc:         "malformed JSON",
			input:       `{`,
			inputBase64: `ew==`,
			expected:    registrytypes.AuthConfig{},
			expectedErr: `invalid X-Registry-Auth header: invalid JSON: unexpected EOF`,
		},
		{
			doc:         "test authConfig",
			input:       `{"username":"testuser","password":"testpassword","serveraddress":"example.com"}`,
			inputBase64: `eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzc3dvcmQiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5jb20ifQ==`,
			expected: registrytypes.AuthConfig{
				Username:      "testuser",
				Password:      "testpassword",
				ServerAddress: "example.com",
			},
		},
		{
			// FIXME(thaJeztah): we should not accept multiple JSON documents.
			doc:         "multiple authConfig",
			input:       `{"username":"testuser","password":"testpassword","serveraddress":"example.com"}{"username":"testuser2","password":"testpassword2","serveraddress":"example.org"}`,
			inputBase64: `eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzc3dvcmQiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5jb20ifXsidXNlcm5hbWUiOiJ0ZXN0dXNlcjIiLCJwYXNzd29yZCI6InRlc3RwYXNzd29yZDIiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5vcmcifQ==`,
			expected: registrytypes.AuthConfig{
				Username:      "testuser",
				Password:      "testpassword",
				ServerAddress: "example.com",
			},
		},
		// We currently only support base64url encoding with padding, so
		// un-padded should produce an error.
		//
		// RFC4648, section 5: https://tools.ietf.org/html/rfc4648#section-5
		// RFC4648, section 3.2: https://tools.ietf.org/html/rfc4648#section-3.2
		{
			doc:         "empty JSON no padding",
			input:       `{}`,
			inputBase64: `e30`,
			expected:    registrytypes.AuthConfig{},
			expectedErr: `invalid X-Registry-Auth header: must be a valid base64url-encoded string`,
		},
		{
			doc:         "test authConfig",
			input:       `{"username":"testuser","password":"testpassword","serveraddress":"example.com"}`,
			inputBase64: `eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzc3dvcmQiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5jb20ifQ`,
			expected:    registrytypes.AuthConfig{},
			expectedErr: `invalid X-Registry-Auth header: must be a valid base64url-encoded string`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			if tc.inputBase64 != "" {
				// Sanity check to make sure our fixtures are correct.
				b64 := base64.URLEncoding.EncodeToString([]byte(tc.input))
				if !strings.HasSuffix(tc.inputBase64, "=") {
					b64 = strings.TrimRight(b64, "=")
				}
				assert.Check(t, is.Equal(b64, tc.inputBase64))
			}

			out, err := DecodeAuthConfig(tc.inputBase64)
			if tc.expectedErr != "" {
				assert.Check(t, cerrdefs.IsInvalidArgument(err))
				assert.Check(t, is.Error(err, tc.expectedErr))
			} else {
				assert.NilError(t, err)
				assert.Equal(t, *out, tc.expected)
			}
		})
	}
}

func BenchmarkDecodeAuthConfig(b *testing.B) {
	cases := []struct {
		doc         string
		inputBase64 string
		invalid     bool
	}{
		{
			doc:         "empty",
			inputBase64: ``,
		},
		{
			doc:         "empty JSON",
			inputBase64: `e30=`,
		},
		{
			doc:         "valid",
			inputBase64: base64.URLEncoding.EncodeToString([]byte(`{"username":"testuser","password":"testpassword","serveraddress":"example.com"}`)),
		},
		{
			doc:         "invalid base64",
			inputBase64: "not-base64",
			invalid:     true,
		},
		{
			doc:         "malformed JSON",
			inputBase64: `ew==`,
			invalid:     true,
		},
	}

	for _, tc := range cases {
		b.Run(tc.doc, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := DecodeAuthConfig(tc.inputBase64)
				if !tc.invalid && err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
