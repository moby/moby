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

// Package idtoken provides functionality for generating and validating ID
// tokens, with configurable options for audience, custom claims, and token
// formats.
//
// For more information on ID tokens, see
// https://cloud.google.com/docs/authentication/token-types#id.
package idtoken

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/internal"
	"cloud.google.com/go/auth/internal/credsfile"
	"cloud.google.com/go/compute/metadata"
)

// ComputeTokenFormat dictates the the token format when requesting an ID token
// from the compute metadata service.
type ComputeTokenFormat int

const (
	// ComputeTokenFormatDefault means the same as [ComputeTokenFormatFull].
	ComputeTokenFormatDefault ComputeTokenFormat = iota
	// ComputeTokenFormatStandard mean only standard JWT fields will be included
	// in the token.
	ComputeTokenFormatStandard
	// ComputeTokenFormatFull means the token will include claims about the
	// virtual machine instance and its project.
	ComputeTokenFormatFull
	// ComputeTokenFormatFullWithLicense means the same as
	// [ComputeTokenFormatFull] with the addition of claims about licenses
	// associated with the instance.
	ComputeTokenFormatFullWithLicense
)

var (
	defaultScopes = []string{
		"https://iamcredentials.googleapis.com/",
		"https://www.googleapis.com/auth/cloud-platform",
	}

	errMissingOpts     = errors.New("idtoken: opts must be provided")
	errMissingAudience = errors.New("idtoken: Audience must be provided")
	errBothFileAndJSON = errors.New("idtoken: CredentialsFile and CredentialsJSON must not both be provided")
)

// Options for the configuration of creation of an ID token with
// [NewCredentials].
type Options struct {
	// Audience is the `aud` field for the token, such as an API endpoint the
	// token will grant access to. Required.
	Audience string
	// ComputeTokenFormat dictates the the token format when requesting an ID
	// token from the compute metadata service. Optional.
	ComputeTokenFormat ComputeTokenFormat
	// CustomClaims specifies private non-standard claims for an ID token.
	// Optional.
	CustomClaims map[string]interface{}

	// CredentialsFile sources a JSON credential file from the provided
	// filepath. If provided, do not provide CredentialsJSON. Optional.
	//
	// Deprecated: This field is deprecated because of a potential security risk.
	// It does not validate the credential configuration. The security risk occurs
	// when a credential configuration is accepted from a source that is not
	// under your control and used without validation on your side.
	//
	// If you know that you will be loading credential configurations of a
	// specific type, it is recommended to use a credential-type-specific
	// NewCredentialsFromFile method. This will ensure that an unexpected
	// credential type with potential for malicious intent is not loaded
	// unintentionally. You might still have to do validation for certain
	// credential types. Please follow the recommendation for that method. For
	// example, if you want to load only service accounts, you can use
	//
	//	creds, err := idtoken.NewCredentialsFromFile(ctx, credentials.ServiceAccount, filename, opts)
	//
	// If you are loading your credential configuration from an untrusted source
	// and have not mitigated the risks (e.g. by validating the configuration
	// yourself), make these changes as soon as possible to prevent security
	// risks to your environment.
	//
	// Regardless of the method used, it is always your responsibility to
	// validate configurations received from external sources.
	//
	// For more details see:
	// https://cloud.google.com/docs/authentication/external/externally-sourced-credentials
	CredentialsFile string
	// CredentialsJSON sources a JSON credential file from the provided bytes.
	// If provided, do not provide CredentialsJSON. Optional.
	//
	// Deprecated: This field is deprecated because of a potential security risk.
	// It does not validate the credential configuration. The security risk occurs
	// when a credential configuration is accepted from a source that is not
	// under your control and used without validation on your side.
	//
	// If you know that you will be loading credential configurations of a
	// specific type, it is recommended to use a credential-type-specific
	// NewCredentialsFromJSON method. This will ensure that an unexpected
	// credential type with potential for malicious intent is not loaded
	// unintentionally. You might still have to do validation for certain
	// credential types. Please follow the recommendation for that method. For
	// example, if you want to load only service accounts, you can use
	//
	//	creds, err := idtoken.NewCredentialsFromJSON(ctx, credentials.ServiceAccount, json, opts)
	//
	// If you are loading your credential configuration from an untrusted source
	// and have not mitigated the risks (e.g. by validating the configuration
	// yourself), make these changes as soon as possible to prevent security
	// risks to your environment.
	//
	// Regardless of the method used, it is always your responsibility to
	// validate configurations received from external sources.
	//
	// For more details see:
	// https://cloud.google.com/docs/authentication/external/externally-sourced-credentials
	CredentialsJSON []byte
	// Client configures the underlying client used to make network requests
	// when fetching tokens. If provided this should be a fully-authenticated
	// client. Optional.
	Client *http.Client
	// UniverseDomain is the default service domain for a given Cloud universe.
	// The default value is "googleapis.com". This is the universe domain
	// configured for the client, which will be compared to the universe domain
	// that is separately configured for the credentials. Optional.
	UniverseDomain string
	// Logger is used for debug logging. If provided, logging will be enabled
	// at the loggers configured level. By default logging is disabled unless
	// enabled by setting GOOGLE_SDK_GO_LOGGING_LEVEL in which case a default
	// logger will be used. Optional.
	Logger *slog.Logger
}

