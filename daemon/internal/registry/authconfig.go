package registry

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	registrytypes "github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/v2/errdefs"
)

// DecodeAuthConfig decodes base64url encoded ([RFC4648, section 5]) JSON
// authentication information as sent through the X-Registry-Auth header.
//
// This function always returns an [AuthConfig], even if an error occurs. It is up
// to the caller to decide if authentication is required, and if the error can
// be ignored.
//
// [RFC4648, section 5]: https://tools.ietf.org/html/rfc4648#section-5
func DecodeAuthConfig(authEncoded string) (*registrytypes.AuthConfig, error) {
	if authEncoded == "" {
		return &registrytypes.AuthConfig{}, nil
	}

	decoded, err := base64.URLEncoding.DecodeString(authEncoded)
	if err != nil {
		var e base64.CorruptInputError
		if errors.As(err, &e) {
			return &registrytypes.AuthConfig{}, invalid(errors.New("must be a valid base64url-encoded string"))
		}
		return &registrytypes.AuthConfig{}, invalid(err)
	}

	if bytes.Equal(decoded, []byte("{}")) {
		return &registrytypes.AuthConfig{}, nil
	}

	return decodeAuthConfigFromReader(bytes.NewReader(decoded))
}

func decodeAuthConfigFromReader(rdr io.Reader) (*registrytypes.AuthConfig, error) {
	authConfig := &registrytypes.AuthConfig{}
	if err := json.NewDecoder(rdr).Decode(authConfig); err != nil {
		// always return an (empty) AuthConfig to increase compatibility with
		// the existing API.
		return &registrytypes.AuthConfig{}, invalid(fmt.Errorf("invalid JSON: %w", err))
	}
	return authConfig, nil
}

func invalid(err error) error {
	return errdefs.InvalidParameter(fmt.Errorf("invalid X-Registry-Auth header: %w", err))
}
