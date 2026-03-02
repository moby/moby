package authconfig

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/moby/moby/api/types/registry"
)

// Encode serializes the auth configuration as a base64url encoded
// ([RFC4648, section 5]) JSON string for sending through the X-Registry-Auth header.
//
// [RFC4648, section 5]: https://tools.ietf.org/html/rfc4648#section-5
func Encode(authConfig registry.AuthConfig) (string, error) {
	// Older daemons (or registries) may not handle an empty string,
	// which resulted in an "io.EOF" when unmarshaling or decoding.
	//
	// FIXME(thaJeztah): find exactly what code-paths are impacted by this.
	// if authConfig == (AuthConfig{}) { return "", nil }
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", errInvalidParameter{err}
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// Decode decodes base64url encoded ([RFC4648, section 5]) JSON
// authentication information as sent through the X-Registry-Auth header.
//
// This function always returns an [AuthConfig], even if an error occurs. It is up
// to the caller to decide if authentication is required, and if the error can
// be ignored.
//
// [RFC4648, section 5]: https://tools.ietf.org/html/rfc4648#section-5
func Decode(authEncoded string) (*registry.AuthConfig, error) {
	if authEncoded == "" {
		return &registry.AuthConfig{}, nil
	}

	decoded, err := base64.URLEncoding.DecodeString(authEncoded)
	if err != nil {
		var e base64.CorruptInputError
		if errors.As(err, &e) {
			return &registry.AuthConfig{}, invalid(errors.New("must be a valid base64url-encoded string"))
		}
		return &registry.AuthConfig{}, invalid(err)
	}

	if bytes.Equal(decoded, []byte("{}")) {
		return &registry.AuthConfig{}, nil
	}

	return decode(bytes.NewReader(decoded))
}

// DecodeRequestBody decodes authentication information as sent as JSON in the
// body of a request. This function is to provide backward compatibility with old
// clients and API versions. Current clients and API versions expect authentication
// to be provided through the X-Registry-Auth header.
//
// Like [Decode], this function always returns an [AuthConfig], even if an
// error occurs. It is up to the caller to decide if authentication is required,
// and if the error can be ignored.
func DecodeRequestBody(r io.ReadCloser) (*registry.AuthConfig, error) {
	return decode(r)
}

func decode(r io.Reader) (*registry.AuthConfig, error) {
	authConfig := &registry.AuthConfig{}
	dec := json.NewDecoder(r)
	if err := dec.Decode(authConfig); err != nil {
		// always return an (empty) AuthConfig to increase compatibility with
		// the existing API.
		return &registry.AuthConfig{}, invalid(fmt.Errorf("invalid JSON: %w", err))
	}
	if dec.More() {
		return &registry.AuthConfig{}, invalid(errors.New("multiple JSON documents not allowed"))
	}
	return authConfig, nil
}

func invalid(err error) error {
	return errInvalidParameter{fmt.Errorf("invalid X-Registry-Auth header: %w", err)}
}

type errInvalidParameter struct{ error }

func (errInvalidParameter) InvalidParameter() {}

func (e errInvalidParameter) Cause() error { return e.error }

func (e errInvalidParameter) Unwrap() error { return e.error }