func (o *Options) client() *http.Client {
	if o == nil || o.Client == nil {
		return internal.DefaultClient()
	}
	return o.Client
}

func (o *Options) validate() error {
	if o == nil {
		return errMissingOpts
	}
	if o.Audience == "" {
		return errMissingAudience
	}
	if o.CredentialsFile != "" && len(o.CredentialsJSON) > 0 {
		return errBothFileAndJSON
	}
	return nil
}

// NewCredentials creates a [cloud.google.com/go/auth.Credentials] that returns
// ID tokens configured by the opts provided. The parameter opts.Audience must
// not be empty. If both opts.CredentialsFile and opts.CredentialsJSON are
// empty, an attempt will be made to detect credentials from the environment
// (see [cloud.google.com/go/auth/credentials.DetectDefault]). Only service
// account, impersonated service account, external account and Compute
// credentials are supported.
func NewCredentials(opts *Options) (*auth.Credentials, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	b := opts.jsonBytes()
	if b == nil && metadata.OnGCE() {
		return computeCredentials(opts)
	}
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes:           defaultScopes,
		CredentialsJSON:  b,
		Client:           opts.client(),
		UseSelfSignedJWT: true,
	})
	if err != nil {
		return nil, err
	}
	return credsFromDefault(creds, opts)
}

func (o *Options) jsonBytes() []byte {
	if len(o.CredentialsJSON) > 0 {
		return o.CredentialsJSON
	}
	var fnOverride string
	if o != nil {
		fnOverride = o.CredentialsFile
	}
	filename := credsfile.GetFileNameFromEnv(fnOverride)
	if filename != "" {
		b, _ := os.ReadFile(filename)
		return b
	}
	return nil
}

// NewCredentialsFromJSON creates a [cloud.google.com/go/auth.Credentials] that
// returns ID tokens from the provided JSON bytes. The credType argument
// specifies the expected credential type. If the JSON does not match the
// expected type, an error is returned.
//
// This method is safe to use with untrusted credential configurations if the
// expected credType is NOT [credentials.ExternalAccount] or
// [credentials.ImpersonatedServiceAccount].
//
// **IMPORTANT**: If you use [credentials.ExternalAccount] or
// [credentials.ImpersonatedServiceAccount], you must validate the credential
// configuration before providing it to this method. Providing an unvalidated
// credential configuration to Google APIs can compromise the security of your
// systems and data. For more information, refer to [Validate credential
// configurations from external sources](https://cloud.google.com/docs/authentication/external/externally-sourced-credentials).
func NewCredentialsFromJSON(credType credentials.CredType, b []byte, opts *Options) (*auth.Credentials, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	// Use credentials.NewCredentialsFromJSON to validate type and create base credentials.
	creds, err := credentials.NewCredentialsFromJSON(credType, b, &credentials.DetectOptions{
		Client:         opts.Client,
		Logger:         opts.Logger,
		UniverseDomain: opts.UniverseDomain,
	})
	if err != nil {
		return nil, err
	}
	return credsFromDefault(creds, opts)
}

// NewCredentialsFromFile creates a [cloud.google.com/go/auth.Credentials] that
// returns ID tokens from the provided file. The credType argument specifies the
// expected credential type. If the file content does not match the expected
// type, an error is returned.
//
// This method is safe to use with untrusted credential configurations if the
// expected credType is NOT [credentials.ExternalAccount] or
// [credentials.ImpersonatedServiceAccount].
//
// **IMPORTANT**: If you use [credentials.ExternalAccount] or
// [credentials.ImpersonatedServiceAccount], you must validate the credential
// configuration before providing it to this method. Providing an unvalidated
// credential configuration to Google APIs can compromise the security of your
// systems and data. For more information, refer to [Validate credential
// configurations from external sources](https://cloud.google.com/docs/authentication/external/externally-sourced-credentials).
func NewCredentialsFromFile(credType credentials.CredType, filename string, opts *Options) (*auth.Credentials, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return NewCredentialsFromJSON(credType, b, opts)
}
