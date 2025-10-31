// Copyright 2024 Google LLC
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

package externalaccount

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"cloud.google.com/go/auth/internal/transport/cert"
)

// x509Provider implements the subjectTokenProvider type for
// x509 workload identity credentials. Because x509 credentials
// rely on an mTLS connection to represent the 3rd party identity
// rather than a subject token, this provider will always return
// an empty string when a subject token is requested by the external account
// token provider.
type x509Provider struct {
}

func (xp *x509Provider) providerType() string {
	return x509ProviderType
}

func (xp *x509Provider) subjectToken(ctx context.Context) (string, error) {
	return "", nil
}

// createX509Client creates a new client that is configured with mTLS, using the
// certificate configuration specified in the credential source.
func createX509Client(certificateConfigLocation string) (*http.Client, error) {
	certProvider, err := cert.NewWorkloadX509CertProvider(certificateConfigLocation)
	if err != nil {
		return nil, err
	}
	trans := http.DefaultTransport.(*http.Transport).Clone()

	trans.TLSClientConfig = &tls.Config{
		GetClientCertificate: certProvider,
	}

	// Create a client with default settings plus the X509 workload cert and key.
	client := &http.Client{
		Transport: trans,
		Timeout:   30 * time.Second,
	}

	return client, nil
}
