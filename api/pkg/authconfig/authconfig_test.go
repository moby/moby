package authconfig

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDecodeAuthConfig(t *testing.T) {
	tests := []struct {
		doc         string
		input       string
		inputBase64 string
		expected    registry.AuthConfig
		expectedErr string
	}{
		{
			doc:         "empty",
			input:       ``,
			inputBase64: ``,
			expected:    registry.AuthConfig{},
		},
		{
			doc:         "empty JSON",
			input:       `{}`,
			inputBase64: `e30=`,
			expected:    registry.AuthConfig{},
		},
		{
			doc:         "malformed JSON",
			input:       `{`,
			inputBase64: `ew==`,
			expected:    registry.AuthConfig{},
			expectedErr: `invalid X-Registry-Auth header: invalid JSON: unexpected EOF`,
		},
		{
			doc:         "test authConfig",
			input:       `{"username":"testuser","password":"testpassword","serveraddress":"example.com"}`,
			inputBase64: `eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzc3dvcmQiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5jb20ifQ==`,
			expected: registry.AuthConfig{
				Username:      "testuser",
				Password:      "testpassword",
				ServerAddress: "example.com",
			},
		},
		{
			doc:         "multiple authConfig (should be rejected)",
			input:       `{"username":"testuser","password":"testpassword","serveraddress":"example.com"}{"username":"testuser2","password":"testpassword2","serveraddress":"example.org"}`,
			inputBase64: `eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzc3dvcmQiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5jb20ifXsidXNlcm5hbWUiOiJ0ZXN0dXNlcjIiLCJwYXNzd29yZCI6InRlc3RwYXNzd29yZDIiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5vcmcifQ==`,
			expected:    registry.AuthConfig{},
			expectedErr: `invalid X-Registry-Auth header: multiple JSON documents not allowed`,
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
			expected:    registry.AuthConfig{},
			expectedErr: `invalid X-Registry-Auth header: must be a valid base64url-encoded string`,
		},
		{
			doc:         "test authConfig",
			input:       `{"username":"testuser","password":"testpassword","serveraddress":"example.com"}`,
			inputBase64: `eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzc3dvcmQiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5jb20ifQ`,
			expected:    registry.AuthConfig{},
			expectedErr: `invalid X-Registry-Auth header: must be a valid base64url-encoded string`,
		},
		{
			doc:         "JSON with trailing whitespace (should accept)",
			input:       `{"username":"testuser","password":"testpassword"}   `,
			inputBase64: `eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzc3dvcmQifSAgIA==`,
			expected: registry.AuthConfig{
				Username: "testuser",
				Password: "testpassword",
			},
		},
		{
			doc:         "JSON with trailing invalid data",
			input:       `{"username":"testuser"}invalid`,
			inputBase64: `eyJ1c2VybmFtZSI6InRlc3R1c2VyIn1pbnZhbGlk`,
			expected:    registry.AuthConfig{},
			expectedErr: `invalid X-Registry-Auth header: multiple JSON documents not allowed`,
		},
		{
			doc:         "multiple empty JSON documents",
			input:       `{}{}`,
			inputBase64: `e317fQ==`,
			expected:    registry.AuthConfig{},
			expectedErr: `invalid X-Registry-Auth header: multiple JSON documents not allowed`,
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

			out, err := Decode(tc.inputBase64)
			if tc.expectedErr != "" {
				assert.Check(t, is.ErrorType(err, errInvalidParameter{}))
				assert.Check(t, is.Error(err, tc.expectedErr))
			} else {
				assert.NilError(t, err)
				assert.Equal(t, *out, tc.expected)
			}
		})
	}
}

func TestEncodeAuthConfig(t *testing.T) {
	tests := []struct {
		doc       string
		input     registry.AuthConfig
		outBase64 string
		outPlain  string
	}{
		{
			// Older daemons (or registries) may not handle an empty string,
			// which resulted in an "io.EOF" when unmarshaling or decoding.
			//
			// FIXME(thaJeztah): find exactly what code-paths are impacted by this.
			doc:       "empty",
			input:     registry.AuthConfig{},
			outBase64: `e30=`,
			outPlain:  `{}`,
		},
		{
			doc: "test authConfig",
			input: registry.AuthConfig{
				Username:      "testuser",
				Password:      "testpassword",
				ServerAddress: "example.com",
			},
			outBase64: `eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzc3dvcmQiLCJzZXJ2ZXJhZGRyZXNzIjoiZXhhbXBsZS5jb20ifQ==`,
			outPlain:  `{"username":"testuser","password":"testpassword","serveraddress":"example.com"}`,
		},
	}
	for _, tc := range tests {
		// Sanity check to make sure our fixtures are correct.
		b64 := base64.URLEncoding.EncodeToString([]byte(tc.outPlain))
		assert.Check(t, is.Equal(b64, tc.outBase64))

		t.Run(tc.doc, func(t *testing.T) {
			out, err := Encode(tc.input)
			assert.NilError(t, err)
			assert.Equal(t, out, tc.outBase64)

			authJSON, err := base64.URLEncoding.DecodeString(out)
			assert.NilError(t, err)
			assert.Equal(t, string(authJSON), tc.outPlain)
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
				_, err := Decode(tc.inputBase64)
				if !tc.invalid && err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
