package registry

import (
	"encoding/base64"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestEncodeAuthConfig(t *testing.T) {
	tests := []struct {
		doc       string
		input     AuthConfig
		outBase64 string
		outPlain  string
	}{
		{
			// Older daemons (or registries) may not handle an empty string,
			// which resulted in an "io.EOF" when unmarshaling or decoding.
			//
			// FIXME(thaJeztah): find exactly what code-paths are impacted by this.
			doc:       "empty",
			input:     AuthConfig{},
			outBase64: `e30=`,
			outPlain:  `{}`,
		},
		{
			doc: "test authConfig",
			input: AuthConfig{
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
			out, err := EncodeAuthConfig(tc.input)
			assert.NilError(t, err)
			assert.Equal(t, out, tc.outBase64)

			authJSON, err := base64.URLEncoding.DecodeString(out)
			assert.NilError(t, err)
			assert.Equal(t, string(authJSON), tc.outPlain)
		})
	}
}
