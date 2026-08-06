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

package impersonate

import (
	"errors"
	"log/slog"
	"net/http"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/credentials/internal/impersonate"
	"cloud.google.com/go/auth/httptransport"
	"cloud.google.com/go/auth/internal"
	"github.com/googleapis/gax-go/v2/internallog"
)

// IDTokenOptions for generating an impersonated ID token.
type IDTokenOptions struct {
	// Audience is the `aud` field for the token, such as an API endpoint the
	// token will grant access to. Required.
	Audience string
	// TargetPrincipal is the email address of the service account to
	// impersonate. Required.
	TargetPrincipal string
	// IncludeEmail includes the target service account's email in the token.
	// The resulting token will include both an `email` and `email_verified`
	// claim. Optional.
	IncludeEmail bool
	// Delegates are the ordered service account email addresses in a delegation
	// chain. Each service account must be granted
	// roles/iam.serviceAccountTokenCreator on the next service account in the
	// chain. Optional.
	Delegates []string

	// Credentials used in generating the impersonated ID token. If empty, an
	// attempt will be made to detect credentials from the environment (see
	// [cloud.google.com/go/auth/credentials.DetectDefault]). Optional.
	Credentials *auth.Credentials
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

func (o *IDTokenOptions) validate() error {
	if o == nil {
		return errors.New("impersonate: options must be provided")
	}
	if o.Audience == "" {
		return errors.New("impersonate: audience must be provided")
	}
	if o.TargetPrincipal == "" {
		return errors.New("impersonate: target service account must be provided")
	}
	return nil
}

var (
	defaultScope = "https://www.googleapis.com/auth/cloud-platform"
)

// NewIDTokenCredentials creates an impersonated
// [cloud.google.com/go/auth/Credentials] that returns ID tokens configured
// with the provided config and using credentials loaded from Application
// Default Credentials as the base credentials if not provided with the opts.
// The tokens produced are valid for one hour and are automatically refreshed.
func NewIDTokenCredentials(opts *IDTokenOptions) (*auth.Credentials, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	client := opts.Client
	creds := opts.Credentials
	logger := internallog.New(opts.Logger)
	if client == nil {
		var err error
		if creds == nil {
			creds, err = credentials.DetectDefault(&credentials.DetectOptions{
				Scopes:           []string{defaultScope},
				UseSelfSignedJWT: true,
				Logger:           logger,
			})
			if err != nil {
				return nil, err
			}
		}
		client, err = httptransport.NewClient(&httptransport.Options{
			Credentials:    creds,
			UniverseDomain: opts.UniverseDomain,
			Logger:         logger,
		})
		if err != nil {
			return nil, err
		}
	}

	universeDomainProvider := resolveUniverseDomainProvider(creds)
	var delegates []string
	for _, v := range opts.Delegates {
		delegates = append(delegates, internal.FormatIAMServiceAccountResource(v))
	}

	iamOpts := impersonate.IDTokenIAMOptions{
		Client: client,
		Logger: logger,
		// Pass the credentials universe domain provider to configure the endpoint.
		UniverseDomain:      universeDomainProvider,
		ServiceAccountEmail: opts.TargetPrincipal,
		GenerateIDTokenRequest: impersonate.GenerateIDTokenRequest{
			Audience:     opts.Audience,
			IncludeEmail: opts.IncludeEmail,
			Delegates:    delegates,
		},
	}
	return auth.NewCredentials(&auth.CredentialsOptions{
		TokenProvider:          auth.NewCachedTokenProvider(iamOpts, nil),
		UniverseDomainProvider: universeDomainProvider,
	}), nil
}
