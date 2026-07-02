//go:build !windows
// +build !windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package wintls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
)

type CertResource = io.Closer

// NoopCertResource implements CertResource for non-Windows platforms
type NoopCertResource struct{}

func (n *NoopCertResource) Close() error {
	return nil
}

// Stub for non-Windows platforms
func SetupTLSFromWindowsCertStore(ctx context.Context, commonName string) (*tls.Config, *x509.CertPool, io.Closer, error) {
	return nil, nil, &NoopCertResource{}, nil
}
