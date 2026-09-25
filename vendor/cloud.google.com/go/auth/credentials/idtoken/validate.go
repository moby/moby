// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package idtoken

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/auth/internal"
	"cloud.google.com/go/auth/internal/jwt"
	"github.com/googleapis/gax-go/v2/internallog"
)

const (
	es256KeySize int = 32
	// googleIAPCertsURL is used for ES256 Certs.
	googleIAPCertsURL string = "https://www.gstatic.com/iap/verify/public_key-jwk"
	// googleSACertsURL is used for RS256 Certs.
	googleSACertsURL string = "https://www.googleapis.com/oauth2/v3/certs"
)

var (
	defaultValidator = &Validator{client: newCachingClient(internal.DefaultClient(), internallog.New(nil))}
	// now aliases time.Now for testing.
	now = time.Now
)

// certResponse represents a list jwks. It is the format returned from known
// Google cert endpoints.
type certResponse struct {
	Keys []jwk `json:"keys"`
}

// jwk is a simplified representation of a standard jwk. It only includes the
// fields used by Google's cert endpoints.
type jwk struct {
	Alg string `json:"alg"`
	Crv string `json:"crv"`
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	E   string `json:"e"`
	N   string `json:"n"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// Validator provides a way to validate Google ID Tokens
type Validator struct {
	client   *cachingClient
	rs256URL string
	es256URL string
}

// Payload represents a decoded payload of an ID token.
type Payload struct {
	Issuer   string                 `json:"iss"`
	Audience string                 `json:"aud"`
	Expires  int64                  `json:"exp"`
	IssuedAt int64                  `json:"iat"`
	Subject  string                 `json:"sub,omitempty"`
	Claims   map[string]interface{} `json:"-"`
}

// ValidatorOptions provides a way to configure a [Validator].
type ValidatorOptions struct {
	// Client used to make requests to the certs URL. Optional.
	Client *http.Client
	// Custom certs URL for RS256 JWK to be used. If not provided, the default
	// Google oauth2 endpoint will be used. Optional.
	RS256CertsURL string
	// Custom certs URL for ES256 JWK to be used. If not provided, the default
	// Google IAP endpoint will be used. Optional.
	ES256CertsURL string
	// Logger is used for debug logging. If provided, logging will be enabled
	// at the loggers configured level. By default logging is disabled unless
	// enabled by setting GOOGLE_SDK_GO_LOGGING_LEVEL in which case a default
	// Logger will be used. Optional.
	Logger *slog.Logger
}

// NewValidator creates a Validator that uses the options provided to configure
// a the internal http.Client that will be used to make requests to fetch JWKs.
func NewValidator(opts *ValidatorOptions) (*Validator, error) {
	if opts == nil {
		opts = &ValidatorOptions{}
	}
	client := opts.Client
	if client == nil {
		client = internal.DefaultClient()
	}
	rs256URL := opts.RS256CertsURL
	es256URL := opts.ES256CertsURL
	logger := internallog.New(opts.Logger)
	return &Validator{client: newCachingClient(client, logger), rs256URL: rs256URL, es256URL: es256URL}, nil
}

// Validate is used to validate the provided idToken with a known Google cert
// URL. If audience is not empty the audience claim of the Token is validated.
// Upon successful validation a parsed token Payload is returned allowing the
// caller to validate any additional claims.
func (v *Validator) Validate(ctx context.Context, idToken string, audience string) (*Payload, error) {
	return v.validate(ctx, idToken, audience)
}

// Validate is used to validate the provided idToken with a known Google cert
// URL. If audience is not empty the audience claim of the Token is validated.
// Upon successful validation a parsed token Payload is returned allowing the
// caller to validate any additional claims.
func Validate(ctx context.Context, idToken string, audience string) (*Payload, error) {
	return defaultValidator.validate(ctx, idToken, audience)
}

// ParsePayload parses the given token and returns its payload.
//
// Warning: This function does not validate the token prior to parsing it.
//
// ParsePayload is primarily meant to be used to inspect a token's payload. This is
// useful when validation fails and the payload needs to be inspected.
//
// Note: A successful Validate() invocation with the same token will return an
// identical payload.
func ParsePayload(idToken string) (*Payload, error) {
	_, payload, _, err := parseToken(idToken)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (v *Validator) validate(ctx context.Context, idToken string, audience string) (*Payload, error) {
	header, payload, sig, err := parseToken(idToken)
	if err != nil {
		return nil, err
	}

	if audience != "" && payload.Audience != audience {
		return nil, fmt.Errorf("idtoken: audience provided does not match aud claim in the JWT")
	}

	if now().Unix() > payload.Expires {
		return nil, fmt.Errorf("idtoken: token expired: now=%v, expires=%v", now().Unix(), payload.Expires)
	}
	hashedContent := hashHeaderPayload(idToken)
	switch header.Algorithm {
	case jwt.HeaderAlgRSA256:
		if err := v.validateRS256(ctx, header.KeyID, hashedContent, sig); err != nil {
			return nil, err
		}
	case jwt.HeaderAlgES256:
		if err := v.validateES256(ctx, header.KeyID, hashedContent, sig); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("idtoken: expected JWT signed with RS256 or ES256 but found %q", header.Algorithm)
	}

	return payload, nil
}

func (v *Validator) validateRS256(ctx context.Context, keyID string, hashedContent []byte, sig []byte) error {
	certResp, err := v.client.getCert(ctx, v.rs256CertsURL())
	if err != nil {
		return err
	}
	j, err := findMatchingKey(certResp, keyID)
	if err != nil {
		return err
	}
	dn, err := decode(j.N)
	if err != nil {
		return err
	}
	de, err := decode(j.E)
	if err != nil {
		return err
	}

	pk := &rsa.PublicKey{
		N: new(big.Int).SetBytes(dn),
		E: int(new(big.Int).SetBytes(de).Int64()),
	}
	return rsa.VerifyPKCS1v15(pk, crypto.SHA256, hashedContent, sig)
}

func (v *Validator) rs256CertsURL() string {
	if v.rs256URL == "" {
		return googleSACertsURL
	}
	return v.rs256URL
}

func (v *Validator) validateES256(ctx context.Context, keyID string, hashedContent []byte, sig []byte) error {
	certResp, err := v.client.getCert(ctx, v.es256CertsURL())
	if err != nil {
		return err
	}
	j, err := findMatchingKey(certResp, keyID)
	if err != nil {
		return err
	}
	dx, err := decode(j.X)
	if err != nil {
		return err
	}
	dy, err := decode(j.Y)
	if err != nil {
		return err
	}

	pk := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(dx),
		Y:     new(big.Int).SetBytes(dy),
	}
	r := big.NewInt(0).SetBytes(sig[:es256KeySize])
	s := big.NewInt(0).SetBytes(sig[es256KeySize:])
	if valid := ecdsa.Verify(pk, hashedContent, r, s); !valid {
		return fmt.Errorf("idtoken: ES256 signature not valid")
	}
	return nil
}

func (v *Validator) es256CertsURL() string {
	if v.es256URL == "" {
		return googleIAPCertsURL
	}
	return v.es256URL
}

func findMatchingKey(response *certResponse, keyID string) (*jwk, error) {
	if response == nil {
		return nil, fmt.Errorf("idtoken: cert response is nil")
	}
	for _, v := range response.Keys {
		if v.Kid == keyID {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("idtoken: could not find matching cert keyId for the token provided")
}

func parseToken(idToken string) (*jwt.Header, *Payload, []byte, error) {
	segments := strings.Split(idToken, ".")
	if len(segments) != 3 {
		return nil, nil, nil, fmt.Errorf("idtoken: invalid token, token must have three segments; found %d", len(segments))
	}
	// Header
	dh, err := decode(segments[0])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("idtoken: unable to decode JWT header: %v", err)
	}
	var header *jwt.Header
	err = json.Unmarshal(dh, &header)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("idtoken: unable to unmarshal JWT header: %v", err)
	}

	// Payload
	dp, err := decode(segments[1])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("idtoken: unable to decode JWT claims: %v", err)
	}
	var payload *Payload
	if err := json.Unmarshal(dp, &payload); err != nil {
		return nil, nil, nil, fmt.Errorf("idtoken: unable to unmarshal JWT payload: %v", err)
	}
	if err := json.Unmarshal(dp, &payload.Claims); err != nil {
		return nil, nil, nil, fmt.Errorf("idtoken: unable to unmarshal JWT payload claims: %v", err)
	}

	// Signature
	signature, err := decode(segments[2])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("idtoken: unable to decode JWT signature: %v", err)
	}
	return header, payload, signature, nil
}

// hashHeaderPayload gets the SHA256 checksum for verification of the JWT.
func hashHeaderPayload(idtoken string) []byte {
	// remove the sig from the token
	content := idtoken[:strings.LastIndex(idtoken, ".")]
	hashed := sha256.Sum256([]byte(content))
	return hashed[:]
}

func decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
